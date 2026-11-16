package poolguard

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// allowlist: explicit, justified exceptions, keyed file:function. Anything not listed here that the
// scanner finds in non-test catalog-api code is a pool-deadlock class member and fails the build.
var allowlist = map[string]string{
	// Runs entirely on ONE *sql.Tx: a transaction pins a single connection, so the nested ExecContext
	// reuses it and never asks the pool for a second one (no pool-deadlock possible).
	"internal/services/playlist_service.go:PlaylistService.RefreshSmartPlaylist": "single-tx, one pinned connection",
}

// TestPoolClassGuard_NoNestedPooledQueryInsideRowsLoop fails when non-test code in catalog-api runs a
// pooled query/exec (directly or through a same-package helper, up to four helper levels) inside a
// `for rows.Next()` loop body. Fix such code by buffering the page and closing the cursor first.
func TestPoolClassGuard_NoNestedPooledQueryInsideRowsLoop(t *testing.T) {
	found, err := Scan("../..")
	require.NoError(t, err)
	var bad []string
	for _, f := range found {
		if _, ok := allowlist[f.Key()]; ok {
			continue
		}
		bad = append(bad, f.String())
	}
	sort.Strings(bad)
	require.Empty(t, bad, "pool-deadlock class members (query inside an open rows cursor):\n%s", strings.Join(bad, "\n"))
}

// Control needles: the instrument must see the positive fixtures and must not flag the negative one.
func TestPoolClassGuard_ControlNeedles(t *testing.T) {
	pos, err := Scan("testdata/positive")
	require.NoError(t, err)
	got := map[string]Finding{}
	for _, f := range pos {
		got[f.Func] = f
	}
	require.Contains(t, got, "Direct")
	require.False(t, got["Direct"].Indirect)
	require.Contains(t, got, "Svc.List")
	require.True(t, got["Svc.List"].Indirect, "pooled query two helpers deep must be found")
	require.Contains(t, got["Svc.List"].Via, "tags")
	require.Contains(t, got, "Svc.ExecPerRow")
	require.Contains(t, got, "Store.TypedLocalNested")
	require.True(t, got["Store.TypedLocalNested"].Indirect)
	for _, fn := range []string{"RangeShadow", "IfaceParam", "EmbedSvc.PromotedRecv", "EmbeddedLocal", "AliasLocal", "ClosureShadow", "TwoTypes", "ExtIfaceParam", "ExtIfaceVar", "ExtIfaceViaHelper", "PkgVarSiblingShadow", "ClosureLocalShadowsPkgVar", "PooledFirstPureLater", "T01StdlibIfaceNameCollides", "T02ExtAliasIface", "T03ExtDefinedIface", "T04ExtStructEmbedsIface", "T05ExtGenericWrapper", "T06TypeParamShadowsPkgType", "T07LocalTypeShadow", "SeenKeyedByRecvType"} {
		fn := fn
		t.Run(fn, func(t *testing.T) {
			require.Contains(t, got, fn, "typed-resolution blind spot: %s", fn)
			require.True(t, got[fn].Indirect, fn)
		})
	}

	neg, err := Scan("testdata/negative")
	require.NoError(t, err)
	require.Empty(t, neg, "buffered shape, non-cursor loops and statically dispatched pure methods must not be flagged: %v", neg)
}

// The allowlist must not rot: every entry must still correspond to a real finding.
func TestPoolClassGuard_AllowlistEntriesStillMatch(t *testing.T) {
	found, err := Scan("../..")
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, f := range found {
		seen[f.Key()] = true
	}
	for k := range allowlist {
		require.True(t, seen[k], "stale allowlist entry %q (remove it)", k)
	}
}

// keysOf returns the file:func identity of each finding.
func keysOf(fs []Finding) map[string]bool {
	m := map[string]bool{}
	for _, f := range fs {
		m[f.File+":"+f.Func] = true
	}
	return m
}

