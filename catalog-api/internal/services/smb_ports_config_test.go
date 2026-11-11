package services

import (
	"testing"

	"catalogizer/internal/config"

	"github.com/stretchr/testify/require"
)

// SMB-FU-12: ports of configured SMB storage roots are allowed in addition to 445 and CATALOG_SMB_ALLOWED_PORTS, and nothing else.
func TestSMBAllowedPorts_ConfiguredRootPortAllowed(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredPorts(t, nil)
	require.Error(t, ValidateSMBPort(1445), "negative control: refused before configuration")
	ConfigureSMBPortsFromConfig(&config.Config{SMB: config.SMBConfig{Hosts: []config.SMBHost{{Name: "nas", Host: "h", Port: 1445}}}})
	require.NoError(t, ValidateSMBPort(1445))
	require.NoError(t, ValidateSMBPort(445))
	require.NoError(t, ValidateSMBPort(0))
	require.Equal(t, []int{445, 1445}, SMBAllowedPorts())
}

func TestSMBAllowedPorts_OtherPortStillRefusedWithConfig(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredPorts(t, nil)
	ConfigureSMBPortsFromConfig(&config.Config{SMB: config.SMBConfig{Hosts: []config.SMBHost{{Port: 1445}}}})
	for _, p := range []int{22, 80, 139, 3389, 1444, 1446, 65535} {
		require.Error(t, ValidateSMBPort(p), "port %d", p)
	}
}

func TestSMBAllowedPorts_MalformedConfigEntriesIgnored(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredPorts(t, nil)
	ConfigureSMBPortsFromConfig(&config.Config{SMB: config.SMBConfig{Hosts: []config.SMBHost{
		{Port: 0}, {Port: -1}, {Port: 65536}, {Port: 99999}, {Port: 2445}, {Port: 2445},
	}}})
	require.Equal(t, []int{445, 2445}, SMBAllowedPorts(), "only the valid, de-duplicated port is added; 0 means default")
	require.Error(t, ValidateSMBPort(65536))
	require.Error(t, ValidateSMBPort(-1))
	ConfigureSMBPortsFromConfig(nil) // nil config clears and must not panic
	require.Equal(t, []int{445}, SMBAllowedPorts())
}

func TestSMBAllowedPorts_ConfigAndEnvCombine(t *testing.T) {
	withSMBConfiguredPorts(t, nil)
	ConfigureSMBPortsFromConfig(&config.Config{SMB: config.SMBConfig{Hosts: []config.SMBHost{{Port: 1445}}}})
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "2445")
	require.ElementsMatch(t, []int{445, 1445, 2445}, SMBAllowedPorts())
	// an invalid env value fails closed for the env part only; configured roots stay
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "abc")
	withSMBWarnf(t, func(string, ...any) {})
	require.Equal(t, []int{445, 1445}, SMBAllowedPorts())
}
