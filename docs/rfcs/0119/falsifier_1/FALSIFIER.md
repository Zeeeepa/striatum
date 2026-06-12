# RFC 0119 Falsifier 1 — Corpus Invariants and State-Transition Dependence

author: falsifier-unknown-model-001

Status of this document: falsifying challenge for the RFC 0119 ratification
gate, posture = corpus invariants + state-transition dependence. It does not
decide the gate; the adjudicator ledger does. Every objection below is
load-bearing: I state the holder's strongest rebuttal and whether the objection
survives it. Objections I could already rebut have been cut (see §6).

## The seam the RFC names is inside a state transition

The whole acceptance case rests on one structural claim (HOLDER_CASE §2; RFC D3
`:97-99`): the hot-tier read and digest render run "at **scaffold time only**,"
and are therefore "**not** a state transition." That claim is false at the two
seams the RFC itself names. Both `buildPacket` and `HandleWorktreeCreate` are
links *inside* state-changing daemon mutations, not a separate scaffold phase.

### Objection 1 (decisive) — `buildPacket` runs inside the claim transition

RFC D3 (`:94-96`) seats the digest at "`buildPacket` (`go/pkg/mutations/claim.go`)."
In source, `buildPacket` is not a standalone scaffold step — it is called at
`go/pkg/mutations/claim.go:229`, in the middle of `claimChosenJob`, whose own
doc comment (`claim.go:187-189`) describes it as performing "the lease +
queue-message + job **state transitions**." The call sequence inside that one
open transaction `tx` is:

1. `INSERT … leases` (`claim.go:205-213`)
2. `UPDATE queue_messages SET state='claimed'` (`claim.go:214-221`)
3. `UPDATE jobs SET state='claimed'` (`claim.go:222-228`)
4. **`buildPacket(ctx, tx, …)`** (`claim.go:229`) ← the RFC's seam
5. `INSERT … work_packets …` (`claim.go:242-260`)
6. `appendEvent(… "queue.claimed" …)` (`claim.go:264`)

`buildPacket`'s signature receives the transaction (`runner any`,
`claim.go:438-449`) — the only DB handle in scope at the seam **is** the claim
`tx`. And the caller fails the whole transition on any error from it:
`if err != nil { return nil, err }` (`claim.go:230-232`), which rolls the claim
back — no lease, no claimed job, no work packet. So any retrieval seated at
`buildPacket` sits squarely on the claim state transition. This directly
contradicts the RFC's "this is **not** a state transition."

It is worse than a wording slip, because Postgres transaction semantics make the
failure non-local. If a recall query executed on `tx` errors *for any reason*
(the FTS index/migration not yet applied — the RFC concedes the projection
"none exists today," RFC `:48`/`:91`; a `websearch_to_tsquery` parse error on an
edge query string; a `statement_timeout`; lock contention on the artifact
stream), Postgres aborts the entire transaction. Every later statement — the
`INSERT … work_packets` at `claim.go:242` — then fails with "current transaction
is aborted" regardless of any Go-level error swallowing. To be safe the read
must run on a *separate* connection, but the seam the RFC names only has the
claim `tx` in scope and the RFC gives no separate-connection requirement. The
unsafe implementation is the default-likely one.

**Holder's strongest rebuttal.** (a) `claim`/`claim_next` is *not* in the
enumerated invariant list — spec.md (`:1243-1245`) names `ack`,
`publish-artifact`, `complete`, `verdict`, recovery, `run prepare`, `run start`,
`corpus export`, not claim. (b) The read is over the *daemon's own PG*, not the
warm tier, so an **absent external consumer** never makes `buildPacket` fail;
invariant (3), as worded, is about the external consumer and is preserved.
(c) RFC D3 promises the read is "fail-soft to an empty shelf" (`:98-99`).

**Does it survive? Yes — sharpened, not rebutted.**
- On (a): the general rule is "**No state transition** … that fails," and the
  source's own vocabulary calls claim a state transition (`claim.go:187-189`).
  Reading the parenthetical as exhaustive would mean the invariant has a silent
  hole exactly where the RFC proposes to write — the gap an invariant exists to
  close. And the wedge is transitive: a claim that cannot complete means the
  lane never gets a packet, so the *enumerated* transitions on that job
  (`ack`/`complete`/`verdict`) become unreachable. Invariant (3)'s purpose —
  "everything works (a thinner shelf, never a wedge)" (RFC `:61-62`) — is
  violated in substance even under the narrow reading.
- On (b): I concede invariant (3) *as literally worded* (external-consumer
  absence) is preserved — but that is not the property the RFC sells. It sells
  "scaffold time only, not a state transition." That property is false here.
  Worse, the RFC trades a *banned external* dependency for a *new local* one
  (the FTS migration/index, the query parser) on the claim critical path, and
  the local dependency is exactly the kind the augmentation boundary exists to
  keep off state transitions.
