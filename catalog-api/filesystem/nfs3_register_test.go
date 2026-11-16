package filesystem

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/netguard"
	"catalogizer/models"
	"digital.vasic.filesystem/pkg/nfs3"
)

func withNFS3(t *testing.T) {
	t.Helper()
	was := IsRegisteredProtocol("nfs3")
	if err := RegisterNFS3Protocol(); err != nil {
		t.Fatal(err)
	}
	if err := RegisterNFS3Protocol(); err != nil { // idempotent
		t.Fatalf("second register: %v", err)
	}
	if !was {
		t.Cleanup(func() { UnregisterProtocol("nfs3") })
	}
}

func nfs3Cfg(s map[string]interface{}) *StorageConfig {
	return &StorageConfig{ID: "r", Name: "r", Protocol: "nfs3", Settings: s}
}

// UNIT: the settings -> nfs3.Config mapping (pure function, nothing dialled).
func TestNFS3ConfigMappingUnit(t *testing.T) {
	cfg, err := nfs3ConfigFromSettings(map[string]interface{}{"host": "nas.example", "path": "/volume1/video", "port": 2049, "uid": 1000, "gid": 1001})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "nas.example" || cfg.Export != "/volume1/video" || cfg.NFSPort != 2049 || cfg.MountPort != 2049 || cfg.UID != 1000 || cfg.GID != 1001 {
		t.Fatalf("mapping wrong: %+v", cfg)
	}
	if cfg.AsRoot {
		t.Fatal("must never default to root")
	}
	d, err := nfs3ConfigFromSettings(map[string]interface{}{"host": "h", "path": "/e"})
	if err != nil {
		t.Fatal(err)
	}
	if d.NFSPort != 0 || d.MountPort != 0 || d.UID != 0 || d.GID != 0 || d.AsRoot {
		t.Fatalf("defaults must be portmapper + client default identity: %+v", d)
	}
	for name, s := range map[string]map[string]interface{}{
		"no host": {"path": "/e"}, "no path": {"host": "h"}, "negative uid": {"host": "h", "path": "/e", "uid": -1}, "negative gid": {"host": "h", "path": "/e", "gid": -5},
	} {
		if _, err := nfs3ConfigFromSettings(s); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// Factory path with the real pkg/nfs3 constructor (no network): the client exists, speaks "nfs3", is not connected.
func TestNFS3FactoryCreatesClient(t *testing.T) {
	withNFS3(t)
	c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(map[string]interface{}{"host": "127.0.0.1", "path": "/export", "port": 12049}))
	if err != nil {
		t.Fatalf("CreateClient: %v", err)
	}
	if c.GetProtocol() != "nfs3" {
		t.Fatalf("protocol %q", c.GetProtocol())
	}
	if c.IsConnected() {
		t.Fatal("a new client must not be connected (Connect is the lifecycle step)")
	}
	found := false
	for _, p := range NewDefaultClientFactory().SupportedProtocols() {
		found = found || p == "nfs3"
	}
	if !found {
		t.Fatal("nfs3 missing from SupportedProtocols")
	}
}

func TestNFS3FactoryRejectsBadSettings(t *testing.T) {
	withNFS3(t)
	f := NewDefaultClientFactory()
	for name, s := range map[string]map[string]interface{}{
		"unknown key":  {"host": "h", "path": "/e", "mount_point": "/mnt"},
		"wrong type":   {"host": "h", "path": "/e", "uid": "root"},
		"bad port":     {"host": "h", "path": "/e", "port": 70000},
		"negative uid": {"host": "h", "path": "/e", "uid": -1},
		"no host":      {"path": "/e"},
	} {
		if c, err := f.CreateClient(nfs3Cfg(s)); err == nil {
			t.Errorf("%s: want error, got client %v", name, c)
		}
	}
}

// LIVE failure leg (real client, real TCP to a port nothing listens on): Connect fails honestly, state stays disconnected, Disconnect is safe.
func TestNFS3LifecycleConnectFailureReal(t *testing.T) {
	withNFS3(t)
	allowLoopbackDial(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close() // nothing listens now
	c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(map[string]interface{}{"host": "127.0.0.1", "path": "/export", "port": port}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err == nil {
		t.Fatal("Connect to a closed port must fail")
	}
	if c.IsConnected() {
		t.Fatal("failed Connect left the client connected")
	}
	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect after failed Connect: %v", err)
	}
}

// LIVE: factory -> Connect -> list -> Disconnect against a REAL NFS server (the unfs3 emulator container; run by
// scripts/test-infra/nfs_emulator_live.sh, which sets NFS_EMU_ADDR and runs this test). Without NFS_EMU_ADDR it SKIPs; no stand-in server is used.
func TestNFS3FactoryLifecycleLive(t *testing.T) {
	addr := os.Getenv("NFS_EMU_ADDR")
	if addr == "" {
		t.Skip("NFS_EMU_ADDR not set: no real NFS server (unfs3 emulator) available; live leg SKIPPED, not stubbed")
	}
	withNFS3(t)
	allowLoopbackDial(t) // the emulator listens on loopback; the operator-class policy would refuse it
	host, ps, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(ps)
	c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(map[string]interface{}{"host": host, "path": "/export", "port": port, "uid": 1000, "gid": 1000}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !c.IsConnected() {
		t.Fatal("not connected after Connect")
	}
	es, err := c.ListDirectory(ctx, "/")
	if err != nil || len(es) == 0 {
		t.Fatalf("ListDirectory: %v entries=%d", err, len(es))
	}
	var names []string
	for _, e := range es {
		names = append(names, e.Name)
	}
	if !strings.Contains(strings.Join(names, ","), "readme.txt") {
		t.Fatalf("seeded readme.txt not listed: %v", names)
	}
	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if c.IsConnected() {
		t.Fatal("still connected after Disconnect")
	}
}

// The settings reach the real client (not only the mapping function): read back through the client's own GetConfig.
func TestNFS3FactorySettingsReachClient(t *testing.T) {
	withNFS3(t)
	c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(map[string]interface{}{"host": "nas.example", "path": "/vol/v", "port": 2049, "uid": 1000, "gid": 1001}))
	if err != nil {
		t.Fatalf("CreateClient with uid+gid: %v", err)
	}
	got, ok := c.GetConfig().(nfs3.Config)
	if !ok {
		t.Fatalf("GetConfig type %T", c.GetConfig())
	}
	if got.Host != "nas.example" || got.Export != "/vol/v" || got.NFSPort != 2049 || got.MountPort != 2049 || got.UID != 1000 || got.GID != 1001 {
		t.Fatalf("settings did not reach the client: %+v", got)
	}
}

// allowLoopbackDial replaces the operator-class dialer by a plain one for tests whose real server is on loopback.
func allowLoopbackDial(t *testing.T) {
	t.Helper()
	old := storageDialContext
	storageDialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
	t.Cleanup(func() { storageDialContext = old })
}

// SEC-DIAL-1: the nfs3 client dials through the operator-class dialer (storageDialContext), never a plain net.Dialer.
func TestNFS3DialsThroughStorageDialer(t *testing.T) {
	withNFS3(t)
	var dialed []string
	old := storageDialContext
	storageDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dialed = append(dialed, addr)
		return nil, errors.New("dial observed")
	}
	t.Cleanup(func() { storageDialContext = old })
	c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(map[string]interface{}{"host": "nas.example", "path": "/export", "port": 2049}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := c.Connect(ctx); err == nil {
		t.Fatal("Connect must fail when the dialer refuses")
	}
	if len(dialed) == 0 {
		t.Fatal("nfs3 never dialled through storageDialContext (SEC-DIAL-1 bypass)")
	}
	for _, a := range dialed {
		if !strings.HasPrefix(a, "nas.example:") {
			t.Errorf("unexpected dial target %q", a)
		}
	}
}

// With the real operator-class dialer, a loopback server is never connected to (a listener on it sees zero connections), and a cloud-metadata
// address is refused too.
func TestNFS3RealPolicyRefusesLoopbackAndMetadata(t *testing.T) {
	withNFS3(t)
	t.Setenv(netguard.AllowCIDRsEnv, "")
	port, accepted := secDialListener(t)
	for _, host := range []string{"127.0.0.1", "169.254.169.254"} {
		p := port
		if host != "127.0.0.1" {
			p = 2049
		}
		c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(map[string]interface{}{"host": host, "path": "/export", "port": p}))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err = c.Connect(ctx)
		cancel()
		if err == nil || c.IsConnected() {
			t.Fatalf("%s: Connect succeeded under the operator-class policy", host)
		}
		if !errors.Is(err, netguard.ErrBlocked) {
			t.Errorf("%s: want a policy refusal, got %v", host, err)
		}
	}
	time.Sleep(100 * time.Millisecond)
	if n := atomic.LoadInt32(accepted); n != 0 {
		t.Fatalf("nfs3 connected to the loopback listener %d times", n)
	}
}

func TestNFS3SettingsSyntaxRefused(t *testing.T) {
	withNFS3(t)
	for name, s := range map[string]map[string]interface{}{
		"host with port":  {"host": "evil:1234", "path": "/e"},
		"host with slash": {"host": "a/b", "path": "/e"},
		"dash host":       {"host": "-oProxy", "path": "/e"},
		"dash export":     {"host": "h", "path": "-o x"},
		"control export":  {"host": "h", "path": "/a\nb"},
		"server list":     {"host": "h", "path": "/a,evil:/b"},
	} {
		if err := ValidateSettings("nfs3", s); err == nil {
			t.Errorf("%s: ValidateSettings accepted it", name)
		}
		if c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(s)); err == nil {
			t.Errorf("%s: factory created %v", name, c)
		}
	}
	if err := ValidateSettings("nfs3", map[string]interface{}{"host": "nas.example", "path": "/volume1/My Videos"}); err != nil {
		t.Errorf("legal value refused: %v", err)
	}
}