// Mutation record (MAIN-POOLGUARD-R3, round-2 fixes): 29 mutants of scan.go/types.go, 25 killed by
// this file. 4 survive and are equivalent by argument, not shown by a killing fixture: M11 (interface
// receiver trusted) and M12 (type-parameter receiver trusted): the method object of an interface or
// constraint has no FuncDecl, so resolution falls back to by name either way; M22 (union terms not
// walked) and M25 (type-parameter constraint not walked): in a program that compiles neither can reach
// incompleteMethodSet from a receiver staticMethod accepts. Z09 and Z10 (zrev) are flagged by neither
// scanner, and so is N09 (zr2): by-name gaps of the base, not head regressions.

// sanctioned lists, per scanned root, the ONLY functions the by-name base flags and the head may drop:
// each one's loop calls a method the type checker proves is statically dispatched to a CLOSED method
// of a concrete type (no func value, interface or type-parameter call, field call or range-over-func
// reachable from it, and nothing outside the reliably type-checked tree).
var sanctioned = map[string]map[string]string{
	"testdata/negative": {
		"same_name_other_type.go:Repo.PureSameName": "Perms.HasPerm, same package, pure",
		"qualified.go:Repo.QualifiedPure":           "pure.QPerms.HasQ, other package, pure",
	},
	"testdata/rev4": {
		"collide.go:R11bCollideConcrete": "conc.T.Foo, other package, pure",
		"main.go:R10SliceTypeMethod":     "fx.L.Load, other package, pure",
	},
	"testdata/rev5": {
		"rev5.go:R39bClosedThroughHelper": "fx2.Pure.Load, closed through a static helper",
	},
	"testdata/rev6": {
		"rev6.go:R49CompleteDespiteUnresolvedField": "Deep.String, same package, pure; complete method set (control)",
	},
	"testdata/zrev": {
		"main.go:Z07GenericPure": "gn.Box[int].Load, other package, pure (generic origin)",
		"main.go:Z12DeferGo":     "Deep.String, same package, pure, called through go/defer",
	},
	"../..": {
		"repository/user_repository.go:UserRepository.adminRoleIDsTx": "models.Permissions.HasPermission, other package, pure",
	},
}

var corpora = []string{
	"testdata/positive", "testdata/negative", "testdata/shapes", "testdata/rev4", "testdata/rev4mod",
	"testdata/rev4/nm", "testdata/rev5", "testdata/rev5mod", "testdata/rev6", "testdata/zrev", "testdata/zrevmod", "testdata/zr2", "testdata/zr3", "../..",
}

// The type-aware head must flag exactly what the by-name base flags on every corpus (reviewer shapes
// S01-S23, T01-T15, R01-R34, Z01-Z12, N01-N12, own R35-R51, and the real tree), minus the sanctioned removals above.
// A head-only finding is not unsafe, but none is expected on these corpora, so it fails too: that keeps
// mutants that make the head follow the wrong decls visible.
func TestPoolClassGuard_NeverLessStrictThanByName(t *testing.T) {
	for _, root := range corpora {
		head, err := scan(root, false, false)
		require.NoError(t, err)
		base, err := scan(root, false, true)
		require.NoError(t, err)
		hk, bk := keysOf(head), keysOf(base)
		require.NotEmpty(t, bk, "%s: control needle: the base sees findings", root)
		for k := range sanctioned[root] {
			require.True(t, bk[k], "%s: control needle: the base flags the sanctioned member %s", root, k)
			require.False(t, hk[k], "%s: sanctioned removal regressed: %s", root, k)
		}
		for k := range bk {
			if _, ok := sanctioned[root][k]; ok {
				continue
			}
			require.True(t, hk[k], "%s: head misses a member that by-name resolution flags: %s", root, k)
		}
		for k := range hk {
			require.True(t, bk[k], "%s: unexpected head-only finding: %s", root, k)
		}
	}
}

