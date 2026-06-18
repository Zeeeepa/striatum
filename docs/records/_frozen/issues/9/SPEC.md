
    # GH #9 -- HIGH: CSRF on /v1/invoke — no Content-Type validation, cross-site command execution possible

    Source: <https://github.com/halbritt/striatum/issues/9> (filed 2026-05-14).
    Labels: bug, security, rfc-0050.
    Captured here verbatim so the runner's `context.docs` is self-contained
    and reviewers do not need GitHub API access mid-run.

    ---

    Found by gemini adversarial review in dogfood-056 (RFC 0050 V2 build review).
Full write-up: `docs/dogfood/056/review/build/gemini/REVIEW.md` Finding 1.

## Attack

`POST /v1/invoke` accepts arbitrary CLI `argv` payloads and executes them as the operator. The server is loopback-bound, but browsers allow cross-origin requests to `localhost`.

`_read_json_body` in `src/striatum/service.py` reads the request body as JSON but **does not validate the `Content-Type` header**. An attacker can send the payload with `Content-Type: text/plain` — a "simple" CORS request that does NOT trigger a preflight.

Example malicious page visited in the same browser as the local UI:
```html
<form action="http://localhost:8080/v1/invoke" method="POST" enctype="text/plain">
  <input name='{"argv":["run","cancel","--run-id","all"]} ' value="">
</form>
<script>document.forms[0].submit()</script>
```

This executes `striatum run cancel --run-id all` on the local runner.

## Impact

Remote command execution on the local runner from any visited website. Affects every operator running `striatum serve` (default web UI) without a Bearer token.

## Mitigations (any subset)

1. **Strict Content-Type check in `_read_json_body`** — refuse anything that isn't `application/json`. This alone defeats simple-request CSRF because CORS preflight will fire.
2. **Origin / Referer enforcement** on non-GET requests when `web_enabled` is true.
3. **CSRF token requirement** — server emits a per-session token in the HTML; the override modal + recovery-panel island include it in POST bodies.

Recommended landing: (1) + (2) for V1, (3) for V1.5 if needed.

## Relation to RFC 0050

V2 surface (introduced when override modal + recovery-panel island started using `/v1/invoke` for write-shaped operations). V1 + V1.5 used it only for read-shaped queries; the CSRF window opens with V2.

Track for v1.48.x security-hardening dogfood.
