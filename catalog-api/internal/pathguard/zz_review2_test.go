package pathguard

import (
	"os"
	"path/filepath"
	"testing"
)

// Review round 2, S1: a dangling RELATIVE link target is resolved by the kernel against the
// link's REAL directory, not its lexical parent. Each case builds a tree, asks Confine, then
// performs the real write through the path and compares Confine's verdict with where the
// kernel actually put the file (the oracle is the kernel, not this package).
func TestConfine_RelativeDanglingUnderSymlinkedParent(t *testing.T) {
	type tc struct {
		name  string
		build func(t *testing.T, base, root string) string // returns the path to write
	}
	mk := func(t *testing.T, p string) {
		t.Helper()
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ln := func(t *testing.T, target, link string) {
		t.Helper()
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	cases := []tc{
		{"reviewer repro: lnk->root/c, c/d->../../out/new", func(t *testing.T, base, root string) string {
			mk(t, filepath.Join(root, "x", "y", "z"))
			mk(t, filepath.Join(root, "c"))
			mk(t, filepath.Join(base, "out"))
			ln(t, filepath.Join(root, "c"), filepath.Join(root, "x", "y", "z", "lnk"))
			ln(t, filepath.Join("..", "..", "out", "new"), filepath.Join(root, "c", "d"))
			return filepath.Join(root, "x", "y", "z", "lnk", "d")
		}},
		{"chain: c/d1->d2 (relative), c/d2->../../out/new", func(t *testing.T, base, root string) string {
			mk(t, filepath.Join(root, "x", "y", "z"))
			mk(t, filepath.Join(root, "c"))
			mk(t, filepath.Join(base, "out"))
			ln(t, filepath.Join(root, "c"), filepath.Join(root, "x", "y", "z", "lnk"))
			ln(t, "d2", filepath.Join(root, "c", "d1"))
			ln(t, filepath.Join("..", "..", "out", "new"), filepath.Join(root, "c", "d2"))
			return filepath.Join(root, "x", "y", "z", "lnk", "d1")
		}},
		{"relative parent link: lnk->../c (relative), c/d->../../out/new", func(t *testing.T, base, root string) string {
			mk(t, filepath.Join(root, "x", "y", "z"))
			mk(t, filepath.Join(root, "x", "y", "c"))
			mk(t, filepath.Join(base, "out"))
			// x/y/z/lnk -> ../c resolves to root/x/y/c; c/d -> ../../../../out/new => base/out/new
			ln(t, filepath.Join("..", "c"), filepath.Join(root, "x", "y", "z", "lnk"))
			ln(t, filepath.Join("..", "..", "..", "..", "out", "new"), filepath.Join(root, "x", "y", "c", "d"))
			return filepath.Join(root, "x", "y", "z", "lnk", "d")
		}},
		{"symlink inside target then ..: l->s/../new, s->outside dir", func(t *testing.T, base, root string) string {
			mk(t, filepath.Join(base, "far", "deep"))
			ln(t, filepath.Join(base, "far", "deep"), filepath.Join(root, "s"))
			ln(t, "s/../new", filepath.Join(root, "l")) // literal: filepath.Join would clean it to "new"
			return filepath.Join(root, "l")
		}},
		{"control: relative dangling target that stays inside", func(t *testing.T, base, root string) string {
			mk(t, filepath.Join(root, "x", "y", "z"))
			mk(t, filepath.Join(root, "c", "k"))
			ln(t, filepath.Join(root, "c", "k"), filepath.Join(root, "x", "y", "z", "lnk"))
			ln(t, filepath.Join("..", "new"), filepath.Join(root, "c", "k", "d"))
			return filepath.Join(root, "x", "y", "z", "lnk", "d")
		}},
		{"control: plain missing file under symlinked dir", func(t *testing.T, base, root string) string {
			mk(t, filepath.Join(root, "real"))
			ln(t, "real", filepath.Join(root, "alias"))
			return filepath.Join(root, "alias", "f")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "root")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			p := c.build(t, base, root)
			got, cerr := Confine(p, []string{root})

			// Oracle: really write through p, then ask the kernel where it landed.
			f, werr := os.Create(p)
			if werr != nil {
				t.Fatalf("oracle write failed: %v", werr)
			}
			_ = f.Close()
			// Locate the created file by inode with a walk that never follows symlinks, so
			// the reported location is the physical one (filepath.EvalSymlinks was measured
			// to collapse "s/../new" lexically and is therefore not a valid oracle here).
			want, err := os.Stat(p)
			if err != nil {
				t.Fatalf("oracle stat: %v", err)
			}
			landed := ""
			_ = filepath.Walk(base, func(q string, fi os.FileInfo, err error) error {
				if err == nil && fi.Mode().IsRegular() && os.SameFile(fi, want) {
					landed = q
				}
				return nil
			})
			if landed == "" {
				t.Fatalf("oracle: created file not found under %s", base)
			}
			realRoot, _ := filepath.EvalSymlinks(root)
			inside := within(realRoot, landed)
			t.Logf("confine=%q err=%v landed=%q", got, cerr, landed)
			if inside && cerr != nil {
				t.Fatalf("false refusal: kernel wrote inside (%s) but Confine said %v", landed, cerr)
			}
			if !inside && cerr == nil {
				t.Fatalf("ESCAPE: Confine accepted %q but the kernel wrote %q outside the root", got, landed)
			}
			if cerr == nil && got != landed {
				t.Fatalf("Confine resolved %q, kernel wrote %q", got, landed)
			}
		})
	}
}

// A ".." after a missing element cannot be resolved before the element exists; refuse it.
// The dangerous shape: once "missing" is created, "missing/../out" is root/out, an EXISTING
// symlink to outside the root, which a lexical join would never look at.
func TestConfine_DotDotAfterMissingRefused(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"missing/../out/f", "missing/../../x"} {
		l := filepath.Join(root, "l")
		_ = os.Remove(l)
		if err := os.Symlink(target, l); err != nil {
			t.Fatal(err)
		}
		if got, err := Confine(l, []string{root}); err == nil {
			t.Fatalf("target %q accepted as %q", target, got)
		}
	}
}
