#!/usr/bin/env python3
"""w2_fixture.py - fixture builder of the W2 register tests (T166 recount, T170/T171 plan-document seeds, T172 ticket families, T174 root inventory).
Pure fixture code: it writes only below the directory it is given. Builds on wp20_fixture.py (the 25-class scratch tree).

  w2_fixture.py freeze2 <tree> <freeze.json> [--untracked a,b] [--gitlink path=sha ...]
        manifest (snapshot_manifest.py) + files-only listing (`<freeze.json>.files.txt`, NUL-terminated, sorted by path bytes, symlinks included,
        no gitlink line) + freeze.json {snapshot, manifest, listing, listing_sha256, gitlinks, untracked_root_entries, remotes, head, frozen_at}
  w2_fixture.py plan-docs <tree>     the three plan documents (docs/21 sections 9.1, 9.3, 9.5; docs/18 tables) and the source documents the seeds cite,
                                     in miniature, under specs/001-full-project-audit-remediation/docs/ of the tree
  w2_fixture.py tickets-qa <tree>    ticket files for T172: QA-instrument closures (four causes + one unclassified), families, platform inference cases
"""
import hashlib, json, os, subprocess, sys

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)
sys.path.insert(0, HERE)
import wp20_fixture as W   # noqa: E402

DOCS = "specs/001-full-project-audit-remediation/docs"


def listing_of(root):
    rb = os.fsencode(root)
    acc = []
    for dp, dns, fns in os.walk(rb, followlinks=False):
        for n in list(dns) + list(fns):
            full = os.path.join(dp, n)
            if os.path.islink(full) or n in fns:
                acc.append(os.path.relpath(full, rb))
    return sorted(set(acc))


def freeze2(root, fj, untracked=(), gitlinks=()):
    out = os.path.abspath(fj)
    mf = out + ".manifest.json"
    r = subprocess.run([sys.executable, os.path.join(REG, "snapshot_manifest.py"), root], stdout=open(mf, "w"))
    if r.returncode != 0:
        sys.exit("snapshot_manifest failed")
    lp = out + ".files.txt"
    data = b"".join(p + b"\0" for p in listing_of(root))
    open(lp, "wb").write(data)
    json.dump({"snapshot": os.path.abspath(root), "manifest": mf, "listing": lp, "listing_sha256": hashlib.sha256(data).hexdigest(),
               "gitlinks": [{"path": p, "sha": s, "listing_lines": 0} for p, s in gitlinks], "untracked_root_entries": list(untracked),
               "frozen_at": "2026-10-08T00:00:00Z", "head": "scratch", "remotes": W.REMOTES}, open(out, "w"))


def recount_extras(root):
    """decoys and nested cases that kill the recount's weak variants (T166): each is a carrier the enumerator does NOT count"""
    W.w(root, "catalog-api/other.go", "package main\n// TODOS and XXXL are other words, not markers\n")
    W.w(root, "catalog-api/skip_test.go", "package main\nfunc TestX(t *testing.T) { t.Skip(\"later\") }\n")
    W.w(root, "catalog-web/a.test.ts", "it.skip('later', () => {})\n")
    W.w(root, "catalogizer-android/A.kt", "@Ignore\nfun f() {}\n")
    W.w(root, "desktop/b.rs", "#[ignore]\nfn t() {}\n")
    W.w(root, "scripts/t.py", "@pytest.mark.skip\ndef test_x(): pass\n")
    W.w(root, "assets/blob.bin", "\x00\x01 TODO inside a binary file\n")
    W.w(root, "challenges/helixqa-banks/b2.yaml", "version: 1\nname: nested\ntest_cases:\n  - id: outer-one\n    name: One\n    steps:\n      - id: step-a\n        action: tap\n      - id: step-b\n        action: tap\n")
    W.w(root, "docs/other/MY_AUDIT.md", "# nested audit note: not matched by docs/*AUDIT*.md\n\nCATAPI-DEFECT-777 is named only in a document that is not an S-14 file\n")
    W.w(root, "specs/x/notes.md", "# notes\n\nDEFER-888 is quoted by a spec document only\n")
    with open(os.path.join(root, "docs/qa/q1.md"), "a") as f:
        f.write("CATAPI-DEFECT-001 is also named here\n")
    with open(os.path.join(root, "MASTER_EXECUTION_CHECKLIST.md"), "a") as f:
        f.write("- [ ]glued to its text is not a checkbox line\n")


