# Persisted interrogation thread (genuine, model-to-model)

Run: run_f9753a8c54ca534f79aea95e08bae53e · interrogation: intg_5c7480019bab1ac0ad74cb0007a8dc46 · topic: threat model: interrogation-log chat UI
Interrogator: codex (sess_aab6…) · Target: claude synthesizer (sess_dcb0…, agent-loop, awaiting_interrogation)
6 turns / 3 rounds. Proves RFC 0084 attestation fix: a headless agent-loop session was interrogated for real.

## [0] interrogation_question

Threat model check: the interrogation-log chat UI will render preserved peer-session turn bodies, which are attacker-controlled Markdown/text in practice. What exact rendering path prevents stored XSS or scriptable HTML injection here, including links, code blocks, and any rich formatting? If the synthesis assumes escaping/sanitization, where is that enforced and what tests prove raw HTML cannot execute?

## [1] interrogation_answer

The XSS posture is the single largest reason the synthesis rejected gemini's client-side `app.js` path and chose server-side rendering, so this is squarely a decision I made, not an assumption I deferred. The exact path:

1. RENDERING ENGINE — `html/template`, not `text/template`. The chosen design renders each turn body server-side through a Go template (`RenderInterrogation(meta, turns)` + `templates/interrogation.html`) where the body appears as `{{.body}}` in HTML element context. This MUST be `html/template`, which context-aware auto-escapes `<`, `>`, `&`, `"`, `'` before bytes leave the process. I want to flag a wording hazard for the implementer: the claude_code source design (its §b) loosely wrote "`text/template` auto-escapes" — that is wrong; `text/template` does NOT escape and would reintroduce stored XSS. The DESIGN_SYNTHESIS rationale (§"the one real divergence") is the authoritative line: it says `html/template`'s automatic escaping. The implementation is only safe with `html/template`. This is load-bearing and the test below must pin it.

