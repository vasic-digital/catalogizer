"""Tests of check_pdf_coverage.py, render_md_to_pdf.sh, check_twin_presence.py. Run: scripts/docs/tests/run_tests.sh scripts/docs/tests/test_check_pdf_coverage.py (stdlib runner, no pytest needed; pytest also works)

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


# ---- round 4 -------------------------------------------------------------------------------------------------------------------------

def _wordtexts(pdf):
    return [w[4] for w in words(pdf)]


def test_short_ids_stay_on_one_line_landscape(tmp_path):
    # F1: ids in a narrow column of a wide table must not wrap ("B10"/"6")
    ids = ["B%d" % (100 + i) for i in range(20)]
    prose = "some quite long explanatory prose for this cell " * 4
    rows = "".join("| %s | %s:%d | %s | %s | %s | %s | %s |\n" % (i, "media_entity_handler.go", 1100 + n, prose, prose, prose, prose, prose) for n, i in enumerate(ids))
    md = make(tmp_path, "# T\n\n| id | loc | a | b | c | d | e |\n|---|---|---|---|---|---|---|\n" + rows, "ids")
    pdf = md[:-3] + ".pdf"
    assert page_size(pdf)[0] > page_size(pdf)[1]
    ws = _wordtexts(pdf)
    for i in ids: assert i in ws, "%s broken across lines" % i
    assert any(w.endswith("go:1100") for w in ws) and any(w.endswith("go:1119") for w in ws), "file:line citation split"
    assert sh(sys.executable, COV, md).returncode == 0


def test_short_ids_stay_on_one_line_portrait(tmp_path):
    prose = "some quite long explanatory prose for this cell " * 3
    rows = "".join("| PCELL%dKK | %s | %s | %s |\n" % (n, prose, prose, prose) for n in range(6))
    md = make(tmp_path, "# T\n\n| id | a | b | c |\n|---|---|---|---|\n" + rows, "pids")
    pdf = md[:-3] + ".pdf"
    assert page_size(pdf)[0] < page_size(pdf)[1]
    ws = _wordtexts(pdf)
    for n in range(6): assert "PCELL%dKK" % n in ws
    assert sh(sys.executable, COV, md).returncode == 0


def test_gate_catches_split_id(tmp_path):
    # the F1 class: a PDF whose ids are split must FAIL the default gate (tokens of 4 characters, ids with digits)
    md = make(tmp_path, "# T\n\n| id | text |\n|---|---|\n| B1006 | " + "word " * 5 + " |\n", "sp")
    # build a PDF from a doc where the id is broken across two lines
    broken = str(tmp_path / "brk.md")
    open(broken, "w").write("# T\n\n| id | text |\n|---|---|\n| B100 6 | " + "word " * 5 + " |\n")
    assert sh(PDF, broken).returncode == 0
    claim = str(tmp_path / "claim.md"); shutil.copy(md, claim)
    shutil.copy(broken[:-3] + ".pdf", claim[:-3] + ".pdf")
    r = sh(sys.executable, COV, claim)
    assert r.returncode == 1 and "B1006" in r.stdout, r.stdout


def test_cross_cell_glue_rejected_for_long_words(tmp_path):
    # M-a: two neighbouring cells must not be accepted as one 26-character word
    real = make(tmp_path, "# T\n\n| x | y |\n|---|---|\n| Abcdefghijkl | mnopqrstuvwxyz |\n", "xc")
    claim = str(tmp_path / "xclaim.md")
    open(claim, "w").write("# T\n\n| x |\n|---|\n| Abcdefghijklmnopqrstuvwxyz |\n")
    shutil.copy(real[:-3] + ".pdf", claim[:-3] + ".pdf")
    r = sh(sys.executable, COV, claim)
    assert r.returncode == 1 and "Abcdefghijklmnopqrstuvwxyz" in r.stdout, r.stdout


def test_non_latin_tokens_checked(tmp_path):
    txt = "# Тест\n\nДокументация проекта содержит подробное описание архитектуры системы.\n"
    md = make(tmp_path, txt, "ru")
    r = sh(sys.executable, COV, md)
    assert r.returncode == 0 and "0/" in r.stdout and "nothing to compare" not in r.stdout, r.stdout
    claim = str(tmp_path / "ruc.md")
    open(claim, "w").write(txt + "\nПропавшееслово отсутствует.\n")
    shutil.copy(md[:-3] + ".pdf", claim[:-3] + ".pdf")
    r = sh(sys.executable, COV, claim)
    assert r.returncode == 1 and "Пропавшееслово" in r.stdout, r.stdout


def test_multi_file_blank_pdf_second_file_fails(tmp_path):
    a = make(tmp_path, WIDE, "a")
    b = str(tmp_path / "b.md"); open(b, "w").write("# T\n\nPlenty of ordinary readable words here, enough tokens.\n")
    blank = str(tmp_path / "blank.html"); open(blank, "w").write("<html><body></body></html>")
    assert sh("weasyprint", "-q", blank, b[:-3] + ".pdf").returncode == 0
    r = sh(sys.executable, COV, a, b)
    assert r.returncode == 1 and "blank" in r.stdout and "OK   " in r.stdout.split("\n")[0], r.stdout


def test_code_span_tokens_no_blind(tmp_path):
    md = make(tmp_path, "# T\n\nThe `RunOption`s and `alpha_beta_gamma` and `a b c` appear here with `zzz`.\n", "cs")
    r = sh(sys.executable, COV, md)
    assert r.returncode == 0, r.stdout


def _load_cov():
    import importlib.util
    spec = importlib.util.spec_from_file_location("cov", COV)
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
    return m


def test_md_side_control_blind_independent_of_fake_value(tmp_path):
    m = _load_cov()
    md = make(tmp_path)
    real = m.present
    m.present = lambda tok, W: False if isinstance(W, str) and tok != m.CTRL_FAKE else real(tok, W)   # Markdown side cannot find its own token
    assert m.main([md]) == 2


def test_pdf_side_control_blind_independent_of_fake_value(tmp_path):
    m = _load_cov()
    md = make(tmp_path)
    real = m.present
    m.present = lambda tok, W: True if tok == m.CTRL_FAKE and not isinstance(W, str) else real(tok, W)   # PDF side "finds" the fabricated token
    assert m.main([md]) == 2


def _forge_hash(pdf, old_md_text, new_hash, tmp):
    # render the OLD content but stamp it with the NEW markdown's hash (what a forger would do)
    src = os.path.join(tmp, "forge.md"); open(src, "w").write(old_md_text)
    html = os.path.join(tmp, "forge.html")
    assert sh("pandoc", "-f", "gfm", "-t", "html5", "-s", "--metadata", "pagetitle=F", "--metadata", "lang=en",
              "--metadata", "keywords=src-sha256:" + new_hash, src, "-o", html).returncode == 0
    assert sh("weasyprint", "-q", html, pdf).returncode == 0


def test_twin_forged_hash_old_content_fails_coverage(tmp_path):
    import hashlib
    _repo(tmp_path)
    md = _twins(tmp_path, "f", "# F\n\nfirst version with original_content tokens\n")
    git(tmp_path, "add", "-A"); git(tmp_path, "commit", "-qm", "f"); git(tmp_path, "tag", "v1")
    old = hashlib.sha256(open(md, "rb").read()).hexdigest()
    open(md, "w").write("# F\n\nsecond version with completely_different tokens added\n")
    new = hashlib.sha256(open(md, "rb").read()).hexdigest()
    assert sh(os.path.join(D, "render_md_to_html.sh"), md).returncode == 0     # html honest; pdf forged
    _forge_hash(md[:-3] + ".pdf", "# F\n\nfirst version with original_content tokens\n", new, str(tmp_path))
    git(tmp_path, "add", "-A"); git(tmp_path, "commit", "-qm", "edit")
    r = sh(sys.executable, TWIN, "--root", str(tmp_path), "--since", "v1")
    assert r.returncode == 1 and "coverage" in r.stdout, r.stdout


def test_twin_untracked_new_doc_without_twins_fails(tmp_path):
    _repo(tmp_path)
    open(tmp_path / "docs/u.md", "w").write("# U\n\nuntracked doc without twins\n")
    r = sh(sys.executable, TWIN, "--root", str(tmp_path), "--since", "v0")
    assert r.returncode == 1 and "u.md" in r.stdout, r.stdout


def test_render_is_reproducible(tmp_path):
    md = make(tmp_path, WIDE, "r")
    a = open(md[:-3] + ".pdf", "rb").read()
    assert sh(PDF, md).returncode == 0
    assert a == open(md[:-3] + ".pdf", "rb").read(), "two renders differ"


def test_short_tokens_are_checked(tmp_path):
    # default min-len 4 and the numeric pass: a missing 4-character word and a missing 2-character id both FAIL
    real = make(tmp_path, "# T\n\nOrdinary readable sentence with several plain words in it.\n", "sh")
    claim = str(tmp_path / "shc.md")
    open(claim, "w").write("# T\n\nOrdinary readable sentence with several plain words in it. Zqxw B7\n")
    shutil.copy(real[:-3] + ".pdf", claim[:-3] + ".pdf")
    r = sh(sys.executable, COV, claim)
    assert r.returncode == 1 and "Zqxw" in r.stdout and "B7" in r.stdout, r.stdout


# ---- round 5: structural wrap rules -----------------------------------------------------------------------------------------------------

CITES = "".join("| B%d | catalog-api/handlers/very_long_collection_handler_%d.go:%d | GetEntityDuplicatesAndMore | http.StatusInternalServerError | `err` | LEAK |\n" % (i, i, 100 + i * 7) for i in range(30))
CITEDOC = "# Cites\n\n| id | file:line | fn | status | expr | class |\n|---|---|---|---|---|---|\n" + CITES


def _bad_pdf(tmp, md_text, name, css):
    """A PDF of md_text rendered WITHOUT the renderer's break rules (anywhere-breaking, narrow fixed columns): what a regression would produce."""
    md = os.path.join(str(tmp), name + ".md"); open(md, "w").write(md_text)
    html = os.path.join(str(tmp), name + ".html"); cssf = os.path.join(str(tmp), name + ".css"); open(cssf, "w").write(css)
    assert sh("pandoc", "-f", "gfm", "-t", "html5", "-s", "--metadata", "pagetitle=x", "--metadata", "document-css=false", md, "-o", html).returncode == 0
    assert sh("weasyprint", "-q", "-s", cssf, html, md[:-3] + ".pdf").returncode == 0
    return md


