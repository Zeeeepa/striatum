You are the **Holder** for the **RFC 0143 Slice A** design run. Slice A is the
maintainer-ratified (D261) **Option 4** floor: make a `striatum-lane` lane that
cannot reseal after a daemon boot-epoch rotation **fail LEGIBLY** with a typed
`session_unrecoverable_across_rotation` signal instead of a silent unsealed exit or a
misleading "permission denied". Per D261 this is **pure, daemon-side observability** —
it **mints no credential, widens no token, and does not touch the credential trust
model.** Slice B (the `CapabilityReseal` authority + the connect-out channel) is
**blocked on RFC 0168 (#585) and is OUT OF SCOPE — do not design any of it.**

Read the required context docs **in full first**: `SEED.md` (your charter — the design
shape, the decoupling premise, the source anchors, the HARD CONSTRAINTS, the
falsifiable assertions A1–A6, and the clearing condition); the committed RFC
`docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md` (especially
`## Current behavior` and `## Decision (D261)`); the D261 row in
`docs/decisions/decision-log.md`; and the `BC1-W1-CAPTURE-FLOOR` finding in the v7
ledger `docs/operator/artifacts/rfc-0143-design-v7/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
(it gives the exact capture-boundary→typed-floor fix shape, which you adapt to the
**decoupled** world — no W1 channel, no kernel-token capture).

Publish a **falsifiable implementation spec** as your `HOLDER.md` artifact — the
contract the `rfc-0143-slice-a-build` `code_change` run executes TDD. Make it
**concrete and falsifiable** (file:line anchors, named Go tests, a mechanically-derived
classification), **not** a restatement of the RFC. **Re-verify every source anchor in
the SEED against current `main`** and correct any line drift.

Your spec MUST:

1. **Specify the reserved agentloop exit code** for the floor (new
   `go/pkg/agentloop/exitcodes.go`, e.g. `ExitUnrecoverableAcrossRotation = 97`).
   Slice A owns ONLY this floor code — NOT the reseal code 98, NOT
   `resealInFlightJob`, NOT the connect-out channel, NOT the kernel-token capture, NOT
   `CapabilityReseal`, NOT owner bundle 0021 (all Slice B).
2. **Specify Spot 1 (credential-chain narrowing)** precisely: which function
   (`ResolveTokenMaterial` / `ResolveTokenMaterialFresh`), how a **non-owner** lane
   reaching the admin runtime `client-token` is detected (owner-uid check vs. the
   `0600`/non-owner-only rejection), the **typed sentinel** returned, and how the
   agentloop (`loop.go:37/:78/:602`, including the #323 rotation-recovery path) maps
   it to a clean exit with the reserved floor code — **without** adding any read path
   to the admin token. An **owner** process must be unaffected.
3. **Specify Spot 2 (daemon observation + recovery routing)** precisely: how the
   daemon observes the reserved code from durable state (`#{pane_dead_status}` on the
   tmux path; `processExitCode` on the direct path), the new typed helper-event /
   recorder branch or recovery-class wiring that records
   `session_unrecoverable_across_rotation`, how the launch/attach failure path
   (`waitForHelperAgentStart`, the `helper_error` phase `launch`) records the **typed
   class instead of a raw error** when the floor applies, and how a **silent death
   with no reserved code falls back to the existing `agent_exited_unsealed`** (no
   over-fire). State how the new class is classified in `recoverStuckJobs` /
   `isNecrosisStallClass`, and how it **relates to** the existing
   `HandleRecoveryCompleteStalled` (#292) verb (route/escalate legibly — do not
   duplicate or override it).
4. **State each load-bearing claim as a falsifiable assertion paired with its named
   test** — at least A1–A6 from the SEED (Spot-1 narrowing + owner-unaffected; Spot-2
   typed-class-not-raw-error; no-over-fire negative; no-widening; C2 forge-resistance
   `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`; no-regression of
   existing recovery/supervise/agentloop tests).
5. **Honor every HARD CONSTRAINT** (SEED): no token widening, no Slice-B artifact,
   daemon-side/process state only (no inbound authenticated frame), no over-fire, no
   raw-error leak, additive-only, product-boundary-safe.
6. **Specify the build slices in contract-first order** (smallest safe first), each
   with its named Go tests and exact file touches, and a short **Acceptance Criteria**
   the build + verify run must meet (the two game-day shapes: a non-owner lane hitting
   the admin-token fallback surfaces the typed floor not a silent unsealed exit; a
   capture-boundary miss produces the typed class not a raw `helper_error`).

Open with an **auditable resolution map** (a short "How this honors the decoupling
premise and the HARD CONSTRAINTS" subsection) so the falsifiers can verify the
no-inbound-frame / no-widening / no-over-fire properties directly rather than infer
them. Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