func TestNFS3VetStorageRoot(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "") // the package TestMain re-admits loopback for fixtures; the policy under test must be the real one
	str := func(s string) *string { return &s }
	root := func(h string) *models.StorageRoot {
		return &models.StorageRoot{Protocol: "nfs3", Host: str(h), Path: str("/export")}
	}
	for _, h := range []string{"169.254.169.254", "127.0.0.1", "localhost", "evil:1234", "-x"} {
		err := VetStorageRoot(context.Background(), root(h))
		if !errors.Is(err, ErrStorageHostRefused) {
			t.Errorf("host %q: want ErrStorageHostRefused, got %v", h, err)
		}
	}
	if err := VetStorageRoot(context.Background(), root("192.168.1.20")); err != nil {
		t.Errorf("private LAN host refused: %v", err)
	}
}

// F4: the Required key list is enforced at root creation (ValidateRoot).
func TestNFS3ValidateRootRequiredKeys(t *testing.T) {
	withNFS3(t)
	str := func(s string) *string { return &s }
	if err := ValidateRoot(&models.StorageRoot{Protocol: "nfs3", Host: str("h"), Path: str("/e")}); err != nil {
		t.Fatalf("complete root refused: %v", err)
	}
	if err := ValidateRoot(&models.StorageRoot{Protocol: "nfs3", Host: str("h")}); err == nil {
		t.Error("root without path accepted")
	}
	if err := ValidateRoot(&models.StorageRoot{Protocol: "nfs3", Path: str("/e")}); err == nil {
		t.Error("root without host accepted")
	}
}

