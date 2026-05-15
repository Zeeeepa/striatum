# Triage Prompt: GH #19 + #21

Read both SPECs:
- `docs/issues/19/SPEC.md` — stale-lease recovery for repo_write jobs has no operator escape path.
- `docs/issues/21/SPEC.md` — `striatum serve` startup clobbers state.sqlite3.

Produce `docs/issues/19/SCOPE.md` with:

## 1. Files to change per issue

For each issue, list:
- The exact source files + functions/methods to change.
- The exact test files to add/extend.
- The exact doc files to update (if any).

## 2. Acceptance checks

Cite each Required Fix section from the SPECs and translate into a verifiable acceptance check. Each check must be testable.

## 3. Minimum viable scope

The fixes should be NARROW. Do not propose broader refactors:
- For #21: the minimum is "serve startup must not init-over an existing healthy state.sqlite3". A `--read-only` flag and full WAL-mode plumbing are SPEC stretch goals — defer them.
- For #19: the minimum is `recovery requeue-stale --force --justification "..."` for repo_write stale jobs (audit-chained). The `operator-publish` verb and auto-recovery-on-lease-expiry are SPEC stretch goals — defer them.

## Byline

`author: triager-unknown-model-<NN>`. Plain markdown line, lowercase author:.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
