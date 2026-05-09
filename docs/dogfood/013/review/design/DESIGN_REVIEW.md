---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0013 step 7 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Verdict: `accept`

## Pinned contracts (verified)

- **`/v1/health` extension**: one-line `allow_mutations: bool`
  add. Backwards-compatible with the existing envelope. ✓
- **Mutation set (5)**: continue blocker, cancel blocker,
  record verdict, record decision, requeue stale review-only.
  All map 1:1 to existing CLI verbs already covered by the RFC
  0012 mutation gate. No new server-side logic; only the SPA
  call-site. ✓
- **`POST /v1/invoke` reuse**: SPA POSTs the literal argv. The
  service already enforces the gate, exit codes, and parser
  validation. The SPA is a thin shell. ✓
- **Hidden when gate is off**: SPA caches `allow_mutations`
  from `/v1/health` and renders / hides accordingly. Server-
  side refusal still works as a defence-in-depth. ✓
- **Confirmation modal**: shows the literal argv before
  firing. Destructive actions (cancel blocker, reject verdict)
  get a red confirm button. ✓
- **CSP unchanged**: no external deps, no `eval`, no inline
  handlers, no `innerHTML` for user data. ✓
- **Test plan**: 10 cases cover both the read-side health
  extension and the four mutation paths plus a parser-error
  guard plus the SPA grep guard plus the no-external-URL
  invariant. ✓

## Notes

- **No `claim-next` button** is the right call. Claiming work
  needs a session id the SPA can't plausibly own. Future RFC.
- **No file-upload UX for `publish-artifact`** is also right
  for V1. Operators publish from disk via existing CLI before
  recording a verdict; V1 ships *verdict* alone (not the
  combined `submit-review`). A V1.5 follow-up could add a
  drop-zone.
- **Lease lookup for verdict**: synthesis flagged the
  two-fetch pattern (run detail + per-job detail to get the
  active lease). Acceptable; the SPA already does sequential
  fetches for other views.
- **Decision path validation**: client-side check (no `..`,
  no leading `/`, no `.striatum/`) plus server-side canonical
  refusal. Defence-in-depth.
- **CSRF**: out of scope for V1, correctly. Loopback service
  has no cross-origin exposure. Tailscale bridges expose the
  service across a network — V1.5 should add an HMAC pattern.

## Decision

`accept`. The design is the cleanest possible step 7: a thin
SPA shell over the existing mutation gate. No new schema, no
new CLI verbs, no new server logic. The runner already enforces
every invariant; the web UI just becomes a click-driven
front-end to the same surface.
