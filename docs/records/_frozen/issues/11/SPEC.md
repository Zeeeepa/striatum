
    # GH #11 -- MEDIUM: Recovery panel dry-run relies on CLI-side read-only guarantee

    Source: <https://github.com/halbritt/striatum/issues/11> (filed 2026-05-14).
    Labels: bug, security, rfc-0050.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    Found by gemini adversarial review in dogfood-056. Full write-up: `docs/dogfood/056/review/build/gemini/REVIEW.md` Finding 3.

## Attack

The recovery-panel island uses `/v1/invoke` to run `recovery auto-publish --dry-run`. The "dry run" is intended to be safe, but the island has no independent verification — it trusts the CLI verb to be read-only when `--dry-run` is present.

Combined with #9 (CSRF on /v1/invoke), an attacker can trigger the dry-run from any visited website. If `auto-publish` ever gains a side-effect path that fires before the dry-run check (lease bookkeeping, audit row, advisory lock acquisition), CSRF can produce unintended state transitions.

## Mitigation

1. Audit `striatum recovery auto-publish` (and any other `--dry-run` CLI verbs reachable through `/v1/invoke`) to guarantee strict read-only semantics when `--dry-run` is set.
2. Pin a regression test: dry-run should not emit any event in the `events` table, take any lease, or write any artifact.
3. Consider a server-side allowlist of dry-run-only CLI verbs reachable from `/v1/invoke` so the island doesn't need to trust the verb's argv shape.

Defense-in-depth against #9; bundle with v1.48.x security-hardening.
