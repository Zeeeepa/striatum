# Releasing striatum

Status: draft policy
Date: 2026-05-18

This document defines the version-bump and release cadence for
`striatum-orchestrator`. It exists because the early-development
cadence (25 minor bumps in 6 days; v1.31 → v1.55 between 2026-05-13
and 2026-05-15) treats tags as snapshots rather than release contracts.
Team adopters pin to versions and read CHANGELOG; the cadence has to
match that expectation.

## What a release means

A pushed `v*` tag triggers `.github/workflows/release.yml`: it builds
the wheel + sdist, cross-compiles the Go daemon for four platforms,
publishes to PyPI via trusted publishing, and creates a GitHub Release
with the CHANGELOG slice. Once tagged, the version exists on PyPI
forever; adopters can `pip install striatum-orchestrator==<version>`.

A tag is therefore a *contract* with adopters, not a development
snapshot. Don't tag what you wouldn't be comfortable with someone
running.

## Versioning policy

Striatum follows a relaxed semver:

- **Major (`vX.0.0`)**: a breaking change to the wire contract, the
  workflow JSON schema, the daemon RPC envelope, or stable exit codes.
  Reserved for substantive product transitions (e.g., the eventual
  `v2.0.0` post-substrate-cutover).
- **Minor (`v1.Y.0`)**: a new feature, a behavior change, or a
  meaningful bug fix. **Batched, not per-commit.** Aim for one minor
  per week maximum; skip weeks where nothing materially changed.
- **Patch (`v1.Y.Z`)**: only for fixes to a tagged release that needs
  re-publishing (e.g., a wheel build regression caught after tag).
  Rare.

## Release cadence

The aspirational pattern is **one release per week**, on Fridays, with
a meaningful changelog block. If a week produces nothing
adopter-visible, skip the week. Better to ship 25 meaningful tags per
year than 25 tags per week.

### Pre-release checklist

Before pushing a `v*` tag:

1. `make release-check` must pass locally. This runs lint, typecheck,
   tests, UI checks, metadata check, wheel-size check, and the
   fresh-clone smoke. Don't skip; don't override.
2. CI on `main` must be green. Treat any red as stop-the-line; do not
   tag through.
3. `CHANGELOG.md` `Unreleased` block must be filled in with
   adopter-impact prose: what behavior changes, which CLI verbs gained
   or lost flags, which exit codes were added, which schema migrations
   ran. Don't list internal refactors that don't change observable
   behavior.
4. `pyproject.toml` version bumped to the target version.
5. `Unreleased` block in `CHANGELOG.md` renamed to the new version with
   today's date.

### Tag command

```bash
git tag -s v1.X.0 -m "v1.X.0"
git push origin v1.X.0
```

The release workflow takes it from there.

## Changelog discipline

Each version block in `CHANGELOG.md` should answer one question for the
adopter: **what breaks or changes if I upgrade?**

Structure:

```markdown
## vX.Y.Z — YYYY-MM-DD

### Breaking changes
- (none) OR (list)

### New behavior
- (verbs added, flags added, semantics changed)

### Deprecations
- (what's still working but will be removed)

### Bug fixes
- (operator-visible fixes; internal refactors don't belong here)
```

If a block is more than ~30 lines, it's probably mixing too many
internal-detail items with the adopter contract. Split into multiple
real releases or trim the noise.

## Pinning guidance for adopters

Adopters pinning to a specific striatum version should:

- Pin to `striatum-orchestrator==<exact-version>` (not a range) until
  the substrate cutover lands (currently in flight per
  `docs/operator/BRIEF.md`). Minor bumps during this period may carry
  internal refactor risk.
- After substrate cutover and v2.0.0, pin to `~=<major>.<minor>` and
  upgrade minor versions on review.
- Read the CHANGELOG entry for every version between your current pin
  and your target. Striatum is alpha-status software; behavior changes
  are recorded but not always backwards compatible.

## Status

This policy is not yet enforced. As of 2026-05-18, version cadence
remains per-commit. The transition is part of P1-VERSIONING-POLICY in
[`docs/reviews/external/STRIATUM_REMEDIATION_PLAN_CLAUDE_OPUS_4_7_2026-05-18.md`](reviews/external/STRIATUM_REMEDIATION_PLAN_CLAUDE_OPUS_4_7_2026-05-18.md).
Treat this document as the target shape; the actual transition lands
once the substrate cutover (P0) closes.
