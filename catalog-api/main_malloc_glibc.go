//go:build cgo && linux && !musl

package main

/*
#define _GNU_SOURCE
#include <malloc.h>
#include <stdlib.h>

// S1 (memory soak): catalog-api links SQLCipher through cgo. Every OS thread that calls into C gets its
// own glibc malloc arena (default cap: 8 x cores). Measured on the live soak: ~54 arenas, RSS +100 MiB over
// 30 identical load rounds while the Go heap stayed flat. The cap MUST be applied before the Go runtime starts
// its threads, so it is a load-time constructor (calling mallopt from a Go init() was measured to be too late:
// the arenas were still created). An explicit MALLOC_ARENA_MAX in the environment always wins.
#define CATALOGIZER_DEFAULT_ARENA_MAX 2
static int catalogizer_arena_cap_applied = 0;
static void __attribute__((constructor)) catalogizer_cap_malloc_arenas(void) {
	const char *e = getenv("MALLOC_ARENA_MAX");
	if (e != NULL && *e != '\0') return; // glibc already honoured the operator's value
	if (mallopt(M_ARENA_MAX, CATALOGIZER_DEFAULT_ARENA_MAX) == 1) catalogizer_arena_cap_applied = CATALOGIZER_DEFAULT_ARENA_MAX;
}
static int catalogizer_arena_cap(void) { return catalogizer_arena_cap_applied; }

*/
import "C"

// mallocArenaCap returns the arena cap applied by the load-time constructor (0 = not applied).
func mallocArenaCap() int { return int(C.catalogizer_arena_cap()) }