BADCSS = "@page{size:A4 landscape;margin:10mm} table{table-layout:fixed;width:100%;font-size:7.5pt} td,th{overflow-wrap:anywhere;word-break:break-all;padding:2px}"
ORACLE = os.path.join(HERE, "oracle_citations.py")


def test_real_render_never_splits_a_citation_or_digit_run(tmp_path):
    md = make(tmp_path, CITEDOC, "cites")
    ws = _wordtexts(md[:-3] + ".pdf")
    for i in range(30):
        assert any(w.endswith("go:%d" % (100 + i * 7)) for w in ws), "citation %d split" % i
        assert "B%d" % i in ws
    assert sh(sys.executable, COV, md).returncode == 0
    assert sh(sys.executable, ORACLE, md).returncode == 0


def test_gate_and_oracle_reject_vertical_split_digit_run(tmp_path):
    md = _bad_pdf(tmp_path, CITEDOC, "bad", BADCSS)
    ws = _wordtexts(md[:-3] + ".pdf")
    assert not all(any(w.endswith("go:%d" % (100 + i * 7)) for w in ws) for i in range(30)), "fixture did not split anything"
    r = sh(sys.executable, COV, md)
    assert r.returncode == 1, r.stdout
    assert sh(sys.executable, ORACLE, md).returncode == 1


