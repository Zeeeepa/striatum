# RFC 0115: Precise token-usage telemetry for supervised Claude and agy lanes

Status: proposed
Date: 2026-06-06
author: proposer-codex-gpt-5-001
Context: RFC 0088 (daemon-owned PTY lanes and agy migration), RFC 0089
(tmux-backed lane monitoring), RFC 0096 (supervised lane trust boundary),
RFC 0101 (robust autonomous workflow execution), RFC 0102 (operator attention
economy), RFC 0109 (agy lane first-class seat), RFC 0111 (in-band failure
legibility); `~/git/token-dashboard`, `.striatum/bin/*-supervised-wrapper.sh`,
`.striatum/bin/token-usage-helper.sh`.

## Problem

The local token-burn dashboard can measure Codex usage exactly from Codex
`token_count` events, but Claude Code and agy/Gemini usage are still less
trustworthy:

- Claude supervised packet logs currently default to plain text unless the
  wrapper invokes Claude Code's structured stream output. The dashboard falls
  back to local Claude task-file estimates when no explicit counters are found.
- agy/Gemini supervised logs already use `--output-format stream-json`, and
  the final `result.stats` object often carries usable token counters, but the
  fidelity and precedence are not yet a Striatum contract.
- The dashboard ingest reads both wrapper instrumentation and packet/PTY logs.
  Without an explicit precedence rule, the same supervised packet can be counted
  once from an instrumented usage record and again from a lower-fidelity scraped
  log.

That makes the operator's burn picture noisy exactly where it matters: the
multi-lane work Striatum supervises. It also makes "what should the computer do
next?" harder to answer, because source/fidelity labels are operational inputs,
not vanity metadata.

## Goals

1. Emit one scrubbed JSONL usage record per supervised Claude and agy packet
   whenever the installed CLI exposes usage counters.
2. Normalize Claude and agy counters into the same lane/day/source vocabulary
   the token dashboard already uses.
3. Make dashboard ingest prefer instrumented packet records over scraped packet
   or PTY-derived events for the same packet.
4. Preserve the privacy boundary: no raw prompts, raw transcripts, capability
   tokens, bearer tokens, auth JSON, cookies, API keys, or provider credential
   material in usage telemetry.

## Non-Goals

- No Striatum database schema change for V1. Usage telemetry remains local
  scratch JSONL and dashboard input.
- No provider billing reconciliation. Provider bills may remain the source of
  financial truth; this RFC is about operational burn attribution.
- No public telemetry, hosted control plane, SaaS integration, or cross-machine
  collection.
- No raw provider transcript capture. PTY/log diagnostics remain local
  diagnostics per RFC 0088/D028 boundaries, not provenance or telemetry payloads.

## Proposal

### 1. Structured Claude supervised output

Change the Claude supervised wrapper's packet invocation to request structured
stream output:

```bash
claude --print --output-format stream-json --verbose \
  --permission-mode acceptEdits --allowedTools "Bash"
```

The wrapper still preserves existing packet behavior and exit semantics:

- Per-packet stdout/stderr continue to land in
  `.striatum/scratch/claude-logs/packet-NNNN.log`.
- The wrapper appends `## exit=<rc>` after the child exits.
- Usage extraction failure is non-fatal and must not change packet completion,
  lease behavior, or wrapper exit policy.

The usage helper reads only structured usage fields from the log stream. It must
not copy message content or tool input/output into telemetry.

### 2. agy/Gemini stream usage as a first-class source

The agy supervised wrapper already runs Gemini in headless stream mode:

```bash
gemini --prompt - --output-format stream-json --approval-mode yolo
```

The helper treats these stream fields as instrumented usage when present:

- Final `result.stats` aggregate counters.
- Nested `usageMetadata` / `usage_metadata` objects.
- Per-model `stats.models` details may be retained only as scrubbed model labels
  and counters, not raw content.

