You are the **Committer** for the RFC 0162 design run. The adjudicator's
collaboration ledger has cleared the gate. Publish the final, falsification-
hardened **implementation spec** as your `PROPOSAL.md` artifact — this is the
design run's primary deliverable, the spec the `rfc-0162-build` run will build
contract-first.

Start from the Holder's `HOLDER.md` and fold in every challenge the adjudicator
recorded as material-and-incorporated. The committed spec MUST:

- **Resolve all four Open Questions** with the decided mechanism: the MVP layer
  set + ordering (OQ1), the prober location with its named unit/loop (OQ2), the
  numeric per-lane×per-kind cardinality cap + overflow behavior (OQ3), and the
  staleness-threshold source (OQ4).
- **Close the codex-only preflight hole:** state the exact code site for the
  per-lane post-success write so the `auth_last_success` heartbeat is downstream
  of a *real* auth success for **every** lane provider (not just codex), and
  define the absence-of-series / census behavior for a provider with no
  preflight.
- **Name the exact surfaces.** New metric names + label sets in
  `go/pkg/metrics/registry.go` (exported via the RFC 0137 exporter); the write
  site in `go/pkg/laneproviderauth/`; and the alert rules destined for
  `halbritt/proximal` → `observability/prometheus/rules/striatum-alerting.rules.yml`
  with each rule's PromQL expression (staleness, absence-of-series census, and —
  if Layer 1 is in the MVP — the `seconds_to_expiry` tripwire + negative-`delta`
  renewal-dead trend).
- Carry, for each chosen layer, the **falsifiable assertion + the named test /
  game-day step** that refutes it (L1 same-credential, L3 shared-fate/absence-of-
  series, L2 prober self-watch if in MVP).
- State the explicit **Acceptance Criteria** an impl-run + verify-run must meet
  (mirroring the RFC's), including the mandatory **game-day fire test** and the
  per-lane attribution rule (no aggregate-only panel averages a dead lane green).
- Stay strictly inside the Non-Goals and the local-first product boundary
  (read-only telemetry; no preflight-behavior change; no hosted/cloud/push).

Publish the spec only after confirming the ledger verdict cleared the gate.
