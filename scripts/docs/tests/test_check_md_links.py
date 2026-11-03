"""Tests of scripts/docs/check_md_links.py. Run: python3 -m pytest -p no:cacheprovider scripts/docs/tests

Oracles: exit codes and the BROKEN rows. Fixtures are real files in a temp tree (no mocks).
Mutation check: CHECK_MD_LINKS_NO_EXISTENCE=1 disables the existence check and the broken-link test MUST then fail.
"""
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
CHECK = os.path.join(HERE, "..", "check_md_links.py")

DOC = """# Title

Good: [ok](other.md) and [dir](sub/) and [anchor ok](#title) and [cross](other.md#second-heading).
Broken file: [bad](missing.md)
Broken anchor: [bada](#no-such-heading)
Broken cross anchor: [badx](other.md#nope)
Anchor-only ok: [a](#title)
External: [e](https://example.invalid/x.md) [m](mailto:a@b.c)
Carrier: `[x](nowhere.md)` stays ignored.

```
[fenced](fenced_missing.md)
```

~~~md
[tilde](tilde_missing.md)
~~~
"""
OTHER = "# Other\n\n## Second Heading\n"


def make(tmp_path):
    (tmp_path / "docs" / "sub").mkdir(parents=True)
    (tmp_path / "docs" / "a.md").write_text(DOC)
    (tmp_path / "docs" / "other.md").write_text(OTHER)
    (tmp_path / "docs" / "sub" / "x.md").write_text("# x\n")
    # relative links in docs/a.md resolve against docs/
    return tmp_path


def run(root, env=None):
    e = dict(os.environ)
    e.update(env or {})
    return subprocess.run([sys.executable, CHECK, "--root", str(root)], capture_output=True, text=True, env=e)


def broken_targets(out):
    return sorted(l.split("\t")[3] for l in out.splitlines() if l.startswith("BROKEN\t"))


def test_finds_exactly_the_planted_broken_links(tmp_path):
    make(tmp_path)
    r = run(tmp_path)
    assert r.returncode == 1, r.stdout + r.stderr
    assert broken_targets(r.stdout) == ["#no-such-heading", "missing.md", "other.md#nope"], r.stdout


def test_ignored_classes_not_reported(tmp_path):
    make(tmp_path)
    r = run(tmp_path)
    for needle in ("nowhere.md", "fenced_missing.md", "tilde_missing.md", "example.invalid", "mailto"):
        assert needle not in r.stdout


def test_clean_tree_exits_zero(tmp_path):
    make(tmp_path)
    (tmp_path / "docs" / "a.md").write_text("# T\n\n[o](other.md) [s](#t)\n")
    r = run(tmp_path)
    assert r.returncode == 0, r.stdout + r.stderr


def test_mutation_disabled_existence_check_is_caught(tmp_path):
    """With the existence check disabled the planted broken links vanish; the test of the real run must then fail."""
    make(tmp_path)
    assert broken_targets(run(tmp_path).stdout) == ["#no-such-heading", "missing.md", "other.md#nope"]
    r = run(tmp_path, {"CHECK_MD_LINKS_NO_EXISTENCE": "1"})
    assert broken_targets(r.stdout) != ["#no-such-heading", "missing.md", "other.md#nope"]


def test_empty_scope_is_blind(tmp_path):
    r = run(tmp_path)
    assert r.returncode == 2
