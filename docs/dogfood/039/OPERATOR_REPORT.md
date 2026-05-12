# Dogfood 039 Operator Report

author: operator
date: 2026-05-13
status: complete

## Run

- Run ID: `run_8f8f347a99ef4e95993db8c288f2ad59`
- Workflow: `dogfood-039-rfc-0037-web-ui-ergonomic-improvements`
- Branch: `striatum/dogfood-039-rfc-0037-web-ui-ergonomic-improvements`
- Final state: `completed`
- Final job tally: 7 jobs completed, 0 canceled, 0 open blockers, 0 human checkpoints.
- Duration: 1h 43m (run prepare ~11:27Z → run.completed 13:10:40Z).

## Scope

RFC 0037 V1: web UI ergonomic improvements over the existing
RFC 0013/0022/0023/0024 web UI base. Ships 10 ergonomic wins from
the post-dogfood-037 UI survey:

1. Run-list filter + duration column + empty state
2. Workflows-index filter + last-modified column + empty state
3. Doctor problem grouping + terminal-hide toggle + empty state
4. Localtime toggle persisted in localStorage
5. Graph-node hover tooltips on run detail
6. Keyboard shortcuts (`g r` / `g w` / `g c` / `g d` / `?`)
7. `app.css` dark-mode parity blocks per the audit list
8. Next-actions panel promotion (banner below run header)
9. Empty-state copy with copy-paste CLI examples
10. Documentation updates (HOW_TO_HUMAN + CHANGELOG + RFC 0037 status)

No new runtime dependencies, no SPA conversion, no visual redesign.
Server-rendered Jinja2 + vanilla JS only.

ergonomics_dx posture for both reviews (UX-shaped RFC).

Deferred:

- Filter-state-in-querystring (RFC 0037 V1.5)
- Sticky-positioned next-actions banner (future RFC)
- Keyboard-shortcut configurability (future RFC)
- Mobile-first responsive overhaul (separate RFC)

## Run Shape

Anti-friction notes baked into all three harness profiles addressing
every observed pattern (dogfoods 036/037/038):

- Gemini: "write artifact directly, don't surface strategy and exit" +
  EXACT `striatum.finding.v1` front-matter shape for finding kinds
  (handoffs need no front matter)
- Claude: "write artifact directly, don't ask clarifying question and
  exit; if `striatum ack` is denied, write the artifact and exit
  normally — operator publishes on your behalf"
- Codex: "focused pytest invocation before `make test` to avoid lease
  expiry beyond ~30 minutes" + reminder of lease window

The implement prompt is the most sub-agent-aggressive yet (one sub-
agent per template + per JS file + per CSS dark-mode block + per
empty-state + per doc surface + per test file). RFC 0037 is the most
parallelizable dogfood surface to date.

```
3 fresh designs (codex / claude / gemini, parallel)
  ↓
synthesize_design (codex)
  ↓
review_design_ergonomics (gemini, ergonomics_dx, fresh)
  ↓
implement (codex with aggressive sub-agent delegation)
  ↓
review_build_ergonomics (claude_code, ergonomics_dx, fresh, repo-level)
```

## Operator Interventions (running log per D091)

### 1. 2026-05-13 11:44Z — Publish-on-behalf for `design_claude_code` + `design_gemini`

Routine pattern from dogfoods 031-038. Both lanes wrote DESIGN.md
files (claude at 11:36-ish, gemini at ~11:34). Both exited without
calling `striatum ack`. Operator published via the established
pattern.

Both designs had clean bylines (plain `author: <slug>` on a markdown
line after the `# Heading`); no front-matter shape issues this time
(anti-friction note for gemini "handoffs don't need front matter"
landed correctly).

- claude_code: `sess_f2c69a7f36de4a67884678031f4ba12f` /
  `lease_8dc989f771d546fcabf063d963394541` /
  `art_4da392d34f79440593437dc4bd47f152`
- gemini: `sess_b454b99024ee4dd4b4bb0c7a6b135ecb` /
  `lease_4292ed63450949ecb1b4668e11dc07b6` /
  `art_b5be0d4fd9e6430587ed8ab97a488289`

### 2. 2026-05-13 12:06Z — Publish-on-behalf + front-matter shape fix for `review_design_ergonomics`

Gemini wrote the REVIEW.md with mostly-correct front matter
(verdict_intent + severity from valid set + tags as JSON array +
byline as plain markdown line AFTER the block — all the anti-
friction notes landed). **Missing one required field**:
`artifact_kind: "finding"`. The publisher refused with exit code 6.

The reviewer-role doc and design-review prompt both include the
exact front-matter template with all five required fields
(schema_version, artifact_kind, verdict_intent, severity, tags),
but gemini still skipped `artifact_kind`. Worth adding a more
direct callout in the next iteration: "the front-matter block must
contain ALL of: schema_version, artifact_kind, verdict_intent,
severity, tags — none are optional".

