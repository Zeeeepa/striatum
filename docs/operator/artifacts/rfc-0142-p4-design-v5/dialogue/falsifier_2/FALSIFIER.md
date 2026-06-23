# FALSIFIER - RFC 0142 P4 v5 owner-ddl exclusion review

author: falsifier-reviewer-004

## Revision check

The v5 holder genuinely resolves the v4 M2 safety break as a design contract.
The v4 reproducer was real in current source: `ApplyOwnerBundles` loads the full
`OwnerBundles()` slice, and on an FMA-007 cross-bundle dependency failure it
calls `ReapplyAllOwnerBundles` over that same slice; the reapply fallback also
loads `OwnerBundles()` when handed `nil` and applies every loaded bundle
(`go/pkg/db/owner.go:269`, `:277-289`, `:332-349`). If the activation binary
embedded 0021 under that shape, `striatum daemon owner-ddl apply` could commit
`REVOKE CREATE` outside the deploy plan.

The v5 text closes that specific branch. It defines
`DDLRevokeOwnerBundleVersion = 21`, `isNonRevokeBundle`, and
`OwnerDDLApplyBundles()` as the only slice for `owner-ddl apply`, while keeping
the full `OwnerBundles()` loader for `revokeEmbedded`, `ExpectedFingerprint`, and
the deploy plan. It binds both barriers to the self-heal path:
`ApplyOwnerBundles` loads the filtered slice, `applyPendingOwnerBundles` and
`ReapplyAllOwnerBundles` both add in-loop `isNonRevokeBundle` guards, and the
`ReapplyAllOwnerBundles(nil, ...)` fallback loads `OwnerDDLApplyBundles()` rather
than the full loader
(`docs/operator/artifacts/rfc-0142-p4-design-v5/dialogue/holder/HOLDER.md:360`,
`:383-424`). That is the missing v4 mechanism.

The specified F16 coverage also targets the right failure mode. It forces the
FMA-007 self-heal path with a synthetic 0021 loaded, then asserts 0021 is not
applied, `owner_bundle_meta` never records version 21, and
`has_schema_privilege('striatumd_rw','striatumd','CREATE')` remains true
(`HOLDER.md:833-837`). F12 and `G-revoke-last` are extended with the
owner-ddl side path before deploy, so the happy deploy-plan revoke-last case is
not the only thing being tested (`HOLDER.md:833`). The proactive audit also
names the current apply routes, the CLI entry, ACL reasserts, the deploy terminal
step, and the non-apply `OwnerBundles()` consumers (`HOLDER.md:769-786`). I did
not find another current `owner-ddl` dry-run/list surface; today the CLI has only
`striatum daemon owner-ddl apply`, which calls `db.ApplyOwnerBundles` and then
`ReassertWriteRevokes` / `ReassertReadRevokes`
(`go/pkg/cli/localcommands/daemon.go:90-144`).

So I do not claim M2 remains open as an early-revoke safety gap. C3's terminal
deploy step, C2's `RequiredOwnerBundleVersion = 20`, the forward-watermark
boundary at 21, BC-N2's non-complete cursor edge, and the P2 watermark/fresh-DB
interlock are carried forward coherently from this lens. M1 is also not
regressed from this lane's review: the v5 spec adds the full stored-transcript
byte check on resume and before the finalizer, plus DB-stamp checks for
already-applied steps and the F15 wrong-binary tests.

## Challenge: F16's production-loader assertion contradicts the rollout order

### Claim attacked

The holder's M2 implementation order says the `owner.go` filter surface and
`TestOwnerDDLApplyExcludesRevokeBundle` land first, while 0021 is not authored
until the final activation step:

- Step 2 lands `DDLRevokeOwnerBundleVersion`, `OwnerDDLApplyBundles()`, both
  guards, the nil-fallback split, `TestOwnerDDLApplyExcludesRevokeBundle`, and
  the grep test; the text says this is inert until 0021 is authored
  (`HOLDER.md:845-849`).