// Every reviewer finding has a named, always-flagged fixture (the head must keep it).
func TestPoolClassGuard_ReviewerShapesStayFlagged(t *testing.T) {
	want := map[string][]string{
		"testdata/rev4": {"R01FuncFieldMethodValue", "R20FuncFieldPlainFunc", "R01bFuncFieldLocal", "R31HelperForwards",
			"R09NamedFuncTypeMethod", "R06GroupedIfaceSecondFile", "R07SamePkgNameOtherDir", "R08EmbedPtrToEmbedIface",
			"R14IfaceHoldsPtrRepo", "R12ParamShadowsImport", "R15GenericConstraint", "R28FieldChain", "R29BlockShadow",
			"R30TypeSwitch", "R11CollideIface", "R32TaggedIfaceFirst", "R33DefinedOverQualifiedIface",
			"R34RecvTypeParamShadow", "R02NestedModuleIface"},
		"testdata/rev4mod": {"R03ReplaceIface"},
		"testdata/rev4/nm": {"R02NestedModuleIface"},
		"testdata/rev5mod": {"R43OtherModuleSameImportPath"},
		"testdata/rev6": {"R44AliasOfUnresolved", "R45GenericInstantiation", "R46PtrDefinedOverUnresolved",
			"R47NamedOverIncompleteStruct", "R48ConstraintOverUnresolved", "R50DotFileIgnored", "R51IgnoreTaggedFile"},
		"testdata/zrev": {"Z01IfaceEmbedsUnresolved", "Z01bSamePkg", "Z02UnderscoreFileIgnored", "Z03TaggedOnlyDecl",
			"Z05DefinedOverUnresolved", "Z06DefinedIfaceOverUnresolved", "Z08GenericOpen", "Z11NamedFuncType"},
		"testdata/zrevmod": {"Z04NestedStaleCopy"},
		"testdata/zr2": {"N01AliasChain", "N02GenericEmbedding", "N03Recursive", "N04PtrToHolder", "N05MethodExpr",
			"N06WindowsSuffix", "N07UnreliablePkgIface", "N08Vendored", "N10GenericUnresolvedArg", "N11CgoFileOnlyDecl",
			"N12DotImport"},
		"testdata/zr3":  {"M1RawStringCgoImport", "M2CgoFileSortsLast", "M3EscapedCgoImport"},
		"testdata/rev5": {"R35TaggedStructFirst", "R36SamePkgExactReachesPooled", "R37InvalidEmbeddingShadow", "R38RangeOverFuncCallback", "R39OpenOneLevelDown", "R40DepthBound", "R42RedeclaredType"},
	}
	for root, fns := range want {
		found, err := Scan(root)
		require.NoError(t, err)
		got := map[string]bool{}
		for _, f := range found {
			got[f.Func] = true
		}
		for _, fn := range fns {
			require.True(t, got[fn], "%s: %s must stay flagged", root, fn)
		}
	}
	// The depth bound is the base's: one level deeper is flagged by neither.
	found, err := Scan("testdata/rev5")
	require.NoError(t, err)
	for _, f := range found {
		require.NotEqual(t, "R41BeyondBound", f.Func)
	}
}

// Cross mode is always by name: it must equal the base cross scan.
func TestPoolClassGuard_CrossModeIsByName(t *testing.T) {
	for _, root := range corpora {
		a, err := ScanCrossPackage(root)
		require.NoError(t, err)
		b, err := scan(root, true, true)
		require.NoError(t, err)
		require.Equal(t, keysOf(b), keysOf(a), root)
	}
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	ents, err := os.ReadDir(src)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dst, 0o755))
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dst, e.Name()), b, 0o644))
	}
}

