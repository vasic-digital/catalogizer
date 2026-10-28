package pathguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func attackFixture(t *testing.T) (base, root, db string) {
	base = t.TempDir()
	root = filepath.Join(base, "data")
	if err := os.MkdirAll(filepath.Join(root, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	db = filepath.Join(root, "catalogizer.db")
	if err := os.WriteFile(db, []byte("SQLite format 3\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	return
}

func TestConfineData_AttackMatrix(t *testing.T) {
	base, root, db := attackFixture(t)
	key := filepath.Join(base, "keys", "db.key")
	_ = os.MkdirAll(filepath.Dir(key), 0o700)
	_ = os.WriteFile(key, []byte("k"), 0o600)
	restore := setProtectedForTest(append(databaseFiles(db), key))
	defer restore()
	roots := []string{root, filepath.Join(base, "keys")}

	must := func(name, p string, wantProtected bool) {
		t.Helper()
		_, err := ConfineData(p, roots)
		gotP := errors.Is(err, ErrProtected)
		if gotP != wantProtected {
			t.Errorf("%s: ConfineData(%q) err=%v wantProtected=%v", name, p, err, wantProtected)
		}
	}
	// a) symlink to the DB inside the root
	_ = os.Symlink(db, filepath.Join(root, "alias"))
	must("symlink alias of db", filepath.Join(root, "alias"), true)
	// relative symlink to the db dir, then companion
	_ = os.Symlink("..", filepath.Join(root, "sub", "up"))
	must("wal via relative dir link", filepath.Join(root, "sub", "up", "catalogizer.db-wal"), true)
	// d) -wal via '..' after symlink (lnk -> sub/deep, lnk/../../x = root/x)
	_ = os.Symlink("sub/deep", filepath.Join(root, "lnk"))
	// a ".." after a symlink is refused before the protected check: Join/Dir on the raw string
	// would read it differently from the kernel
	for _, p := range []string{root + "/lnk/../../catalogizer.db-wal", root + "/lnk/../../catalogizer.db-journal"} {
		if _, err := ConfineData(p, roots); !errors.Is(err, ErrAmbiguousPath) {
			t.Errorf(".. after link: ConfineData(%q) = %v, want ErrAmbiguousPath", p, err)
		}
	}
	must("shm direct", db+"-shm", true)
	// e) key file and its ancestor directory
	must("key file", key, true)
	must("key dir", filepath.Dir(key), true)
	// g) root itself == db parent
	must("root holds db", root, true)
	must("root with trailing slash", root+"/", true)
	must("root via dot", root+"/.", true)
	must("sibling ok", filepath.Join(root, "sub"), false)
	must("prefix-similar name ok", db+"x", false)
	must("db-wal.bak ok", db+"-wal.bak", false)
	// dangling symlink pointing at the (future) wal
	_ = os.Symlink(db+"-wal", filepath.Join(root, "sub", "dang"))
	must("dangling link to wal", filepath.Join(root, "sub", "dang"), true)
	// symlinked directory holding the db
	_ = os.Symlink(root, filepath.Join(base, "dlink"))
	_, err := ConfineData(filepath.Join(base, "dlink", "catalogizer.db"), []string{filepath.Join(base, "dlink")})
	if !errors.Is(err, ErrProtected) {
		t.Errorf("db via symlinked root: %v", err)
	}
	// b) a hard link is an inode alias that no path comparison can see; ConfineData compares
	// file identity (os.SameFile) with every existing protected file.
	hl := filepath.Join(root, "sub", "hl")
	if err := os.Link(db, hl); err != nil {
		t.Fatalf("hard link fixture: %v", err)
	}
	must("hard link of db", hl, true)
}

// protected path that is itself reached through a symlink (DB path configured via a link)
func TestConfineData_ProtectedPathIsASymlink(t *testing.T) {
	base, root, db := attackFixture(t)
	link := filepath.Join(base, "dblink.db")
	_ = os.Symlink(db, link)
	restore := setProtectedForTest(databaseFiles(link))
	defer restore()
	if _, err := ConfineData(db, []string{root}); !errors.Is(err, ErrProtected) {
		t.Errorf("real db while protected path is a link: %v", err)
	}
	// Which name SQLite gives the companions of a symlinked database (next to the link or
	// next to the target) is not assumed: both are protected.
	for _, p := range []string{db + "-wal", db + "-journal", link + "-wal"} {
		if _, err := ConfineData(p, []string{root, base}); !errors.Is(err, ErrProtected) {
			t.Errorf("companion %q while the protected path is a link: %v, want ErrProtected", p, err)
		}
	}
}