def test_short_token_does_not_inherit_wrap_permission_from_long_word(tmp_path):
    # the enclosing Markdown word is long; its digit run must still not be allowed to split
    doc = "# T\n\n| c |\n|---|\n" + "".join("| %d%d%d.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa |\n" % ((1000 + i * 111,) * 3) for i in range(40))
    md = _bad_pdf(tmp_path, doc, "enc", "@page{size:A4;margin:10mm} table{table-layout:fixed;width:60pt;font-size:7.5pt} td{overflow-wrap:anywhere;padding:0}")
    r = sh(sys.executable, COV, md)
    assert r.returncode == 1, r.stdout


def _fake_words(m, pieces):
    W = m.Words.__new__(m.Words)
    W.w = [(1, xa, ya, xb, yb, t) for (xa, ya, xb, yb, t) in pieces]
    W._index()
    return W


def test_wrap_geometry_same_line_or_right_of_end_rejected():
    m = _load_cov()
    ok = _fake_words(m, [(10, 10, 50, 17, "alpha-"), (10, 20, 40, 27, "beta")])        # next piece on a later line, starting left
    same = _fake_words(m, [(10, 10, 50, 17, "alpha-"), (60, 10, 90, 17, "beta")])       # same line, two cells
    right = _fake_words(m, [(10, 10, 50, 17, "alpha-"), (70, 20, 100, 27, "beta")])     # later line but right of the previous end (next column)
    assert m.present("alpha-beta", ok) and not m.present("alpha-beta", same) and not m.present("alpha-beta", right)