For V1 the dashboard lane remains `gemini_agy`; the source label should name the
record as supervised agy/Gemini usage rather than a generic PTY scrape.

### 3. Normalized per-packet usage record

The helper appends compact JSONL records to:

```text
.striatum/scratch/token-usage.jsonl
```

Each record should include only scrubbed metadata:

- `schema_version`
- `timestamp`
- `lane`
- `packet_number`
- `packet_id`, `run_id`, `job_id`, `session_id`, `supervisor_id` when available
- `source_log_path`
- `exit_code`
- `fidelity`
- `evidence`
- token fields:
  `input_tokens`, `cached_input_tokens`, `cache_creation_input_tokens`,
  `cache_read_input_tokens`, `output_tokens`, `reasoning_output_tokens`,
  `thinking_tokens`, `total_tokens`
- optional non-secret attribution fields when available:
  `model`, `provider_request_id`

`total_tokens` is authoritative only when the CLI/provider emits it. Otherwise
the helper computes it from known components and labels the record accordingly.

Fidelity vocabulary:

- `instrumented` for supervised wrapper records derived from CLI-emitted usage.
- `exact` only for provider/API/billing counters that are authoritative for the
  request.
- `scraped` for visible terminal/log counters.
- `estimated` for inferred local-file estimates.

### 4. Dashboard ingest precedence

The token dashboard ingest must prefer higher-fidelity events when multiple
events describe the same supervised packet.

Precedence key order:

1. Exact `packet_id` match.
2. Exact `source_log_path` match.
3. Exact `(lane, session_id, packet_number)` match when all three are present.

If an event from `striatum-instrumented-usage` matches a lower-fidelity
`striatum-supervised-packet` or `striatum-supervisor-pty` event, the lower
fidelity event is suppressed from aggregation. It may remain in private
diagnostics if useful, but it must not contribute to daily totals.

Codex remains unchanged: Codex session JSONL `last_token_usage` is still the
preferred Codex source for the dashboard, with wrapper instrumentation serving
only as supervised-packet corroboration.

### 5. Optional Claude Code OpenTelemetry follow-up

For Claude usage outside Striatum supervised packets, the dashboard may ingest
Claude Code OpenTelemetry metrics when the operator configures a local exporter.
That follow-up is optional for this RFC because it crosses from Striatum wrapper
behavior into operator machine telemetry configuration.

If implemented, OTel ingestion must keep the same privacy rule: counters and
stable request attribution only, no prompt or tool payload export.

## Acceptance Criteria

- A supervised Claude packet whose stream contains usage fields emits a nonzero
  `claude_code` record in `.striatum/scratch/token-usage.jsonl`.
- A supervised agy packet whose stream contains `result.stats` emits a nonzero
  `gemini_agy` record in `.striatum/scratch/token-usage.jsonl`.
- Wrapper syntax checks pass for all supervised wrappers and the usage helper.
- Token-dashboard ingest suppresses lower-fidelity scraped events when a
  matching instrumented packet record exists.
- Dashboard public data contains no raw prompts, raw logs, full home paths,
  credentials, bearer tokens, auth JSON, capability tokens, or API keys.
- Unit tests cover Claude stream usage, Gemini `result.stats`, no-counter
  fallback, and instrumented-over-scraped precedence.

## Open Questions

1. Should Claude Code OpenTelemetry be part of this RFC's implementation or a
   separate dashboard-only follow-up?
2. Should per-model Gemini stats be surfaced in the dashboard detail drawer, or
   retained only in private `token-events.jsonl`?
3. Should supervised-packet usage records become daemon-visible read data after
   V1, or remain operator-local scratch indefinitely?

## Domain Modeling

This RFC adds a boundary value object: **usage telemetry record**. It is not a
workflow event, artifact, verdict, or audit-chain entry. It is local operational
metadata derived from a supervised packet's CLI output and consumed by operator
dashboards. That placement keeps token-burn observability useful without
turning provider transcript material into Striatum workflow state.
