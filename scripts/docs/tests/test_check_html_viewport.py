"""Tests of scripts/docs/check_html_viewport.py and render_md_to_html.sh. Run: python3 -m pytest -p no:cacheprovider scripts/docs/tests

Oracles: exit codes and the MISSING rows. Fixtures are real files in a temp tree (no mocks); the generator test runs real pandoc.
"""
import os
import shutil
import subprocess
import sys

import pytest

HERE = os.path.dirname(os.path.abspath(__file__))
CHECK = os.path.join(HERE, "..", "check_html_viewport.py")
RENDER = os.path.join(HERE, "..", "render_md_to_html.sh")
GOOD = '<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1.0"></head><body>x</body></html>'
FRAGMENT = '<h1 id="t">T</h1>\n<p>body only</p>\n'


def run(root):
    return subprocess.run([sys.executable, CHECK, "--root", str(root)], capture_output=True, text=True)


def tree(tmp_path, pages):
    for name, text in pages.items():
        p = tmp_path / "docs" / name
        p.parent.mkdir(parents=True, exist_ok=True)
        p.write_text(text)
    return tmp_path


def test_all_pages_with_viewport_pass(tmp_path):
    r = run(tree(tmp_path, {"a.html": GOOD, "sub/b.html": GOOD}))
    assert r.returncode == 0, r.stdout + r.stderr


def test_fragment_without_viewport_fails_and_is_named(tmp_path):
    r = run(tree(tmp_path, {"a.html": GOOD, "sub/frag.html": FRAGMENT}))
    assert r.returncode == 1
    assert "MISSING docs/sub/frag.html" in r.stdout
    assert "MISSING docs/a.html" not in r.stdout


def test_empty_scope_is_blind_not_pass(tmp_path):
    (tmp_path / "docs").mkdir()
    assert run(tmp_path).returncode == 2


def test_viewport_inside_comment_text_only_still_matches_real_meta_form(tmp_path):
    r = run(tree(tmp_path, {"a.html": "<head><meta name='viewport' content='width=device-width'></head>"}))
    assert r.returncode == 0


@pytest.mark.skipif(shutil.which("pandoc") is None, reason="pandoc absent: SKIP, hardware/tool not present")
def test_generator_emits_complete_page_with_viewport_and_check_mode(tmp_path):
    md = tmp_path / "d.md"
    md.write_text("# Title here\n\ntext\n")
    r = subprocess.run(["bash", RENDER, str(md)], capture_output=True, text=True)
    assert r.returncode == 0, r.stderr
    html = (tmp_path / "d.html").read_text()
    assert html.startswith("<!DOCTYPE html>") and '<meta name="viewport"' in html and "<title>Title here</title>" in html
    assert subprocess.run(["bash", RENDER, "--check", str(md)], capture_output=True).returncode == 0
    (tmp_path / "d.html").write_text(FRAGMENT)  # drift
    c = subprocess.run(["bash", RENDER, "--check", str(md)], capture_output=True, text=True)
    assert c.returncode == 1 and "DRIFT" in c.stdout
