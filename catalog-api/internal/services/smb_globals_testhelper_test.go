package services

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// SMB-FU-14: tests that replace package-level SMB globals (SMBHostResolver, SMBDialer, smbWarnf/smbWarnedRaw, the configured-port
// registry) MUST do it through these helpers: they serialise on one mutex and restore the old value. A test that uses them must never
// call t.Parallel(); TestSMBGlobalsTestsNotParallel enforces that.
var (
	smbGlobalsMu     sync.Mutex
	smbGlobalsHolder *testing.T
)

func lockSMBGlobals(t *testing.T) {
	t.Helper()
	if smbGlobalsHolder == t { // already held by this test (several helpers in one test)
		return
	}
	smbGlobalsMu.Lock()
	smbGlobalsHolder = t
	t.Cleanup(func() {
		smbGlobalsHolder = nil
		smbGlobalsMu.Unlock()
	})
}

func withSMBHostResolver(t *testing.T, fn func(string) ([]net.IP, error)) {
	t.Helper()
	lockSMBGlobals(t)
	old := SMBHostResolver
	t.Cleanup(func() { SMBHostResolver = old })
	SMBHostResolver = fn
}

func withSMBDialer(t *testing.T, fn func(context.Context, string, time.Duration) (net.Conn, error)) {
	t.Helper()
	lockSMBGlobals(t)
	old := SMBDialer
	t.Cleanup(func() { SMBDialer = old })
	SMBDialer = fn
}

// withSMBWarnf replaces smbWarnf and resets the warn-once memory; both are restored afterwards.
func withSMBWarnf(t *testing.T, fn func(string, ...any)) {
	t.Helper()
	lockSMBGlobals(t)
	oldW, oldR := smbWarnf, smbWarnedRaw
	t.Cleanup(func() { smbWarnf, smbWarnedRaw = oldW, oldR })
	smbWarnf = fn
	smbWarnedRaw = ""
}

// resetSMBWarnedRaw clears the warn-once memory inside a test that already holds the lock via withSMBWarnf.
func resetSMBWarnedRaw(t *testing.T) {
	t.Helper()
	lockSMBGlobals(t)
	smbWarnedRaw = ""
}

func withSMBConfiguredPorts(t *testing.T, ports []int) {
	t.Helper()
	lockSMBGlobals(t)
	old := SMBConfiguredPorts()
	t.Cleanup(func() { SetSMBConfiguredPorts(old) })
	SetSMBConfiguredPorts(ports)
}

func withSMBNow(t *testing.T, fn func() time.Time) {
	t.Helper()
	lockSMBGlobals(t)
	old := smbNow
	t.Cleanup(func() { smbNow = old })
	smbNow = fn
}
