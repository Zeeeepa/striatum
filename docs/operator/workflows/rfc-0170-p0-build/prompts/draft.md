# Draft — RFC 0170 P0 implementation

You are the **author**. Build RFC 0170 P0 in Go, contract-first, exactly as the
falsification-cleared SPEC specifies.

**Read first, in full:**
- `docs/operator/artifacts/rfc-0170-p0-design-v5/commit/proposal/PROPOSAL.md` — the SPEC
  (the build contract; the authority — do **not** re-derive it). Every `file:line` anchor
  in it is verified against the tree; follow them.
- `docs/operator/artifacts/rfc-0170-p0-design-v5/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
  — the cleared gate dispositions + the two build-carry verification gates (`BC-618`, `BC-619`).
- `docs/rfcs/0170-self-culling-repository-and-cull-workflow-class.md` — background only; the
  SPEC §1 corrects the RFC's stale pointers (use the SPEC).

**P0 is OBSERVE-ONLY.** It writes a candidacy ledger that nothing acts on — no deletion, no
tombstone, no page, no `doctor` RED/amber, no run-admission effect. The dangerous error is a
**false positive** (nominating a live artifact); a conservative false negative costs nothing.

**Build (all under `go/`):**
1. **Migration** `go/pkg/db/sql/0045_cullable_entity.sql` — confirm `0045` is still free
   (`ls go/pkg/db/sql/0045*` empty). Table `striatumd.cullable_entity(kind, ref,
   last_reinforced_at, decay_score, reachable_from_root, candidacy_state)` PK `(kind, ref)`,
   the two CHECKs, modeled on `0042`/`0043`/`0044`. **No FK, no owner DDL** (≥27 guards).
   `DO $$` guarded `GRANT SELECT, INSERT, UPDATE … TO striatumd_rw` (**no DELETE**).
2. **RESERVATIONS** — add `ordinal=45, file="0045_cullable_entity.sql"` to
   `go/pkg/db/sql/RESERVATIONS.toml` in the same change.
3. **Authority inventory (C1)** — both rows:
   `readAuthorityInventory["cullable_entity"] = ReadClassRuntimeOperational` and
   `writeAuthorityInventory["cullable_entity"] = ClassRuntimeDML`.
4. **No `SELECT *`** on `cullable_entity` anywhere (C3 — the #614/bundle-0022 trap). The
   sweep's existing-state read projects exactly `kind, ref, candidacy_state`.
5. **Tier-1 predicate** (§3, clauses 1–5) — pure greppable, no LLM, no external/network/
   open-issue state. Structural-status-only (clause 1), live-successor extraction incl. the
   `kind=decision` cell-3-then-cell-5 regex rule (clause 2), the **fully-static tree-local**
   protected pathspec (clause 3), the active-baseline inbound-citation 4a–4f rule with the
   fixed closure lexicon + link/row regex (clause 4), `kind=branch` excluded (clause 5).
   `decay_score = 0.0` sentinel; `candidacy_state ∈ {nominated, withdrawn}`; never delete a row.
6. **`DecayTickSweep.SweepOnce`** in `go/pkg/recovery` (§4) — rides the recovery tick at the
   RFC-0137 metrics-fold position **OFF the wait-gating path**: the fold does O(1) slot-check +
   spawn-or-skip and **never joins** the detached scan. `DefaultCullFoldTimeout = 10s` (static
   test `< DefaultSweepInterval`). Inside the detached goroutine: `cullCtx` with the deadline
   (owned in the goroutine), L2 DB read under `cullCtx` + `SET LOCAL statement_timeout='10000'`,
   L3 cooperative `fs.WalkDir` abort + bounded front-matter reader, L4 compute-then-commit (one
   all-or-nothing UPSERT on its own pooled conn, only if the scan finished before `cullCtx`
   fired — zero writes on timeout/non-return), and the `recover()` seam at the goroutine's top
   frame (`runDecayTickSweep`). Wire it into `cmd/striatumd/main.go`. **BC-619**: keep it
   detached + O(1) with no select-on-scan-done join.
7. **Tests** — the **BC-618** known-set corpus (zero false positives on the preserved set;
   `decision:D267` **nominated**; `decision:D081` the documented **#618-withheld** member with
   the `docs/records/audits/STRIATUM_DECISION_RECORD_AUDIT_OPUS_4_8_2026-06-16.md` citation as
   cause) + a protected-pathspec fixture (tree-local, no network/GitHub) + a bare-`superseded`
   negative-control fixture row; the **B2** panic-isolation test; the **B5** HANG A/B +
   refresh-not-deferred test with its on-wait-path negative control + the cooperative-hang
   bound; the **BC-619** late-return-zero-write guard; the **B3** cost test; **A5** (verdicts
   column is not an edge); **A6** (no branch candidacy); and a correctly-written two-role
   pgtest (it SKIPs in bwrap without PG — proven live in the verifier/operator stage).
8. **Docs** — a `CHANGELOG.md` entry. `D271` is already in the decision log; add **no** new
   decision (`D267` is an existing nominated decision, not a new record).

Ensure `cd go && go build ./... && go vet ./...` pass in your worktree. Publish **DRAFT.md**
mapping each deliverable to the gate (G1–G4) and assertion it discharges, the verified free
`0045` slot, and the BC-618/BC-619 dispositions. Stay inside `write_scope.allowed_paths`;
source changes are captured via `publish_source_changes`.
