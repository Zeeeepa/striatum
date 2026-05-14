
    # GH #12 -- LOW: copy-on-click works on any data-copy element — clipboard poisoning surface

    Source: <https://github.com/halbritt/striatum/issues/12> (filed 2026-05-14).
    Labels: enhancement, security, rfc-0050.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    Found by gemini adversarial review in dogfood-056. Full write-up: `docs/dogfood/056/review/build/gemini/REVIEW.md` Finding 4.

## Attack

`src/striatum/web/static/copy_on_click.js` hooks every element with a `data-copy` attribute globally. A malicious PR or XSS injection could add `data-copy="rm -rf /"` to a large transparent overlay, or to a common navigation link, so any operator click silently places a malicious command in the clipboard. The operator may later paste blindly into a terminal.

## Impact

Clipboard poisoning → potential command execution if the operator pastes without inspection. Low severity because it requires either a malicious PR landing or a separate XSS vulnerability, but the affordance design itself is the bug — it auto-arms any element on the page.

## Mitigation

Restrict the `copy_on_click` behavior to specific allowed container classes (e.g., `.recipe-list`, `.code-recipe`, `.copyable-token`) rather than the entire document. Refuse `data-copy` outside those containers.

Bundle with the V2 ergonomics polish pass.