# ------------------------------------------------------------------------------------------------ plan documents (T170, T171)
DOC21 = """# 21 master plan (fixture)

### 9.1 Itemised seeds by source document

Severity labels are the source documents' own.

| Source | Ids | Application | Count | Severity distribution (source labels) |
|---|---|---|---:|---|
| doc01 §15 | O-01..O-03 | cross-system | 3 | unrated |
| doc02 §2.1-2.2 | F-INDEX-001..002 | indexes (governance) | 2 | unrated |
| doc07 §9 | C1..C4 | catalog-api | 4 | High 1, Medium 2, Low 1 |
| doc08 §4 | WEB-F01..F03 (WEB-F03, the doubled API prefix, counted from revision 8) | catalog-web | 3 | High 1, Medium 2 |
| doc09 §5.3 | S-01, S-02 | desktop and installer shared | 2 | unrated |
| doc12 §4, §14.1 | QF-01..QF-02, plus the stale constitution index hash | HelixQA | 3 | unrated |
| doc13 §2 | 3 measured defect classes (orphans, broken links, stale README version) | documentation | 3 | unrated |
| doc15 §13 | S-01..S-03 | security | 3 | unrated |
| doc18 §3.2 to §12.2 (revision 6) | `R-ADJ` candidates T1-A, T2-A (2) plus the `R-ADJ` parts of the mixed candidates T5-D (authenticate `/ws` first) and T7-C (the backup procedure) (2) | cross-system | 4 | unrated |
| **Total itemised** | | | **27** | |

Not counted as findings: doc14 H-01..H-20.

### 9.2 Rated seeds by application and severity

(not read by the importer)

### 9.3 Cross-document duplicate families

Many seeds describe the same defect from different angles.

| Family | Members |
|---|---|
| Unauthenticated `/ws` | doc01 O-02; doc07 C2; doc08 WEB-F02; doc15 S-02; doc18 T5-D (`R-ADJ` part) |
| Image proxy SSRF | doc07 C1; doc15 S-01 (linked, not counted: doc15 fact B5, doc14 hypothesis H-11) |
| Stubs | doc01 O-01; doc07 C3 |
| Prefix drift | doc08 WEB-F03; doc12 QF-01..QF-02; doc13 class "broken links" |
| Backup | doc18 T7-C (`R-ADJ` part); doc15 S-03 (part) |

These 5 families list 15 member ids, of which one, doc15 S-03, is only partly folded (marked `(part)`).

### 9.4 Legacy backlog population

(not read)

### 9.5 Innovation candidates from doc18 (revision 6)

Routing text.

| Entry | Kind | Candidate (doc18 wording shortened) | Ranked game changer (doc18 §13) |
|---|---|---|---|
| T1-B | `PROPOSAL` | manual re-match action | - |
| T2-B | `PROPOSAL` | persist match evidence and confidence | rank 1 |
| T5-D | `PROPOSAL` part | playback handoff over the authenticated WebSocket | rank 7 |
| T7-C | `PROPOSAL` part | verified-restore backup job exposed in health | rank 4 |

Counts: 4 entries (2 `PROPOSAL`, 2 `PROPOSAL` parts).

---

## 10. Cross-document inconsistencies
"""

