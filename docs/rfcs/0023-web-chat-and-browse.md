# RFC 0023: Web Chat And Codebase Browse

Status: accepted (V1+V1.5)
Date: 2026-05-09
Context:
RFC 0009 (long-lived process supervision, accepted) — the
supervisor primitive an alternative chat backend could use,
RFC 0012 (local service API, accepted V1) — the HTTP /
SSE plumbing,
RFC 0013 (local web UI, accepted V1+step 7) — the existing
mutation gate + CSP shape,
RFC 0022 (web UI redesign, accepted V1) — the server-rendered
multi-page foundation; V1.5 deferred inline Markdown rendering
to a future dogfood, which this RFC subsumes,
`docs/AGENTS.md` § "Product Boundary" — *no hosted striatum
services* and *no implicit cloud calls without an RFC*; this
RFC is that explicit decision for chat,
`docs/DECISION_LOG.md` D006/D009/D028, superseded for current
substrate/interface behavior by D094/D104 — daemon methods are the
legal production write boundary; daemon-owned PostgreSQL is
authoritative live state; transcripts deliberately off.

## Problem

The browser and the terminal are split today, and the seam shows
up in three places:

1. **Driving an agent / chatting with a model requires a
   terminal.** The web UI shows runs, jobs, verdicts, blockers,
   and (post RFC 0022 V1) a live SVG dependency graph. To
   actually *talk* to a model — read its output, type a follow-
   up, watch it stream — the operator switches to a separate
   terminal program (Claude Code, Codex, Gemini CLI, or a
   browser tab on Claude.ai / ChatGPT / etc.). Some of those
   terminal programs are themselves cloud-API clients; the
   "stay local" rule never meant "no cloud APIs," it meant
   "no hosted *striatum* service."
2. **Browsing the codebase happens elsewhere.** Operators
   inspecting why a job failed open the file in an IDE, on
   GitHub, or via `cat`/`less` in another pane. None of those
   surfaces know which run is open, which artifact a path
   belongs to, or whether the file is one of the seven
   DDD-scaffolded canonical docs (RFC 0021).
3. **Markdown artifacts are linked but not rendered.** Per RFC
   0022 V1's BUILD_HANDOFF, inline Markdown rendering on
   `/run/<id>/artifact/<id>` was deferred to V1.5. The current
   page links to the raw API; the operator copy-pastes into a
   Markdown viewer or `bat` to read it.

This RFC adds two surfaces to the web UI — **chat** (talk to a
configured model provider through the browser) and **browse**
(inspect the target repo, with inline Markdown rendering for
artifacts the runner already knows about). Chat is a
*provider-neutral client*: striatum streams HTTP requests to a
chat-completion endpoint the operator chooses (Anthropic,
OpenAI, OpenRouter, Ollama, vLLM, LiteLLM proxy — anything
speaking the OpenAI Chat Completions or Anthropic Messages
shape). Striatum is not a SaaS; it doesn't host a model; it
doesn't handle the operator's API key past forwarding it on
outbound requests; it doesn't telemeter to anyone except the
provider the operator chose.

## Goals

- **Chat surface in the web UI, backed by a configurable
  provider.** A new `/chat/<session_id>` page streams model
  responses via SSE and posts user input to a chat-completion
  endpoint. The provider, base URL, model name, and API key
  are operator-configured (env vars at startup; per-session
  overrides via UI deferred to V1.5). Striatum is a client,
  not a provider; the operator points it at whichever endpoint
  they trust.
- **Codebase browse surface.** A new `/browse/<path>` page
  walks the target repo's working tree, scoped to `--repo`,
  with `.git/` and `.striatum/` excluded by default. File
  contents render with light syntax shading (server-side, no
  client-side highlighting library). Markdown files render as
  HTML; everything else renders as `<pre>` with monospace.
- **Inline Markdown rendering on artifact pages.** Closes RFC
  0022 V1.5 deferred. The artifact-view page renders `.md`
  artifacts as HTML in the same way `/browse/<path>` does,
  with the artifact's metadata as the page header.
- **Mutation-gated chat startup.** Starting a chat session
  involves outbound HTTP — that's a state-changing operation
  worth gating, so it sits behind the existing
  `--allow-mutations` flag (or a new `STRIATUM_WEB_CHAT=1`
  env, per Open Question 1).
