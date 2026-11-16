# Evidence label rewrite, 2026-11-16 (CAT-GENERIC-HOSTS-1)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-16 |
| Status | PROVISIONAL (independent review pending) |
| Roadmap-Item | CAT-GENERIC-HOSTS-1 |
| Original text | git commit `bd86adcad` (all originals and original manifests) |

## What was done and why

By operator decision of 2026-11-16 a server exposing storage protocols (SMB, NFS, WebDAV, FTP, SFTP) is not a product and not a direct target, and the repository is host-agnostic. On 2026-11-16, under CAT-GENERIC-HOSTS-1, the former NAS-vendor name and the labels derived from it were substituted in code, scripts, configuration, documents and dated evidence. In the evidence files the content is byte-identical apart from host labels and vendor names; counts, hashes of listed names and measurements were not changed. The text before the substitution, and the manifests before regeneration, are at git commit `bd86adcad`.

Because this edits dated evidence, per 11.4.5 / 11.4.6 / 11.4.268 the rewrite is disclosed here and the affected `SHA256SUMS` / `MANIFEST.sha256` files were regenerated only for files whose bytes changed (entries for files that were already absent, for example gitignored `*.out` and `*.log`, were kept as they were).

## Mapping (old to new)

| Former (vendor-named) | New |
|---|---|
| work item id `<vendor>-LIVE-1` | `PROTOCOL-LIVE-1` |
| env variables `<VENDOR>_SMB_USER`, `<VENDOR>_SMB_PASSWORD`, `<VENDOR>_IP_n`, `<VENDOR>_HOST_n`, `<VENDOR>_SECRET` | `LIVE_HOST_SMB_USER`, `LIVE_HOST_SMB_PASSWORD`, `LIVE_HOST_IP_n`, `LIVE_HOST_n`, `LIVE_HOST_SECRET` (no alias is read; rename them in your local gitignored `.env`) |
| host aliases `<Vendor><n>` (n = 1..7) | `LiveHost<n>` (host 1 is `LiveHost1` everywhere) |
| directory `docs/qa/<vendor>-live-20261115/` | `docs/qa/protocol-live-20261115/` |
| `docs/infrastructure/<vendor>-hosts.md` | `docs/infrastructure/protocol-live-hosts.md` |
| `docs/qa/stack3-rebuild-live-20261105/live_<vendor>.txt` | `live_protocol_host.txt` |
| `.../evidence/wp10/<vendor>-survey/` | `.../evidence/wp10/protocol-host-survey/` |
| `docs/qa/baseline-v2.4.2-20261114/gate_<vendor>.rec` (product line item) | removed; the host is not a product |
| (none) | `gate_api.rec` new, see below |
| Go test `...Skips<Vendor>MetadataDirectories` | `...SkipsNASMetadataDirectories` |
| test hostname `<vendor>.local`, mount `/mnt/smb/<vendor>` | `nas.local`, `/mnt/smb/nas` |
| test data vendor string | `ExampleNAS` |
| other prose uses of the vendor name | `NAS` / `protocol live host` |
| third-party reference names (vendor documentation host, API guide file name, client and MCP project names) in the nasemu research | generic placeholders (`cndl.vendor-docs.example`, `Vendor_File_Station_API_Guide`, `py-vendordsm-api`, ...); the original names are at `bd86adcad` |

Abbreviation ids that do not contain the vendor name (`CAT-SYN-LIVE-1`, `synlive_*` log names) were not changed.

## Exact substitution used for the raw logcat

File `docs/reports/qa-sessions/2026-04-22-T16-28/logcat.txt` (NetBIOS names in tool output). The logcat is an EXCEPTION to the general label scheme: it was relabelled with an older, cruder ordered rule set, and it is committed as-is (it was not altered afterwards; it is raw evidence, relabelled, and the rules below reproduce the committed bytes exactly). Let V stand for the former vendor name (written here as `V` so this note does not contain it). The ordered rules, applied with perl, each over the whole file, were:

1. `V NAS` becomes `NAS` (this collapsed recorded `storage_root_name` values, for example `V NAS Data1` became `NAS Data1`);
2. `V` followed by a digit becomes `LiveHost` followed by that digit (hosts 2 to 7 are `LiveHost2` to `LiveHost7`);
3. remaining upper-case `V` becomes `LIVEHOST`;
4. remaining `V` (mixed case) becomes `NAS`;
5. lower-case `v` becomes `nas`.

Consequence: in this one logcat host 1 appears as `NAS` (device `NAS`, serviceName `NAS`, `NAS.local`), NOT as `LiveHost1`, and some recorded storage root names lost their vendor word entirely. Everywhere else in the rewrite host 1 is `LiveHost1`. The general rule set (environment names, id, `<Vendor><n>` aliases, path and file names, third-party reference names, generic prose fallbacks) applied to all other files; the scanner copies under `.../scanner/fix-r2-reviewer-probes-verbatim/*_test.go` were relabelled by it as well.

## Evidence directories and files touched

`docs/qa/`: `anton-smb-n3-20261112`, `baseline-v2.4.2-20261114`, `d4-doc-search-fix-20261105`, `findings_20260626`, `live-map-20261103/80_session_scratchpad_evidence`, `protocol-live-20261115` (and `nas-clients`, `smb-readonly`), `retest-20261112`, `smb-subdir-20261110/live-run-anton-20261111T190325Z`, `stack3-rebuild-live-20261105`, `storage-clients-fix-20261104`, and two index files in `docs/qa/`; `docs/reports/qa-sessions/2026-04-21-T/challenges`, `docs/reports/qa-sessions/2026-04-22-T16-28`; `docs/research` and `docs/research/nasemu`; `docs/design/identity_share_discovery`; `specs/001-full-project-audit-remediation` (progress, decisions, `evidence/reconcile`, `evidence/wp10/protocol-host-survey`, `evidence/wp11`, `evidence/wp12` with `fabric`, `ftp`, `nas-clients`, `nas-protocols`, `nas-survey`, `nfs3`, `protocol-analysis`, `scanner`, `sftp`, `wf17/green`, `evidence/wp24`).

Manifests regenerated for changed files: those in the directories above that carry a `SHA256SUMS` or `MANIFEST.sha256`. Pre-existing mismatches that were not caused by this change (for example `evidence/wp11/SHA256SUMS` entries `README.md` and `t124-race-red.txt.done`, `evidence/wp12/SHA256SUMS` entry `README.md` (it FAILED on main before this change and now verifies OK only because the README was rewritten), and `docs/qa/stack3-rebuild-live-20261105/MANIFEST.sha256` entry `live_storage_done.txt`) were left as they were.

## gate_api.rec

`docs/qa/baseline-v2.4.2-20261114/gate_api.rec` was generated afterwards, on 2026-11-16, by `scripts/release/release_gate.sh --product api` with absent hardware, live-run and comparison records; exit 20, state `BLOCKED-NO-HARDWARE`. The output was compared with `cmp` and is byte-identical to the tool's output. It replaces the former vendor-named record in the six-product set (api, web, desktop, wizard, Android, Android TV).

## Removed side-effect files

Five scratch files `specs/001-full-project-audit-remediation/evidence/disk/nas-ro-*-3*.json` (test side-effects with scratch paths) that an earlier commit of this branch had added were removed again.