DOC18 = """# 18 research (fixture)

### 1.1 Routing of every candidate (revision 2)

| ID | Kind | Evidence label | Destination |
|---|---|---|---|
| T1-A | R-ADJ | PROVEN (1.6, 3.4) | doc 07; docs/21 WP-30, fix in WP-51 |
| T1-B | PROPOSAL | PROVEN (1.3) | WP-20 intake: Feature item |
| T2-A | R-ADJ | PROVEN (2.2) | docs 05 and 07; WP-61 |
| T2-B | PROPOSAL | PROVEN components (1.4, 1.7); integrated form not found | WP-20 intake: Feature item |
| T5-D | mixed | PROVEN resume pattern (5.4) | R-ADJ part: doc 15 S-02; WP-50; PROPOSAL part: WP-20 intake as a Feature item |
| T7-C | mixed | PROVEN restore (7.3); UNCONFIRMED marker-row verify | R-ADJ part: doc 15 6.1; WP-50; PROPOSAL part: WP-20 intake as a Feature item |

### 3.2 Candidate improvements for this codebase

| ID | Candidate | Tag | Touches | Effort | Impact | Risk | Evidence label (finding) |
|---|---|---|---|---|---|---|---|
| T1-A | Treat peers' missing-not-deleted semantics as the design for removed files. | R-ADJ | catalog-api | M | High | Low | PROVEN (1.6, 3.4) |
| T1-B | A manual Identify / re-match action with provider-id input on every client. | PROPOSAL | catalog-api, catalog-web, android | M | High | Low | PROVEN (1.3) |

### 4.3 Candidate improvements

| ID | Candidate | Tag | Touches | Effort | Impact | Risk | Evidence label (finding) |
|---|---|---|---|---|---|---|---|
| T2-A | Parser regression corpus. | R-ADJ | catalog-api tests | M | High | Low | PROVEN (2.2) |
| T2-B | Persist match evidence and confidence. | PROPOSAL | catalog-api DB, API | M | High | Low | PROVEN components (1.4, 1.7); integrated form not found |

### 8.2 Candidates

| ID | Candidate | Tag | Touches | Effort | Impact | Risk | Evidence label (finding) |
|---|---|---|---|---|---|---|---|
| T5-D | Playback handoff over the authenticated WebSocket (resume pattern). | R-ADJ and PROPOSAL | catalog-api, androidtv | M | Medium | Medium | PROVEN resume pattern (5.4) |

### 9.3 Candidates

| ID | Candidate | Tag | Touches | Effort | Impact | Risk | Evidence label (finding) |
|---|---|---|---|---|---|---|---|
| T7-C | Verified-restore backup job exposed in health. | R-ADJ and PROPOSAL | catalog-api, scripts | M | High | Low | PROVEN restore (7.3); UNCONFIRMED marker-row verify |

## 13. Ranked game-changer candidates with falsifiable experiments

| Rank | Game changer | Label | PROPOSAL / R-ADJ | Why it could be decisive | Main risk |
|---|---|---|---|---|---|
| 1 | Explainable matching (T2-B, T2-C, T7-A) | PROVEN components | PROPOSAL | Attacks the number one complaint | Review burden |
| 4 | Verified-restore backups (T7-B, T7-C, T3-F) | PROVEN | PROPOSAL | Restorable backups | Cost |

### 13.1 Falsifiable experiments

**E1 - Explainable matching (rank 1).**
- Hypothesis: with scoring plus a review queue, wrong matches fall.
- Pass: precision of auto-accepted at least 99 percent.

**E4 - Verified restore (rank 4).**
- Hypothesis: a scripted backup plus restore proves the backup is restorable.
- Pass: the corrupted case is detected in 100 percent of 20 trials.

## 14. Bibliography
"""

# source documents the seeds cite: one table or heading row per id
SRC_DOCS = {
    "01-system-architecture-map.md": "# doc01\n\n## 15. Observations\n\n| ID | Observation | Severity |\n|---|---|---|\n| O-01 | FTP, NFS and WebDAV scanners are stubs | High |\n| O-02 | The WebSocket endpoint accepts any origin | High |\n| O-03 | The web client calls routes the server does not register | Medium |\n",
    "02-audit-methodology-and-index-strategy.md": "# doc02\n\n## 2.1\n\n- F-INDEX-001: index scope includes vendored code\n- F-INDEX-002: index health probe is a count\n",
    "07-backend-catalog-api-audit-plan.md": "# doc07\n\n## 9. Findings\n\n| ID | Finding | Severity |\n|---|---|---|\n| C1 | Image proxy allow-list is a substring check | High |\n| C2 | WebSocket upgrade without authentication | High if confirmed |\n| C3 | Scanner stubs return nil | Medium |\n| C4 | Query-string tokens | Low-Medium |\n",
    "08-web-client-audit-plan.md": "# doc08\n\n## 4. Findings\n\n| ID | Finding | Severity |\n|---|---|---|\n| WEB-F01 | Mock data in production bundle | Medium |\n| WEB-F02 | WebSocket without token check | High |\n| WEB-F03 | Doubled API prefix | High |\n",
    "09-desktop-and-installer-audit-plan.md": "# doc09\n\n### 5.3 Shared\n\n| ID | Finding |\n|---|---|\n| S-01 | Shared updater trusts any host |\n| S-02 | Shared config is world readable |\n",
    "12-helixqa-challenges-and-governance-plan.md": "# doc12\n\n## 4. Findings\n\n| ID | Finding |\n|---|---|\n| QF-01 | Prose-only banks |\n| QF-02 | Bank ids are not unique |\n\n## 14.1\n\nThe constitution index hash in the pre-push gate is stale.\n",
    "13-documentation-program-plan.md": "# doc13\n\n## 2. Measured defect classes\n\n| Class | Measured |\n|---|---|\n| orphans | 120 documents are linked from nowhere |\n| broken links | 40 links are broken |\n| stale README version | README says 2.3.0 |\n",
    "15-security-and-danger-zone-plan.md": "# doc15\n\n## 13. Seeds\n\n| ID | Seed |\n|---|---|\n| S-01 | SSRF through the image proxy |\n| S-02 | Unauthenticated WebSocket |\n| S-03 | Backup without a verified restore |\n",
}


def plan_docs(root):
    base = os.path.join(DOCS)
    W.w(root, base + "/21-master-plan-phases-risks-and-traceability.md", DOC21)
    W.w(root, base + "/18-research-product-innovation-and-game-changers.md", DOC18)
    for n, t in SRC_DOCS.items():
        W.w(root, base + "/" + n, t)