- **Preserve the local-first invariants that still apply.**
  No hosted *striatum* service; no telemetry to anyone other
  than the provider the operator chose; no transcript
  persistence by default beyond the supervisor's existing
  scratch JSONL. The wheel adds at most one Python runtime
  dep (Markdown renderer); no Node toolchain.

## Non-Goals

- **No striatum-as-a-service.** Striatum binds localhost by
  default; the chat endpoint is loopback-only; the API key
  the operator sets is forwarded outbound to the provider but
  is never written to SQLite or to durable artifacts.
- **No hidden default provider.** V1 refuses to spawn a chat
  session unless `STRIATUM_CHAT_API_BASE_URL` and
  `STRIATUM_CHAT_API_KEY` (and `STRIATUM_CHAT_MODEL`) are set
  on the running service. There is no "fall back to
  api.anthropic.com" or "use ollama if no key" behavior;
  operators opt in explicitly.
- **No tool use / agentic loop in V1.** V1 ships pure
  text-only chat: messages in, tokens streamed out. The model
  cannot call `striatum status` or read files on its own.
  V1.5 adds a small tool-use surface bound to a closed set of
  read-only striatum verbs. Agentic loops with file editing
  remain the agent CLIs' (Claude Code, Codex) territory.
- **No transcript persistence by default.** Per D028
  (transcripts off), V1 holds the chat exchange in browser
  memory + a per-session scratch file under
  `.striatum/scratch/chat-<session>/transcript.jsonl` for
  reload purposes. The scratch file is gitignored
  (`.striatum/` is gitignored). It is *not* an artifact; it
  is *not* indexed by SQLite. V1.5 may add an opt-in
  "promote this chat to a durable artifact" mutation.