2. NO RICH FORMATTING IS PARSED — this is what neutralizes links and code blocks specifically. Markdown rendering is explicitly DEFERRED (synthesis "Deferred:" list). Bodies are rendered as plain text, never as Markdown or raw HTML. So there is no Markdown→HTML step that could emit an `<a href="javascript:...">`, no fenced-code-block parser, no autolinker. A body containing `[x](javascript:alert(1))`, a fenced ```` ```<img onerror> ```` block, or a raw `<script>` tag all render as their literal escaped characters and display as inert text. Readability of multi-line bodies comes purely from CSS (`white-space: pre-wrap` on the body div), not from any HTML interpretation. There is no path by which authored content becomes live DOM.

3. WHERE ENFORCEMENT LIVES — at the render boundary, server-side, before the response is written; not in the browser. This is the whole point of rejecting gemini's path, which shipped the raw RPC JSON to the client and re-implemented escaping in JS — a weaker, easy-to-regress posture. By keeping escaping in `html/template`, the curated-field guarantee (D028) and the escaping guarantee share one enforcement point.

4. ATTACK SURFACE IS NARROWED BY CURATION (D028) — only the curated `kind` and `body` fields are projected by the existing read (`reads/interrogation.go`), and the template binds only those. Provider stdout/stderr/terminal data is never rendered, so the attacker-controlled surface is exactly the authored `body` text, which is escaped.

5. THE TEST THAT PROVES IT — Test #3 in the test plan (the `webassets` render+escaping test) seeds a turn whose `body` contains `<script>` and asserts the rendered output contains the escaped sequence (`&lt;script&gt;`) and not an executable `<script>` tag. That single assertion guards D028 and XSS together. I'd strengthen it on the strength of this question: extend the malicious fixture beyond a bare `<script>` to also cover an `<img src=x onerror=...>` attribute-injection vector and a `javascript:` pseudo-URL inside `[...](...)` text, asserting all three appear escaped/inert — which also locks in the "no Markdown parsing" decision as a regression guard, so a future implementer who adds a Markdown library trips the test.

Residual scope note: the sibling chat page and the existing run `<pre>` JSON dump (`renderRPCPage`) are both server-rendered and escaped; nothing in this slice introduces a client-side innerHTML sink. If a future slice adds Markdown bodies or live SSE turn updates (both explicitly deferred), the threat model must be revisited at that point because they reintroduce an HTML-generation or DOM-mutation path that this slice deliberately does not have.

## [2] interrogation_question

Threat model check: what prevents the new interrogation-log read/UI endpoint from becoming an IDOR or cross-scope disclosure path? Concretely, if a user can guess or obtain an interrogation_id, how does the design enforce repository/run/session scope and capability authorization before returning curated turns, and are closed or peer-session interrogations handled differently?

## [3] interrogation_answer

There are three enforcement layers, and I want to be precise about exactly which one stops IDOR and which boundaries this slice deliberately does NOT add, because the honest answer includes a gap.

1. REPOSITORY SCOPE (RPC boundary) — `callAndWrite` injects `repository_id` from `h.config.RepositoryID` (`service.go:208`); the web service instance is bound to one registered repository. Every read, including `interrogation.show`/`interrogation.list`, is therefore repository-scoped at the daemon read layer. A guessed `interrogation_id` belonging to a *different repository* does not resolve, because the read is constrained to the configured repository. So cross-repository disclosure is closed by construction.

2. RUN-OWNERSHIP CHECK (HTTP route boundary) — this is the specific IDOR mitigation, and it is the one point the synthesis ELEVATED above the source designs. `interrogation.show` is keyed by `interrogation_id`, NOT by `run_id` — that is exactly the IDOR hazard you're pointing at. The chosen design (adopted from codex §Read, lines 47–54, and made REQUIRED in the synthesis "Adopted from codex specifically" section) is: the nested route `GET /v1/runs/{runID}/interrogations/{id}` calls `show`, then asserts the returned `interrogation.run_id == runID` and returns 404 on mismatch — before writing any turn bytes. This stops a URL under one run from leaking another run's thread within the same repository. claude_code and gemini both OMITTED this check; the synthesis explicitly marks it as not optional, and Test #2 (run-ownership 404) exists solely to prove it: a `show` whose `run_id` differs from the path `runID` returns 404.

3. TRANSPORT/CAPABILITY (ServeHTTP boundary) — the route inherits the existing loopback-host bind, service-token requirement, and CSP applied in `ServeHTTP` (`service.go:47`), and `interrogation.list`/`show` are registered as read-capability methods (`rpc/registry_methods.go:12`). So an unauthenticated remote caller cannot reach the endpoint at all; this is a local-first, single-operator daemon on loopback, not a multi-tenant service.

NOW THE HONEST GAPS, because your question deserves them:

a. The run-ownership 404 protects the *run-nested* path only. The underlying `interrogation.show` RPC itself is still keyed solely by `interrogation_id` and is reachable via the generic `POST /v1/invoke` escape hatch. Anyone already past the service token, operating within the configured repository, can fetch any interrogation in that repository by id via `/v1/invoke` — the run check lives in the HTTP route, not in the read. The synthesis intentionally does NOT build UI on `/v1/invoke` and does not harden it; within-repository, same-operator access is treated as in-scope by the local-first threat model.

b. There is NO per-session / peer-session authorization in this slice. The interrogation carries `interrogator_session_id` and `target_session_id`, but the design does not enforce "a session may only read interrogations it participated in." If the threat model requires that boundary, it is NOT here and would need a new authz predicate keyed on session identity, which the existing reads do not implement. I'm flagging this rather than implying scope I didn't design.

c. CLOSED vs OPEN / peer-session handling — there is no differential handling, and that is deliberate. The read projects the same curated fields (`kind`, `body`, ordering) regardless of interrogation state; `state` is surfaced in the thread header for the operator's context (codex chat model), but a closed interrogation still returns its curated turns — it is read-only history. Peer-session turns are projected through the same curated `body`/`kind` fields (D028), so no provider stdout/stderr ever crosses the boundary regardless of which session authored a turn.

Summary: cross-repository disclosure → closed by repository_id injection; cross-run IDOR via the UI → closed by the required run-ownership 404 (+ Test #2); cross-session / within-repo `/v1/invoke` access → explicitly out of scope for this local-first slice and worth a follow-up decision if a stricter tenancy model is ever required.

## [4] interrogation_question

Threat model check: the design relies on curated interrogation turns rather than raw session logs. What exact fields are exposed in the chat page and supporting read calls, and what prevents accidental expansion to raw provider transcripts, stdout/stderr, tool payloads, secrets, or private diagnostics? Also, does the page introduce any external asset, referrer, caching, or copy/export path that could leak those curated bodies beyond the local operator?

## [5] interrogation_answer

EXACT FIELDS EXPOSED. The chat page binds only what the existing reads project; the design adds no new field. Per turn, from the `interrogation.show` turn projection (`reads/interrogation.go:71-82`): `kind` (the enum `interrogation_question`/`interrogation_answer`, used only to pick the speaker label "Reviewer"/"Target" and the CSS class), `body` (the authored turn text), and ordering fields `turn`/`turn_index`. The thread header renders interrogation metadata the `show` already returns: topic, state, interrogator session id, target session id, and opened/closed timestamps (codex chat model). The list route exposes the per-interrogation summary (`interrogation_id`, state, counts). That is the complete set. No `process_executions` row, no session log, no transcript, no stdout/stderr, no tool-call payload is read or rendered — the design calls ONLY `interrogation.list` and `interrogation.show`, neither of which projects those fields in the first place.

WHAT PREVENTS EXPANSION TO RAW TRANSCRIPTS. Three things, in order of strength:
1. Structural, not stylistic: the curated fields are all that exist in the read projection. To leak raw provider output an implementer would have to (a) change the read projection at `reads/interrogation.go:71-82` to SELECT and project new columns AND (b) add those fields to the template — two deliberate, reviewable steps. The render boundary can only emit what the read hands it.
2. D028 is the named invariant ("authored text only, never raw stdout/stderr"), and the synthesis pins "curated fields only (D028)" in its agreement spine. The source designs all explicitly state "do not render provider stdout/stderr or terminal data."
3. The escaping test (Test #3) doubles as a curation guard: it asserts the rendered output is exactly the escaped curated `body` and nothing else.

HONEST RESIDUAL ON CONTENT. The remaining exposure is authored-content risk, not transcript-scraping risk: `body` is free text a session wrote. If a target session pasted a secret INTO an interrogation answer, that secret lives in the curated `body` and would render. Nothing in this slice scans bodies for secrets — and that is correct scope (it's an authoring-discipline problem, not a rendering problem), but I'm flagging it rather than implying the curation guarantee sanitizes content. The guarantee is "no fields beyond curated authored text," not "authored text contains no secrets."

LEAKAGE PATHS — and here the local-first posture does most of the work:
- External assets: the page links only the already-embedded `static/base.css` (`page.html:7`, embedded via `webassets/assets.go:16`). No CDN, no external fonts, no analytics, no third-party script. So no outbound request carries the run/interrogation id or body to any external host — consistent with AGENTS.md's "no hosted services, cloud APIs, telemetry." The CSP applied in `ServeHTTP` (`service.go:47`) backstops this by restricting asset origins.
- Referrer: bodies are rendered as plain text with Markdown DEFERRED, so the page renders no clickable outbound `<a href>`. There is therefore no navigation that would emit a Referer header leaking the run/interrogation id to a third party. (Caveat: if the deferred Markdown slice later renders links, it must add `referrerpolicy`/`rel="noreferrer"` — out of scope now, noted for that future slice.)
- Caching: the GET routes are bookmarkable/cacheable by design, but they are served only over the loopback bind behind the service token, so there is no shared proxy/CDN cache — only the operator's own browser disk cache, which is local to the operator's machine. If stricter handling is wanted, a `Cache-Control: no-store` on the interrogation routes is a cheap hardening option; it is not in the current slice and I'd treat it as optional given loopback-only serving.
- Copy/export: the slice adds NO new export endpoint. The HTML page is copyable by the operator (local), and Slice 1's JSON GET route returns the same curated turns to curl/CI — that is the intended, curated read surface, not a new leak. No file is written, no archive is produced.

Net: field exposure is bounded by the read projection + D028 + the escaping/curation test; cross-boundary leakage is bounded by embedded-only assets, CSP, no outbound links, and loopback+token serving. The two things I will not overstate are (a) bodies can contain authored secrets that this slice does not scrub, and (b) browser-local disk caching exists and could be suppressed with `no-store` if the operator wants belt-and-suspenders.

