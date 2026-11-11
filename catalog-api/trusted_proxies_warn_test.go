package main

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func warnMessages(t *testing.T, tp string) []observer.LoggedEntry {
	t.Helper()
	core, logs := observer.New(zapcore.DebugLevel)
	warnTrustedProxies(zap.New(core), tp)
	return logs.AllUntimed()
}

// TP-FU-1: the startup warnings are driven through the real helper main.go calls.
func TestWarnTrustedProxies_CatchAll(t *testing.T) {
	for _, tp := range []string{"0.0.0.0/0", "::/0", "10.0.0.0/8, 0.0.0.0/0", " ::/0 ,127.0.0.1"} {
		got := warnMessages(t, tp)
		if len(got) != 1 || got[0].Level != zapcore.WarnLevel || !strings.Contains(got[0].Message, "catch-all CIDR") {
			t.Fatalf("tp=%q: want exactly one catch-all warning, got %+v", tp, got)
		}
	}
}

func TestWarnTrustedProxies_Auto(t *testing.T) {
	got := warnMessages(t, "auto")
	if len(got) != 1 || got[0].Level != zapcore.WarnLevel || !strings.Contains(got[0].Message, "DEPRECATED") {
		t.Fatalf("want exactly one auto-deprecation warning, got %+v", got)
	}
}

// Golden-good: no warning for unset or specific lists (incl. near-misses).
func TestWarnTrustedProxies_Quiet(t *testing.T) {
	for _, tp := range []string{"", "10.0.0.0/8", "192.168.1.5/32,172.16.0.0/12", "0.0.0.0/8", "::/1", "0.0.0.0", "AUTO"} {
		if got := warnMessages(t, tp); len(got) != 0 {
			t.Fatalf("tp=%q: want no warning, got %+v", tp, got)
		}
	}
}
