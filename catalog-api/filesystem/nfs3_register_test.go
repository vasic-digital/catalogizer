package filesystem

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

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

// LIVE: factory -> Connect -> list -> read -> Disconnect against a REAL NFS server (the unfs3 emulator container of
// scripts/test-infra/nfs_emulator_live.sh). Without NFS_EMU_ADDR it SKIPs; no stand-in server is used.
func TestNFS3FactoryLifecycleLive(t *testing.T) {
	addr := os.Getenv("NFS_EMU_ADDR")
	if addr == "" {
		t.Skip("NFS_EMU_ADDR not set: no real NFS server (unfs3 emulator) available; live leg SKIPPED, not stubbed")
	}
	withNFS3(t)
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
