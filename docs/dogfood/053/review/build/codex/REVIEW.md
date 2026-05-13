---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["threat_model", "rfc-0046", "v1.7", "build", "lane-evidence"]
---

author: reviewer-unknown-model-001

# Threat Model Review: RFC 0046 Lane Evidence Guard

## Verdict

NEEDS REVISION.

RFC 0046 correctly places the fix at `publish-artifact` rather than the
byline derivation layer, and it acknowledges the right trust boundary:
model bylines require independent process evidence. The current design still
permits model-byline artifacts whose bytes were not produced by the supervised
lane process, because the proposed proof is path membership in
`observed_output_paths_json`, not artifact-content production.

## Trust Boundaries

- Operator and lane subprocess: the operator controls the filesystem and can
  write repository files outside the supervised subprocess.
- Supervised wrapper and process execution row: the wrapper records
  `process_executions`, but the RFC treats the resulting row as authoritative
  evidence for artifact authorship.
- Filesystem and live runner state: repository files are durable provenance;
  `.striatum/state.sqlite3` remains the live state and audit surface.
- Publisher CLI and artifact store: `publish-artifact` validates scope,
  byline, schemas, and now lane evidence before admitting an artifact row.
- Downstream consumers and UI/dashboard: consumers may read the artifact row,
  event log, dashboard, UI, or only the checked-in Markdown artifact.

## Attack Surfaces

- `observed_output_paths_json` path matching.
- Operator writes to an expected artifact path before or during supervised
  process execution.
- Process execution state transitions (`completed`, `lost`, `timed_out`) and
  any crash/recovery path that might leave stale observed paths.
- The override flags:
  `--allow-no-process-execution --override-rationale`.
- Audit and presentation surfaces for overridden publications: artifact row,
  event log, dashboard, UI, and repository Markdown.

## Findings

### F1: Path-only evidence does not prove lane-authored artifact content

Severity: High.

The RFC's core acceptance criterion says a model-byline artifact may publish
when a completed `process_executions` row for the same session includes the
artifact path in `observed_output_paths_json`. That proves only that the path
was observed in relation to the supervised session. It does not prove that the
supervised lane process wrote the artifact bytes, wrote the final bytes, or
even emitted meaningful output.

An operator can still run or keep alive an attested lane process while writing
the required Markdown file directly. If the observer later records the path,
the guard passes and the artifact receives a model byline. This is close to
the threat RFC 0046 is meant to close: operator-authored content can still be
admitted as lane-authored content, with the only added requirement being that
a supervised process overlapped the path observation.

The non-goal of cryptographic byte signatures is reasonable for V1.7, but the
RFC should not claim "the supervised subprocess actually produced the
artifact" unless it adds a stronger production check. Minimal mitigations
within V1.7 could include recording artifact content hashes at observation and
publish time, requiring the final publish hash to match the process-observed
hash, and refusing paths that existed before process start unless the process
also changed them during its execution window. A weaker but still important
mitigation is to downgrade the guarantee in product language to
"process evidence covered this path" rather than "the model produced this
artifact."

### F2: Override preserves the model byline while moving the warning out of band

Severity: Medium.

The override path intentionally allows publication without process evidence
when the operator supplies a rationale. The RFC records the rationale in a new
artifact column and event log, and it plans UI/dashboard chips. That is useful
for live-state consumers, but it does not change the artifact byline itself.

The durable Markdown artifact can still contain a canonical model byline such
as `author: reviewer-codex-gpt-5.5-001` even when the database knows the
publish was operator-overridden. Any consumer that reads only repository files
or rendered artifact text loses the override signal and sees an apparently
normal model-authored artifact.

For this threat model, the override needs an artifact-local signal or a byline
rewrite. Acceptable mitigations include rewriting overridden publications to
an operator byline, requiring an override field in the artifact front matter
for front-matter-bearing artifacts, or documenting that repository Markdown is
not a sufficient provenance surface once override is used. The current design
otherwise reintroduces a model-byline forgery path for consumers outside the
live database.

## Acceptance Notes

- Operator bylines passing without process evidence is acceptable; the trust
  claim is explicit.
- Reusing exit code 6 for artifact refusal is acceptable.
- A schema column for `attestation_override_rationale` is appropriate, but it
  should be paired with a consumer-visible provenance marker or byline policy.
- Dashboard and web UI parity are useful secondary surfaces, not sufficient
  mitigations for repository-file-only readers.