// Real-code control needle: on a copy of the real models and repository packages, the removal of the
// adminRoleIDsTx false positive must not blind the scanner to a genuine pooled call (direct, or through
// a same-package DB-backed method) written right next to perms.HasPermission in the same rows loop.
func TestPoolClassGuard_RealCodeControlNeedle(t *testing.T) {
	const anchor = "if perms.HasPermission(models.PermissionSystemAdmin) {"
	const key = "repository/user_repository.go:UserRepository.adminRoleIDsTx"
	for _, tc := range []struct {
		name, inject string
		indirect     bool
	}{
		{"unchanged", "", false},
		{"direct", "_ = r.db.QueryRow(\"SELECT 1\")\n", false},
		{"same-package-method", "_, _ = r.HasPermission(nil, id, models.PermissionSystemAdmin)\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module catalogizer\n\ngo 1.25\n"), 0o644))
			copyDir(t, "../../models", filepath.Join(dir, "models"))
			copyDir(t, "../../repository", filepath.Join(dir, "repository"))
			p := filepath.Join(dir, "repository", "user_repository.go")
			b, err := os.ReadFile(p)
			require.NoError(t, err)
			require.Equal(t, 1, strings.Count(string(b), anchor), "anchor must exist exactly once")
			if tc.inject != "" {
				b = []byte(strings.Replace(string(b), anchor, tc.inject+anchor, 1))
				require.NoError(t, os.WriteFile(p, b, 0o644))
			}
			base, err := scan(dir, false, true)
			require.NoError(t, err)
			require.True(t, keysOf(base)[key], "control: the base flags the loop")
			head, err := Scan(dir)
			require.NoError(t, err)
			var hit []Finding
			for _, f := range head {
				if f.Key() == key {
					hit = append(hit, f)
				}
			}
			if tc.inject == "" {
				require.Empty(t, hit, "the pure models.Permissions.HasPermission must not be flagged")
				return
			}
			require.NotEmpty(t, hit, "the injected pooled call must be flagged")
			require.Equal(t, tc.indirect, hit[0].Indirect, hit[0].String())
		})
	}
}

// PGR3R-m2: gofmt rewrites an import path literal to its canonical interpreted form, so a repo-wide gofmt -w
// would turn these fixtures into `"C"` and silently drop the raw and escaped coverage while the findings
// test still passed. Pin the literal forms on disk.
func TestPoolClassGuard_CgoFixturesKeepTheirLiteralForm(t *testing.T) {
	for path, want := range map[string]string{
		"testdata/zr3/raw/z.go": "import `C`",
		"testdata/zr3/esc/z.go": `import "\x43"`,
	} {
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Contains(t, string(b), want, "%s was rewritten (gofmt -w?): the literal form it exists to cover is gone", path)
	}
}

func importsCOf(t *testing.T, lit string) bool {
	t.Helper()
	f := &ast.File{Imports: []*ast.ImportSpec{{Path: &ast.BasicLit{Kind: token.STRING, Value: lit}}}}
	return importsC(f)
}

// PGR3R-m1 / PGR3R-n3: the import-path decision is made on the unquoted value; a literal that cannot be
// unquoted is unreliable and is treated as cgo (the file may be dropped by the go tool), never as plain.
func TestPoolClassGuard_ImportsCUnquoting(t *testing.T) {
	require.True(t, importsCOf(t, `"C"`))
	require.True(t, importsCOf(t, "`C`"))
	require.True(t, importsCOf(t, `"\x43"`), "escaped interpreted literal")
	require.True(t, importsCOf(t, `"C"`), "unicode-escaped literal")
	require.False(t, importsCOf(t, `"D"`))
	require.False(t, importsCOf(t, `"\x44"`))
	require.False(t, importsCOf(t, `"database/sql"`))
	require.True(t, importsCOf(t, `"C`), "unterminated literal: unreliable, treated as cgo")
	require.True(t, importsCOf(t, `"\q"`), "invalid escape: unreliable, treated as cgo")
	require.False(t, importsC(&ast.File{Imports: []*ast.ImportSpec{{}}}), "nil Path is skipped")
}