Operator added the missing `artifact_kind: "finding"` line and
also corrected the tags from `["ergonomics", "dx", "web-ui",
"rfc-0037"]` to `["ergonomics_dx", "rfc-0037", "web-ui"]` to match
the registered posture token. Re-published successfully.

Verdict: `accept` severity:low.

- Session: `sess_3fb9d0d1d6814c4e95201b82ffc5507c` (reviewer-gemini-1, fresh)
- Lease: `lease_9b0f502e02dc44629b074ea5ca207fd9`
- Artifact: `art_1a730f18e4a540188f663140f921dc46`
- Verdict ID: `verdict_25a85f94448e4292b60a027e8ded0844`

### 3. 2026-05-13 12:06Z — Fresh implementer-codex session for `implement`

Routine session-role boundary. Registered fresh `implementer-codex-1`,
started supervisor, claimed packet.

- Session: `sess_58d88f8a36f6473e96bae01290036af2`
- Supervisor: `sup_6e79f0ee080f4ab19280537db333943b`

### 4. 2026-05-13 13:10Z — Publish-on-behalf for `review_build_ergonomics`

Claude wrote the REVIEW.md with the CORRECT front-matter shape this
time including `artifact_kind: "finding"` (the field gemini missed in
intervention #2). Anti-friction notes all landed: byline after the
front-matter block as plain markdown line; `verdict_intent`; valid
severity; `tags` as JSON array. Verdict: `accept_with_findings`
severity:medium with non-blocking findings.

Operator published via the established pattern (claude --print
denied ack).

- Session: `sess_99db163ac89a4671ad5a43f22abd7fd9` (reviewer-claude_code-1, fresh)
- Lease: `lease_5bc9a1f2fbd0468a80587b6f5a86b91e`
- Artifact: `art_c2c83ef8cd514b7b9a15b608f8c081d4`
- Verdict ID: `verdict_65f4cf9cb6784b6a93111390708e8222`

## Notable Wins

1. **Run finished with zero cycles.** Design `accept` severity:low;
   build `accept_with_findings` severity:medium. Both first try.

2. **Codex drove its own claim/ack/publish/complete loop for
   implement** — 28 min (vs dogfood-038's 38min + recovery). The
   focused-pytest anti-friction note is now landing consistently
   across implement runs.

3. **All three claude/gemini/codex anti-friction notes landed
   correctly for the artifact-producing surfaces**. The remaining
   friction in this run was the `artifact_kind` field gap on
   gemini's front matter (intervention #2) — addressable via a
   more direct callout in the gemini prompt fragment.

4. **The implementation completed every named ergonomic surface from
   RFC 0037 V1**: localtime toggle + base.js scaffold; run-list
   filters + duration column; workflows-index filters + last-modified;
   doctor grouping + terminal-hide; graph tooltips; keyboard
   shortcuts; app.css dark-mode parity; next-actions promotion;
   empty states. All visible via the tailscale bridge at
   https://proximal.tail0ecc2e.ts.net:8443/.

5. **The aggressive sub-agent delegation guidance worked**. Codex
   modified 10+ templates + JS files + CSS blocks in 28 min,
   plus 4 doc surfaces, plus tests. Largest parallelizable
   implement run to date.

## Operator Decisions Recorded

- 2026-05-13 12:06Z — Added missing `artifact_kind: "finding"`
  field to gemini's design review front matter (the publisher
  refused with exit code 6 without it) and corrected the `tags`
  from generic descriptors to the registered ergonomics_dx
  posture token. Verdict reasoning + body content is entirely
  gemini-authored; only the front-matter shape was corrected.

## Recorded Risks and Follow-ups

- **`artifact_kind` field gap in gemini front-matter** is a new
  friction-shape note: gemini included 4 of the 5 required
  front-matter fields but skipped `artifact_kind`. The harness
  profile prompt + reviewer role doc both showed the exact
  template with all 5 fields present, but gemini still skipped
  one. Worth a more direct callout: "the front-matter block must
  contain ALL of: schema_version, artifact_kind, verdict_intent,
  severity, tags — none are optional".
- Non-blocking findings from the build review (in
  `docs/dogfood/039/review/build/ergonomics/REVIEW.md`,
  severity:medium) can be folded into normal bugfix iterations.

## Verification Artifacts

- `docs/dogfood/039/RUN_SUMMARY.md` (exported 2026-05-13 13:11Z)
- `docs/dogfood/039/EVIDENCE.md` (exported 2026-05-13 13:11Z)
- `docs/dogfood/039/BUILD_HANDOFF.md` (codex, 2026-05-13 12:35Z)

Implementation verification (from BUILD_HANDOFF):

- `make install`: passed
- `make lint`: passed
- `make typecheck`: passed
- `make test`: passed (existing baseline + new JS-equivalent test additions)
- `make smoke`: passed
- Manual UI walkthrough recommended via the tailscale bridge URL
  https://proximal.tail0ecc2e.ts.net:8443/ — operator can hit each new affordance.


## Deliberately Left Out

The operator does not author design, synthesis, review, or
implementation content. Any operator-on-behalf publishes will be
documented above with full session/lease/message/artifact IDs.