def test_wrap_allowed_only_at_separators_and_never_in_digit_run():
    m = _load_cov()
    seg = lambda a, b: _fake_words(m, [(10, 10, 50, 17, a), (10, 20, 40, 27, b)])
    assert m.present("collection-handler", seg("collection-", "handler"))              # after a separator
    assert not m.present("collection-handler", seg("collect", "ion-handler"))          # inside a segment
    assert not m.present("1234", seg("12", "34")) and not m.present("go:1234", seg("go:12", "34"))   # digit run
    assert m.present("a" * 35, seg("a" * 20, "a" * 15))                                  # over-long segment (>= 30) may break
    assert not m.present("a" * 25, seg("a" * 10, "a" * 15))                              # short segment of a long WORD may not (no enclosing-word permission)


def test_second_reading_order_is_load_bearing():
    # on a shipped wide-table doc the physical (line by line) order alone interleaves cells and misses wrapped tokens; the cell-by-cell order finds them
    root = os.path.abspath(os.path.join(HERE, "..", "..", ".."))
    md = os.path.join(root, "docs/research/nasemu/report.md")
    r = sh(sys.executable, COV, "--root", root, md)
    assert r.returncode == 0, r.stdout
    m = _load_cov()
    phys = m.Words(md[:-3] + ".pdf", False)
    toks = m.md_tokens(m.clean_md(open(md).read()), 4)
    assert [t for t in toks if not m.present(t, [phys])], "physical order alone is enough here: the second reading order is not load-bearing"


def test_glyph_widths_are_not_flat():
    sys.path.insert(0, D)
    import importlib; cols = importlib.import_module("pdf_table_cols")
    assert cols.word_pt("WWWWWWWW", 7.5) > 1.6 * cols.word_pt("iiiiiiii", 7.5)
    assert cols.word_pt("88888888", 7.5) > cols.word_pt("llllllll", 7.5)


def test_capital_ids_in_narrow_column_stay_whole(tmp_path):
    doc = "# T\n\n| id | a | b | c |\n|---|---|---|---|\n" + "".join("| WWMMWWMM%d | %s | %s | %s |\n" % (i, "prose " * 12, "prose " * 12, "prose " * 12) for i in range(6))
    md = make(tmp_path, doc, "caps")
    ws = _wordtexts(md[:-3] + ".pdf")
    for i in range(6): assert "WWMMWWMM%d" % i in ws


