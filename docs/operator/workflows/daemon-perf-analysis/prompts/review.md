# Task: Review the performance-analysis draft (evidence audit / final review)

Review the upstream `DRAFT.md` and its `SUPPORT_LEDGER.md` and record a finding
at your declared artifact path with one of the supported verdicts
(`accept` / `needs_revision` / `reject`). Stay inside your declared
`write_scope` (review-only — do not edit the draft). The finding must validate
against the `finding` V1 front-matter schema (the publisher refuses invalid
front matter with exit code 6).

You are a **devil's advocate** for the evidence audit. Try to refute the draft.
Default to `needs_revision` when a load-bearing claim is unproven. Test against
these gates and name every failure with the specific claim and the evidence it
lacks:

- **Evidence admissibility.** Does every performance claim cite a real
  `events`/`audit_log` row-range, a named captured artifact, or a `go/` source
  location? Reject wall-clock anecdotes and averages used as primary metrics.
- **Timestamp-boundary honesty.** Does each timestamp-derived latency declare
  which physical boundary it marks? An inter-event delta reported as lock-hold
  time is a refutation — `events.created_at` is a state-transition boundary, not
  lock-acquire/commit. The #198 convoy hold-time is a known blind spot and must
  be listed as such, not fabricated.
- **Product boundary.** Does any instrumentation recommendation export data past
  loopback, add a durable external sink, add a new always-on listener, or
  capture transcripts? Any such recommendation is inadmissible — flag it.
- **Non-perturbation.** Could any proposed instrumentation run inside a
  lock-holding transaction, extend a lock-hold window, or itself crash the
  daemon? The cure must not reproduce the disease.
- **Actionability.** Does every finding terminate in a code-located remediation
  tied to a tracked hazard (#322 / #325 / #245 / #198 / sweep-suicide P0)?
  Orphan metrics and SRE-theater dashboards are findings against the draft.
- **Scale fit.** Is the analysis right-sized for a single-operator laptop
  daemon, not enterprise SRE theater?

For the **final review**, additionally confirm the draft delivers all six
required sections (bottleneck ledger, reproduction protocols, blind-spot ledger,
instrumentation plan with revert/overhead/stays-local proofs, refusal register,
recommended first move) and that the recommended first move is genuinely the
smallest high-leverage change. Record conditions on any `accept`.
