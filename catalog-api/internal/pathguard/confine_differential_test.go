package pathguard

// Differential property test (adopted from the review-round-3 reviewer). Random trees with symlinks (relative,
// absolute, dangling, loops, chains, ".."-bearing targets), random raw input paths
// (with "..", ".", "//", trailing slash). For each case: Confine verdict vs what the
// kernel actually touched when the raw path is opened O_CREAT (and, separately,
// MkdirAll(raw) as sync callers do). Escape = Confine accepted AND kernel created
// something outside the real root. FalseRefusal counted separately (not a security bug).

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fsSnap map[string]bool

func snap(base string) fsSnap {
	s := fsSnap{}
	_ = filepath.Walk(base, func(q string, fi os.FileInfo, err error) error {
		if err == nil {
			s[q] = true
		}
		return nil
	})
	return s
}

func TestConfine_DifferentialKernelOracle(t *testing.T) {
	seed := int64(20261028)
	if v := os.Getenv("PATHGUARD_FUZZ_SEED"); v != "" {
		fmt.Sscan(v, &seed)
	}
	iters := 4000
	if v := os.Getenv("PATHGUARD_FUZZ_ITERS"); v != "" {
		fmt.Sscan(v, &iters)
	}
	rng := rand.New(rand.NewSource(seed))
	names := []string{"a", "b", "c", "l1", "l2", "l3", "f"}
	escapes, falseRef, accepted, kernelOK := 0, 0, 0, 0
	var examples []string
	for it := 0; it < iters; it++ {
		base := t.TempDir()
		root := filepath.Join(base, "root")
		out := filepath.Join(base, "out")
		_ = os.MkdirAll(root, 0o755)
		_ = os.MkdirAll(out, 0o755)
		dirs := []string{root, out, base}
		// random dirs
		for k := 0; k < 4; k++ {
			d := filepath.Join(dirs[rng.Intn(len(dirs))], names[rng.Intn(3)])
			if os.MkdirAll(d, 0o755) == nil {
				dirs = append(dirs, d)
			}
		}
		// random files
		for k := 0; k < 2; k++ {
			f := filepath.Join(dirs[rng.Intn(len(dirs))], "f")
			_ = os.WriteFile(f, nil, 0o644)
		}
		// random symlinks
		var tgtParts []string
		for k := 0; k < 5; k++ {
			ld := dirs[rng.Intn(len(dirs))]
			l := filepath.Join(ld, names[3+rng.Intn(3)])
			var target string
			switch rng.Intn(5) {
			case 0: // absolute to a dir/missing
				target = filepath.Join(dirs[rng.Intn(len(dirs))], []string{"", "new", "a"}[rng.Intn(3)])
			case 1: // relative with ..
				n := rng.Intn(4)
				target = strings.Repeat("../", n) + []string{"out", "root", "new", "a", "l1", "out/new"}[rng.Intn(6)]
			case 2: // tricky literal
				target = []string{"l1/../new", "l2/..", "..", ".", "/", "l1", "l2/../../out", "./l3", "a/../l1/x"}[rng.Intn(9)]
			case 3: // self / loop
				target = filepath.Base(l)
			default:
				rel, _ := filepath.Rel(ld, dirs[rng.Intn(len(dirs))])
				target = rel + "/" + []string{"", "new", "l1"}[rng.Intn(3)]
			}
			if os.Symlink(target, l) == nil {
				tgtParts = append(tgtParts, l+" -> "+target)
			}
		}
		// random raw input path rooted at root
		comps := []string{}
		n := 1 + rng.Intn(5)
		for k := 0; k < n; k++ {
			comps = append(comps, []string{"a", "b", "c", "l1", "l2", "l3", "..", ".", "", "new", "f"}[rng.Intn(11)])
		}
		raw := root + "/" + strings.Join(comps, "/")
		if rng.Intn(5) == 0 {
			raw += "/"
		}
		if !strings.HasSuffix(raw, "/") && rng.Intn(2) == 0 {
			raw += "/newfile"
		}

		got, cerr := Confine(raw, []string{root})
		realRoot, _ := filepath.EvalSymlinks(root)
		before := snap(base)
		// Callers also derive paths from the raw string with lexical functions (review round 4,
		// H1), so the oracle exercises those too: mode 2 creates Join(raw, "child"), mode 3
		// creates a sibling in Dir(raw) (multi-page conversion output, LibreOffice --outdir),
		// mode 4 Walks raw and records where each visited entry really is (sync upload walkers).
		mode := it % 5
		var kerr error
		create := func(q string) error {
			fh, err := os.OpenFile(q, os.O_CREATE|os.O_WRONLY, 0o644)
			if err == nil {
				fh.Close()
			}
			return err
		}
		var walkedOutside []string
		switch mode {
		case 0:
			kerr = create(raw)
		case 1:
			kerr = os.MkdirAll(raw, 0o755)
		case 2:
			kerr = create(filepath.Join(raw, "child"))
		case 3:
			if kerr = os.MkdirAll(filepath.Dir(raw), 0o755); kerr == nil {
				kerr = create(filepath.Join(filepath.Dir(raw), "sibling"))
			}
		case 4:
			kerr = filepath.Walk(raw, func(q string, fi os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				// where the entry itself lives: its parent directory resolved by the kernel
				// (a symlinked leaf is the per-file check's business, not Confine's)
				if d, derr := filepath.EvalSymlinks(filepath.Dir(q)); derr == nil && !within(realRoot, d) {
					walkedOutside = append(walkedOutside, q)
				} else if q == raw || q == filepath.Clean(raw) {
					if r, rerr := filepath.EvalSymlinks(q); rerr == nil && !within(realRoot, r) {
						walkedOutside = append(walkedOutside, q)
					}
				}
				return nil
			})
		}
		after := snap(base)
		if kerr == nil {
			kernelOK++
		}
		outsideNew := append([]string(nil), walkedOutside...)
		insideNew := 0
		for q := range after {
			if before[q] {
				continue
			}
			if within(realRoot, q) {
				insideNew++
			} else {
				outsideNew = append(outsideNew, q)
			}
		}
		if cerr == nil {
			accepted++
		}
		if cerr == nil && len(outsideNew) > 0 {
			escapes++
			if len(examples) < 8 {
				examples = append(examples, fmt.Sprintf("ESCAPE mode=%d raw=%q confine=%q created=%v links=%v", mode, raw, got, outsideNew, tgtParts))
			}
		}
		if cerr != nil && kerr == nil && len(outsideNew) == 0 && insideNew > 0 {
			falseRef++
		}
		// Also: accepted, kernel wrote inside, but Confine's returned path differs from where a
		// mode-0 file landed (informational)
	}
	t.Logf("seed=%d iters=%d accepted=%d kernelOK=%d escapes=%d falseRefusals=%d", seed, iters, accepted, kernelOK, escapes, falseRef)
	for _, e := range examples {
		t.Log(e)
	}
	if escapes > 0 {
		t.Fatalf("%d escapes", escapes)
	}
}
