# Pull-request delivery (operator decision 2026-11-17)

Mode `pr` is declared (constitution §11.4.297). One task id = one branch `pr/<id>-<slug>` = one open
pull request on `origin` (github.com/vasic-digital/catalogizer). After the independent Opus source-correctness
GO for the exact head, the author agent self-reviews on the platform (a review naming the head it read, with
line comments for findings), then merges with a merge commit (never squash or rebase), reads the PR back
(state, merge commit, form), pushes `main` to the other upstreams (codeberg, m3w, gitlab mirrors) fast-forward,
and retires the branch only when its head is on `main` everywhere.

Honest limits (§11.4.6): the only authenticated GitHub account is the repository owner's token (`GITHUB_TOKEN`);
GitHub does not allow approving one's own pull request, so the "approval" is a review comment/`COMMENT` review, not an
`APPROVE` state, and no separate reviewer can be requested until the operator supplies a second account
(`reviewers=` in `pr_delivery.conf` is empty: UNCONFIRMED). Branch protection could not be read with this token
(HTTP 403), so server-side protection is UNCONFIRMED. Reviewer: the Opus independent review remains the real gate.
