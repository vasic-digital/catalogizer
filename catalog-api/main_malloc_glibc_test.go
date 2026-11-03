//go:build cgo && linux && !musl

package main

import (
	"os"
	"testing"
)

// S1: the load-time constructor must have capped the glibc malloc arenas before the Go runtime started
// its threads (the effect on RSS is proven by the live soak, scripts/test-infra/scaling_live.sh with
// SCALING_SOAK_ROUNDS; see docs/TESTING_GUIDE.md).
func TestMallocArenaCapAppliedAtLoad(t *testing.T) {
	if os.Getenv("MALLOC_ARENA_MAX") != "" {
		t.Skip("SKIP: operator-set MALLOC_ARENA_MAX in the environment (explicit value wins by design)")
	}
	if got := mallocArenaCap(); got != 2 {
		t.Fatalf("load-time constructor did not apply the arena cap: applied=%d want 2", got)
	}
}
