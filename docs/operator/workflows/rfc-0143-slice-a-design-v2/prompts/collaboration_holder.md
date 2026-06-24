You are the **Holder** for the **RFC 0143 Slice A** design run, and **this is the v2
REVISION.** The v1 run returned `needs_revision` on two findings; your job is to REVISE
the v1 spec, not write a new one from scratch.

Read the required context docs **in full first**: `SEED.md` (your charter — what v1
credited and you must carry forward unregressed, the two binding fixes FIX-1 /
FIX-2, the observability-only clarification, the HARD CONSTRAINTS, and the clearing
condition); the **v1 `HOLDER.md`**
(`docs/operator/artifacts/rfc-0143-slice-a-design/dialogue/holder/HOLDER.md` — the spec
you revise); the **v1 cycle_2 collaboration ledger**
(`docs/operator/artifacts/rfc-0143-slice-a-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_2.md`
— read its `findings:` for SA-ROTATION-UNDERFIRE and SA-C2-TMUX-FORGE, the `rationale:`
recommended-fix paragraphs, and the credited `status: accepted` carry-forward set); the
committed RFC `## Decision (D261)`; and the v7 `BC1-W1-ORACLE` / `BC1-W1-CAPTURE-FLOOR`
findings (the shared-uid mutable-tmux oracle context).

Publish the **revised** falsifiable implementation spec as your `HOLDER.md` artifact.
Open with an **auditable resolution map** ("Addressing the v1 findings") so the
falsifiers can verify FIX-1 and FIX-2 are resolved and the credited skeleton is
preserved, rather than infer it. **Re-verify every source anchor against current
`main`** and correct line drift.

Your revised spec MUST:

1. **Resolve FIX-1 (SA-ROTATION-UNDERFIRE) — the gating requirement.** Make the typed
   floor FIRE on the real #512 rotation path (a session-bound lane carrying
   `STRIATUM_MCP_TOKEN` that presents a stale boot epoch the daemon rejects as
   `stale_daemon_identity`). Per the SEED, the RECOMMENDED forge-resistant shape is a
   **daemon-side observation**: when `validateBootEpoch` rejects a request
   (`http.go:166-169/:681-699`), record the presenting session as
   unrecoverable-across-rotation as durable daemon state, and have the recovery sweep
   record the typed class for a session observed presenting a stale epoch + complete
   on-disk deliverable + lane-lost. **You MUST resolve the pre-auth attribution
   sub-question concretely** (`validateBootEpoch` runs before bearer validation,
   `http.go:159-169`): specify exactly how the daemon attributes the rejection to a
   session without widening any token, and how an unattributable rejection avoids
   over-firing. If clean pre-auth attribution is not achievable, use the fallback
   (map the `stale_daemon_identity` response on the lane's own MCP client path → 97)
   and say why. **Route the codex rotated-endpoint wedge (`loop.go:625-646`) to the
   floor.** Add the SEED's named tests + keep the no-over-fire negatives.

2. **Resolve FIX-2 (SA-C2-TMUX-FORGE).** Make the TRUSTED floor carrier
   forge-resistant (the daemon-observed rejection / the direct-path
   `agent_exited.exit_code`). Do NOT claim the tmux `#{pane_dead_status}` carrier is
   forge-resistant — either corroborate it against a forge-resistant signal before
   recording the typed class, or honestly scope its forge-resistance as
   RFC-0168-bounded (the shared-uid oracle that makes Slice B unsolvable). Add
   `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation`; keep
   `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` as insufficient
   alone.

3. **State the OBSERVABILITY-ONLY clarification:** the typed floor is a classification
   refinement of `agent_exited_unsealed` that grants NO new auto-seal authority (the
   lane still needs an operator requeue / Slice B to seal), so a forged class is no
   more privileged than a forged `agent_exited_unsealed`. Keep the typed floor's
   recovery routing no-more-privileged than the existing class.

4. **Carry the v1-credited skeleton forward UNREGRESSED** (verbatim where applicable):
   §1 reserved code + sentinel; §2 Spot-1 narrowing (refuse-before-read, no widening);
   §3.2–3.4 exact-code-only classification; §3.5 launch-handshake dissolution; §3.6
   #292 relationship; §4 direct-path C2; the no-widening invariant; the additive
   `isNecrosisStallClass` growth.

5. **Honor every HARD CONSTRAINT** (no token widening, no Slice-B artifact,
   daemon-side/own-observation only, no over-fire / no raw-error leak / no silent
   rotation exit, additive-only, product-boundary-safe).

6. **Keep the build slices in contract-first order** (smallest safe first), each with
   its named Go tests and exact file touches, and a short Acceptance Criteria with the
   rotation game-day shape (a session-bound lane on a stale epoch surfaces the typed
   floor) and the forge negative (a same-uid respawn does not forge the class).

Do not treat falsifier completion as acceptance — the adjudicator's collaboration
ledger decides whether the gate clears.
