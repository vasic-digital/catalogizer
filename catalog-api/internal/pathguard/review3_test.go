package pathguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// N1 (review round 3): Confine used to run filepath.Clean on the raw path before resolving it,
// so "root/lnk/../pwn.txt" with lnk -> /outside/sub was cleaned lexically to "root/pwn.txt"
// and accepted, while the kernel resolves lnk first and writes /outside/pwn.txt. The kernel
// is the oracle here: the file is created through the raw path and then located.
func TestConfine_DotDotAfterSymlinkFollowsTheKernel(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside", "sub")
	for _, d := range []string{root, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "lnk")); err != nil {
		t.Fatal(err)
	}
	raw := root + "/lnk/../pwn.txt"

	got, err := Confine(raw, []string{root})

	f, werr := os.Create(raw)
	if werr != nil {
		t.Fatalf("kernel oracle: create %q: %v", raw, werr)
	}
	f.Close()
	if _, serr := os.Stat(filepath.Join(base, "outside", "pwn.txt")); serr != nil {
		t.Fatalf("kernel oracle: expected the write to land in %s/outside: %v", base, serr)
	}
	if err == nil {
		t.Fatalf("Confine accepted %q as %q; the kernel wrote %s", raw, got, filepath.Join(base, "outside", "pwn.txt"))
	}

	// Control: ".." that the kernel resolves back inside the root is still accepted, and the
	// returned path is where the kernel actually writes.
	if err := os.MkdirAll(filepath.Join(root, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	inside := root + "/d/../ok.txt"
	got, err = Confine(inside, []string{root})
	if err != nil {
		t.Fatalf("control %q refused: %v", inside, err)
	}
	if want := filepath.Join(root, "ok.txt"); got != want {
		t.Fatalf("control resolved to %q, want %q", got, want)
	}
}

// N2 (review round 3): the default write root ./data also holds the default SQLite database
// ./data/catalogizer.db. A protected path, an ancestor of it, or a SQLite companion file may
// not be named by a request, even inside a write root.
func TestConfineData_RefusesProtectedPathsAndAncestors(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "catalogizer.db")
	if err := os.WriteFile(db, []byte("db"), 0o600); err != nil {
		t.Fatal(err)
	}
	restore := setProtectedForTest(nil)
	defer restore()
	ProtectDatabase(db)

	if err := os.MkdirAll(filepath.Join(root, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(db, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		db,                                        // the database itself
		db + "-wal", db + "-shm", db + "-journal", // SQLite companions (may not exist yet)
		root,                         // an ancestor: a sync of it would read/overwrite/delete the database
		filepath.Join(root, "alias"), // a symlink to the database
		filepath.Join(root, "x", "..", "catalogizer.db"),
	} {
		if _, err := ConfineData(p, []string{root}); !errors.Is(err, ErrProtected) {
			t.Errorf("ConfineData(%q) = %v, want ErrProtected", p, err)
		}
	}
	// Plain Confine is unaffected (it is also used for read-only cover-art lookups).
	if _, err := Confine(db, []string{root}); err != nil {
		t.Errorf("Confine(%q) = %v, want nil", db, err)
	}
	// Siblings and sub-directories of the database directory remain usable.
	for _, p := range []string{filepath.Join(root, "out.html"), filepath.Join(root, "sync", "movies"), filepath.Join(root, "catalogizer.db.bak")} {
		if _, err := ConfineData(p, []string{root}); err != nil {
			t.Errorf("ConfineData(%q) = %v, want nil", p, err)
		}
	}
	// Outside the roots is still ErrOutsideRoots, not ErrProtected.
	if _, err := ConfineData("/etc/passwd", []string{root}); !errors.Is(err, ErrOutsideRoots) {
		t.Errorf("ConfineData(/etc/passwd) = %v, want ErrOutsideRoots", err)
	}
}

// The default database location is protected even when main never registers it.
func TestConfineData_DefaultDatabaseProtectedWithoutRegistration(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatal(err)
	}
	restore := setProtectedForTest(defaultProtected())
	defer restore()
	roots := RootsFromEnv("PATHGUARD_TEST_UNSET_ROOTS", DataDir)
	abs, _ := filepath.Abs(filepath.Join("data", "catalogizer.db"))
	if _, err := ConfineData(abs, roots); !errors.Is(err, ErrProtected) {
		t.Fatalf("default database %q: %v, want ErrProtected", abs, err)
	}
	absData, _ := filepath.Abs("data")
	if _, err := ConfineData(absData, roots); !errors.Is(err, ErrProtected) {
		t.Fatalf("default data dir %q: %v, want ErrProtected", absData, err)
	}
	if _, err := ConfineData(filepath.Join(absData, "sync"), roots); err != nil {
		t.Fatalf("data/sync must stay usable: %v", err)
	}
}

// A root that is itself a symlink (e.g. /data -> /mnt/disk1/data) is resolved like the path,
// so a path below it is accepted and one that leaves it is not; comparing the lexical root with
// the resolved path would refuse everything under such a root.
func TestConfine_SymlinkedRootResolvedLikeThePath(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(alias, "x.txt"), filepath.Join(real, "x.txt")} {
		if _, err := Confine(p, []string{alias}); err != nil {
			t.Errorf("Confine(%q, [%q]) = %v, want nil", p, alias, err)
		}
	}
	if _, err := Confine(filepath.Join(base, "x.txt"), []string{alias}); err == nil {
		t.Errorf("a path beside the symlinked root was accepted")
	}
}
