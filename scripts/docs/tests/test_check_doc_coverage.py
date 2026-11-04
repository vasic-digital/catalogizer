"""Tests of scripts/docs/check_doc_coverage.py. Run: python3 -m pytest -p no:cacheprovider scripts/docs/tests

Oracles: exit codes and GAP rows. Fixtures are real files in a temp git repository (no mocks).
Every check has a deliberate-gap test that MUST fail with the named GAP row, and a clean fixture that MUST pass.
The live test runs the checker over this repository and expects 0 gaps.
"""
import os
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
CHECK = os.path.join(HERE, "..", "check_doc_coverage.py")
REPO = os.path.abspath(os.path.join(HERE, "..", "..", ".."))

MAIN_GO = '''package main
func main() {
	router.GET("/health", h)
	api := router.Group("/api/v1")
	api.GET("/a/:id", h)
	g := api.Group("/g")
	root_handlers.RegisterThingRoutes(g, x)
	router.GET("/debug/pprof/heap", h)
}
'''
HELPER_GO = '''package handlers
func RegisterThingRoutes(grp *gin.RouterGroup, h *H) {
	grp.POST("", h.Create)
	grp.GET("/:job_id", h.Get)
}
'''
SPEC = '''openapi: 3.0.0
paths:
  /health: {get: {}}
  /api/v1/a/{id}: {get: {}}
  /api/v1/g: {post: {}}
  /api/v1/g/{job_id}: {get: {}}
'''
ENV_GO = 'package x\nfunc f() { _ = os.Getenv("FOO_BAR_BAZ"); os.Setenv("IGNORED_SET", "1") }\n'


def fixture():
    return {
        "catalog-api/main.go": MAIN_GO,
        "catalog-api/handlers/thing_routes.go": HELPER_GO,
        "catalog-api/x.go": ENV_GO,
        "docs/api/openapi.yaml": SPEC,
        "docs/deployment/ENVIRONMENT_VARIABLES.md": "| `FOO_BAR_BAZ` | x |\n",
        "docs/ENV_VARIABLES.md": "# env\n",
        "scripts/run.sh": "#!/bin/sh\n",
        "docs/scripts/run.md": "# run\n",
        "README.md": "[Docs](docs/README.md)\n",
        "docs/README.md": "# index\n",
    }


def build(files):
    d = tempfile.mkdtemp(prefix="dcov.")
    for rel, txt in files.items():
        p = os.path.join(d, rel)
        os.makedirs(os.path.dirname(p), exist_ok=True)
        with open(p, "w") as f:
            f.write(txt)
    subprocess.run(["git", "init", "-q"], cwd=d, check=True)
    subprocess.run(["git", "add", "-A"], cwd=d, check=True)
    return d


def run(root, *args):
    r = subprocess.run([sys.executable, CHECK, "--root", root, "--exempt", os.devnull] + list(args),
                       capture_output=True, text=True)
    return r.returncode, r.stdout


def with_fixture(mutate, *args):
    files = fixture()
    mutate(files)
    d = build(files)
    try:
        return run(d, "--check", "routes", "--check", "env", "--check", "scripts", "--check", "readme", *args)
    finally:
        shutil.rmtree(d)


def test_clean_fixture_passes_including_helper_routes_and_pprof_exclusion():
    rc, out = with_fixture(lambda f: None)
    assert rc == 0, out
    assert "check routes: 4 items, 0 gaps" in out  # helper-registered routes counted, pprof excluded


def test_undocumented_route_is_a_gap():
    rc, out = with_fixture(lambda f: f.__setitem__("catalog-api/main.go", MAIN_GO.replace('router.GET("/health", h)', 'router.GET("/health", h)\n\trouter.GET("/new", h)')))
    assert rc == 1 and "UNDOCUMENTED\tGET /new" in out, out


def test_phantom_spec_route_is_a_gap():
    rc, out = with_fixture(lambda f: f.__setitem__("docs/api/openapi.yaml", SPEC + "  /ghost: {get: {}}\n"))
    assert rc == 1 and "PHANTOM\tGET /ghost" in out, out


def test_helper_route_missing_from_spec_is_a_gap():
    rc, out = with_fixture(lambda f: f.__setitem__("docs/api/openapi.yaml", SPEC.replace("  /api/v1/g: {post: {}}\n", "")))
    assert rc == 1 and "UNDOCUMENTED\tPOST /api/v1/g" in out, out


def test_undocumented_env_is_a_gap_and_setenv_is_ignored():
    rc, out = with_fixture(lambda f: f.__setitem__("docs/deployment/ENVIRONMENT_VARIABLES.md", "nothing\n"))
    assert rc == 1 and "UNDOCUMENTED_ENV\tFOO_BAR_BAZ" in out and "IGNORED_SET" not in out, out


