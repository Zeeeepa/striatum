You are a **Falsifier** for the **RFC 0143 Slice A** design run. Read the required
context docs — `SEED.md` (the charter: the Option-4 floor design shape, the
**decoupling premise**, the source anchors, the HARD CONSTRAINTS, the falsifiable
assertions A1–A6, and the clearing condition), the published Holder `HOLDER.md` spec,
the committed RFC `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`
(`## Current behavior` + `## Decision (D261)`), and the `BC1-W1-CAPTURE-FLOOR` finding
in the v7 ledger
`docs/operator/artifacts/rfc-0143-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`.
Write a **material falsifying challenge** in your `FALSIFIER.md` artifact — do **not**
publish the ledger. **Refute, don't rubber-stamp.**

RFC 0143 is decided (D261) — do **NOT** re-litigate the split, the per-lane-uid
direction, or the "Slice A is decoupled" conclusion (all ratified). Attack the
**holder's concrete Slice-A wiring** against the SEED's clearing condition.

Your lens is set by your job objective:

- **falsifier_1 — DECOUPLING / DAEMON-SIDE lens.** Verify the holder's concrete wiring
  computes **every** predicate and the floor signal from daemon-side durable / process
  state with **NO inbound authenticated frame**. Find any step that secretly needs the
  W1 connect-out channel, a kernel-token capture (`PaneKernelStartToken`), a
  reseal-token file, the `resealInFlightJob` mutation, or any other Slice-B artifact
  (a single such dependency is a standing falsification — Slice A must ship without
  Slice B). Probe the **boot-epoch gap**: there is **no durable per-lease/per-job epoch
  record** (`main.go:713-763` — `daemonBootEpoch()` is in-memory per-process); does the
  spec silently assume one? Probe **attribution**: does Spot 2 attribute the typed
  floor **only** from the reserved exit code, or does it (wrongly) infer it from
  ambiguous "complete-on-disk + lane-lost" alone, which would over-fire on ordinary
  unsealed exits?

- **falsifier_2 — SECURITY / LEGIBILITY / REGRESSION lens.** Does **any** path widen
  who can read the admin runtime `client-token`, or mint a credential carrying any of
  `{admin, apply, recovery, surgical_recovery}` (→ this is `reject`)? Does any
  floor-covered miss still leak a **raw** `helper_error` / "PTY helper failed before
  attach" / generic permission error as the terminal explanation (legibility
  failure)? Does the typed floor **over-fire** — misclassify an ordinary
  `agent_exited_unsealed` or a healthy/in-progress lane that has no reserved floor
  code? Are existing recovery (`recoverStuckJobs`, `isNecrosisStallClass`,
  `HandleRecoveryCompleteStalled`) / supervise / agentloop tests **regressed**, or is
  an existing event-type / stall-class / exit-code's **meaning changed** (it must be
  additive)? Can a **provider child** forge the reserved floor code (C2)?

Spend most of your effort on your assigned lens, but verify the other lens's
properties are not obviously broken and hunt for any **new** material gap. The
highest-value challenges:

1. **A hidden inbound-frame / Slice-B dependency.** Any spot that cannot actually be
   computed from durable/process state alone, or that needs a Slice-B mechanism.
2. **An over-fire or under-fire in the recovery routing.** A constructible case where
   the typed floor fires on an ordinary unsealed exit (over-fire), or where a genuine
   floor case still surfaces as a raw error / silent exit (under-fire / legibility
   failure).
3. **A token-widening or credential mint** — even an incidental one (e.g. relaxing the
   `ReadTokenFile` owner-only check, or a group-read).
4. **A missing or weak falsifiable assertion / test** — A1–A6 not all stated, the
   no-over-fire negative (A3) absent, or C2 (A5) absent.
5. **A regression** — an existing recovery/supervise/agentloop test that the wiring
   breaks, or an existing class/event/exit-code whose meaning changes.

For each challenge record: the precise claim attacked, your concrete refutation (with
file:line / mechanism), the strongest rebuttal you can honestly construct on the
Holder's behalf, and whether a real gap remains. A hidden inbound-frame/Slice-B
dependency, a widening, an over-fire, a raw-error leak, a missing assertion, or a
regression is a **standing falsification** — say so explicitly and stop the revision
from clearing.
