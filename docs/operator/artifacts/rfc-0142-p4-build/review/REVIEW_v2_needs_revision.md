---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "needs_revision"
severity: "high"
tags: ["rfc-0142", "p4", "deploy", "shadow-first", "regression", "needs-revision"]
---

author: reviewer-reviewer-002

# Review (attempt 2) — RFC 0142 P4 build (`striatum daemon deploy` decoupler)

**Verdict: `needs_revision`.** One revision cycle is available. This is a
re-opened round: I reviewed the **current** revision of the run branch
`striatum/rfc-0142-p4-build` (HEAD `1abef173`, the attempt-2 draft that advanced
past the prior review's `7eda803e`), built/vetted it in an isolated detached
worktree, and read every changed file against
`docs/operator/artifacts/rfc-0142-p4-design-v9/commit/proposal/PROPOSAL.md`
(§0.2 / §3.3 / §3.3a / §3.5 / §6 / §6.5 / B1) and `SEED.md`.

The author did **real, substantive revision** — the prior round's M1 DB-stamp
arm, the per-step receipt, and Q3-A atomicity are now genuinely implemented
(credit below). But the revision **resolved the prior D7 "embed 0021?" tension
in the embedding direction**, and **embedding owner bundle 0021 in the build-run
binary breaks the hard shadow-first invariant**: the production daemon and the
entire pg-test suite now refuse to boot when `STRIATUM_DEPLOY_DECOUPLED` is
absent (the default). That is a blocking, `main`-breaking regression and the sole
reason for `needs_revision`.

---

## What I verified independently (build + test)

In a throwaway detached worktree at `1abef173` (`go1.25.0`):

- `go build ./...` — **clean** (exit 0). ✅
- `go vet ./...` — **clean** (exit 0). ✅
- The local **non-PG** `go test ./...` is green for the P4 packages, and the two
  `pkg/agentloop/mcpconfig_test.go` failures the DRAFT flags are genuinely
  pre-existing and unrelated to P4 (an `X-Striatum-Boot-Epoch` test/code mismatch
  from `f53969fc`; this run touches zero `agentloop` files). ✅ Correctly not
  attributable to this build.

**But the green local suite does not exercise the serve-boot path.** The only
callers of `ConnectAndMigrate` are the production boot (`authority_bootstrap.go:193`),
the deploy verb, and **`pgtest.go:70`** (the shared pg-test harness). Every test
that bootstraps via `pgtest.Pool/Pools` is gated on `STRIATUM_PG_TEST_URL`, which
is unset on this host — so the regression below is **masked locally and only
bites in CI / on a real cluster** (and at production boot).

---

## BLOCKING DEFECT — D7' (NEW): embedding 0021 bricks the flag-OFF serve-boot path (shadow-first violation)

The attempt-1 reviewer flagged the 0021-embed question (their D7) as needing an
explicit operator decision. This revision moved 0021 from the prior
`sql/owner_staged_activation/` (outside `ownerBundleFS`) into
**`go/pkg/db/sql/owner/0021_revoke_create_privilege.sql`** — i.e. it is now
**embedded** in `ownerBundleFS` (`owner.go:215` `//go:embed sql/owner/*.sql`,
`ownerBundleLabels[21]` present, `OwnerBundles()` returns it). That fixes the
prior D2/D7 (F16b-production now runs; the revoke is in the plan) **but flips
`RevokeBundleEmbedded()` to `true` for the production binary** (`owner.go:71-82`).

Consequence in `DecideDeployActivation` (`deploy_activation.go:61-67`), **step 0,
which fires BEFORE the cursor-state switch for EVERY cursor state**:

```go
if in.RevokeEmbedded && !in.DecoupledEnabled {   // true && !false
    return DeployHaltAwaitingConfig               // awaiting_deploy_config
}
```

So with the flag **OFF (default)**, `ConnectAndMigrate` (`connection.go:362-407`)
hits the `default:` branch and returns `&AwaitingDeployConfigError{}` — **refuse
to serve** — for *every* cursor state, **including a fresh `none` cursor**.

**I confirmed this empirically** (throwaway probe in the scratch worktree, since
deleted):

```
RevokeBundleEmbedded() = true   (production binary)
flag-OFF, none-cursor decision  = "awaiting_deploy_config"   (NOT serve_legacy)
```

This breaks three things the moment it lands on `main`:

1. **Production serve-boot.** `authority_bootstrap.go:193` → `ConnectAndMigrate`
   now halts `awaiting_deploy_config` on every default-config boot. After this
   lands, `striatumd` cannot serve unless every operator sets
   `STRIATUM_DEPLOY_DECOUPLED` — a breaking change, not a shadow-first inert
   landing.
2. **The entire pg-test suite.** `pgtest.Pools` (`pgtest.go:70`) calls
   `ConnectAndMigrate` at setup and asserts `version == LatestDaemonDBVersion`;
   it now `t.Fatalf`s with the `awaiting_deploy_config` error. **Every**
   `*_pg_test.go` using the shared harness — the existing authority / two-role /
   owner / watermark suites **and the author's own new deploy pg tests** — fails
   at bootstrap in CI. (`pgtest.go`, `owner_watermark_pg_test.go`, and
   `migrations_two_role_pg_test.go` were **not** modified this run and set no
   flag.)
3. **B1.1's own live arm.** `TestDeployActivationCompleteInSyncServesVerifyLive`
   and `TestDeployActivationNoneCursorLive` call `pgtest.Pool(t)` — so the very
   tests meant to discharge B1.1 / (f) on a cluster can't reach one either.

This violates the SEED's hard constraints verbatim:

- *"Default OFF. … The existing `ConnectAndMigrate` serve-boot path is UNCHANGED
  when the flag is absent or false."* — it is not unchanged; it halts.
- *"The existing serve-boot test suites MUST pass UNCHANGED."* — they fail in CI.
- *"Owner bundle 0021 lands INERT."* — embedding it is not inert; it bricks
  flag-OFF boot.

The DRAFT's central self-verification claim is therefore **false**: *"Existing
serve-boot test suites use a `none` cursor → `serve_legacy` → the legacy path runs
byte-identically."* Step 0 (the M3 gate) fires before the cursor switch, so a
`none` cursor on a revoke-embedding binary returns `awaiting_deploy_config`,
never `serve_legacy`. The author landed the embed without flagging the breakage,
without updating the serve-boot suites/harness, and without the operator decision
the prior round explicitly requested.

### This needs an explicit operator decision — the contract is internally contradictory here

The §6 step-7 / "all seven steps ship" / F16b-production language reads *embed
0021*; the §3.3 / §3.3a-step-0 / shadow-first language requires *flag-OFF boot
unchanged*. **Both cannot hold in one binary**: by design, a binary that embeds
0021 (`revokeEmbedded == true`) must run decoupled (M3), which is exactly what
"unchanged flag-OFF boot" forbids. Pick one and make it coherent:

- **Option B (shadow-first-faithful, recommended for this build run).** Do NOT
  embed 0021 in the build-run binary — stage it outside `ownerBundleFS` (as
  attempt 1 did) so `RevokeBundleEmbedded()` stays `false` and flag-OFF boot is
  byte-identical. Per §4.3's two-binary choreography, re-scope F16b-production,
  the revoke-in-plan, and criterion (l) to the activation/verify binary, and say
  so explicitly in the DRAFT + roadmap. (Trade-off: re-opens prior D2 — F16b's
  production assertions go back to dormant — but that is the *documented* §4.3
  posture, not a silent gap.)
- **Option A (embed + accept the breaking change).** Keep 0021 embedded, but then
  this is **not** a shadow-first inert landing: it must be ratified as a breaking
  change, the existing serve-boot pg suites + `pgtest` harness must set
  `STRIATUM_DEPLOY_DECOUPLED` (or the M3 gate must be scoped to fire only on a
  *pending* revoke, which contradicts §3.3a step 0 as written), and the operator
  must accept that `main`'s daemon henceforth requires the flag. This contradicts
  the SEED's hard invariants and so requires an explicit, recorded operator
  decision — not a draft-level reinterpretation.

As it stands, the build cannot land: it red-lines `main`'s daemon boot and the CI
pg suite.

---

## Resolved since attempt 1 (real credit — do not regress)

- **D3 — M1 DB-stamp arm: RESOLVED.** `VerifyAppliedDBStamps`
  (`deploy.go:332-351`) checks every already-applied step's stored sha256 against
  the live `schema_migrations` / `owner_bundle_meta` stamp and returns a typed
  `DeployPlanDBStampMismatchError`; it is actually **called** in `resume()`
  (`deploy_apply.go:121`) before any apply/finalize, alongside
  `VerifyStoredTranscript`. The dead halt arm from attempt 1 is now live. ✅
- **D4 — per-step receipt: RESOLVED.** Migration 0044 now ships a real
  `deploy_receipt` table (`sql/0044_deploy_cursor.sql:68-78`, keyed
  `(plan_hash, step_index)`), `receiptRowHash` (`deploy.go:481-485`) is a genuine
  `prev → row` hash chain, and `applyDeployStep` writes it atomically
  (`deploy_apply.go:229-235`). The step-6 doctor block (`doctor_deploy.go`)
  enumerates the transcript against the trail (`schema_deploy_unrecorded`) and
  adds the M1 stamp/byte WARN. The attempt-1 no-op stub is gone. ✅
- **D5 — Q3-A per-step atomicity: RESOLVED.** `applyDeployStep`
  (`deploy_apply.go:175-241`) applies the step DDL + the version stamp + the
  cursor advance (→ `step_committed(k)`) + the receipt **in one transaction**, so
  a crash leaves the cursor at the last fully-committed step (resume re-runs the
  next idempotently). The prior separate-statement cursor advance is gone. ✅
- **D2 — F16b production assertions: now executable** (because 0021 is embedded):
  `TestOwnerBundle0021ProductionEmbedListingSplit` runs its production split. But
  this resolution is precisely what introduced the blocking D7' regression above
  — it is credit only if Option A is ratified.
- Migration **0044** is clean: additive **runtime-owned** (no owner DDL, no
  `owner_bundle_meta` touch), `state` CHECK includes `finalizing`, role-guarded
  GRANT, three tables keyed correctly. `LatestOwnerBundleVersion` /
  `RequiredOwnerBundleVersion` **stay 20**; `DDLRevokeOwnerBundleVersion = 21`.
  `LatestDaemonDBVersion = 44`. ✅
- The pure activation predicate is still correct at the predicate level: the
  decoupled-complete branch reads **neither `applied_owner` nor `revokeEmbedded`**
  (`deploy_activation.go:78-84`), so rows 15/16 take the identical conditional
  outcome (M6 + M7 honored in the predicate). ✅

---

## Still open / partial (fold into the revision)

### D1' — B1.1 is improved but still partial at the executable layer

- The pure F18 (`TestDeployBootPathDecisionTable`,
  `deploy_activation_test.go:65`) still passes `FingerprintInSync` /
  `PlanHashMatch` as **free booleans** to `DecideDeployActivation`, and its
  "spy" is the `got == DeployServeLegacy` **tautology** (the enum equals
  `serve_legacy`), not a behavioral spy on the real
  `ApplyMigrations`/`RecordSchemaFingerprint`. As an oracle of the pure predicate
  this is fine, but it does not, by itself, *prove* the orthogonality B1.1
  demands.
- The genuine construction was moved to the pg-gated
  `TestDeployActivationCompleteInSyncServesVerifyLive`
  (`deploy_pg_test.go:58`), which **does** record a real
  `schema_state.fingerprint == ExpectedFingerprint()` and a frontier-targeting
  plan, and proves revoke-independence live. Good — but (a) it constructs only
  the **`owner_bundle_meta`-absent (`==0`) bucket**, not the `==20` and `>=21`
  buckets B1.1 explicitly names; (b) it asserts the **decision value**
  (`DeployServeVerify`), still not a real call-tracking spy on the mutating
  functions; and (c) it cannot run at all while D7' breaks `pgtest` bootstrap.
  Net: B1.1's "across all three buckets over a real DB, without firing the
  `ApplyMigrations`/`RecordSchemaFingerprint` spies" is honored for the `==0`
  bucket + revoke-independence, but not for `==20`/`>=21` and not with real
  mutation spies. Tighten the live arm to seed the `==20` and `>=21` buckets and
  to assert the mutating path is not entered (a real spy / a
  `serve_verify`-implies-no-`ApplyMigrations` structural check), once D7' unblocks
  it.

### D6 — C3 per-step ownership reconcile (§3.3b) still absent (honestly conceded)

`applyDeployStep` does not snapshot new owner-owned oids and `ALTER … OWNER TO
striatumd_rw` before the terminal revoke, nor assert
`has_schema_privilege('striatumd_rw','striatumd','CREATE')` pre-step
(`deploy_apply.go:168-174` comment + DRAFT "Known remaining items" #1). The author
correctly fixed the attempt-1 doc-overstatement comment and is honest that it is
latent for the base-20 activation (only the terminal 0021 step, which creates no
new ownable object) and required for a fresh-DB deploy from base 0. §6.5 **(l)**
(two-role activation + post-deploy CREATE denial) depends on it. Acceptable to
defer to the verify/apply run **only if explicitly re-scoped** in the
PROPOSAL/roadmap; otherwise it is an under-delivery of (l).

---

## §6.5 acceptance-criteria coverage

| | Criterion | Status |
| --- | --- | --- |
| (a) | crash-resume stable key (BC-N1) | met at unit level (resume off stored transcript; receipt exactly-once); live GD → verify |
| (b) | divergent-binary resume refuses (M1) | **met** — byte arm (`VerifyStoredTranscript`) + **DB-stamp arm now real** (`VerifyAppliedDBStamps`, D3 resolved) |
| (c) | universal pre-revoke serve edge (BC-N2) | met at predicate level (F18 rows 5/7/9/11) + barrier-b in `connection.go` |
| (d) | self-heal does not commit revoke early (M2) | met at unit level (F16a + in-loop guard + filtered nil-fallback); forced FMA-007 pgtest → verify |
| (e) | complete-cursor flag-OFF revoke refusal (M3) | predicate + verb preflight met; **but see D7' — this gate now also fires on the production serve-boot path** |
| (f) | fresh-DB serve + shortfall halt (M5) | predicate met; **live arm blocked by D7' (pgtest bootstrap fails)** |
| (g)/(h) | no-revoke complete in/out-of-sync; legacy no-op (M6 r13/r15) | met at predicate level (F18) |
| (i)/(j) | revoke-embedding complete (M7 r16, `==0/==20/≥21`) | predicate + row15≡row16 met; **B1.1 live construction partial (D1')** |
| (k) | hash-chained receipt + doctor (BC-N1) | **now delivered** — real `deploy_receipt` chain + doctor enumeration (D4 resolved) |
| (l) | two-role activation + post-deploy CREATE denial (C3) | **not delivered** — C3 reconcile absent (D6); revoke-in-plan present but blocked behind D7' |

---

## Why `needs_revision` (mapped to the prompt's triggers)

- **Shadow-first invariant broken / existing serve-boot suites do NOT pass
  unchanged** → D7'. The flag is technically default-OFF, but its *absence no
  longer yields the unchanged legacy path* — a revoke-embedding binary
  self-halts `awaiting_deploy_config`, bricking production boot and the CI pg
  suite. This alone is dispositive.
- **A named test suite is effectively broken** → D7' (every `pgtest`-based
  suite + the author's deploy pg arms fail at bootstrap in CI), masked only by
  the local absence of `STRIATUM_PG_TEST_URL`.

None of the *accept*-direction disqualifiers are otherwise hit (flag default OFF
✅; `Latest`/`Required` stay 20 ✅; M7 not asserted unconditionally ✅;
`applied_owner`/`revokeEmbedded` not read on the decoupled complete branch ✅;
0044 owner-DDL-free ✅; 0021 not reachable via `owner-ddl apply` ✅;
build/vet/non-PG-test green ✅), and D3/D4/D5 are genuinely fixed — which is why
this is a *revisable* `needs_revision`, not a `reject`.

### Suggested revision focus (one cycle)

1. **Resolve D7' with an explicit operator decision** (Option B recommended:
   un-embed 0021 to restore the inert-landing / unchanged flag-OFF boot, and
   re-scope F16b-production + revoke-in-plan + (l) to the activation binary per
   §4.3; or Option A: ratify the breaking change and update the serve-boot
   suites/harness to set the flag). **Whichever path, prove the chosen flag-OFF
   serve-boot behavior with a test that actually runs** (a non-PG unit arm over
   `DecideDeployActivation` for the production `RevokeBundleEmbedded()` value, and
   the `pgtest` harness must bootstrap green).
2. Tighten the B1.1 live arm to seed the `==20` and `>=21` buckets and assert the
   mutating path is not entered with a real spy (D1') — once D7' unblocks the
   pgtest harness.
3. Either wire the C3 §3.3b reconcile or record its re-scope explicitly (D6).
4. Re-state the DRAFT's shadow-first claim accurately (the `none`-cursor →
   `serve_legacy` assertion is false for a revoke-embedding binary).
