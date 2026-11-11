package services

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"strings"
	"testing"

	"catalogizer/internal/config"

	"github.com/stretchr/testify/require"
)

// SMB-FU-12: a configured non-default port is allowed ONLY for its storage-root host. 445 and CATALOG_SMB_ALLOWED_PORTS stay global.
func cfgHosts(hs ...config.SMBHost) *config.Config {
	return &config.Config{SMB: config.SMBConfig{Hosts: hs}}
}

func TestSMBPortForHost_ConfiguredPortAllowedOnlyForItsHost(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, nil)
	require.Error(t, ValidateSMBPortForHost("192.168.1.10", 1445), "negative control: refused before configuration")
	ConfigureSMBPortsFromConfig(cfgHosts(config.SMBHost{Name: "nas", Host: "192.168.1.10", Port: 1445}))
	require.NoError(t, ValidateSMBPortForHost("192.168.1.10", 1445))
	require.Error(t, ValidateSMBPortForHost("192.168.1.11", 1445), "another RFC1918 host must not get the configured port")
	require.Error(t, ValidateSMBPortForHost("10.0.0.5", 1445))
	require.Error(t, ValidateSMBPortForHost("192.168.1.10", 1446), "other port on the same host stays refused")
	for _, h := range []string{"192.168.1.10", "192.168.1.11"} {
		require.NoError(t, ValidateSMBPortForHost(h, 445))
		require.NoError(t, ValidateSMBPortForHost(h, 0))
	}
	require.Equal(t, []int{445}, SMBAllowedPorts(), "the global list is not widened by config")
}

func TestSMBPortForHost_DangerousConfiguredPortsStillOnlyForThatHost(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, nil)
	ConfigureSMBPortsFromConfig(cfgHosts(config.SMBHost{Host: "192.168.1.10", Port: 22}, config.SMBHost{Host: "192.168.1.10", Port: 6379}))
	for _, p := range []int{22, 6379} {
		require.NoError(t, ValidateSMBPortForHost("192.168.1.10", p))
		require.Error(t, ValidateSMBPortForHost("192.168.1.20", p), "port %d must not open for another host", p)
	}
}

func TestSMBPortForHost_NameAndResolvedAddressMatch(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, nil)
	withSMBHostResolver(t, func(h string) ([]net.IP, error) {
		switch strings.ToLower(h) {
		case "nas.lan", "alias.lan":
			return []net.IP{net.ParseIP("192.168.1.10")}, nil
		case "other.lan":
			return []net.IP{net.ParseIP("192.168.1.99")}, nil
		}
		return nil, errors.New("nxdomain")
	})
	ConfigureSMBPortsFromConfig(cfgHosts(config.SMBHost{Host: "NAS.lan", Port: 1445}))
	require.NoError(t, ValidateSMBPortForHost("nas.lan", 1445), "case-insensitive name")
	require.NoError(t, ValidateSMBPortForHost("alias.lan", 1445), "same resolved address")
	require.NoError(t, ValidateSMBPortForHost("192.168.1.10", 1445), "IP literal of the root")
	require.Error(t, ValidateSMBPortForHost("other.lan", 1445))
	require.Error(t, ValidateSMBPortForHost("unknown.lan", 1445), "unresolvable host never matches")
}

func TestSMBPortForHost_MalformedConfigEntriesIgnored(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, nil)
	ConfigureSMBPortsFromConfig(cfgHosts(
		config.SMBHost{Host: "192.168.1.10", Port: 0}, config.SMBHost{Host: "192.168.1.10", Port: -1},
		config.SMBHost{Host: "192.168.1.10", Port: 65536}, config.SMBHost{Host: "", Port: 2445},
		config.SMBHost{Host: "bad host!", Port: 2445}, config.SMBHost{Host: "192.168.1.10", Port: 2445},
		config.SMBHost{Host: "192.168.1.10", Port: 2445}, config.SMBHost{Host: "192.168.1.10", Port: 445},
	))
	require.Equal(t, []SMBRootPort{{Host: "192.168.1.10", Port: 2445}}, SMBConfiguredRoots(), "only the valid, de-duplicated pair; 0 and 445 add nothing")
	require.NoError(t, ValidateSMBPortForHost("192.168.1.10", 2445))
	require.Error(t, ValidateSMBPortForHost("192.168.1.10", 65536))
	require.Error(t, ValidateSMBPortForHost("192.168.1.10", -1))
	ConfigureSMBPortsFromConfig(nil)
	require.Empty(t, SMBConfiguredRoots())
}

func TestSMBPortForHost_EnvPortsStayGlobal(t *testing.T) {
	withSMBConfiguredRoots(t, nil)
	ConfigureSMBPortsFromConfig(cfgHosts(config.SMBHost{Host: "192.168.1.10", Port: 1445}))
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "2445")
	require.NoError(t, ValidateSMBPortForHost("192.168.1.77", 2445), "operator-set env port is global")
	require.Error(t, ValidateSMBPortForHost("192.168.1.77", 1445))
	require.NoError(t, ValidateSMBPortForHost("192.168.1.10", 1445))
}

