You are the **Committer** for the RFC 0143 design run. The adjudicator's
collaboration ledger has cleared the gate. Publish the final, falsification-
hardened **implementation spec** as your `PROPOSAL.md` artifact — this is the
design run's primary deliverable, the spec the `rfc-0143-build` run will build
contract-first.

Start from the Holder's `HOLDER.md` and fold in every challenge the adjudicator
recorded as material-and-incorporated. The committed spec MUST:

- **Resolve all four Open Questions** with the decided mechanism: the chosen
  trust-model option(s) (OQ1), the surviving capability set + lifecycle +
  ownership/mode + invalidation triggers (OQ2), the exact code sites it touches
  (OQ3 — name files and functions: `session_token.go`, `supervision_env.go`,
  `token.go`, `loop.go`, `bootstrap.go`, `cmd/striatumd/main.go`), and the
  legible-failure fallback wiring (OQ4).
- **Carry the security invariant explicitly:** the new credential never carries
  `{admin,apply,recovery,surgical_recovery}`; no lane ever reads the bootstrap
  admin client-token; any durable token file is `striatum-lane`-owned `0600`,
  TTL- and epoch-bound, and invalidated on session close. State each as a
  falsifiable assertion + the named test that proves it.
- **Specify the build slices in contract-first order** (smallest safe first —
  e.g. option 4 legible-failure as the immediate safety net, then the survival
  mechanism), each with its named Go tests and the migration/owner-bundle
  changes (if any). Apply the shadow-first convention for any risky new
  credential/boot path: new behavior defaults OFF behind an env flag; additive
  migrations only; self-record before enforce.
- **State the explicit Acceptance Criteria** an impl-run + verify-run must meet,
  including the mandatory **game-day fire test** (restart the daemon mid-job and
  show the lane survives-and-reseals OR fails legibly-and-is-routed, with no
  silent unsealed exit and no elevated-capability exposure).
- **Open with the maintainer-ratification banner:** the chosen option is a
  security/authz trust-model change; the spec is a RECOMMENDATION the maintainer
  ratifies before the build lands. State the recommended option and the one-line
  security rationale up front.
- Stay strictly inside the Non-Goals and the local-first product boundary.

Publish the spec only after confirming the ledger verdict cleared the gate.
