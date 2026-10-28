package pathguard

// Extended differential oracle (adopted from the review-round-5 reviewer, with the reviewer's
// root exemption removed). For every case a fresh tree is built from a per-case seed and EVERY
// derived operation a caller performs runs on its own fresh copy of that tree:
//
//	0 create(raw)                         file sink        -> judged against ConfineFile
//	1 MkdirAll(raw)                       directory sink   -> judged against Confine
//	2 create(Join(raw, "child"))          directory sink   -> judged against Confine
//	3 MkdirAll(Join(raw, "d", "e"))       directory sink   -> judged against Confine
//	4 create(Join(Dir(raw), "sib"))       file sink        -> judged against ConfineFile
//	  (the pages a multi-page converter writes next to its target)
//	5 Walk(raw), every regular entry      directory sink   -> judged against Confine
//	6 Rename(Join(Dir(raw), "x.html"), raw) file sink      -> judged against ConfineFile
//	  (LibreOffice writes into Dir(target), the converter renames onto the target)
//
// Escape = the guard for that operation accepted raw AND the kernel created (or the walk
// reached) something outside the real root. Raw paths start at the root, at a symlink to the
// root, at base, at out, or at a random directory; the root argument is the real root or a
// symlink to it. PATHGUARD_FUZZ_SEED / PATHGUARD_FUZZ_ITERS select the run.

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type extCase struct {
	raw, rootArg string
	links        []string
}

func extBuild(t *testing.T, seed int64) (base string, c extCase) {
	rng := rand.New(rand.NewSource(seed))
	base = t.TempDir()
	root := filepath.Join(base, "root")
	out := filepath.Join(base, "out")
	_ = os.MkdirAll(root, 0o755)
	_ = os.MkdirAll(out, 0o755)
	_ = os.Symlink("root", filepath.Join(base, "rl"))
	names := []string{"a", "b", "c", "l1", "l2", "l3", "f"}
	dirs := []string{root, out, base}
	for k := 0; k < 4; k++ {
		d := filepath.Join(dirs[rng.Intn(len(dirs))], names[rng.Intn(3)])
		if os.MkdirAll(d, 0o755) == nil {
			dirs = append(dirs, d)
		}
	}
	for k := 0; k < 3; k++ {
		_ = os.WriteFile(filepath.Join(dirs[rng.Intn(len(dirs))], "f"), []byte("x"), 0o644)
	}
	for k := 0; k < 6; k++ {
		ld := dirs[rng.Intn(len(dirs))]
		l := filepath.Join(ld, names[3+rng.Intn(3)])
		var target string
		switch rng.Intn(6) {
		case 0:
			target = filepath.Join(dirs[rng.Intn(len(dirs))], []string{"", "new", "a", "f"}[rng.Intn(4)])
		case 1:
			target = strings.Repeat("../", rng.Intn(4)) + []string{"out", "root", "rl", "new", "a", "l1", "out/new", "f"}[rng.Intn(8)]
		case 2:
			target = []string{"l1/../new", "l2/..", "..", ".", "/", "l1", "l2/../../out", "./l3", "a/../l1/x", "a/", "f/", "../rl/a"}[rng.Intn(12)]
		case 3:
			target = filepath.Base(l)
		case 4:
			target = filepath.Join(base, "rl", []string{"", "a", "l1", "f"}[rng.Intn(4)])
		default:
			rel, _ := filepath.Rel(ld, dirs[rng.Intn(len(dirs))])
			target = rel + "/" + []string{"", "new", "l1"}[rng.Intn(3)]
		}
		if os.Symlink(target, l) == nil {
			c.links = append(c.links, l+" -> "+target)
		}
	}
	starts := []string{root, root, root, filepath.Join(base, "rl"), base, out, dirs[rng.Intn(len(dirs))]}
	start := starts[rng.Intn(len(starts))]
	var comps []string
	for k := 0; k < 1+rng.Intn(5); k++ {
		comps = append(comps, []string{"a", "b", "c", "l1", "l2", "l3", "..", ".", "", "new", "f", "root", "rl"}[rng.Intn(13)])
	}
	c.raw = start + "/" + strings.Join(comps, "/")
	if rng.Intn(5) == 0 {
		c.raw += "/"
	}
	if !strings.HasSuffix(c.raw, "/") && rng.Intn(2) == 0 {
		c.raw += "/newfile"
	}
	c.rootArg = root
	if rng.Intn(3) == 0 {
		c.rootArg = filepath.Join(base, "rl")
	}
	return base, c
}

