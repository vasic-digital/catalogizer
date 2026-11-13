"""Tests of check_pdf_coverage.py, render_md_to_pdf.sh, check_twin_presence.py. Run: python3 -m pytest -p no:cacheprovider scripts/docs/tests

Real files and real tools (pandoc, weasyprint, pdftotext, git); no mocks. Oracles are exit codes and the FAIL/MISSING rows.
"""
import os, re, shutil, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
D = os.environ.get("DOCS_SUT_DIR") or os.path.join(HERE, "..")  # mutation runs point this at a mutated copy
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
    open(root / "docs/new.md", "w").write("# New\n\nbody text here with enough_longer tokens present\n")
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


def bbox(pdf):
    """(max word xMax over all pages, page width) from pdftotext -bbox."""
    out = sh("pdftotext", "-bbox", pdf, "-").stdout
    pw = float(re.search(r'<page width="([\d.]+)"', out).group(1))
    mx = max(float(x) for x in re.findall(r'xMax="([\d.]+)"', out))
    return mx, pw


def page_size(pdf):
    m = re.search(r"Page size:\s+([\d.]+) x ([\d.]+)", sh("pdfinfo", pdf).stdout)
    return float(m.group(1)), float(m.group(2))


LONGCODE = "# Code\n\n```\n/p/" + "seg" * 120 + "/TAILMARKERKK\n```\n\n" + ("A reasonably long paragraph of ordinary words to fill the line. " * 12) + "\n"


def test_long_code_line_not_clipped_and_body_width_used(tmp_path):
    md = make(tmp_path, LONGCODE, "c")
    pdf = md[:-3] + ".pdf"
    mx, pw = bbox(pdf)
    assert mx <= pw - 20, "text beyond the page content box: clipped"       # nothing past the right margin
    assert mx >= pw * 0.85, "body does not use the page width (pandoc body max-width still applies)"
    r = sh(sys.executable, COV, md)
    assert r.returncode == 0, r.stdout


def test_wide_table_uses_landscape_width(tmp_path):
    md = make(tmp_path)
    pdf = md[:-3] + ".pdf"
    w, h = page_size(pdf)
    assert w > h
    mx, pw = bbox(pdf)
    assert mx >= pw * 0.85 and mx <= pw - 20


def test_landscape_threshold_five_columns(tmp_path):
    five = make(tmp_path, "# T\n\n| a | b | c | d | e |\n|---|---|---|---|---|\n| 1 | 2 | 3 | 4 | 5 |\n", "five")
    four = make(tmp_path, "# T\n\n| a | b | c | d |\n|---|---|---|---|\n| 1 | 2 | 3 | 4 |\n", "four")
    w, h = page_size(five[:-3] + ".pdf"); assert w > h
    w, h = page_size(four[:-3] + ".pdf"); assert w < h


def test_long_cell_url_not_clipped(tmp_path):
    t = "# T\n\n| a | b |\n|---|---|\n| x | https://example.org/" + "q" * 140 + "/ENDMARK |\n"
    md = make(tmp_path, t, "u")
    mx, pw = bbox(md[:-3] + ".pdf")
    assert mx <= pw - 20
    assert sh(sys.executable, COV, md).returncode == 0


def test_lost_cell_not_matched_by_gluing_neighbours(tmp_path):
    # R2-2: the PDF is rendered from a doc whose cells are 'gate' | 'alpha' and 'Con' | 'nection'; the Markdown under test claims the
    # words 'gatealpha' and 'Connection'. Whitespace-deleting matchers found both; the word-boundary matcher must not.
    real = make(tmp_path, "# T\n\n| x | y |\n|---|---|\n| gate | alpha |\n| Con | nection |\n", "g")
    claim = str(tmp_path / "claim.md")
    open(claim, "w").write("# T\n\n| x | y |\n|---|---|\n| gatealpha | Connection |\n")
    shutil.copy(real[:-3] + ".pdf", claim[:-3] + ".pdf")
    r = sh(sys.executable, COV, claim)
    assert r.returncode == 1 and "Connection" in r.stdout and "gatealpha" in r.stdout, r.stdout


def test_wrapped_long_token_still_found(tmp_path):
    md = make(tmp_path, "# T\n\n| a | b |\n|---|---|\n| x | sha256:" + "0123456789abcdef" * 6 + " |\n", "h")
    assert sh(sys.executable, COV, md).returncode == 0


def test_markdown_side_needle_blind(tmp_path):
    md = make(tmp_path)
    src = open(COV).read()
    broken = str(tmp_path / "cov_b2.py")
    open(broken, "w").write(src.replace("CTRL_FAKE = 'zqxjvkwfabricatedneedle'", "CTRL_FAKE = 'row0_alphaTOKEN'"))
    assert sh(sys.executable, broken, md).returncode == 2


def _repo(root):
    git(root, "init", "-q")
    os.makedirs(root / "scripts/docs"); os.makedirs(root / "docs")
    for f in ("render_md_to_html.sh", "print.css", "print_landscape.css"):
        shutil.copy(os.path.join(D, f), root / "scripts/docs" / f)
    open(root / "docs/base.md", "w").write("# base\n")
    git(root, "add", "-A"); git(root, "commit", "-qm", "base"); git(root, "tag", "v0")


def _twins(root, name, text):
    md = str(root / "docs" / (name + ".md"))
    open(md, "w").write(text)
    assert sh(os.path.join(D, "render_md_to_html.sh"), md).returncode == 0
    assert sh(PDF, md).returncode == 0
    return md


