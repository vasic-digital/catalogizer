package pathguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Review round 5 (RV5-B): Confine accepts a path equal to the root, so filepath.Dir of it is the
// root's PARENT. File-target sinks (conversion output, WebDAV download, copy destination, the
// pages a multi-page converter writes next to the target) take Dir(target) and write there.
// ConfineFile and ConfineDataFile accept only paths strictly below the root they matched.
func TestConfineFile_RefusesTheRootItself(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("root", filepath.Join(base, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".", filepath.Join(root, "self")); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	for _, raw := range []string{root, root + "/", root + "/.", root + "//", root + "/sub/..", root + "/self", alias, alias + "/"} {
		for _, rootArg := range []string{root, alias} {
			if _, err := Confine(raw, []string{rootArg}); err != nil {
				t.Errorf("control: Confine(%q, %q) must accept the root as a directory argument: %v", raw, rootArg, err)
			}
			if got, err := ConfineFile(raw, []string{rootArg}); err == nil || !errors.Is(err, ErrRootItself) {
				t.Errorf("ConfineFile(%q, %q) = %q, %v; want ErrRootItself", raw, rootArg, got, err)
			}
			if got, err := ConfineDataFile(raw, []string{rootArg}); err == nil || !errors.Is(err, ErrRootItself) {
				t.Errorf("ConfineDataFile(%q, %q) = %q, %v; want ErrRootItself", raw, rootArg, got, err)
			}
		}
	}
	// positive controls: a file or directory strictly below the root, including a new name.
	for _, raw := range []string{root + "/f", root + "/sub", root + "/sub/new.html", alias + "/f", root + "/self/f"} {
		if _, err := ConfineFile(raw, []string{root}); err != nil {
			t.Errorf("ConfineFile(%q) refused a path below the root: %v", raw, err)
		}
		if _, err := ConfineDataFile(raw, []string{root}); err != nil {
			t.Errorf("ConfineDataFile(%q) refused a path below the root: %v", raw, err)
		}
	}
	// still refuses what Confine refuses
	if _, err := ConfineFile(filepath.Join(base, "x"), []string{root}); !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("ConfineFile outside the root: err=%v, want ErrOutsideRoots", err)
	}
}

// With two roots, one nested inside the other, the nested root itself is strictly below the
// outer root, so it is a valid file target there (its Dir is inside the outer root).
func TestConfineFile_NestedRootIsBelowTheOuterRoot(t *testing.T) {
	outer := t.TempDir()
	inner := filepath.Join(outer, "in")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ConfineFile(inner, []string{inner, outer}); err != nil {
		t.Errorf("ConfineFile(inner, [inner, outer]) = %v; want accepted via the outer root", err)
	}
	if _, err := ConfineFile(inner, []string{inner}); !errors.Is(err, ErrRootItself) {
		t.Errorf("ConfineFile(inner, [inner]) = %v; want ErrRootItself", err)
	}
}