// The 445 seed of the registry is never stored (it is global already).
func TestSetSMBConfiguredRoots_Drops445(t *testing.T) {
	withSMBConfiguredRoots(t, nil)
	SetSMBConfiguredRoots([]SMBRootPort{{Host: "192.168.1.10", Port: 445}})
	require.Empty(t, SMBConfiguredRoots())
}

// Wiring: the production constructor registers the ports before the service exists, and main.go uses it (SMB-FU-12 review item 3).
func TestNewSMBDiscoveryServiceFromConfig_RegistersPorts(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, nil)
	require.NotNil(t, NewSMBDiscoveryServiceFromConfig(cfgHosts(config.SMBHost{Host: "192.168.1.10", Port: 1445}), nil))
	require.NoError(t, ValidateSMBPortForHost("192.168.1.10", 1445))
	require.NotNil(t, NewSMBDiscoveryService(nil))
}

// serviceCall is a call services.<Name>(args...) found in Go source; comments never count.
type serviceCall struct {
	Name     string
	FirstNil bool
}

func servicesCalls(t *testing.T, src string) []serviceCall {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	require.NoError(t, err)
	var out []serviceCall
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if se, ok := c.Fun.(*ast.SelectorExpr); ok {
				if x, ok := se.X.(*ast.Ident); ok && x.Name == "services" {
					first := false
					if len(c.Args) > 0 {
						if id, ok := c.Args[0].(*ast.Ident); ok && id.Name == "nil" {
							first = true
						}
					}
					out = append(out, serviceCall{se.Sel.Name, first})
				}
			}
		}
		return true
	})
	return out
}

func mainWiresConfiguredPorts(t *testing.T, src string) bool {
	ok := false
	for _, c := range servicesCalls(t, src) {
		switch c.Name {
		case "NewSMBDiscoveryService":
			return false
		case "NewSMBDiscoveryServiceFromConfig":
			if !c.FirstNil {
				ok = true
			}
		}
	}
	return ok
}

func TestMainWiresSMBConfiguredPorts(t *testing.T) {
	b, err := os.ReadFile("../../main.go")
	require.NoError(t, err)
	require.True(t, mainWiresConfiguredPorts(t, string(b)), "main.go must build the discovery service with NewSMBDiscoveryServiceFromConfig")
	// self-check: golden-bad forms (old constructor, commented-out call) must be flagged, golden-good must pass
	wrap := func(body string) string { return "package main\nfunc f() {\n" + body + "\n}\n" }
	require.False(t, mainWiresConfiguredPorts(t, wrap("x := services.NewSMBDiscoveryService(logger)")))
	require.False(t, mainWiresConfiguredPorts(t, wrap("// x := services.NewSMBDiscoveryServiceFromConfig(cfg, logger)\n_ = services.NewSMBDiscoveryService(l)")))
	require.False(t, mainWiresConfiguredPorts(t, wrap("/* services.NewSMBDiscoveryServiceFromConfig(cfg, logger) */")))
	require.False(t, mainWiresConfiguredPorts(t, wrap("x := services.NewSMBDiscoveryServiceFromConfig(nil, logger)")), "nil config registers nothing")
	require.False(t, mainWiresConfiguredPorts(t, wrap("x := other.NewSMBDiscoveryServiceFromConfig(cfg, logger)")), "receiver must be services")
	require.True(t, mainWiresConfiguredPorts(t, wrap("x := services.NewSMBDiscoveryServiceFromConfig(cfg, logger)")))
}

// Service layer (defence in depth to the handler check): the configured port is accepted for its host and refused for another host
// before any mount.
func TestSMBDiscovery_BrowseConfiguredPortBoundToHost(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []SMBRootPort{{Host: "192.0.2.10", Port: 1445}})
	m := newMem()
	var opened []string
	cfg := discCfg("Public")
	cfg.Port = 1445
	_, err := discSvc(m, &opened).BrowseShare(context.Background(), cfg, ".")
	require.False(t, errors.Is(err, ErrInvalidSMBInput), "root host with its configured port: %v", err)
	require.Equal(t, []string{"Public"}, opened)

	opened = nil
	cfg.Host = "192.0.2.11"
	_, err = discSvc(m, &opened).BrowseShare(context.Background(), cfg, ".")
	require.True(t, errors.Is(err, ErrInvalidSMBInput), "another host must be refused: %v", err)
	require.Empty(t, opened)
}

func TestSetSMBConfiguredRoots_WarnsOnMalformedEntryWithoutSecrets(t *testing.T) {
	var msgs []string
	withSMBWarnf(t, func(f string, a ...any) { msgs = append(msgs, fmt.Sprintf(f, a...)) })
	withSMBConfiguredRoots(t, nil)
	SetSMBConfiguredRoots([]SMBRootPort{{Host: "bad host!", Port: 2445}, {Host: "192.168.1.10", Port: 70000}, {Host: "192.168.1.10", Port: 2445}, {Host: "192.168.1.10", Port: 2445}})
	require.Len(t, msgs, 2, "one warning per malformed entry; the valid entry and its duplicate stay silent")
	require.Contains(t, msgs[0], "bad host!")
	require.Contains(t, msgs[1], "70000")
}
