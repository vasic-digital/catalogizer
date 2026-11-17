# Pull-request delivery (operator decision OPD-2026-11-17-PR)

Declared by recorded operator decision, relayed in the session (identifier UNCONFIRMED as an operator-side id):
1. 2026-11-17 21:03 CET: "Make sure everything at least gets merged to main repo via PRs you create, review and approve!"
2. 2026-11-17 21:15 CET (answer to the reviewer question): "All PRs MUST BE assigned to milos85vasic, however YOU perform full PRs review, approval and merging into main!"

## Flow
One task id = one branch `pr/<id>-<slug>` = one open PR on `origin` (github.com/vasic-digital/catalogizer). Every PR is
assigned to `milos85vasic` (the operator's account). After the independent Opus source-correctness GO for the exact head, the
author agent self-reviews on the platform (a review naming the head it read, line comments for findings), merges with a merge
commit (never squash or rebase), reads the PR back (state, merge commit, form), pushes `main` to every other upstream
(codeberg, m3w, gitlab, gitflic, gitverse, github orgs: `origin` already pushes to all nine mirror URLs) as a fast-forward, and retires the branch on
every upstream only when its head is on `main` everywhere. A merge also needs the remote base tip to be an ancestor of the
reviewed head (merge `main` into the branch first, never rebase).

## Honest limits (11.4.6)
- This is a deliberate operator deviation from 11.4.297(D)(2) (author alone on the reviewer list ends non-zero): the operator
  assigns the PRs to the author's own account and delegates review, approval and merge to the agent.
- Two credentials exist: the environment `GITHUB_TOKEN` cannot create PRs (HTTP 403); the keyring login of the same account
  can and is used (`env -u GITHUB_TOKEN -u GH_TOKEN gh ...`). GitHub does not allow approving one's own PR, so the agent's
  "approval" is a review COMMENT naming the head; no APPROVE state can exist. The Opus review is the real gate.
- `main` is NOT server-side protected (read back 2026-11-17 with the keyring login: HTTP 404 "Branch not protected").
  `protected_branches=main` is a policy statement only. No direct-push guard exists (`direct_push_guard=off`).
- No delivery tooling reads `pr_delivery.conf` yet: the whole flow is manual and PROVISIONAL.
- The platform merge settings offered squash and rebase when this was written; they are restricted to merge-commit only
  after the settings read-back recorded in the PR.
