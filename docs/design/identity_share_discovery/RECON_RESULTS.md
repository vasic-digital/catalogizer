# Identity-Share-Discovery — Real-NAS Recon Results (2026-06-26)

**Revision:** 1
**Last modified:** 2026-06-26T11:05:00Z
**Authority:** Identity-share-discovery epic; §11.4.6 (no-guessing), §11.4.10 (no secret values here — identities referenced by env-var only), §11.4.27 (real-system test), §11.4.74.
**Anti-bluff:** every row below is a captured FACT from a live SMB probe of the real LAN, not a guess.

> ⚠️ No credential value appears in this document. Identities are referenced by env-var index only (`CATALOGIZER_IDENTITY_1/2`, user `milosvasic`).

## Method
A recon probe (go-smb2, password read from env, never argv/log) dialed each SMB host on `192.168.0.0/24`, tried **anonymous first**, then each stored identity in order, and on success enumerated the **real** shares via SRVSVC `ListSharenames()`.

## Hosts + working (host, identity, real-shares) bindings — FACT

| Host | MAC OUI | Anonymous | Working identity | Real shares (SRVSVC) |
|---|---|---|---|---|
| 192.168.0.108 | `90:09:d0` = **NAS** | rejected (guest required) | #1 | `Data` |
| 192.168.0.109 | NAS | rejected | #1 | `DATA18` |
| 192.168.0.110 | NAS | rejected | #1 | `DATA12` |
| 192.168.0.111 | NAS | rejected | #1 | `DATA20`, `music`, `WORK20` |
| 192.168.0.105 | — | rejected | #1 | `Public`, `TimeMachineBackup` |
| 192.168.0.213 | — | rejected | **#2** (#1 fails) | `Public`, `Music`, `Projects` |

## Findings that drive implementation
1. **§11.4.6 anti-bluff fix landed (this PWU):** `smb_discovery.go enumerateShares` previously **guessed** from a hardcoded common-name list (`shared/public/media/music/videos/data/...`) — it MISSED every real NAS share (`Data`, `DATA18`, `DATA12`, `DATA20`, `WORK20`, `Projects`). Replaced with real `ListSharenames()` SRVSVC enumeration (IPC$ filtered). Proven by `TestDiscoverShares_RealShares_NoGuessing` (§11.4.27 real-system, §11.4.3 SKIP-gated, §11.4.115 RED-on-guessing → GREEN-on-fix).
2. **Anonymous-first needs GUEST, not true-anonymous:** go-smb2 rejects a true-anonymous bind ("Anonymous account is not supported yet. Use guest account instead"). The epic's anon-first step MUST attempt a **guest** session, then fall through to identities.
3. **Multi-identity fallback is real + required:** host `.213` rejects identity #1 and authenticates only with identity #2 — exactly the "try each identity until one passes, remember the working binding" mechanic the operator specified. The remembered binding key is `(host, share, identity-index, protocol=smb)`.
4. **Catalog ingest targets:** the NAS media lives under `DATA*`/`WORK20`/`music` shares on `.108–.111` — these become `storage_roots` (one per working `(host, share, identity)`), scanned by `UniversalScanner` to populate the catalog (then dedup + group per epic pillars 4–5).

## Next PWUs (per DESIGN.md)
- Identity store (multi-scheme: credentials/api_token/ssh_key) + secure storage (no plaintext in `storage_roots` — §11.4.10 debt).
- Guest/anon-first → multi-identity probe service + remembered-binding table.
- Auto-register working bindings as `storage_roots` → scan → dedup → group.
- UI: identity-manager + discovered-shares + grouped browse (OpenDesign §11.4.162), per app.
- Full §11.4.169 test matrix.
