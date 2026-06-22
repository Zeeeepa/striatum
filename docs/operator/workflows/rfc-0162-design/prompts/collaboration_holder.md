You are the **Holder** for the RFC 0162 design run. Read the required context
doc `SEED.md` in full first — it carries the charter, a pointer to the committed
RFC `docs/rfcs/0162-lane-auth-silent-failure-observability.md`, the four Open
Questions, and an operator anchor-verification table (a load-bearing subtlety in
the auth preflight has been pinned for you; build on the corrected anchors).

Author the **leading falsifiable implementation spec** for lane-auth
silent-failure observability as your published `HOLDER.md` artifact. This is the
claim the falsifiers will attack and the adjudicator will gate — make it
concrete and falsifiable, not a restatement of the RFC. Hold the root reframe:
we must **alert on the absence of success, not the presence of errors**.

Your spec MUST:

1. **Resolve every one of the four Open Questions** with an explicit decision
   (in-MVP / deferred; which mechanism; why). Leaving any unresolved fails the
   charter:
   - **OQ1 Layer ordering / MVP.** Which of Layer 1 (expiry/renewal-health
     telemetry — most on-target for the root cause), Layer 2 (cross-lane
     differential + negative probe), Layer 3 (dead-man's-switch
     `auth_last_success`, cheapest backstop) ships first — and exactly which
     compose the MVP vs. follow-up. Decide and justify.
   - **OQ2 Prober location.** Layer 2's active probe in the daemon (recovery-
     sweep-adjacent loop) or as an external systemd timer on `proximal` (outside
     the daemon blast radius — "who watches the watcher"). Decide; name the
     unit/loop.
   - **OQ3 Metric cardinality.** The per-lane × per-credential-kind series cap
     against the RFC 0137 cardinality budget (`MetricsCardinalityClipped`). Give
     the numeric lane/kind cap and the overflow behavior.
   - **OQ4 Staleness-threshold source.** Is
     `striatum_lane_auth_staleness_threshold_seconds{lane}` auto-derived from the
     credential lifetime or operator-declared in the registry backbone? Decide.

2. **Re-anchor to current source, and resolve the codex-only preflight hole.**
   The SEED table records that `laneproviderauth.Check()` only supports
   `provider == "codex"` (anything else returns `FailureUnsupported`), so a
   naive "each lane writes `auth_last_success` in the preflight success path"
   only ever fires for **codex** lanes — claude/agy/gemini lanes get no preflight
   and would emit no success heartbeat at all, looking permanently dead (or, if
   absence is silently ignored, permanently healthy). The spec MUST specify
   exactly where the post-success write lives so the heartbeat is downstream of a
   **real** per-lane auth success across providers, not just codex — and what the
   absence-of-series / census rule does for a provider that has no preflight.

3. **Specify the exact surfaces and the cross-repo split.** Metric surface lands
   in **this** repo: `go/pkg/metrics/registry.go`, exported via the RFC 0137
   striatumd Prometheus exporter; the success write is in
   `go/pkg/laneproviderauth/`. Alert rules land in a **separate** repo,
   `halbritt/proximal` → `observability/prometheus/rules/striatum-alerting.rules.yml`
   (note: the RFC's `…/prometheus/prometheus/rules/…` path has a stray segment;
   the real file is `observability/prometheus/rules/`). Name every new metric and
   its label set; name every alert rule and its expression.

4. **State each load-bearing claim as a falsifiable assertion + the named test /
   game-day step that would refute it.** At minimum:
   - **L1 same-credential risk:** the exporter reads the *same* credential the
     lane actually presents at runtime (the `auth.json` / session-bound token the
     lane resolves), not a fresh file at rest the process never reloaded.
   - **L3 shared-fate / absence-of-series:** a dead exporter / crashed prober /
     vanished metric series pages **as loudly** as a stale value (census rule
     `count(...) < striatum_expected_lane_count`).
   - **L2 prober self-watch (if in MVP):** a dead-man's-switch on the prober +
     an always-expected-fail synthetic lane proving the prober can still detect
     failure.
   - **Game-day:** each layer's alert provably fires before a real incident would
     (an alert that has never fired is a liability).

5. **Stay inside the product boundary and the Non-Goals.** Read-only telemetry
   over the auth boundary; do NOT change preflight behavior, timeouts, or the
   credential trust model (that is RFC 0143). Local-first, pull-only; no hosted/
   cloud/push/remote-write; no per-repo private-data leak (lane identity as a
   label must respect the RFC 0137 redaction/cardinality contract). Honor the
   rejected traps (fever/throttle, circadian shredding, sacrificial canary).

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
