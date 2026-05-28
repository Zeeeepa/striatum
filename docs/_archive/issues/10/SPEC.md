
    # GH #10 -- MEDIUM: Override modal trusts DOM data-attributes for job/session IDs

    Source: <https://github.com/halbritt/striatum/issues/10> (filed 2026-05-14).
    Labels: bug, security, rfc-0050.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    Found by gemini adversarial review in dogfood-056. Full write-up: `docs/dogfood/056/review/build/gemini/REVIEW.md` Finding 2.

## Attack

`src/striatum/web/static/override_verdict.js` builds its `argv` using `data-job-id` / `data-session-id` attributes read from the DOM. An attacker who can manipulate the DOM (XSS, malicious user script, or even browser extension) can change these identifiers to target different jobs or sessions than the UI context implies.

## Impact

Unauthorized verdict overrides on unintended jobs. The server validates the `argv` is well-formed but cannot tell from the request whether the modal targeted the right job for the operator's current visual context.

## Mitigation

Client should verify that the targeted `job_id` belongs to the current `run_id` URL context before posting. Server should additionally accept a context token (signed run_id + job_id pair issued at page render) to defeat the cross-context attack.

Bundle with #9 in v1.48.x security-hardening pass.