def test_zero_width_space_break_opportunities_inserted_and_stripped():
    sys.path.insert(0, D)
    import importlib; cols = importlib.import_module("pdf_table_cols")
    out = cols.add_wbr("<code>a/b_c.go:118</code> 2026-11-12 v7.67")
    assert "a/\u200bb_\u200bc.\u200bgo:118" in out                  # after '/', '_', '.' before a letter; ':' and digits stay together
    assert "2026-11-12" in out and "v7.67" in out                # no break inside dates or decimals


def test_needle_control_blind_if_matcher_ignores_the_stream(tmp_path):
    # a matcher that finds everything (except the fabricated needle) cannot tell real from destroyed text: BLIND, on both sides
    m = _load_cov()
    md = make(tmp_path)
    m.present = lambda tok, W: tok != m.CTRL_FAKE
    assert m.main([md]) == 2


def test_render_is_reproducible_across_clock_and_cwd(tmp_path):
    md = make(tmp_path, "# R\n\nsome plain text for the reproducibility check\n", "rep")
    a = open(md[:-3] + ".pdf", "rb").read()
    env = dict(os.environ); env.pop("SOURCE_DATE_EPOCH", None)
    import time; time.sleep(2)
    r = subprocess.run([PDF, md], capture_output=True, text=True, cwd="/", env=env)
    assert r.returncode == 0
    assert a == open(md[:-3] + ".pdf", "rb").read()


def test_digit_runs_never_split_even_inside_an_over_long_segment():
    m = _load_cov()
    seg = lambda a, b: _fake_words(m, [(10, 10, 50, 17, a), (10, 20, 40, 27, b)])
    assert not m.present("a" * 35 + "12", seg("a" * 35 + "1", "2"))
    assert m.present("a" * 35 + "12", seg("a" * 20, "a" * 15 + "12"))


def test_word_boundary_match_not_substring():
    m = _load_cov()
    W = _fake_words(m, [(10, 10, 80, 17, "concatenate")])
    assert not m.present("cat", W) and m.present("concatenate", W) and not m.present("catenat", W)


def test_needle_control_pdf_side_needs_the_differential(tmp_path):
    # the Markdown side behaves, the PDF-side matcher finds everything: only the PDF-side destroyed-copy control can notice
    m = _load_cov(); md = make(tmp_path); real = m.present
    m.present = lambda tok, W: (tok != m.CTRL_FAKE) if (isinstance(W, list) and W and isinstance(W[0], m.Words)) else real(tok, W)
    assert m.main([md]) == 2


def test_needle_control_markdown_side_needs_the_differential(tmp_path):
    m = _load_cov(); md = make(tmp_path); real = m.present
    m.present = lambda tok, W: (tok != m.CTRL_FAKE) if isinstance(W, str) else real(tok, W)
    assert m.main([md]) == 2


def test_table_font_shrinks_before_segments_break():
    sys.path.insert(0, D)
    import importlib; cols = importlib.import_module("pdf_table_cols")
    row = "<tr>" + "".join("<td>WWWWWWWWWWWW</td>" for _ in range(7)) + "</tr>"
    cg, font, fit = cols.layout("<table>" + row + row + "</table>", 527.0, 7.5)
    assert fit and font < 7.5, (font, fit)


def test_column_padding_covers_the_css_cell_padding():
    sys.path.insert(0, D)
    import importlib; cols = importlib.import_module("pdf_table_cols")
    css = open(os.path.join(D, "print.css")).read()
    px = float(re.search(r"th, td \{[^}]*padding:\s*[\d.]+px\s+([\d.]+)px", css).group(1))
    assert cols.PAD_PT >= 2 * px * 0.75 + 1.0, "column minimum does not leave room for the cell padding"