- **No code editing in the browser.** Read-only file browser.
  Code edits flow through the agent's own tool calls
  (Edit/Write in Claude Code's case); striatum doesn't expose
  a "save this file" button. The bounded context is
  coordination, not authoring.
- **No multi-operator chat.** Localhost-only. One browser tab
  drives one chat session. Two operators wanting to share a
  session are out of scope (and would push striatum into
  hosted-service territory).
- **No replacement for terminal agent CLIs.** Claude Code,
  Codex, and Gemini CLI are full agent surfaces with
  features (slash commands, custom skills, file attachments)
  striatum's chat can't replicate in V1. Operators wanting
  full agentic file-editing workflows still run those tools;
  V1 of this RFC is for the "I want to ask a model
  questions about my repo" workflow.
- **No syntax-highlighting library.** The browse surface uses
  a tiny server-side mapping (extension → muted CSS class) for
  visual differentiation. A real syntax library
  (Prism / highlight.js / Pygments) is V1.5 if operators ask.

## Proposal

V1 ships in four landable steps.

### Step 1. Provider configuration + chat session lifecycle

The operator starts the service with chat-provider env vars:

```
STRIATUM_CHAT_API_BASE_URL=https://api.anthropic.com
STRIATUM_CHAT_API_KEY=sk-ant-...
STRIATUM_CHAT_MODEL=claude-opus-4-5
STRIATUM_CHAT_API_FLAVOR=anthropic_messages   # or openai_chat
```

`STRIATUM_CHAT_API_FLAVOR` selects which request/response shape
striatum sends:

- `anthropic_messages` — POST `/v1/messages` with `model`,
  `messages`, `stream: true`. Headers: `x-api-key`,
  `anthropic-version`. Streamed response uses Anthropic's SSE
  event vocabulary (`message_start`, `content_block_delta`,
  etc.).
- `openai_chat` — POST `/v1/chat/completions` with `model`,
  `messages`, `stream: true`. Header: `Authorization: Bearer`.
  Streamed response uses OpenAI's `data: {...}\n\n` SSE shape
  with `choices[0].delta.content` chunks.

Two flavors cover the Anthropic Messages API + every
OpenAI-Chat-compatible endpoint (OpenAI, OpenRouter, Together,
Groq, Mistral, vLLM, LiteLLM proxy, Ollama with `--api-base
http://localhost:11434/v1`). V1.5 may add `gemini_messages`
or per-provider quirks (Anthropic's Messages-Beta features,
OpenAI's Responses API).

Routes:

- `POST /chat/new` (mutation-gated) creates a chat session,
  generates a `chat_<short-uuid>` id, creates
  `.striatum/scratch/chat-<id>/transcript.jsonl`, and
  redirects to `/chat/<id>`.
- `GET /chat/<id>` server-renders `chat.html` with the
  transcript replayed up to the last 200 messages (cap is
  Open Question 6).
- `POST /chat/<id>/send` accepts a `message` form field,
  appends it to the transcript JSONL, fires off an outbound
  request to the configured provider, and streams the
  response chunks both to the SSE listener AND to the
  transcript JSONL (so a page reload mid-stream picks up the
  partial message).
- `GET /chat/<id>/events` SSE-streams new transcript-JSONL
  appends to the browser. Reuses the existing SSE
  infrastructure (RFC 0012); adds a per-chat-session
  concurrent-listener cap (Open Question 4).
- `POST /chat/<id>/stop` (mutation-gated) marks the session
  closed (transcript is left in scratch; cleanup on operator-
  triggered `striatum chat purge` deferred to V1.5).

The chat page UI:

- Left rail: chat session list (with state, started-at,
  message count). Click navigates to `/chat/<other-id>`. "+
  New Chat" button (mutation-gated).
- Center: message stream. User messages right-aligned, model
  responses left-aligned. Markdown rendering via the same
  `striatum.web.markdown` module Step 3 introduces — code
  fences render properly; the operator can paste a code
  block and read it back nicely formatted.
- Bottom: textarea + send button. Enter sends; Shift+Enter
  newline. Disabled while a response is streaming.

A small JS island (`/static/chat.js`) handles the SSE listen
+ form posting; no inline scripts; CSP unchanged.

### Step 2. File browser (`/browse/<path>`)

`GET /browse/` shows the repo root.

`GET /browse/<path>` shows:

- If `<repo>/<path>` is a directory: a sorted list of
  children (directories first, files second, hidden files
  hidden by default but reachable via `?show=hidden`).
- If `<repo>/<path>` is a regular file:
  - `.md` → rendered to HTML via the chosen Markdown library
    (Open Question 3) and embedded in `browse_file.html`.
  - Anything else → embedded as
    `<pre><code class="lang-{ext}">...</code></pre>` with HTML
    escaping. The page does not load a syntax-highlighting
    library; the `lang-{ext}` class lets the CSS apply muted
    differentiation if desired.

Path safety: `..`, leading `/`, null bytes, and any path that
escapes `<repo>/` resolve as 400. Symlinks pointing outside
the repo are refused. `.git/` and `.striatum/` are
hidden-by-default; reachable only via `?show=hidden`. Binary
files (detected via the first 1024 bytes containing a null
byte, or extension blacklist `.png`/`.jpg`/`.pdf`/etc.) render
as a metadata panel + a "raw bytes" link instead of the file
contents.

The header includes a breadcrumb of the path components. A
small JS island provides an "Add to chat" button on each
file: clicking it appends `path: <repo-relative-path>` to the
input of the most-recent open chat tab via `BroadcastChannel`
(or `postMessage` fallback). V1 is the path string only;
V1.5 may attach the file *content* as a chat message.

### Step 3. Inline Markdown rendering on `/run/<id>/artifact/<id>`

The artifact-view page loaded for a `.md` artifact reads the
artifact's bytes from disk and renders the body as HTML in the
same way `/browse/<path>` does. The metadata header (artifact
ID, sha256, kind, author byline) stays at the top.

For non-Markdown artifacts (`.json`, `.svg`, etc.), behavior
is unchanged — metadata + raw API pointer.

This closes RFC 0022 V1.5's deferred Markdown rendering
without a separate dogfood.

### Step 4. Per-page navigation + chat index

`base.html` gains two new top-nav links: **Chat** (lists open
chat sessions; "+ New Chat" button when mutations are
allowed) and **Browse** (`/browse/`). The existing **Runs** /
**Doctor** links stay.

The `chat_index.html` page is a list of currently-open chat
sessions (those with a transcript JSONL under
`.striatum/scratch/chat-*/`), each with started-at timestamp,
message count, and a "Resume" link. When mutations are
allowed: a "+ New Chat" button at the top.

## Acceptance Criteria

- A fresh `pip install striatum-orchestrator` + a
  startup with the four chat-provider env vars +
  `striatum serve --web --allow-mutations` produces working
  pages at `/chat/new` (POST), `/chat/<id>` (GET),
  `/browse/`, `/browse/<path>`, and an updated
  `/run/<id>/artifact/<id>` that renders Markdown inline for
  `.md` artifacts.
- A startup *without* the chat env vars still produces
  working `/browse/<path>` and `/run/<id>/artifact/<id>`
  pages; only the chat surface refuses (the chat-index nav
  link renders a "Configure provider to enable chat"
  empty-state).
- Sending a message via `POST /chat/<id>/send` produces an
  outbound HTTPS request to the configured base URL with the
  configured headers + body, and the SSE stream surfaces the
  streamed response within the provider's first-token latency.
- The chat transcript JSONL under
  `.striatum/scratch/chat-<id>/transcript.jsonl` is the only
  on-disk record. It is gitignored. SQLite is unchanged. No
  artifacts are published.
- The browse surface refuses path-traversal attempts (`..`,
  absolute paths, symlinks escaping the repo) with HTTP 400.
  `.git/` and `.striatum/` are hidden by default.
- Markdown rendering on `/browse/<path>` and
  `/run/<id>/artifact/<id>` produces HTML that passes the
  CSP — no inline `<style>`, no inline event handlers in the
  rendered body, no `<script>` (sanitizer enforced).
- The CSP header is byte-identical to v1.11.0; no
  `unsafe-inline` / `unsafe-eval` added. The chat page's
  outbound HTTPS to the configured provider is initiated by
  the *server*, not the browser; from the browser's
  perspective everything is same-origin (`connect-src
  'self'` is sufficient).
- A network failure on the outbound chat request produces a
  visible error in the chat stream with a retry button; the
  transcript records the failure as a `system` role entry
  and stays consistent.
- `tests/test_web_chat.py` covers: chat-session creation,
  outbound request shape against both flavors via a mock
  HTTP server, SSE streaming round-trip, transcript JSONL
  round-trip, the no-chat-config-no-chat-page path, and
  graceful provider-error handling.
- `tests/test_web_browse.py` covers: directory listing,
  file rendering, Markdown rendering, path-traversal
  refusal, hidden-file gating, binary-file fallback, and
  the "add to chat" island wiring.
- `make lint`, `make typecheck`, full `make test` pass.

## Open Questions

- **Q1: Mutation gate or separate `STRIATUM_WEB_CHAT=1` env?**
  V1 reuses the existing `--allow-mutations` flag for
  consistency. The risk is that an operator who wants
  read-only mutation buttons on `/run/<id>` (verdict, decision)
  but no chat-spawning ability has no way to express that.
  V1.5 may split the gates if the friction shows up.
  *Recommendation: reuse `--allow-mutations` for V1.*
- **Q2: API flavor surface.** Two flavors (`anthropic_messages`
  + `openai_chat`) cover Anthropic + every OpenAI-Chat-
  compatible endpoint. Adding `gemini_messages` is a V1.5
  ask if Gemini-only operators show up. *Recommendation:
  ship the two flavors; document that
  Gemini-via-OpenAI-compat is the V1 path for Gemini
  operators (e.g. via Google's Vertex OpenAI-compat shim).*
- **Q3: Markdown library choice.** Three candidates:
  - `markdown-it-py` — Python port of `markdown-it`, ~150
    KB, well-maintained, supports CommonMark + GFM. Pulls in
    `mdurl` (~10 KB).
  - `mistune` — fast, single-file, ~80 KB, hand-rolled.
    Looser CommonMark conformance but enough for striatum's
    artifacts.
  - Hand-roll a CommonMark subset (~200 LoC of Python). Zero
    deps, but maintenance cost on edge cases (escaping,
    nested lists, code fences with attributes).
  *Recommendation: `markdown-it-py`. Striatum already added
  Jinja2 in v1.11.0; one more well-maintained Python lib for
  HTML correctness is the same trade-off.*
- **Q4: SSE backpressure for chat.** A chat stream emitting
  thousands of tokens per second through SSE could
  back-pressure the browser. The existing SSE infrastructure
  has a `SSE_MAX_CONCURRENT_PER_RUN = 32` cap; the chat
  stream isn't run-scoped. V1 caps each chat at one
  concurrent SSE listener (additional listeners get 429); the
  SSE flush is naturally rate-limited by the provider's
  upstream rate.
- **Q5: File-browser pagination.** A directory with 10,000
  entries is rare in striatum-managed repos but possible.
  V1 lists everything in one page; V1.5 may paginate when
  child count exceeds 500.
- **Q6: Chat history replay scope.** A chat session that has
  been running for hours produces megabytes of transcript
  JSONL. Re-rendering the full history on every page load is
  expensive. V1 caps initial replay to the last 200 messages;
  V1.5 may add infinite scroll. The full transcript is always
  on disk in the scratch JSONL.
- **Q7: Markdown sanitization.** `markdown-it-py` produces
  HTML that may include `<script>` tags from raw HTML in the
  source (when CommonMark "raw HTML" is enabled). V1 runs
  the rendered HTML through a small allowlist sanitizer
  (block `<script>`, `<iframe>`, `<object>`, `<embed>`,
  `on*=` attributes, `javascript:` hrefs). Operators trust
  their own repo's Markdown, but the sanitizer is a defense-
  in-depth layer, especially for chat-streamed Markdown
  produced by a model that didn't necessarily intend HTML
  injection.
- **Q8: Should chat use the supervisor primitive (RFC 0009)
  instead?** Two architectures considered:
  - Direct provider client (this RFC's V1): striatum sends
    HTTP, streams response back. ~300 LoC of new client code.
  - Supervised agent CLI: striatum starts `claude code` /
    `codex` / `gemini` as a subprocess, multiplexes its
    stdio. Reuses RFC 0009; ~50 LoC of new shell code; but
    requires the operator to have an agent CLI installed
    AND configured.
  *Recommendation: V1 ships direct-provider only; V1.5 adds
  a `--chat-backend supervised` flag for the supervised-CLI
  path. Two backends, one UI.*
- **Q9: API-key handling.** V1 reads from env var only;
  the key is forwarded outbound but never logged, never
  written to SQLite, never echoed in error messages
  (errors say "provider returned 401" not "API key
  sk-ant-... rejected"). V1.5 may add an OS-keyring
  integration (`keyring` Python lib) for users who don't
  want the key in their shell history.
- **Q10: Cost surface.** Chat sessions cost the operator
  money on metered providers. V1 surfaces the model name +
  the number of tokens in/out per response (parsed from the
  provider's response metadata when available); does not
  attempt to compute dollar costs (provider price tables
  shift, and striatum doesn't ship one).

## Implementation Path

V1 ships as **v1.12.0** (minor bump because RFC 0023
introduces a new top-level UI surface and a new runtime
dependency).

The four steps land in order:

1. **Step 1 (chat lifecycle):** new
   `striatum.web.chat_provider` module with
   `AnthropicMessagesClient` + `OpenAIChatClient` classes
   wrapping outbound streaming HTTP. `service.py` adds
   `/chat/*` route handlers; new `chat.html`,
   `chat_index.html` templates; `chat.js` JS island for SSE
   + form posting; tests at `tests/test_web_chat.py` (with
   a fake HTTP server fixture to assert request shape).
2. **Step 2 (file browser):** `service.py` adds
   `/browse/*` route handlers; new `browse_dir.html`,
   `browse_file.html` templates; `striatum.web.markdown`
   module wrapping the chosen Markdown library + sanitizer;
   tests at `tests/test_web_browse.py`.
3. **Step 3 (artifact Markdown rendering):** updates
   `artifact_view.html` to call into the same
   `striatum.web.markdown` module; tests extending
   `tests/test_web_ui_redesign.py`.
4. **Step 4 (nav + chat index):** `base.html` adds the new
   top-nav links; `chat_index.html` renders a list of open
   sessions; tests for nav presence + index rendering.

V1.5 candidates (each its own dogfood):

- Tool use (chat can call read-only striatum verbs).
- Supervised-agent-CLI backend (`--chat-backend supervised`).
- "Promote chat to artifact" durable-save mutation.
- Multi-agent side-by-side comparison.
- File-attachment in chat (paste-a-file from browse).
- File-browser pagination.
- Syntax highlighting via Pygments (server-side).
- OS-keyring API-key storage.

## Domain Modeling

This RFC adds four value objects and one boundary
clarification:

- **chat session** (value object) — a named conversation
  with a configured provider. Identity is a `chat_<short-uuid>`
  string; equality by id. Lifecycle: created on `POST
  /chat/new`, closed on `POST /chat/<id>/stop`, transcript
  retained until manually purged.
- **chat provider config** (value object) — the four-tuple
  `(api_base_url, api_key, model, api_flavor)`. Constructed
  at service startup from env vars; never mutated in flight;
  never persisted by striatum (the api_key field passes
  through in-memory only).
- **browse path** (value object) — a repo-relative path
  resolved through the path-safety check. Identity is the
  resolved string; equality by string. Constructed at
  request time; never persisted.
- **markdown render policy** (value object) — the
  sanitization allowlist. Closed set in V1: block `<script>`,
  `<iframe>`, `<object>`, `<embed>`, all `on*=` attributes,
  all `javascript:` hrefs. V1.5 may make the policy
  configurable.

Boundary clarification (cite this when future RFCs are
tempted): **striatum is a local client, not a service.**
Outbound network calls to operator-chosen endpoints are
fine when an RFC says they are; striatum *itself* binds
loopback, doesn't accept inbound traffic from non-localhost
peers, and doesn't telemeter. The chat surface is the first
case-by-case opt-in to outbound HTTP from the runtime; future
features (e.g., automatic embedding generation, web-fetch
tool calls) can cite this boundary when proposing similar
opt-ins.

Per `docs/DDD.md § "Adding to the model"`:

1. **Glossary** — `docs/UBIQUITOUS_LANGUAGE.md` adds entries
   for `chat session`, `chat provider config`, `browse
   path`, `markdown render policy`.
2. **Pattern** — four value objects + one boundary
   clarification (above).
3. **Validator** — `chat provider config` is validated at
   startup: missing env vars produce a clear "chat disabled;
   set STRIATUM_CHAT_API_BASE_URL etc." log line. Unknown
   `STRIATUM_CHAT_API_FLAVOR` values refuse with exit code 8
   (workflow validation parity).
4. **Surface** — visible in the chat / browse pages.
   Provider config is also surfaced in `striatum doctor` as
   a "chat configured: yes/no" line (V1 doesn't print the
   key or model; just the boolean and the flavor).
5. **Citation** — DECISION_LOG D074 cites the new entries
   when the V1 PR lands. The D074 row also explicitly
   accepts the "outbound HTTP from striatum" boundary
   carve-out so future RFCs can cite it.

## Relationship To Other RFCs

- **RFC 0009** (long-lived process supervision) — V1 does
  not use the supervisor for chat; V1.5's
  supervised-CLI-backend option does. The supervisor
  primitive remains the canonical answer for "long-running
  agent process" use cases.
- **RFC 0012** (local service API) — extends the route
  table; no API protocol changes; SSE infrastructure
  reused. The chat SSE stream is a new subscription kind
  in the same `/events` family.
- **RFC 0013** (local web UI) — the existing
  `--allow-mutations` flag gates chat-session startup. No
  CSP relaxation; no new mutation verbs at the JSON API
  boundary.
- **RFC 0022** (web UI redesign) — V1 extends RFC 0022's
  Jinja2 template tree with five new templates
  (`chat.html`, `chat_index.html`, `browse_dir.html`,
  `browse_file.html`, plus updates to `artifact_view.html`).
  Same SSR pattern, same CSS palette + dark mode.
- **RFC 0021** (DDD layout scaffold) — unrelated.
- **D028** (transcripts off) — *carved out* for chat. The
  scratch JSONL transcript is retained for reload purposes
  but stays in `.striatum/scratch/` (gitignored, not
  SQLite-tracked). D074 documents the carve-out and its
  scope: only the chat surface; only the operator's local
  filesystem; never an artifact.
- **D006/D009** (CLI as only-legal-write-surface; SQLite
  as authoritative live state) — preserved. The chat UI
  does not introduce new write paths to SQLite. The
  scratch JSONL is durable but ephemeral by convention,
  the same way `.striatum/scratch/<process>/` is for
  supervised processes.
- **AGENTS.md "Product Boundary"** — *partially carved
  out for V1*. The boundary said "no cloud APIs without an
  explicit product decision"; this RFC is that decision.
  The carve-out is *narrow*: outbound HTTP to an
  operator-configured chat endpoint, only when the operator
  starts the service with the chat env vars set. No
  default endpoint; no opportunistic cloud calls; no
  telemetry. Future RFCs proposing other outbound calls
  cite this RFC's boundary clarification.
