"""Tests of check_pdf_coverage.py, render_md_to_pdf.sh, check_twin_presence.py. Run: python3 -m pytest -p no:cacheprovider scripts/docs/tests

Real files and real tools (pandoc, weasyprint, pdftotext, git); no mocks. Oracles are exit codes and the FAIL/MISSING rows.
"""
import os, re, shutil, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
D = os.path.join(HERE, "..")
COV = os.path.join(D, "check_pdf_coverage.py")
PDF = os.path.join(D, "render_md_to_pdf.sh")
TWIN = os.path.join(D, "check_twin_presence.py")

WIDE = "# Wide\n\n" + "| a | b | c | d | e | f |\n|---|---|---|---|---|---|\n" + "".join(
    "| row%d_alphaTOKEN | https://example.org/very/long/path/segment%d/and/more | gamma%dvalue | delta%dvalue | eps%dvalue | zeta%dvalue |\n" % ((i,) * 6) for i in range(60))


def sh(*a, **k):
    return subprocess.run(list(a), capture_output=True, text=True, **k)


def make(tmp, text=WIDE, name="w"):
    md = os.path.join(str(tmp), name + ".md")
    open(md, "w").write(text)
    r = sh(PDF, md)
    assert r.returncode == 0, r.stderr
    return md


def test_renderer_output_has_full_coverage(tmp_path):
    md = make(tmp_path)
    r = sh(sys.executable, COV, md)
    assert r.returncode == 0, r.stdout
    assert "0/" in r.stdout


def test_mutation_no_table_css_fails(tmp_path):
    md = make(tmp_path)
    html = str(tmp_path / "p.html")
    sh("pandoc", "-f", "gfm", "-t", "html5", "-s", "--metadata", "pagetitle=x", md, "-o", html)
    # default print rules (pandoc body max-width, no print.css): the class the renderer fixes
    assert sh("weasyprint", "-q", html, md[:-3] + ".pdf").returncode == 0
    r = sh(sys.executable, COV, md)
    assert r.returncode == 1, r.stdout


def test_mutation_truncated_pdf_fails(tmp_path):
    md = make(tmp_path)
    pdf = md[:-3] + ".pdf"
    data = open(pdf, "rb").read()
    open(pdf, "wb").write(data[: len(data) // 2])
    assert sh(sys.executable, COV, md).returncode == 1


def test_mutation_broken_needle_is_blind(tmp_path):
    md = make(tmp_path)
    src = open(COV).read()
    broken = str(tmp_path / "cov_broken.py")
    # needle control broken: the "fabricated" needle is a token that IS present in the PDF
    open(broken, "w").write(src.replace("CTRL_FAKE = 'zqxjvkwfabricatedneedle'", "CTRL_FAKE = 'alphaTOKEN'"))
    assert sh(sys.executable, broken, md).returncode == 2


def test_absent_pdf_fails(tmp_path):
    md = str(tmp_path / "n.md")
    open(md, "w").write("# T\n\nsomething_long_token here\n")
    assert sh(sys.executable, COV, md).returncode == 1


def git(cwd, *a):
    return sh("git", "-C", str(cwd), "-c", "user.name=t", "-c", "user.email=t@t", *a)


def test_twin_presence_pass_then_mutation(tmp_path):
    root = tmp_path
    git(root, "init", "-q")
    os.makedirs(root / "scripts/docs"); os.makedirs(root / "docs")
    for f in ("render_md_to_html.sh", "print.css", "print_landscape.css"):
        shutil.copy(os.path.join(D, f), root / "scripts/docs" / f)
    open(root / "docs/base.md", "w").write("# base\n")
    git(root, "add", "-A"); git(root, "commit", "-qm", "base"); git(root, "tag", "v0")
    open(root / "docs/new.md", "w").write("# New\n\nbody text here\n")
    assert sh(str(root / "scripts/docs/render_md_to_html.sh"), str(root / "docs/new.md")).returncode == 0
    assert sh(str(root / "scripts/docs/render_md_to_pdf.sh") if False else PDF, str(root / "docs/new.md")).returncode == 0
    git(root, "add", "-A"); git(root, "commit", "-qm", "new")
    ok = sh(sys.executable, TWIN, "--root", str(root), "--since", "v0")
    assert ok.returncode == 0, ok.stdout
    os.remove(root / "docs/new.pdf")  # mutation: delete one twin
    bad = sh(sys.executable, TWIN, "--root", str(root), "--since", "v0")
    assert bad.returncode == 1 and "new.md" in bad.stdout
    os.remove(root / "docs/new.html")
    bad2 = sh(sys.executable, TWIN, "--root", str(root), "--since", "v0")
    assert bad2.returncode == 1


def test_twin_presence_empty_scope_is_blind(tmp_path):
    root = tmp_path
    git(root, "init", "-q"); open(root / "a", "w").write("x")
    git(root, "add", "-A"); git(root, "commit", "-qm", "b"); git(root, "tag", "v0")
    assert sh(sys.executable, TWIN, "--root", str(root), "--since", "v0").returncode == 2