// NFS3-WIRE-1-f5: mount_port / portmap_port are optional; mount_port defaults to port, portmap_port defaults to 0 (the client default, 111); one `port` still sets mount and NFS.
func TestNFS3ConfigSplitPorts(t *testing.T) {
	base := func(extra map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{"host": "nas.example", "path": "/e"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	for _, c := range []struct {
		name                string
		in                  map[string]interface{}
		mount, nfs, portmap int
	}{
		{"port only", base(map[string]interface{}{"port": 2049}), 2049, 2049, 0},
		{"mount_port overrides", base(map[string]interface{}{"port": 2049, "mount_port": 20048}), 20048, 2049, 0},
		{"portmap_port set", base(map[string]interface{}{"port": 2049, "portmap_port": 1111}), 2049, 2049, 1111},
		{"mount_port without port", base(map[string]interface{}{"mount_port": 20048}), 20048, 0, 0},
		{"none", base(nil), 0, 0, 0},
	} {
		cfg, err := nfs3ConfigFromSettings(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if cfg.MountPort != c.mount || cfg.NFSPort != c.nfs || cfg.PortmapPort != c.portmap {
			t.Errorf("%s: mount=%d nfs=%d portmap=%d, want %d/%d/%d", c.name, cfg.MountPort, cfg.NFSPort, cfg.PortmapPort, c.mount, c.nfs, c.portmap)
		}
	}
	for _, bad := range []map[string]interface{}{base(map[string]interface{}{"mount_port": 0 + 70000}), base(map[string]interface{}{"portmap_port": -1})} {
		if _, err := nfs3ConfigFromSettings(bad); err == nil {
			t.Errorf("out-of-range port %v must be refused", bad)
		}
	}
	// the keys are accepted by the registered spec (factory path)
	withNFS3(t)
	if err := ValidateSettings("nfs3", base(map[string]interface{}{"mount_port": 20048, "portmap_port": 111})); err != nil {
		t.Fatalf("spec must accept mount_port/portmap_port: %v", err)
	}
}

// NFS3-WIRE-1-f6: the identity the real client carries (GetConfig readback), uid 0 / gid 0 -> 65534 (never root), explicit ids kept.
func TestNFS3EffectiveIdentityReadback(t *testing.T) {
	withNFS3(t)
	read := func(extra map[string]interface{}) nfs3.Config {
		s := map[string]interface{}{"host": "127.0.0.1", "path": "/export", "port": 12049}
		for k, v := range extra {
			s[k] = v
		}
		c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(s))
		if err != nil {
			t.Fatal(err)
		}
		cfg, ok := c.GetConfig().(nfs3.Config)
		if !ok {
			t.Fatalf("GetConfig type %T", c.GetConfig())
		}
		return cfg
	}
	if cfg := read(map[string]interface{}{"uid": 0, "gid": 0}); cfg.UID != 65534 || cfg.GID != 65534 || cfg.AsRoot {
		t.Fatalf("uid/gid 0 must become 65534, never root: %+v", cfg)
	}
	if cfg := read(nil); cfg.UID != 65534 || cfg.GID != 65534 {
		t.Fatalf("absent uid/gid must be 65534: %+v", cfg)
	}
	if cfg := read(map[string]interface{}{"uid": 1000, "gid": 1001}); cfg.UID != 1000 || cfg.GID != 1001 {
		t.Fatalf("explicit ids kept: %+v", cfg)
	}
}

// NFS3W-FU-m1 (direct layer): nfs3ConfigFromSettings range-checks every port key as 0-65535 (0 = default; at THIS layer port 0 = ask the portmapper).
// This is defence in depth for direct callers; at the factory `port` is stricter (1-65535), see TestNFS3FactoryPortContract. The refusal must be named
// by its own key: the message must START with "nfs3: <key> must", so "mount_port" can never stand in for "port".
func TestNFS3ConfigPortRangeContract(t *testing.T) {
	mk := func(extra map[string]interface{}) map[string]interface{} {
		m := map[string]interface{}{"host": "nas.example", "path": "/e"}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	for _, k := range []string{KeyPort, KeyMountPort, KeyPortmapPort} {
		for _, v := range []int{-1, 65536, 70000} {
			// each bad value with every combination of the OTHER keys set to valid values or absent: the error must still name k
			for _, others := range []map[string]interface{}{
				{}, {"port": 2049}, {"mount_port": 2049}, {"portmap_port": 111}, {"port": 2049, "mount_port": 2049, "portmap_port": 111},
			} {
				in := map[string]interface{}{}
				for ok, ov := range others {
					in[ok] = ov
				}
				in[k] = v
				_, err := nfs3ConfigFromSettings(mk(in))
				want := "nfs3: " + k + " must be 0-65535 (0 = default)"
				if err == nil {
					t.Errorf("%s=%d with %v must be refused", k, v, others)
				} else if err.Error() != want {
					t.Errorf("%s=%d with %v: error %q, want exactly %q", k, v, others, err, want)
				}
			}
		}
		for _, v := range []int{0, 1, 65535} {
			if _, err := nfs3ConfigFromSettings(mk(map[string]interface{}{k: v})); err != nil {
				t.Errorf("%s=%d must be accepted at the direct layer: %v", k, v, err)
			}
		}
	}
}

// NFS3W-FU-m1 (factory layer, the only production entry): `port` is 1-65535 (ValidateSettings refuses 0, negatives and > 65535 with "port 1-65535");
// mount_port / portmap_port are 0-65535 with 0 = default and are accepted at 0.
func TestNFS3FactoryPortContract(t *testing.T) {
	withNFS3(t)
	f := NewDefaultClientFactory()
	for _, v := range []int{0, -1, 65536, 70000} {
		_, err := f.CreateClient(nfs3Cfg(map[string]interface{}{"host": "127.0.0.1", "path": "/export", "port": v}))
		if err == nil || !strings.Contains(err.Error(), "port (want a port 1-65535)") {
			t.Errorf("factory port=%d: %v, want the \"port 1-65535\" refusal", v, err)
		}
	}
	for _, k := range []string{KeyMountPort, KeyPortmapPort} {
		c, err := f.CreateClient(nfs3Cfg(map[string]interface{}{"host": "127.0.0.1", "path": "/export", "port": 12049, k: 0}))
		if err != nil {
			t.Errorf("factory %s=0 (default) must be accepted: %v", k, err)
		} else if c == nil {
			t.Errorf("factory %s=0: nil client", k)
		}
	}
}

// NFS3W-FU2-FU1: with `port` AND `portmap_port` both out of range, `port` is reported first (mutant: check portmap_port before port).
func TestNFS3ConfigPortOrderBothBad(t *testing.T) {
	_, err := nfs3ConfigFromSettings(map[string]interface{}{"host": "nas.example", "path": "/e", "port": 70000, "portmap_port": -1})
	want := "nfs3: port must be 0-65535 (0 = default)"
	if err == nil || err.Error() != want {
		t.Fatalf("both bad: %v, want exactly %q", err, want)
	}
	_, err = nfs3ConfigFromSettings(map[string]interface{}{"host": "nas.example", "path": "/e", "mount_port": 70000, "portmap_port": -1})
	if want := "nfs3: mount_port must be 0-65535 (0 = default)"; err == nil || err.Error() != want {
		t.Fatalf("mount_port+portmap_port bad: %v, want exactly %q", err, want)
	}
}

// NFS3W-FU2-FU2: JSON-decoded settings carry numbers as float64; they must map to the real ports (never silently 0), directly and through the factory.
func TestNFS3FloatPortsFromJSONSettings(t *testing.T) {
	in := map[string]interface{}{"host": "nas.example", "path": "/e", "port": float64(2049), "mount_port": float64(20048), "portmap_port": float64(1111)}
	cfg, err := nfs3ConfigFromSettings(in)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NFSPort != 2049 || cfg.MountPort != 20048 || cfg.PortmapPort != 1111 {
		t.Fatalf("direct float64 ports: nfs=%d mount=%d portmap=%d, want 2049/20048/1111", cfg.NFSPort, cfg.MountPort, cfg.PortmapPort)
	}
	cfg, err = nfs3ConfigFromSettings(map[string]interface{}{"host": "nas.example", "path": "/e", "port": float64(2049)})
	if err != nil || cfg.NFSPort != 2049 || cfg.MountPort != 2049 {
		t.Fatalf("direct float64 port only: %+v err=%v, want NFSPort=MountPort=2049", cfg, err)
	}
	withNFS3(t)
	c, err := NewDefaultClientFactory().CreateClient(nfs3Cfg(map[string]interface{}{"host": "nas.example", "path": "/e", "port": float64(2049)}))
	if err != nil {
		t.Fatalf("factory float64 port: %v", err)
	}
	got, ok := c.GetConfig().(nfs3.Config)
	if !ok {
		t.Fatalf("GetConfig type %T", c.GetConfig())
	}
	if got.NFSPort != 2049 || got.MountPort != 2049 {
		t.Fatalf("factory float64 port: NFSPort=%d MountPort=%d, want 2049/2049", got.NFSPort, got.MountPort)
	}
}
