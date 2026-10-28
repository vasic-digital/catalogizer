package pathguard

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Review round 6, F2: confine resolved every root entry and skipped only those that failed to
// resolve. An empty entry splits to no components and resolves to "/", so
// ConfineFile(root+"/x", []string{""}) was ACCEPTED with "/" as the effective root, and a
// relative entry resolved against nothing meaningful. A root list with any entry that is
// empty, relative or carries a NUL is a configuration error: the whole call fails closed.
// The adapted reviewer probe is TestRV6_Probes "empty-root-entry".
func TestConfine_RV6_InvalidRootEntriesFailClosed(t *testing.T) {
	base := t.TempDir()
	base, _ = filepath.EvalSymlinks(base)
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "x")

	bad := []struct {
		name  string
		roots []string
	}{
		{"empty-entry", []string{""}},
		{"empty-entry-beside-valid-root", []string{root, ""}},
		{"empty-entry-before-valid-root", []string{"", root}},
		{"relative-entry", []string{"root"}},
		{"relative-entry-beside-valid-root", []string{root, "rel/dir"}},
		{"dot-entry", []string{"."}},
		{"nul-entry", []string{root + "\x00"}},
	}
	for _, tc := range bad {
		for _, form := range []struct {
			name string
			fn   func(string, []string) (string, error)
		}{
			{"Confine", Confine}, {"ConfineFile", ConfineFile},
			{"ConfineData", ConfineData}, {"ConfineDataFile", ConfineDataFile},
		} {
			got, err := form.fn(target, tc.roots)
			if err == nil {
				t.Errorf("%s/%s: accepted %q (resolved %q), want a refusal", tc.name, form.name, target, got)
				continue
			}
			if !errors.Is(err, ErrInvalidRoot) {
				t.Errorf("%s/%s: refused with %v, want ErrInvalidRoot", tc.name, form.name, err)
			}
		}
	}

	// golden-FALSE controls: valid absolute roots, including the filesystem root, still work
	for _, ok := range []struct {
		name  string
		p     string
		roots []string
		file  bool
	}{
		{"valid-root-file", target, []string{root}, true},
		{"valid-root-dir", root, []string{root}, false},
		{"slash-root-file", target, []string{"/"}, true},
		{"trailing-slash-root", target, []string{root + "/"}, true},
	} {
		var err error
		if ok.file {
			_, err = ConfineFile(ok.p, ok.roots)
		} else {
			_, err = Confine(ok.p, ok.roots)
		}
		if err != nil {
			t.Errorf("%s: valid root refused: %v", ok.name, err)
		}
	}
}