// ConfineDataFile keeps every ConfineData check (protected file, hard link to it).
func TestConfineDataFile_KeepsProtectedChecks(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "catalogizer.db")
	if err := os.WriteFile(db, []byte("SQLite format 3\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer setProtectedForTest(databaseFiles(db))()
	if err := os.Link(db, filepath.Join(root, "hl")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{db, db + "-wal", filepath.Join(root, "hl")} {
		if _, err := ConfineDataFile(p, []string{root}); !errors.Is(err, ErrProtected) {
			t.Errorf("ConfineDataFile(%q) = %v; want ErrProtected", p, err)
		}
	}
}

// Reviewer round-5 probe table, as assertions: whenever ConfineFile accepts raw, raw resolves
// strictly below a root and Dir(raw) (the directory a file sink writes into) resolves inside one.
func TestConfineFile_DirOfAcceptedPathStaysInside(t *testing.T) {
	type row struct {
		name  string
		setup func(base, root string) (raw string, roots []string)
	}
	rows := []row{
		{"chain rel->abs->rel out", func(b, r string) (string, []string) {
			_ = os.Symlink("../out", filepath.Join(b, "x"))
			_ = os.Symlink(filepath.Join(b, "x"), filepath.Join(r, "l2"))
			_ = os.Symlink("l2", filepath.Join(r, "l1"))
			return r + "/l1/f", []string{r}
		}},
		{"trailing slash on link to out", func(b, r string) (string, []string) {
			_ = os.Symlink(filepath.Join(b, "out"), filepath.Join(r, "l"))
			return r + "/l/", []string{r}
		}},
		{"dotdot after link (inside target)", func(b, r string) (string, []string) {
			_ = os.MkdirAll(filepath.Join(r, "a", "b"), 0o755)
			_ = os.Symlink("a/b", filepath.Join(r, "l"))
			return r + "/l/../f", []string{r}
		}},
		{"root is symlink, raw via real", func(b, r string) (string, []string) {
			_ = os.Symlink("root", filepath.Join(b, "rl"))
			return r + "/f", []string{filepath.Join(b, "rl")}
		}},
		{"root is symlink, raw via alias", func(b, r string) (string, []string) {
			_ = os.Symlink("root", filepath.Join(b, "rl"))
			return b + "/rl/f", []string{filepath.Join(b, "rl")}
		}},
		{"root is a dangling link", func(b, r string) (string, []string) {
			_ = os.Symlink("nonexist", filepath.Join(b, "rl"))
			return b + "/rl/f", []string{filepath.Join(b, "rl")}
		}},
		{"empty and dot components", func(b, r string) (string, []string) { return r + "//./././/f", []string{r} }},
		{"case variant root", func(b, r string) (string, []string) { return filepath.Join(b, "ROOT", "f"), []string{r} }},
		{"unicode lookalike", func(b, r string) (string, []string) {
			_ = os.MkdirAll(filepath.Join(b, "rоot"), 0o755)
			return filepath.Join(b, "rоot", "f"), []string{r}
		}},
		{"link to root from outside, file below", func(b, r string) (string, []string) {
			_ = os.Symlink(r, filepath.Join(b, "out", "alias"))
			return b + "/out/alias/f", []string{r}
		}},
		{"link into root subdir from outside", func(b, r string) (string, []string) {
			_ = os.MkdirAll(filepath.Join(r, "s"), 0o755)
			_ = os.Symlink(filepath.Join(r, "s"), filepath.Join(b, "out", "alias"))
			return b + "/out/alias/f", []string{r}
		}},
		{"raw == root", func(b, r string) (string, []string) { return r, []string{r} }},
		{"raw == root/", func(b, r string) (string, []string) { return r + "/", []string{r} }},
		{"raw == root/.", func(b, r string) (string, []string) { return r + "/.", []string{r} }},
		{"link in root to root", func(b, r string) (string, []string) {
			_ = os.Symlink(".", filepath.Join(r, "self"))
			return r + "/self", []string{r}
		}},
		{"link in root to its own parent then back", func(b, r string) (string, []string) {
			_ = os.Symlink("..", filepath.Join(r, "up"))
			return r + "/up/root/f", []string{r}
		}},
		{"two roots, link between", func(b, r string) (string, []string) {
			_ = os.MkdirAll(filepath.Join(b, "r2"), 0o755)
			_ = os.Symlink(filepath.Join(b, "r2"), filepath.Join(r, "x"))
			return r + "/x/f", []string{r, filepath.Join(b, "r2")}
		}},
		{"file component then slash", func(b, r string) (string, []string) {
			_ = os.WriteFile(filepath.Join(r, "f"), nil, 0o644)
			return r + "/f/", []string{r}
		}},
	}
	// "root is a dangling link" and "link to root from outside, file below" are accepted on
	// purpose: Dir of the path resolves to the root itself (through the link), which is inside.
	mustRefuse := map[string]bool{"raw == root": true, "raw == root/": true, "raw == root/.": true, "link in root to root": true,
		"chain rel->abs->rel out": true, "trailing slash on link to out": true,
		"case variant root": true, "unicode lookalike": true,
		"link into root subdir from outside": true, "link in root to its own parent then back": true}
	for _, rw := range rows {
		t.Run(rw.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "root")
			_ = os.MkdirAll(root, 0o755)
			_ = os.MkdirAll(filepath.Join(base, "out"), 0o755)
			raw, roots := rw.setup(base, root)
			got, err := ConfineFile(raw, roots)
			if mustRefuse[rw.name] && err == nil {
				t.Fatalf("ConfineFile(%q) accepted %q; this case must be refused", raw, got)
			}
			if err != nil {
				return
			}
			var realRoots []string
			for _, r := range roots {
				if rr, e := resolve(r); e == nil { // resolve follows a dangling root link; EvalSymlinks does not
					realRoots = append(realRoots, rr)
				}
			}
			inside := func(q string, strict bool) bool {
				for _, rr := range realRoots {
					if within(rr, q) && (!strict || q != rr) {
						return true
					}
				}
				return false
			}
			if !inside(got, true) {
				t.Errorf("accepted %q resolves to %q, not strictly below a root %v", raw, got, realRoots)
			}
			if d, derr := filepath.EvalSymlinks(filepath.Dir(filepath.Clean(raw))); derr == nil && !inside(d, false) {
				t.Errorf("accepted %q but Dir resolves to %q, outside %v", raw, d, realRoots)
			}
		})
	}
}
