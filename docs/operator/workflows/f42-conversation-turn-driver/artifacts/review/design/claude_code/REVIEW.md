---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["f42", "design-review", "ergonomics_dx"]
---

author: operator

# F42 turn-driver — ergonomics / developer-experience review

Posture: `ergonomics_dx`. I evaluated `DESIGN_SYNTHESIS.md` + `TASK.md` from a
first-time operator's perspective: is the proposed surface discoverable, hard to
misuse, and clear when it fails?

## Interrogation: 0 rounds (target unavailable)

I attempted to interrogate the live synthesizer as required, but completed **zero
rounds** — not by choice. `interrogation.open` failed with `target session is not
live (must be active)`. `list.sessions` confirms the target
`sess_ba69622e0b1301a866a437278a81e030` is **closed**, `close_reason:
interrogation_window_closed`, closed at `19:04:09Z`. My reviewer session
registered at `19:07:21Z` — roughly three minutes *after* the synthesizer's
interrogation window had already lapsed. There was no live target to question.

I proceeded with the document-only evaluation, which is consistent with this
packet's `review_policy` (`document_only`, `fresh` context). I record the missed
window as **finding 0** below, because the same timing fragility is squarely an
ergonomics problem for this very workflow.

## Verdict: accept_with_findings (medium)

The three load-bearing decisions are sound and internally consistent — the
supervisor-mode surface (§2), the `work.await_packet` envelope as turn signal
(§3), and the credential-strip boundary (§4). The design also lands two concrete
DX wins: it deletes the `/tmp/gemini-driver.sh` hack (TASK DoD #3) and inherits
crash-safety from the daemon so operators get no new recovery surface. None of
the findings below invalidate the architecture; they are gaps to close at
implementation. Severity is **medium** because finding #2 describes a *silent*
misconfiguration path that reproduces the exact bug F42 exists to fix.

## Findings

### 0 — (process) The interrogation window closed before reviewers could attach
The synthesizer closed on `interrogation_window_closed` ~3 min before this
review session existed. Whatever the idle/window timeout is, it is shorter than
the gap between "synthesis published" and "reviewers spun up." For a workflow
whose whole point is *interrogate the synthesizer before verdict*, the window
should be lease-extended or re-openable on a reviewer's `interrogation.open`,
or the synthesizer should be held live until all expected reviewers have
attached. As shipped, the required interrogation step is racy and silently
un-satisfiable. (Severity: medium — it defeated a REQUIRED step here.)

### 1 — F42 ships **no operator-facing command**; the only discovery path is a doc
The synthesis demotes codex's `striatum conversation drive` verb to an
optional/deferred debug entrypoint (§2, §6). The production affordance lives
entirely in adapter metadata (`self_driving: false`). Consequence: an operator
who wants "drive gemini autonomously in a conversation" finds **nothing** under
`striatum --help` or `striatum conversation --help`. The behavior is invisible
at the CLI — it is implicit in config the operator must already know to set.
That is a real discoverability cost the synthesis never weighs against the
"minimal surface" benefit. Recommend either (a) keep a documented, help-listed
`striatum conversation drive` / `-turn-driver` thin wrapper around the same
`Loop` so the capability is *findable*, or (b) explicitly state in docs and in
`supervise start` output that driven-mode exists and how it is selected. The
genericity-by-capability choice (§1.9) is right; it just must not also be
*undiscoverable*.

### 2 — Driven-vs-self-driving mode has no operator-visible signal (silent misconfig)
§2 says `supervise start` "branches on `self_driving`," but no operator-facing
indicator is specified: no `supervise status` field, no dashboard column, no
startup log line. So if gemini's adapter is missing the flag (defaults
self-driving per §6), the lane launches self-driving, gemini exits early — *the
original F42 bug* — and the operator gets no signal that the wrong mode was
chosen. "Hard to misuse" requires the mode be observable and a self-driving
single-shot lane be detectable. Recommend: surface the resolved mode in
`supervise status` + dashboard, and warn at launch when a known single-shot
adapter starts self-driving.

### 3 — The `-turn-driver` debug override is a footgun without a named failure mode
§2/§6 describe an override that "forces the mode regardless of adapter
metadata." Forcing *driven* mode on claude/codex would credential-strip a
self-driving agent and silently reduce it to a content generator. The synthesis
doesn't say whether this mismatch is gated, warned, or just breaks. Acceptable
for a debug flag, but document it as debug-only and name the mismatch behavior.

### 4 — "Floor parked" is well-intentioned but its operator-visibility is unspecified
§5 parks the floor on retry exhaustion and fires `session.report` +
`work.block` — good, it surfaces to the work queue. But the operator's actual
question is unanswered: *how do I see a conversation is stalled on a parked
floor, and distinguish it from "gemini is just slow"?* No `conversation.show` /
dashboard state for "floor parked, awaiting operator" vs "floor held,
generating" is specified. The recipe should spell out the exact command an
operator runs to diagnose a stalled conversation.

### 5 — The new recipe is named as a deliverable but not drafted; it is now load-bearing
§6 commits to rewriting the conversation operator recipe, but shows no
end-to-end example (register adapter w/ flag → open conversation → `supervise
start` each lane → observe → close). Per finding #1 the recipe is the *only*
discovery path for F42, so its quality carries the entire ergonomics surface.
Flagging that the design's DX rests on a recipe that does not yet exist;
implementation review should hold the recipe to a high bar.

### 6 — (low) `self_driving: false` reads as a double negative at the config site
Capability-keyed naming is correct (§1.9), but operators set this flag in
adapter config; `self_driving: false` is a double negative. `single_shot: true`
or `driven: true` would read more clearly where it is actually typed.
Naming-only, low severity.

## Bottom line
Accept with findings. The architecture is right; the gaps are about *surfacing*
it to operators — discoverability of the capability (#1), visible mode + misconfig
guard (#2), and stall observability (#4) are the ones that matter before this is
something a first-time operator can use without reading the source. Finding #0 is
a workflow-mechanics bug worth fixing independently so future interrogation steps
are not racy.
