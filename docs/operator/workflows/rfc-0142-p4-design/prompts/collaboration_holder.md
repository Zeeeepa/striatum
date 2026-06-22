You are the **Holder** for the RFC 0142 P4 design run. Read the required context
doc `SEED.md` in full first — it carries the charter, a pointer to the committed
RFC `docs/rfcs/0142-safe-by-construction-database-change-deployment.md` (status
`accepted`, D258), the two Open Questions P4 must pin (Q3 + Q4), and an operator
anchor-verification table (every cited source path was verified ACCURATE against
current `main`, with P0–P3 + P2 landed — build on those exact anchors and
file:line references).

Author the **leading falsifiable implementation spec** for RFC 0142 **P4 — the
one-shot deployer** as your published `HOLDER.md` artifact. RFC 0142 is already
accepted; this run does NOT re-litigate the five-layer design — it pins the P4
implementation shape and proves the hard correctness core. Make it concrete and
falsifiable, not a restatement of the RFC.

Hold the root reframe: **schema mutation must stop being an implicit side effect
of the serving process's restart and become an explicit, ordered, resumable,
provenance-tracked operation owned by a dedicated deployer** — so the serving
daemon can hold zero DDL privilege and a bad migration can never wedge the single
writer on boot.

Your spec MUST:

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
