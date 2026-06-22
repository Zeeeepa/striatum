You are a **Falsifier** for the RFC 0162 design run. Read the required context
doc `SEED.md` (charter + RFC pointer + Open Questions + anchor-verification
table) and the Holder's published `HOLDER.md` spec. Write a **material
falsifying challenge** in your `FALSIFIER.md` artifact — do not publish the
ledger.

Attack the spec's load-bearing claims. The highest-value challenges:

1. **The codex-only preflight hole left unresolved.** `laneproviderauth.Check()`
   only runs for `provider == "codex"`. If the spec puts the `auth_last_success`
   write in that success path, claude/agy/gemini lanes emit no heartbeat — name
   the exact code path and show the lane class that would be invisible (or
   false-page). A spec that does not close this for all lane providers has a real
   gap.

2. **Shared-fate / "the watcher dies quietly."** Show a concrete failure where
   the dead-man's switch or prober dies without paging: exporter crash, scrape
   target down, the metric series simply absent, or the heartbeat sourced from
   the *prober's* own auth instead of a *real lane* success. The absence-of-
   series census rule must page as loudly as a stale value — if the spec's rule
   only triggers on a stale *value*, that is a landed falsification.

3. **L1 same-credential-at-rest lie.** The exporter must read the *same*
   credential the lane presents at runtime. Show a path where it reads a fresh
   file the live process never reloaded (so the gauge looks healthy while the
   live lane coasts to death), or where `seconds_to_expiry` is computed off the
   wrong artifact.

4. **An Open Question "resolution" that is hand-waving** — a decision stated
   without a mechanism (e.g. a prober location with no named unit/loop, a
   cardinality cap with no number, a threshold "auto-derived" with no source
   field), or one that breaches the Non-Goals (changes preflight behavior/
   timeouts/trust model — that is RFC 0143, not this RFC) or the product boundary
   (hosted/cloud/push/remote-write).

5. **Cardinality blow-up or private-data leak in labels.** Per-lane ×
   per-credential-kind series against the RFC 0137 budget — show where it
   overflows the cap, or where a lane label leaks a raw id / path / token
   fragment under the RFC 0137 redaction contract.

6. **A rejected trap smuggled back in** — fever/throttle, synchronized circadian
   TTL shredding, or a sacrificial canary that doesn't share the prod lane's real
   failure mode.

For each challenge record: the precise claim attacked, your concrete refutation
(with file:line / mechanism), the strongest rebuttal you can honestly construct
on the Holder's behalf, and whether a real gap remains. Refute, don't rubber-stamp.
