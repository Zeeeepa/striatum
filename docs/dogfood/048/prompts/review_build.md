# Build Review Prompt (RFC 0043 V1, 3-way)

Produce REVIEW.md at `docs/dogfood/048/review/build/<lane>/REVIEW.md`.

Use the posture supplied in your work packet's `review_policy`:

- **codex**: `threat_model`
- **claude**: `ergonomics_dx`
- **gemini**: `threat_model` (adversarial angle)

Front matter (ALL FIVE FIELDS REQUIRED — `schema_version: "striatum.finding.v1"` exact string):

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "low"
tags: ["<posture>", "rfc-0043", "v1", "build"]
---

author: reviewer-unknown-model-<NN>
```

`schema_version` is the exact string `"striatum.finding.v1"`. `artifact_kind` is `"finding"`. `verdict_intent` is one of `accept | accept_with_findings | needs_revision | reject`. `severity` is one of `low | medium | high | critical`. The byline is a plain markdown line AFTER the front matter — no lane prefix, no markdown bold.

Read both implementation handoffs at `docs/dogfood/048/build/track_a/HANDOFF.md` and `docs/dogfood/048/build/track_b/HANDOFF.md`. Cross-cut review across both tracks.

Per-lane angle:

- **codex (threat_model)**: schema invariants (append-only grants on `events`/`artifacts` retained; `repository_id` NOT NULL; correct indexes). Audit-chain byte-equivalent re-anchor in `migrate-repo-local` actually verifies. Method registry exhaustive (no mutation in `src/striatum/cli/mutations.py` bypasses a registered method). `--no-daemon` truly removed — no silent SQLite fallback path remains. D094 framing cited; D006/D007/D036/SQLite-half-of-D009 superseded.
- **claude (ergonomics_dx)**: `striatum daemon migrate-repo-local --dry-run` output legible; tombstone semantics obvious; daemon-unreachable stderr (exit 11) names socket + platform remediation; unmigrated-repo stderr (exit 12) names `migrate-repo-local`; `daemon doctor` still works without the daemon; `--keep-sqlite-readonly` default + `--confirm-delete` UX safe.
- **gemini (adversarial threat_model)**: migrate-repo-local idempotency under concurrent invocations; `--confirm-delete` + `--keep-sqlite-readonly` flag-conflict handling; partial-migrate crash recovery (does the tombstone marker make sense after a half-applied tx?); unmigrated repo silently using migrate-repo-local from an older client; method-registry holes (a mutation that snuck in without a registered method); `--no-daemon` escape paths (env var, alias, sub-command bypass, config file).

Required checks (all lanes):

- **Migrate works**: `striatum daemon migrate-repo-local --repo <path>` against the V1 fixture produces a populated Postgres schema with byte-equivalent audit-chain anchors.
- **Idempotent re-run**: second invocation reports "already migrated" and exits 0.
- **Exit codes 11/12 fire**: daemon-down and unmigrated-repo paths produce the documented codes + stderr remediation.
- **`--no-daemon` retired**: parsing it produces the unknown-option error; no SQLite fallback executes.
- **Method registry complete**: every mutation in `src/striatum/cli/mutations.py` has a registered method per RFC 0043 §5 (named test asserts this).
- **Backward-compat**: existing test fixtures still pass against `daemon_mode=on`.
- **Tests green**: `make test` clean across both tracks.

Cite specific files / lines / test names. "Looks good" is not a review.

Use the verdict vocabulary exactly: `accept`, `accept_with_findings`, `needs_revision`, `reject`.

**IMPORTANT — write the REVIEW.md artifact directly.** If `striatum ack` is denied, write and exit normally.