- On (c): "fail-soft to an empty shelf" is a *behavioral* guard, and it cannot
  rescue an aborted Postgres transaction once the recall SQL has errored on the
  shared `tx`. Fail-soft also only covers "no rows / empty result," not
  "statement raised." The safety the RFC advertises as *structural* (read is off
  the transition path) does not exist; what is left is a fragile behavioral
  promise the proposed guardrail does not test (Objection 3).

### Objection 2 — the worktree seam wedges create and can orphan a worktree

The RFC names the same render at "`HandleWorktreeCreate`
(`go/pkg/mutations/worktree.go`)" (RFC `:94-95`). That handler is a transactional
mutation: it validates and records worktree state under `withTx`
(`worktree.go:64`, `:100-133`) and performs the irreversible `git worktree add
--detach` between the two tx blocks (`worktree.go:92-98`). A digest render placed
here (a `RecallMemory` read plus a `.striatum/memory/relevant.md` file write)
that errors and propagates makes `HandleWorktreeCreate` return an error — i.e.
the worktree-create transition fails. Because the physical `git worktree add`
already ran at `:92` while the `job_worktrees` row is only inserted at `:106`,
a failure in the render window can also leave a **dangling git worktree with no
recorded row** — a state-integrity regression, not just a thinner shelf.

**Holder's rebuttal.** Same "fail-soft to an empty shelf" (RFC `:98`); a careful
implementer wraps the read/write in a swallowing helper and skips the file.

**Does it survive? Yes, partially.** Fail-soft *can* be implemented safely here
because the file write is outside the tx and a separate connection is available
in this handler — but nothing in the RFC *requires* it, the acceptance bar does
not test it (Objection 3), and the orphaned-worktree failure mode is unaddressed.
The objection survives as "the RFC authorizes a seam whose safe implementation is
unspecified and untested," which is enough to block a clean ratification of the
"not a state transition" claim.

### Objection 3 — the proposed guardrail tests the wrong failure mode

RFC D4 (`:104-108`) and Acceptance (`:122-126`) pin the safety with a guardrail:
"absent warm tier ⇒ all state transitions still succeed." But the recall read is
over the **daemon's own PG** (RFC D3 `:90-93`), decoupled from warm-tier
presence. Removing the warm tier does not exercise the recall path at all, so a
green "warm-tier-absent" suite proves nothing about the real wedge surface from
Objections 1–2 (missing FTS migration, tsquery parse error, statement timeout,
tx-abort poisoning). The acceptance criterion the holder calls "precisely the
falsification surface" (HOLDER_CASE §"Why the gate should clear") is aimed at the
one failure mode that *cannot* occur on this path, and is blind to the ones that
can.

**Holder's rebuttal.** D4 also says "a guardrail test asserting … no `memory.*`
ever enters the registry"; the FTS errors are ordinary read-path bugs caught by
normal tests.

**Does it survive? Yes.** The no-`memory.*` guardrail is real and I do not
contest it (see §6). But "ordinary read-path bug" is the point: by seating an
*ordinary read that can error* inside the claim/worktree-create transactions, the
RFC converts a read-path bug into a transition wedge, and the acceptance bar it
proposes is specified so as not to detect it. Ratification should require a
guardrail that injects a recall-read failure and asserts claim/worktree-create
still commit — which the current acceptance text does not.

## Durable-provenance posture

### Objection 4 — the eviction list evicts durable-provenance kinds

D1/Goal 3 promise "**durable provenance stays canonical in git**" and that
eviction touches "only run exhaust + unsynthesized intermediates" (RFC `:50-52`,
`:69-74`). But the only concrete instantiation the RFC gives — the Open-Questions
default (RFC `:137-141`) — is "progress_note, operator_report, *_ledger,
unsynthesized design candidates." Those are not exhaust:

- `operator_report` is emitted **as durable provenance by `corpus export`**
  itself — spec.md (`:1218-1220`) lists "operator reports" in the redacted
  durable-provenance bundle.
- `operator_report`, `progress_note`, and the `*_ledger` family
  (`findings_ledger`, `support_ledger`, `action_item_ledger`,
  `collaboration_ledger`) are all **front-matter-carrying durable artifacts**
  with V1 schemas (AGENTS.md "Front-matter–carrying artifacts"); the
  `collaboration_ledger` is a required published artifact of *this very ratify
  workflow*.

So the RFC's own default eviction list moves out of the git tree exactly the
kinds spec.md treats as durable provenance — contradicting "durable provenance
stays canonical in git."

