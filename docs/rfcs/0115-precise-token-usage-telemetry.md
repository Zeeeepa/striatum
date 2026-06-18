# RFC 0115: Precise token-usage telemetry for supervised agent-loop lanes

Status: proposed — deferred (D226, closes #404): product-boundary-clean as proposed (local scrubbed JSONL, no hosted/DB telemetry), but low/med priority and dependent on dashboard-ingest; schedule when dashboard-ingest lands or telemetry becomes a priority
Date: 2026-06-06
Updated: 2026-06-07
author: proposer-codex-gpt-5-001
Context: RFC 0088 (daemon-owned PTY lanes and agy migration), RFC 0089
(tmux-backed lane monitoring), RFC 0096 (supervised lane trust boundary),
RFC 0101 (robust autonomous workflow execution), RFC 0109 (agy lane
first-class seat), RFC 0111 (in-band failure legibility), v2.29.0/#199
(`claude --print` retirement); `~/git/token-dashboard`, daemon supervision
trajectory logs, optional provider OpenTelemetry.

## Problem

The local token-burn dashboard can measure Codex usage exactly from Codex
`token_count` events, but supervised Claude Code and agy/Gemini usage still
need a precise, current Striatum-shaped source.

The previous implementation sketch depended on tracked per-packet wrappers
under `.striatum/bin/` and a structured `claude --print` invocation. That path
is now invalid:

- v2.29.0 untracked `.striatum/bin/*` wrappers because they are operational
  scratch, not repository source.
- `claude --print` / `-p` is retired and refused by validation, prepare, and
  supervise launch unless explicitly overridden. After the June 15, 2026
  deadline it also burns API-token dollars rather than plan usage.
- Supported supervised lanes are daemon-owned long-lived PTY agent-loop
  sessions. The agent calls MCP (`work.await_packet`, `work.ack`,
  `artifact.publish`, `work.complete`, `review.submit`) instead of receiving
  one JSON packet on stdin.

The dashboard still needs the same outcome: a scrubbed daily burn picture whose
Claude and agy counts are not double-counted with lower-fidelity PTY scrapes.
The implementation has to attach to the supported agent-loop model rather than
reintroducing the retired wrapper model.

## Goals

1. Emit or derive one scrubbed usage event per supervised agent-loop turn when
   the installed CLI exposes counters or a configured provider metric source
   reports them.
2. Normalize Claude Code and agy/Gemini counters into the same lane/day/source
   vocabulary the token dashboard already uses.
3. Make dashboard ingest prefer instrumented per-turn records over scraped PTY
   or estimated local-file events for the same supervised work.
4. Preserve the privacy boundary: no raw prompts, raw transcripts, capability
   tokens, bearer tokens, auth JSON, cookies, API keys, provider credentials, or
   durable PTY transcript capture in telemetry.

## Non-Goals

- No revival of tracked `.striatum/bin/*` wrappers.
- No `claude --print` / `-p` implementation path.
- No Striatum database schema change for V1. Usage telemetry remains local
  operator scratch and dashboard input.
- No provider billing reconciliation. Provider bills may remain the financial
  source of truth; this RFC is about operational burn attribution.
- No hosted telemetry, SaaS integration, or cross-machine collection.
- No raw provider transcript capture.

## Proposal

### 1. Agent-loop telemetry source contract

Define a local-only usage event file:

```text
.striatum/scratch/token-usage.jsonl
```

Each line is a compact, scrubbed record produced by an operator-local collector
or by dashboard ingest from a configured local metrics source. It is not
workflow state, not an artifact, and not an audit-chain event.

V1 records only counters and stable non-secret attribution:

- `schema_version`
- `timestamp`
- `lane`
- `run_id`, `job_id`, `session_id`, `supervisor_id`, `message_id`, `lease_id`
  when available
- `source_log_path` or `metrics_source` when available, scrubbed before public
  dashboard output
- `fidelity`
- `evidence`
- token fields:
  `input_tokens`, `cached_input_tokens`, `cache_creation_input_tokens`,
  `cache_read_input_tokens`, `output_tokens`, `reasoning_output_tokens`,
  `thinking_tokens`, `total_tokens`
- optional non-secret attribution fields when available:
  `model`, `provider_request_id`

`total_tokens` is authoritative only when emitted by the CLI/provider metric.
Otherwise the collector computes it from known components and labels the record
accordingly.

### 2. Claude Code path: local metrics first, PTY scrape as fallback

Claude Code supervised lanes use the supported long-lived interactive agent
loop. V1 must not launch a one-shot `claude --print` process.

The preferred Claude path is optional local OpenTelemetry ingestion when the
operator configures Claude Code to export usage metrics to a local-only
collector. The collector may read only metric names, labels, timestamps,
request/session attribution, and token counts. It must discard or reject any
payload field that resembles prompt text, tool input/output, headers,
credentials, or transcript content.

When OTel is not configured, dashboard ingest may still scrape visible token
counters from the private supervisor trajectory log:

```text
.striatum/scratch/<supervisor_id>/pty.log
```

Those scraped events remain `fidelity: "scraped"` and must never be upgraded to
`instrumented`. `supervise.trajectory` and dashboard evidence may expose path,
size, and counter summaries, but not transcript content.

### 3. agy/Gemini path: agent-loop metrics and trajectory counters

agy is a first-class agent-loop seat (`adapter_capabilities.agent_loop: true`)
and should stay on the supported `agy --dangerously-skip-permissions` path.
The V1 collector may use either:

- CLI-emitted usage metadata in local diagnostic streams when the installed
  CLI exposes it without transcript payloads.
- Provider/local metric events that include `result.stats`,
  `usageMetadata`, or equivalent token counters.
- Visible PTY counters as a scraped fallback.

Structured agy/Gemini counters are `fidelity: "instrumented"` only when they
come from CLI/provider usage fields, not from free-form terminal text.

### 4. Dashboard ingest precedence

The token dashboard ingest must prefer higher-fidelity events when multiple
events describe the same supervised work.

Precedence key order:

1. Exact `(lane, session_id, message_id)` match when present.
2. Exact `(lane, session_id, lease_id)` match when present.
3. Exact `(lane, session_id, job_id, timestamp bucket)` match when the bucket is
   narrow enough to avoid unrelated turns.
4. Exact `source_log_path` match for trajectory-derived records.

If an event from `striatum-instrumented-usage` matches a lower-fidelity
`striatum-supervisor-pty`, `striatum-trajectory-scrape`, or estimated local
file event, the lower-fidelity event is suppressed from aggregation. It may
remain in private diagnostics, but it must not contribute to daily totals.

Codex remains unchanged: Codex session JSONL `last_token_usage` is still the
preferred Codex source for the dashboard.

### 5. Privacy and ownership boundary

Usage telemetry is a boundary value object, not workflow provenance. The
collector is allowed to look at local scratch and provider metric streams only
to extract counters. It must not copy:

- prompts, assistant messages, tool arguments, tool output, terminal
  transcripts, or raw JSONL session contents;
- bearer tokens, capability tokens, provider API keys, cookies, auth JSON,
  `.gemini/settings.json`, `.mcp.json`, or credential paths;
- private project names or absolute home-directory paths into public dashboard
  output.

The dashboard public data remains scrubbed daily aggregates with fidelity labels
and sanitized source evidence.

## Acceptance Criteria

- RFC 0115 no longer proposes or requires `claude --print`, `-p`, or tracked
  `.striatum/bin` wrapper changes.
- A supervised Claude agent-loop session with configured local usage metrics
  produces nonzero `claude_code` usage records without storing transcript
  content.
- A supervised agy/Gemini agent-loop session whose CLI/provider stream exposes
  usage metadata produces nonzero `gemini_agy` usage records.
- Without structured counters, visible PTY token counters are scraped only as
  `fidelity: "scraped"`.
- Token-dashboard ingest suppresses lower-fidelity scraped/estimated events
  when a matching instrumented agent-loop record exists.
- Public dashboard data contains no raw prompts, raw logs, full home paths,
  credentials, bearer tokens, auth JSON, capability tokens, API keys, or
  provider credential material.
- Unit tests cover Claude metric usage, agy/Gemini structured usage, no-counter
  fallback, and instrumented-over-scraped precedence.

## Open Questions

1. Which Claude Code local OTel metric names and labels are stable enough to
   support without pinning to one CLI build?
2. Should the collector live in token-dashboard only, or should Striatum expose
   a local read method that returns scrubbed usage-event candidates from
   supervisor metadata?
3. Should per-model Gemini stats be surfaced in private dashboard detail views,
   or retained only in private `token-events.jsonl`?
4. What is the safest timestamp bucketing rule for matching instrumented usage
   to scraped PTY counters when message/lease IDs are absent?

## Domain Modeling

This RFC adds a boundary value object: **usage telemetry record**. It is not a
workflow event, artifact, verdict, or audit-chain entry. It is local
operational metadata derived from a supervised agent-loop session's metrics and
consumed by operator dashboards. That placement keeps token-burn observability
useful without turning provider transcript material into Striatum workflow
state.