def test_env_name_must_match_whole_word():
    rc, out = with_fixture(lambda f: f.__setitem__("docs/ENV_VARIABLES.md", "XFOO_BAR_BAZY\n") or f.__setitem__("docs/deployment/ENVIRONMENT_VARIABLES.md", "x\n"))
    assert rc == 1 and "FOO_BAR_BAZ" in out, out


def test_script_without_guide_is_a_gap():
    rc, out = with_fixture(lambda f: f.__setitem__("scripts/other-tool.sh", "#!/bin/sh\n"))
    assert rc == 1 and "NO_SCRIPT_GUIDE\tscripts/other-tool.sh" in out, out


def test_script_guide_name_normalisation_accepts_hyphen_underscore():
    rc, out = with_fixture(lambda f: (f.__setitem__("scripts/my-tool.sh", "x"), f.__setitem__("docs/scripts/my_tool.md", "x")))
    assert rc == 0, out


def test_readme_without_index_link_is_a_gap():
    rc, out = with_fixture(lambda f: f.__setitem__("README.md", "no link\n"))
    assert rc == 1 and "README_NO_DOCS_INDEX_LINK" in out, out


def test_exempt_file_silences_only_the_listed_item_and_needs_a_reason():
    files = fixture()
    files["scripts/other-tool.sh"] = "x"
    d = build(files)
    try:
        ex = os.path.join(d, "ex.tsv")
        open(ex, "w").write("script\tother-tool.sh\tlegacy\n")
        r = subprocess.run([sys.executable, CHECK, "--root", d, "--exempt", ex, "--check", "scripts"], capture_output=True, text=True)
        assert r.returncode == 0, r.stdout
        open(ex, "w").write("script\tother-tool.sh\n")
        r = subprocess.run([sys.executable, CHECK, "--root", d, "--exempt", ex, "--check", "scripts"], capture_output=True, text=True)
        assert r.returncode == 2
    finally:
        shutil.rmtree(d)


def test_empty_extraction_is_blind_not_clean():
    rc, out = with_fixture(lambda f: f.__setitem__("docs/api/openapi.yaml", "openapi: 3.0.0\npaths: {}\n"))
    assert rc == 3 and "BLIND" in out, out


def test_claims_check_detects_missing_doc_statement_and_missing_code():
    # copy the real claim files, then break one doc statement and one code anchor
    claim_files = ["docs/UPGRADE_NOTES.md", "catalog-api/handlers/auth_handler.go", "docs/deployment/ENVIRONMENT_VARIABLES.md",
                   "docs/api/API_DOCUMENTATION.md", "catalog-api/handlers/playlist_handler.go",
                   "catalog-web/src/components/collections/CollectionSharing.tsx", "docs/deployment/DOCKER_SETUP.md",
                   "catalog-api/Dockerfile.runtime", "docs/api/openapi.yaml", "catalog-api/internal/metrics/health.go", "catalog-api/services/pdf_page_render.go"]
    files = {f: open(os.path.join(REPO, f), encoding="utf-8", errors="replace").read() for f in claim_files}
    d = build(files)
    try:
        rc, out = run(d, "--check", "claims")
        assert rc == 0, out
        shutil.rmtree(d)
        files["docs/UPGRADE_NOTES.md"] = files["docs/UPGRADE_NOTES.md"].replace("go-fitz", "gofitz")
        files["catalog-api/Dockerfile.runtime"] = files["catalog-api/Dockerfile.runtime"].replace("ffmpeg imagemagick", "")
        d = build(files)
        rc, out = run(d, "--check", "claims")
        assert rc == 1 and "CLAIM_NOT_IN_DOC\tgo-fitz-removed-poppler-renders" in out and "CLAIM_NOT_IN_CODE\tconverters-in-image" in out, out
    finally:
        shutil.rmtree(d, ignore_errors=True)


def test_live_repository_has_no_gap():
    rc, out = subprocess.run([sys.executable, CHECK, "--root", REPO], capture_output=True, text=True).returncode, None
    assert rc == 0


if __name__ == "__main__":
    # no pytest on some hosts: run every test_* function and report
    failed = 0
    for name, fn in sorted(globals().items()):
        if name.startswith("test_") and callable(fn):
            try:
                fn()
                print("PASS", name)
            except AssertionError as e:
                failed += 1
                print("FAIL", name, str(e)[:300])
    print("failed: %d" % failed)
    sys.exit(1 if failed else 0)