- Step 7 later authors owner bundle 0021 (`HOLDER.md:870-872`).
- But the step-2 unit test is specified to assert that production
  `OwnerBundles()` **does contain 0021** so `revokeEmbedded` /
  `ExpectedFingerprint` see it (`HOLDER.md:439-442`).

Those three requirements cannot all be true in a green, incremental rollout. If
0021 is not yet in the embedded production `ownerBundleFS`, `OwnerBundles()` will
not contain 21. A test that asserts the opposite fails before activation. If the
assertion is delayed until 0021 is authored, then the step-2 claim that F16 lands
with the filter surface is overstated. If the test uses a synthetic 0021, then it
does not prove the production full-loader/revokeEmbedded condition as worded.

### Concrete refutation

Build phase 2 exactly as specified, before adding
`go/pkg/db/sql/owner/0021_*.sql`:

```text
OwnerBundles() -> production embedded files 0001..0020
OwnerDDLApplyBundles() -> production embedded files 0001..0020
TestOwnerDDLApplyExcludesRevokeBundle assertion (b):
  OwnerBundles() DOES contain 0021
```

Assertion (b) fails. The filter is in fact a no-op before 0021 exists, which the
rollout text acknowledges, but the named test's production-loader assertion
requires the opposite state. That makes the "lands first, inert" phase
un-buildable as written, or forces implementers to weaken/skip part of F16 until
later without the spec saying so.

### Strongest rebuttal

The holder can argue this is a test staging detail, not a safety flaw: the F16
pgtest already says "embed a synthetic 0021 revoke bundle," and the in-loop
guards plus `OwnerDDLApplyBundles()` can be tested against a synthetic slice
before production 0021 exists. Once 0021 is authored, a second production-loader
assertion can prove `OwnerBundles()` sees it while `OwnerDDLApplyBundles()` does
not.

That rebuttal is plausible, but it is not the contract currently written. The
spec names one `TestOwnerDDLApplyExcludesRevokeBundle`, assigns it to the
pre-0021 phase, and gives it a production `OwnerBundles()`-contains-0021
assertion. Without splitting the test, the implementation either breaks `make
test` before activation or leaves the embed/listing half of M2 unverified until a
later phase by convention rather than contract.

### Required repair

Split F16 into phase-aware checks:

1. **Pre-0021 / inert phase:** use a synthetic bundle list or a test hook to prove
   `OwnerDDLApplyBundles`/`isNonRevokeBundle` exclude every bundle `>= 21`,
   `applyPendingOwnerBundles` and `ReapplyAllOwnerBundles` skip a hand-passed
   synthetic 0021, and `ReapplyAllOwnerBundles(nil, ...)` uses the filtered
   loader. Do not assert production `OwnerBundles()` contains 0021 yet.
2. **Activation phase, after 0021 is authored:** assert production
   `OwnerBundles()` contains 0021, `ExpectedFingerprint()` includes its bytes,
   `revokeEmbedded` is derived from the full loader/presence of the file, and
   production `OwnerDDLApplyBundles()` excludes it.
3. Keep the forced-self-heal pgtest in the activation phase or make its synthetic
   fixture explicit, but require it to prove it actually reaches
   `ReapplyAllOwnerBundles` through `isCrossBundleDependencyError`, not just the
   pending loop.

## Verdict

M2's safety invariant is substantively resolved: v5 binds 0021 out of every
`owner-ddl apply` route, including the FMA-007 self-heal reapply branch, while
preserving the full-loader path needed for `revokeEmbedded`,
`ExpectedFingerprint`, and the deploy plan. I found no remaining path that
commits the revoke early or regresses C2/C3 from the owner-ddl lens.

The concrete falsification is narrower but still material to the implementation
spec: F16's stated test contract does not fit the stated rollout order. The
adjudicator should require the phase-aware split above so the M2 filters can
land green before 0021 exists and the production embed/listing split is proven
once 0021 is authored.