**Holder's rebuttal.** It is an *open question* with a revisitable default; D178
ratifies only the *axis* ("eviction scope = exhaust"), not the list, and "exhaust"
can later be defined to exclude these kinds.

**Does it survive? Yes, partially (a must-fix, not a kill).** Ratifying an axis
whose sole worked definition contradicts the durable-provenance invariant is not
safe: "exhaust" is left undefined while the one example overlaps the corpus-export
durable set. The gate should not clear until D178 pins an "exhaust" boundary that
provably excludes every `corpus export` durable-provenance kind and every
front-matter artifact kind.

### Objection 5 — the holder overclaims "purely additive; no invariant relaxed"

HOLDER_CASE §"Why the gate should clear" asserts "every line the RFC touches in
`spec.md` it touches *additively* … No existing invariant is relaxed." That is
false for the transcript-export prohibition. spec.md (`:2447-2454`) says PTY /
agent-loop logs are "operational scratch, not transcript provenance … **not
included in evidence exports, corpus exports, or run archives**." RFC D2
(`:78-84`) adds a `lane_trajectory` corpus-export class sourced from "agent-loop
transcripts (tmux capture / `conversation_trajectories`)." Authorizing transcript
capture into `corpus export` **relaxes** that prohibition; it does not merely add
to it.

**Holder's rebuttal.** The spec text itself says "no durable transcript
capture/export *without an explicit product decision*" — RFC 0119 *is* that
decision, so this is relaxation-by-decision, which is allowed.

**Does it survive? Yes, narrowed.** The RFC may indeed be that decision, so this
does not block acceptance of the *mechanism* — but it falsifies the holder's
framing that nothing is relaxed, and it surfaces an unmet obligation: corpus
export guarantees byte-identical, deterministic re-export (spec.md `:1224-1226`),
and the RFC asserts `lane_trajectory` stays "pull-only and deterministic"
(RFC `:86`) without specifying how free-form transcripts ("may contain provider
output, tool text, prompts, or secrets," spec.md `:2451-2452`) are normalized and
redacted to a deterministic byte stream. Ratification should require that
redaction/normalization contract, not accept the determinism claim on assertion.

## What I am NOT claiming (cut as non-load-bearing)

- **"`lane_trajectory` is a streaming/push surface."** The falsify prompt invites
  this, but RFC D2 (`:86`) states the class is "pull-only and deterministic …
  never streamed," consistent with spec.md `:1228-1230`. I can rebut a streaming
  objection from the RFC text alone, so I cut it.
- **"The RFC adds `import engram` / a consumer-client link into daemon source."**
  RFC Goal 4 / Non-Goals (`:53-65`) keep the warm tier reached only via delivered
  inert content; the hot-tier read is over local PG. Invariant (1) is not
  broken — it is generalized (RFC `:40-41`). I cut this; it is not load-bearing.
- **"A `memory.*` capability enters the registry."** D4 names the read `recall.*`
  and adds a guardrail asserting no `memory.*` (RFC `:103-108`). I do not contest
  invariant (2).

## Verdict

The acceptance case does **not** survive its own central structural claim. The
load-bearing, surviving objections are:

1. **(Decisive)** The hot-tier read's named seams (`buildPacket` at
   `claim.go:229`; `HandleWorktreeCreate`) are *inside* state-changing
   transactions, not a separate scaffold phase, so "this is not a state
   transition" (RFC D3 `:97`) is false; seating a recall read there puts the
   claim/worktree-create transitions at risk of a tx-abort wedge over the
   daemon's own PG (FTS migration drift, query errors, timeouts). Invariant (3)'s
   letter (external-consumer absence) holds; its spirit and the RFC's own safety
   claim do not.
2. The proposed acceptance guardrail ("warm tier absent ⇒ transitions green")
   is blind to that wedge because the read is local-PG, decoupled from warm-tier
   presence. The bar tests the one failure mode that cannot occur and misses the
   ones that can.
3. The default eviction list evicts `operator_report` / `progress_note` /
   `*_ledger` — kinds spec.md exports as durable provenance — contradicting
   "durable provenance stays canonical in git" until "exhaust" is pinned to
   exclude them.

Minimal conditions for the gate to clear honestly: (a) re-ground the hot-tier
safety on a *structural* guarantee — recall reads on a separate connection,
never the claim/worktree-create `tx`, with a guardrail that injects a recall-read
failure and asserts those transitions still commit; (b) restate D3 to drop the
false "not a state transition" framing and own the fail-soft-on-a-separate-path
mechanism; (c) define "exhaust" in D178 to provably exclude every corpus-export
durable-provenance kind and every front-matter artifact kind; (d) specify the
`lane_trajectory` redaction/determinism contract rather than asserting it.
