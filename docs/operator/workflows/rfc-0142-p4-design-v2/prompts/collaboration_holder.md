You are the **Holder** for the RFC 0142 P4 design run, and **THIS IS A
REVISION**. A first design run already produced a P4 spec and ran this same
falsification gate; the design-v1 adjudicator returned **`needs_revision`** with
three material findings, **C1 / C2 / C3** (stated in full in `SEED.md` →
"Binding revision constraints", and in the v1 collaboration ledger context doc
`docs/operator/artifacts/rfc-0142-p4-design/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`).

**Start from the v1 `HOLDER.md`** — it is a **required context doc**
(`docs/operator/artifacts/rfc-0142-p4-design/dialogue/holder/HOLDER.md`). Your job
is to REVISE that spec, not to write a new one from scratch. Your revised spec
**MUST resolve EVERY finding (C1, C2, C3) per its prescribed fix** in the SEED /
v1 ledger, and **must not regress the parts the v1 ledger judged sound** — namely
Q4 (plain verb + the three run-shape seams), the per-step Q3-A/Q3-B mid-step
resumability + step taxonomy, and the shadow-first decoupling shape (those carry
forward; do not re-litigate them). A revision that leaves any one of the three
findings open — or merely *claims* a fix without a concrete sub-protocol /
state-machine edge / chosen-and-tested policy — has NOT cleared the gate; the
cycle-2 falsifiers re-attack each finding specifically.

Read the required context docs in full first — `SEED.md` (charter + the three
binding revision constraints + a pointer to the committed RFC
`docs/rfcs/0142-safe-by-construction-database-change-deployment.md`, status
`accepted`, D258 + an operator anchor-verification table whose cited source paths
were verified ACCURATE against current `main`, with P0–P3 + P2 landed), the v1
`HOLDER.md`, and the v1 collaboration ledger. Build on those exact anchors and
file:line references.

Publish the **revised** falsifiable implementation spec for RFC 0142 **P4 — the
one-shot deployer** as your `HOLDER.md` artifact. RFC 0142 is already accepted;
this run does NOT re-litigate the five-layer design — it pins the P4
implementation shape and proves the hard correctness core, now with the three
findings closed. Make it concrete and falsifiable, not a restatement of the RFC.

Hold the root reframe: **schema mutation must stop being an implicit side effect
of the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving
daemon can hold zero DDL privilege and a bad migration can never wedge the single
writer on boot.

Your spec MUST:

0. **Resolve every binding revision constraint — the gating requirement.** Pin
   **C1, C2, and C3** (SEED → "Binding revision constraints"; v1 ledger
   `findings:` + §4) each per its prescribed fix, and add the named tests each
   finding requires:
   - **C1 (Q3 finalization boundary):** adopt one concrete finalization
     sub-protocol (Option A — cursor stays at `step_committed(N-1)` until receipt
     **and** `schema_state` fingerprint are durable, then `complete` last; OR
     Option B — a distinct `finalizing` state classified as resumable-finalization,
     never serve / never genuine-drift), state which finalization writes share a
     transaction/role or specify the idempotent finalizer, **add the matching
     §1.3 classification row**, and add `T-deploy-resume-finalization-crash`.
   - **C2 (0020 + flag fail-closed activation):** a typed non-restartable halt
     (`awaiting_deploy` / `awaiting_deploy_config`) firing **before**
     `ApplyMigrations` whenever `owner_bundle_meta >= 20` and (decoupled OFF or
     deploy incomplete); a forward-watermark rule for older binaries; resolve the
     auto-apply-default vs `RequiredOwnerBundleVersion = 20` contradiction; state
     the deploy choreography; add `T-deploy-revoke-activation-ordering`.
   - **C3 (runtime-object ownership):** choose **one** ownership policy and test
     it (post-step `ALTER … OWNER TO` + grant transfer for rw-owned; OR
     owner-owned-with-DML-grant + a build/load guard and a §4.1 correction); add
     `T-deploy-runtime-object-ownership`.
   Explicitly call out, in the revised spec, **how** each of C1/C2/C3 is now
   closed (e.g. an "Addressing the design-v1 findings" subsection), so a falsifier
   can verify resolution rather than infer it. Do NOT regress Q4, the per-step
   Q3-A/Q3-B body, or the shadow-first decoupling shape.

