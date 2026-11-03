package utils

import (
	"os"
	"strings"
	"testing"
	"time"
)

// Review r2 G2: every probe in the reviewer's corpus (attacks the base detector
// caught) must still be detected, trailing text after the comment included.
func TestContainsSQLInjection_CommentAttackCorpus(t *testing.T) {
	raw, err := os.ReadFile("testdata/sqli_comment_attacks.txt")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		n++
		if !ContainsSQLInjection(line) {
			t.Errorf("missed: %q", line)
		}
	}
	if n != 19 {
		t.Fatalf("corpus lines = %d, want 19", n)
	}
}

func TestContainsSQLInjection_CommentLegit(t *testing.T) {
	for _, p := range []string{
		"Season 1 -- Episode 4", "Back in Black (live) -- remaster", "Episode 3 -- The Return", "Best of 2020 -- 2021",
		"Chapter 1 -- intro", "Movie (1999) -- director's cut", `"Quoted" -- review`, "Price: $10 -- sale",
		"U2 -- One", "correct-horse--battery", "Ep. 7 #", "Tag #1 #2", "Part (2) -- the end", "Vol. 3 -- Greatest Hits",
	} {
		if ContainsSQLInjection(p) {
			t.Errorf("false positive: %q", p)
		}
	}
}

// G3: the terminator check is one linear pass. 4 MB of balanced quotes and "--"
// markers took 46 s with a per-marker strings.Count.
func TestHasSQLCommentTerminator_LinearTime(t *testing.T) {
	big := strings.Repeat("''--", 1<<20) // 4 MiB
	start := time.Now()
	_ = hasSQLCommentTerminator(big)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("4 MiB took %v, want well under 1s (quadratic?)", d)
	}
	// numeric-prefix shape that would rescan the prefix at every marker
	big2 := strings.Repeat("(", 1<<20) + "x1" + strings.Repeat("--1", 1<<19)
	start = time.Now()
	_ = hasSQLCommentTerminator(big2)
	if d := time.Since(start); d > time.Second {
		t.Fatalf("numeric-prefix shape took %v", d)
	}
}
