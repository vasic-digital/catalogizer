# lumenignore_skeleton.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08 |
| Status | new, uncommitted; independent review owed (constitution 11.4.142); indexed by `docs/scripts/README.md`; origin: review WP-02-T028-r2 N3 |
| Source | `scripts/audit/lumenignore_skeleton.py`; tests `scripts/audit/tests/test_scope_drift_check.sh` (D11, D13), mutants k01..k04 in `scripts/audit/tests/mutate_wp02_fix_r6.sh` |

## Purpose

`gen_lumenignore.py` lists the real children of the ancestors of the allowed roots with `os.listdir`, so its output depended on the untracked directories of the host. This tool writes a directory skeleton of the TRACKED tree so the render is a function of tracked inputs only.

## Usage

```bash
scripts/audit/lumenignore_skeleton.py --repo DIR --scope config/index/lumen_scope.json --out EMPTYDIR [--host-local NAME]...
gen_lumenignore.py --repo EMPTYDIR --scope config/index/lumen_scope.json --out .lumenignore     # normally via scope_drift_check.sh --regen
```

It reads `git -C DIR ls-files -z -s --recurse-submodules` and the `allow` list of the scope JSON; it writes only below `--out` (absent or empty). Entries down to the depth of the deepest allowed root are created (a gitlink, mode 160000, as a directory; a file as an empty file), plus the fixed host-local names `.git .claude .remember` (default; they are denied whether or not they exist on this host).

Exit: 0 ok; 2 usage; 3 fail closed (git failed, unreadable scope, empty allow, `--out` not empty, an allowed root inside a submodule that is not checked out).

## Honest limits

- A fresh clone needs the submodules that contain allowed roots checked out (here `constitution`, which holds nested own submodules); otherwise exit 3, never a wrong render.
- An untracked, non-ignored directory is not denied to Lumen by the rendered `.lumenignore` (only tracked structure and the fixed list are). Track it or `.gitignore` it.
- The skeleton lists what the INDEX holds, not the working tree: an `git add`-ed path counts, an untracked one does not.