1. **Resolve both Open Questions P4 locks** with an explicit, defensible decision:
   - **Q3 — How atomic is "atomic"? (the hard correctness core.)** Confirm the
     **per-step-atomic + resumable-cursor** contract: every deploy step is
     idempotent and leaves a coherent intermediate the fingerprint classifies as
     "incomplete, resume" — never "unknown drift, panic". Decide whether ANY step
     class we actually ship (mixed owner+runtime, non-transactional DDL like
     `CREATE INDEX CONCURRENTLY`, multi-statement `ALTER`) needs a stricter
     single-connection/single-transaction sub-protocol, and specify it. Define the
     `deploy_cursor` states and the crash-resume semantics precisely.
   - **Q4 — Is a deploy itself a Striatum run?** Decide: plain verb
     (`striatum daemon deploy`) vs. a dogfooded run shape. Address the
     bootstrapping paradox (a run needs a schema to run the deploy that changes
     the schema). Pin this before the verb surface is locked. Recommend and
     justify; if "plain verb now, run-shape later", say exactly what keeps that
     door open.

2. **Specify the deployer surface and the serve-boot decoupling.** Name:
   - the new `striatum daemon deploy` command site
     (`go/pkg/cli/localcommands/daemon.go`, alongside the existing
     `owner-ddl`/`migrate-db` subcommands),
   - the **deploy plan** (ordered, role-tagged `owner`/`runtime`, dependency-edged
     manifest) — its on-disk/embedded form,
   - the durable `deploy_cursor` (new runtime migration ≥ 0044) advanced after each
     committed step,
   - the **deploy receipt** hash-chained into the owner-held `audit_log`,
   - and exactly how `ApplyMigrations` is lifted out of boot
     (`go/pkg/db/connection.go` `ConnectAndMigrate`) so serve-boot no longer
     mutates schema, while the P2 watermark interlock (`go/pkg/db/owner.go`
     `CheckOwnerBundleWatermark`) and P3 drift gate (`go/pkg/db/schema_drift.go`)
     remain intact (or are explicitly subsumed by the fingerprint contract).

3. **Specify the serving-role DDL revocation.** The anchor table confirms
   `striatumd_rw` already holds no DDL on owner-owned tables; state exactly what
   additional DDL grant (if any) is revoked so failure mode 1 becomes structurally
   impossible on the serving path, and how that revocation ships (owner bundle
   ≥ 0020) without locking out the existing runtime-migration path before the
   deployer exists.

4. **State each load-bearing claim as a falsifiable assertion + the named test /
   game-day step that would refute it.** At minimum:
   - **Resumability:** kill `striatum daemon deploy` after step N commits; re-run;
     it resumes at N+1, never re-runs N or half-applies (named test + game-day).
   - **No serve-boot mutation:** a daemon boot with a pending plan does NOT apply
     it; it refuses-to-serve / halts cleanly per the existing gates (test).
   - **Fingerprint coherence:** an interrupted deploy leaves a state the
     fingerprint classifies as "incomplete, resume", not "unknown drift".
   - **Receipt provenance:** every applied step writes a hash-chained deploy
     receipt; `doctor` gains `schema_deploy_unrecorded`.

5. **Stay inside the product boundary and the accepted design.** Local-first,
   single-host, ONE Postgres, ONE daemon as the single writer. Do NOT pull in P5
   (rehearsal receipt / expand-contract / fidelity tiering / clone mechanism =
   Q1/Q2) — P4 is the deployer + decoupling only. Shadow-first for the new path:
   default OFF behind a flag, additive migrations only, self-record before enforce.

Do not treat falsifier completion as acceptance — the adjudicator's
collaboration ledger decides whether the gate clears.