# ------------------------------------------------------------------------------------------------ tickets (T172)
def tk(i, slug, title, status="closed", platform="", screen="", resolution="", body=""):
    fm = "---\nid: HELIX-%03d\nseverity: high\ncategory: functional\nplatform: %s\nscreen: %s\nstatus: %s\nfound_date: 2026-04-13\n" % (i, platform, screen, status)
    if resolution:
        fm += "resolution: %s\nclosed_date: 2026-04-17\n" % resolution
    fm += "---\n\n# %s\n\n%s\n" % (title, body or "Step: do something\nActual: it failed")
    return "docs/issues/HELIX-%03d-%s.md" % (i, slug), fm


GENERIC = "QA infrastructure failure: Test did not reach expected app state due to screenshot timing, wrong ADB commands, or test sequencing issues. No reproducible app bug."
WRONGSCR = "QA infrastructure failure: screenshot showed login screen instead of video playback. Wrong screen captured. No reproducible bug in app."
ADBCMD = "QA infrastructure failure: test used KEYCODE_HOME instead of media key. Claim of crash is unsubstantiated with no logs. No reproducible bug in app."
BLACK = "QA infrastructure failure: the captured frame is a black frame, nothing rendered. No reproducible bug in app."
OTHERQA = "QA infrastructure failure: the harness restarted the emulator mid-test. No reproducible bug in app."


def tickets_qa(root):
    rows = [
        tk(501, "tv-focus-step-1", "Test Case Failed: TV Focus Management - Step 1", platform="androidtv", resolution=GENERIC),
        tk(502, "tv-focus-step-2", "Test Case Failed: TV Focus Management - Step 2", platform="androidtv", resolution=GENERIC),
        tk(503, "playback-wrong-screen", "Playback never started", platform="", screen="androidtv-004-playback.png", resolution=WRONGSCR),
        tk(504, "home-key", "Crash on resume", platform="", screen="androidtv-005-home.png", resolution=ADBCMD),
        tk(505, "black", "Home screen not drawn", platform="", screen="web-curiosity-006.png", resolution=BLACK),
        tk(506, "emulator-restart", "Settings test failed", platform="", resolution=OTHERQA),
        # not QA-instrument tickets: a real product ticket, a family pair (same slug+platform+screen), and a platform inference set
        tk(510, "login-spacing", "Excessive white space on login page", status="fixed", platform="androidtv", screen="androidtv-001-loginform.png", body="There is a large amount of empty space around the text fields and buttons."),
        tk(511, "login-spacing-again", "Excessive white space on login page", status="open", platform="androidtv", screen="androidtv-001-loginform.png", body="There is a large amount of empty space around the text fields and buttons."),
        tk(512, "login-spacing-other", "Excessive white space on login page", status="fixed", platform="androidtv", screen="androidtv-009-settings.png", body="The settings screen wastes space at the top; unrelated to the login form."),
        tk(520, "contrast-web", "Lack of text contrast", status="resolved", platform="", screen="web-curiosity-006.png", body="Text contrast is low."),
        tk(521, "contrast-tv", "Lack of text contrast", status="resolved", platform="", screen="androidtv-curiosity-015.png", body="Text contrast is low."),
        tk(522, "api-timeout", "API unreachable", status="fixed", platform="", screen="", body="GET http://localhost:3000/api/v1/entities/stats failed: context deadline exceeded"),
        tk(523, "mixed-evidence", "Conflicting evidence", status="fixed", platform="", screen="androidtv-001-loginform.png", body="Also seen at web-curiosity-006.png after the redirect."),
        tk(524, "no-evidence", "No evidence at all", status="fixed", platform="", screen="", body="Nothing to infer from."),
        tk(525, "declared", "Declared platform wins", status="fixed", platform="desktop", screen="androidtv-001-loginform.png", body="platform is declared"),
    ]
    for p, t in rows:
        W.w(root, p, t)


if __name__ == "__main__":
    a = sys.argv[1:]
    if a[:1] == ["freeze2"]:
        un, gl, i = [], [], 3
        while i < len(a):
            if a[i] == "--untracked":
                un = [x for x in a[i + 1].split(",") if x]; i += 2
            elif a[i] == "--gitlink":
                p, _, s = a[i + 1].partition("="); gl.append((p, s)); i += 2
            else:
                sys.exit("bad option " + a[i])
        freeze2(a[1], a[2], un, gl)
    elif a[:1] == ["plan-docs"]: plan_docs(a[1])
    elif a[:1] == ["tickets-qa"]: tickets_qa(a[1])
    else: sys.exit(__doc__)