// fileSinkMode reports whether operation mode writes a FILE at raw or next to it.
func fileSinkMode(mode int) bool { return mode == 0 || mode == 4 || mode == 6 }

func TestConfine_ExtendedDifferentialSevenOperations(t *testing.T) {
	seed := int64(20261028)
	if v := os.Getenv("PATHGUARD_FUZZ_SEED"); v != "" {
		fmt.Sscan(v, &seed)
	}
	iters := 1500
	if v := os.Getenv("PATHGUARD_FUZZ_ITERS"); v != "" {
		fmt.Sscan(v, &iters)
	}
	const modes = 7
	escapes, accDir, accFile, rootRefusedAsFile, falseRef := 0, 0, 0, 0, 0
	var ex []string
	for it := 0; it < iters; it++ {
		cs := seed*1000003 + int64(it)
		_, c0 := extBuild(t, cs)
		_, dirErr := Confine(c0.raw, []string{c0.rootArg})
		_, fileErr := ConfineFile(c0.raw, []string{c0.rootArg})
		if dirErr == nil {
			accDir++
		}
		if fileErr == nil {
			accFile++
		}
		if dirErr == nil && fileErr != nil {
			rootRefusedAsFile++
		}
		anyInsideOK := false
		for mode := 0; mode < modes; mode++ {
			base, c := extBuild(t, cs)
			realRoot, _ := filepath.EvalSymlinks(filepath.Join(base, "root"))
			before := snap(base)
			var outside []string
			create := func(q string) error {
				fh, err := os.OpenFile(q, os.O_CREATE|os.O_WRONLY, 0o644)
				if err == nil {
					fh.Close()
				}
				return err
			}
			var kerr error
			raw := c.raw
			switch mode {
			case 0:
				kerr = create(raw)
			case 1:
				kerr = os.MkdirAll(raw, 0o755)
			case 2:
				kerr = create(filepath.Join(raw, "child"))
			case 3:
				kerr = os.MkdirAll(filepath.Join(raw, "d", "e"), 0o755)
			case 4:
				if kerr = os.MkdirAll(filepath.Dir(raw), 0o755); kerr == nil {
					kerr = create(filepath.Join(filepath.Dir(raw), "sib"))
				}
			case 5:
				kerr = filepath.Walk(raw, func(q string, fi os.FileInfo, err error) error {
					if err != nil {
						return nil
					}
					// a symlinked leaf is skipped by the walkers (uploadableFile); count only
					// regular files and directories reached through the walk path
					if r, rerr := filepath.EvalSymlinks(q); rerr == nil && !within(realRoot, r) && fi.Mode()&os.ModeSymlink == 0 {
						outside = append(outside, "walk:"+q+"=>"+r)
					}
					return nil
				})
			case 6:
				src := filepath.Join(filepath.Dir(raw), "x.html")
				_ = create(src)
				kerr = os.Rename(src, raw)
			}
			for q := range snap(base) {
				if !before[q] && !within(realRoot, q) {
					outside = append(outside, q)
				}
			}
			if kerr == nil && len(outside) == 0 {
				anyInsideOK = true
			}
			guardErr := dirErr
			if fileSinkMode(mode) {
				guardErr = fileErr
			}
			if guardErr == nil && len(outside) > 0 {
				escapes++
				if len(ex) < 10 {
					ex = append(ex, fmt.Sprintf("ESCAPE mode=%d raw=%q root=%q out=%v links=%v", mode, raw, c.rootArg, outside, c.links))
				}
			}
		}
		if dirErr != nil && anyInsideOK {
			falseRef++
		}
	}
	t.Logf("seed=%d cases=%d acceptedAsDir=%d acceptedAsFile=%d rootRefusedAsFile=%d escapes=%d refusedButSomeModeSafe=%d",
		seed, iters, accDir, accFile, rootRefusedAsFile, escapes, falseRef)
	for _, e := range ex {
		t.Log(e)
	}
	if escapes > 0 {
		t.Fatalf("%d escapes", escapes)
	}
	// the oracle must actually exercise both verdicts, or a guard that refuses everything passes
	if accFile == 0 || rootRefusedAsFile == 0 {
		t.Fatalf("degenerate run: acceptedAsFile=%d rootRefusedAsFile=%d", accFile, rootRefusedAsFile)
	}
}
