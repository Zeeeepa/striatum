# Dogfood 061 — RFC 0051 V1 auto-finalize from frontmatter

**Closes:** [RFC 0051](../../rfcs/0051-auto-finalize-from-frontmatter.md) (V1
landing), [TODO #41](../TODO.md), [ROADMAP §4.2](../ROADMAP.md).

**Why:** Across dogfood-054b / 055 / 055b / 056 the operator landed **8
on-behalf publishes in a single session** — every one of them
RFC 0046 audit-chained but cumulatively expensive. The pattern was
*gemini-class lane stall*: the agent wrote a well-formed artifact to
disk, then exited or stalled before invoking the closing CLI verbs
(`publish-artifact`, `verdict`, `complete`). The operator had to do all
three by hand.

RFC 0051 closes this by having the runner auto-finalize on the
periodic lease-tick when an `expected_artifacts[]` path exists on
disk with a valid front matter (byline matches, `verdict_intent`
parsed). The on-disk artifact already carries the information the
runner needs.

**Scope (RFC 0051 V1 §Design):**

- Two new event types: `artifact.auto_finalized`, `job.auto_finalized`.
- Reconciliation tick: visit each `claimed`/`running` session whose lease
  is healthy; check each declared `expected_artifacts[].path`; if present
  + parses + byline matches + frontmatter validates, auto-publish →
  auto-verdict (review jobs, from `verdict_intent`) → auto-complete.
- Feature flag: `STRIATUM_AUTO_FINALIZE_ENABLE=1` for V1; default-on in
  V1.1 once a clean dogfood validates.
- Refusal cases preserved: malformed frontmatter, byline mismatch,
  missing required artifact — all fall through to existing lane-stall
  behavior with operator-override available (RFC 0046 V1).

**Acceptance:** one end-to-end dogfood with **zero** operator-on-behalf
publishes on jobs whose agents wrote valid artifacts.

**Shape:** standard 8-job dogfood — 3 designs (codex/claude/gemini) →
synth (codex) → review_design (claude ergonomics_dx) → implement (codex,
single track) → 3-way build review (codex threat_model + claude
ergonomics_dx + gemini adversarial).

**Branch:** `striatum/dogfood-061-rfc-0051-auto-finalize`.

**Operating mode:** v1.55.0 daemon-required (no `STRIATUM_DAEMON_REQUIRED=0
STRIATUM_TEST_HARNESS=1` escape). Postgres is live; the RFC 0048 V1.5
substrate flip is complete.

**Pre-flight:**

```bash
striatum --version                                    # expect 1.55.0+
systemctl --user status striatumd.service             # expect active (running)
striatum daemon doctor --explain --json | jq '.data.explain | {method_count, pg_backed_count}'
striatum --repo . status --json                       # expect ok
git status --short --branch                           # expect clean ## main
```

**Post-landing:** version bump to v1.56.0; CHANGELOG entry; ROADMAP
§4.2 marked shipped; RFC 0051 status updated; merge to main; tag.