def test_twin_garbage_pdf_fails(tmp_path):
    _repo(tmp_path)
    md = _twins(tmp_path, "n", "# New\n\nbody text here with enough_longer tokens\n")
    open(md[:-3] + ".pdf", "wb").write(b"%PDF garbage")
    git(tmp_path, "add", "-A"); git(tmp_path, "commit", "-qm", "n")
    r = sh(sys.executable, TWIN, "--root", str(tmp_path), "--since", "v0")
    assert r.returncode == 1 and "n.md" in r.stdout, r.stdout


def test_twin_stale_pdf_and_html_fail_for_modified_doc(tmp_path):
    _repo(tmp_path)
    md = _twins(tmp_path, "m", "# M\n\nfirst version of the body\n")
    git(tmp_path, "add", "-A"); git(tmp_path, "commit", "-qm", "m"); git(tmp_path, "tag", "v1")
    open(md, "a").write("\nsecond paragraph added later\n")   # modified after the tag, twins not regenerated
    git(tmp_path, "add", "-A"); git(tmp_path, "commit", "-qm", "edit")
    r = sh(sys.executable, TWIN, "--root", str(tmp_path), "--since", "v1")
    assert r.returncode == 1 and "stale" in r.stdout and "drifts" in r.stdout, r.stdout
    assert sh(os.path.join(D, "render_md_to_html.sh"), md).returncode == 0 and sh(PDF, md).returncode == 0
    git(tmp_path, "add", "-A"); git(tmp_path, "commit", "-qm", "regen")
    assert sh(sys.executable, TWIN, "--root", str(tmp_path), "--since", "v1").returncode == 0


def _load_gen():
    import importlib.util
    spec = importlib.util.spec_from_file_location("gdi", os.path.join(D, "gen_dir_indexes.py"))
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
    return m


def test_relink_rewrites_target_and_equal_text(tmp_path=None):
    g = _load_gen()
    assert g.relink("see [../a/b.md](../a/b.md) and [x](c.md) and [u](https://e.org/x) and [h](#f)", "sub") == \
        "see [a/b.md](a/b.md) and [x](sub/c.md) and [u](https://e.org/x) and [h](#f)"


def test_body_font_size_rule_applies(tmp_path):
    # R2-4 (M1): the body rule of print.css sets 9.5pt; without it (or with pandoc's own CSS outranking it) paragraph text is larger
    md = make(tmp_path, LONGCODE, "f")
    out = sh("pdftotext", "-bbox", md[:-3] + ".pdf", "-").stdout
    hs = [float(b) - float(a) for a, b in re.findall(r'yMin="([\d.]+)" xMax="[\d.]+" yMax="([\d.]+)"', out)]
    para = sorted(hs)[len(hs) // 2]
    assert para < 14.5, "median word height %.1f: body font-size rule not applied" % para


def words(pdf):
    """[(page, xMin, xMax, yMin, text)] from pdftotext -bbox."""
    out = sh("pdftotext", "-bbox", pdf, "-").stdout
    res = []
    for pg, body in enumerate(out.split("<page ")[1:], 1):
        for xa, ya, xb, yb, t in re.findall(r'<word xMin="([\d.]+)" yMin="([\d.]+)" xMax="([\d.]+)" yMax="([\d.]+)">(.*?)</word>', body):
            res.append((pg, float(xa), float(xb), float(ya), t))
    return res


def test_table_columns_sized_from_content(tmp_path):
    # R2-4 (MD): a 1-character id column must be narrow: the second column starts near the left edge (equal widths would put it at >= 1/6 of the page)
    rows = "".join("| %d | alpha%d | beta | gamma | delta | epsilon |\n" % (i, i) for i in range(5))
    md = make(tmp_path, "# T\n\n| i | name | b | c | d | e |\n|---|---|---|---|---|---|\n" + rows, "cols")
    ws = words(md[:-3] + ".pdf")
    pw = bbox(md[:-3] + ".pdf")[1]
    x = min(w[1] for w in ws if w[4].startswith("alpha"))
    assert x < pw * 0.15, "second column starts at %.0f of %.0f: widths not computed from content" % (x, pw)


def test_table_rows_not_split_across_pages(tmp_path):
    # R2-4 (MC): every row has a start and an end marker 25 lines apart; a row split across a page break puts them on different pages
    rows = "".join("| START%02d %s END%02d | x |\n" % (i, "filler words to make the cell tall " * 25, i) for i in range(30))
    md = make(tmp_path, "# T\n\n| a | b |\n|---|---|\n" + rows, "rows")
    ws = words(md[:-3] + ".pdf")
    for i in range(30):
        s = [w[0] for w in ws if w[4] == "START%02d" % i]; e = [w[0] for w in ws if w[4] == "END%02d" % i]
        assert s and e and s[0] == e[0], "row %d split across pages" % i


def test_cell_and_code_wrap_independent_of_body_inheritance(tmp_path):
    # R2-4 (MB, MF): long unbroken words in a cell and in a code block both stay on the page even though several rules overlap
    t = "# T\n\n| a | b |\n|---|---|\n| x | " + "w" * 400 + "CELLEND |\n\n```\n" + "c" * 400 + "CODEEND\n```\n"
    md = make(tmp_path, t, "wrap")
    ws = words(md[:-3] + ".pdf")
    pw = bbox(md[:-3] + ".pdf")[1]
    assert max(w[2] for w in ws) <= pw - 20
    joined = "".join(w[4] for w in ws)
    assert "CELLEND" in joined and "CODEEND" in joined
