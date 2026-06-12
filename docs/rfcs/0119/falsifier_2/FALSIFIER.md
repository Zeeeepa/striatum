# RFC 0119 Falsifier 2 — Durable-Provenance Boundary

author: falsifier-unknown-model-002

Status of this document: falsifying challenge for the RFC 0119 ratification
gate, posture = **durable-provenance boundary** (does the eviction policy or any
export class erode the `spec.md` durable-provenance boundary?). It does not
decide the gate; the adjudicator ledger does. Every objection below is
load-bearing: I state the holder's strongest rebuttal and whether the objection
survives it. Objections I could already rebut — and the parts of the
durable-provenance story that an *accepted* RFC already settled — are cut in §5
so the surviving objections are not padded with already-decided baseline.

The decisive finding: the eviction policy's only worked definition of "exhaust"
evicts artifact kinds that **accepted RFC 0072** and `spec.md` both pin as
git-tracked durable provenance. This is not a wording gap; it is RFC 0119
contradicting an accepted decision it cites as its own mechanism.

## What the accepted baseline already settled (so I do not re-litigate it)

Eviction of run exhaust to PG/blobs is **not new** and not, by itself, an
erosion. [RFC 0072](../../0072-blob-backed-artifact-storage.md) (Status:
**accepted**, `:3`) already moved per-run artifact *bodies* out of the git tree
into S3 while keeping the authoritative reference (id, kind, byline, run/job
linkage, content sha256) in PG, and it explicitly preserved "every existing
audit and provenance contract: append-only artifacts, sha256-anchored audit
chain, byline integrity, redacted corpus export, replay-stable hashes"
(RFC 0072 `:30-31`). Its acceptance bar proved corpus export stays
byte-stable for blob-backed bodies ("artifact-body sha256s match the
pre-migration values," RFC 0072 `:266-268`), and `striatum archive create` is
already `artifact_content_policy: metadata_only` (`spec.md:1261-1263`) on the
post-0072 assumption that bodies live in S3.

So I **cut** three objections a naïve durable-provenance attack would raise (see
§5): "eviction breaks corpus-export determinism," "eviction defeats the
metadata-only archive," and "moving bodies to PG/blobs erodes provenance." For
the kinds RFC 0072 already moved (`finding`, `synthesis`, `support_ledger`,
`action_item_ledger`, `findings_ledger`, `harness_improvement_proposal`,
`progress_note`, RFC 0072 `:60`), those questions are answered and I will not
pretend RFC 0119 re-opens them.

What RFC 0119 newly evicts — and where the erosion actually lives — is the gap
between RFC 0072's **carve-outs** and RFC 0119's **default exhaust list**.

## Eviction-policy posture

### Objection 1 (decisive) — the default exhaust list evicts `operator_report`, which accepted RFC 0072 pins as git-tracked durable provenance

RFC 0119 gives exactly one concrete instantiation of its "eviction scope =
exhaust" axis: the Open-Questions default (RFC `:137-141`) —
"progress_note, **operator_report**, *_ledger, unsynthesized design candidates."
It reaches that list via "the RFC 0072 pattern" (RFC `:51`).

But RFC 0072 — the very mechanism RFC 0119 cites — makes evicting
`operator_report` an explicit **Non-Goal**:

> "Moving decisional artifacts out of the working tree. `decision`,
> `escalation`, `work_plan`, `operator_brief`, **`operator_report`** kinds
> remain git-tracked: they exist for human PR review and cold-start reading,
> not for transient run inspection." (RFC 0072 `:44-47`)

and its boundary table puts `operator_report` in the **Git-tracked** row, not
the blob row (RFC 0072 `:59`). `spec.md` independently lists "operator reports"
inside the `corpus export` **durable-provenance** bundle (`spec.md:1217-1220`).
So `operator_report` is durable provenance by an accepted RFC *and* by the spec,
and RFC 0119's sole worked definition of "exhaust" evicts it from git — directly
contradicting RFC 0119's own promise that "**durable provenance stays canonical
in git**" (RFC `:52`, `:74`).

This is sharper than Falsifier 1's Objection 4 (which cited only the
corpus-export bundle). The contradiction is with an **accepted decision**:
RFC 0119 never cites, acknowledges, or supersedes RFC 0072's boundary table
while reusing its mechanism and inverting its result. Ratifying D178 as written
would stand up two accepted decisions that disagree on whether `operator_report`
is evictable, and an operator following RFC 0119's worked list would evict
content RFC 0072 forbids moving.

**Holder's strongest rebuttal.** It is an *open question* with a revisitable
default (RFC `:137`); D178 ratifies only the *axis* ("eviction scope =
exhaust"), not the list, and "exhaust" can later be defined to exclude
`operator_report`.

**Does it survive? Yes — decisive.** Ratifying an axis whose *only* worked
definition reclassifies a kind that an accepted RFC and the spec both pin as
durable is not safe: "exhaust" is left undefined while its one example overlaps
both RFC 0072's git-tracked carve-out and the corpus-export durable set. The
gate must not clear until D178 pins "exhaust" as a **strict subset of RFC 0072's
blob set** that provably excludes every RFC 0072 git-tracked kind (`decision`,
`escalation`, `operator_brief`, `operator_report`, `work_plan`) and every
corpus-export durable-provenance kind — and until RFC 0119 explicitly reconciles
with or supersedes RFC 0072's boundary table rather than silently contradicting
it.

### Objection 2 — the `*_ledger` glob over-sweeps decisional ledgers RFC 0072 never classified, including this gate's own `collaboration_ledger`

RFC 0072's boundary table is an **enumerated, per-kind, individually-reviewed**
set: it names `support_ledger`, `action_item_ledger`, `findings_ledger` as blob
(RFC 0072 `:60`) and stops there. RFC 0119 replaces that discipline with a
**glob**: "*_ledger" (RFC `:139`). A glob is structurally looser than an
enumerated table with carve-outs — it auto-captures any present or future
`*_ledger` kind with no decisional review.

The glob sweeps in `collaboration_ledger`, which RFC 0072 never placed in blob.
`collaboration_ledger` is a schemaed, front-matter–carrying kind
(`go/pkg/artifactcontracts/contracts.go:241-244`, accepting
`striatum.collaboration_ledger.v1` / `v1.1` with a required `entries` list) — it
is decisional, not exhaust. It is also the **required, gate-deciding output of
this very ratify workflow**: the adjudicator job in this run publishes
`docs/rfcs/0119/adjudicator/COLLABORATION_LEDGER_${cycle}.md` with
`kind: collaboration_ledger`. A `*_ledger` eviction glob would move the artifact
that records *whether a ratification gate cleared* out of the git tree — durable
provenance erosion at exactly the point it matters most.

**Holder's rebuttal.** Same open-question/axis-not-list defense; and a careful
implementer would write an explicit per-kind policy table (as RFC 0072 did), not
a literal glob.

**Does it survive? Yes.** The objection is precisely that the RFC's *only stated
form* of the eviction scope is a glob that abandons the enumerated, carve-out
discipline RFC 0072 established and that the spec relies on. "A careful
implementer would not glob" concedes the point: the ratified axis must
*require* an explicit allow-list, never a glob, and must exclude every
front-matter–carrying decisional kind (`collaboration_ledger`,
`operator_report`, `escalation`, `operator_brief`, `work_plan`, `decision`).
Until D178 pins that, the eviction scope authorizes evicting decisional
provenance by pattern.

## Export-class posture

### Objection 3 — the `lane_trajectory` class must defeat an enforced corpus guardrail and cannot meet the bundle's determinism contract

Falsifier 1 (Objection 5) flagged that `lane_trajectory` *relaxes* the
transcript-export prohibition by citing the `spec.md` text. The
durable-provenance posture lets me make this concrete and harder: the relaxation
is not a doc-line edit, it is **weakening an active, tested guardrail that
currently fails closed**, and the result erodes the corpus bundle's
*durable-provenance* property.

The corpus export path enforces a hard source-path denial in source.
`go/pkg/reads/redaction.go` denies any source path whose parts include
`transcripts`, `terminal_output`, `raw_model_output`, or `.striatum`
(`redaction.go:24-29`) and denies `transcript*`-named `.txt`/`.md`/`.log` files
(`redaction.go:84-87`); `validateCorpusSourcePath` is invoked on every emitted
path at `go/pkg/reads/exports.go:408`. `spec.md` describes this as "Corpus
source-path checks deny transcript/output/private path shapes case-insensitively"
(`spec.md:1231-1232`), and PTY/agent-loop logs are "not included in evidence
exports, corpus exports, or run archives" (`spec.md:2447-2450`).

RFC D2 sources `lane_trajectory` from "agent-loop transcripts (tmux capture /
`conversation_trajectories`)" (RFC `:80-81`) — exactly the shapes this guardrail
denies. So D2 does not "add an optional class"; it must **bypass or weaken a
guardrail the spec pins as protecting the boundary**, and the RFC says nothing
about that amendment.

Worse, the corpus bundle is by definition "read-only **durable provenance**"
with a **byte-identical re-export** guarantee (`spec.md:1223-1226`). Free-form
transcripts "may contain provider output, tool text, prompts, or secrets"
(`spec.md:2451-2452`). Heuristic secret/PII redaction is not a deterministic
function of its input, so RFC D2's bare assertion that `lane_trajectory` stays
"deterministic" (RFC `:86`) is unproven and most likely false for live
transcript bodies. Injecting a non-deterministically-redactable, transcript-
sourced class into the durable-provenance bundle erodes both the transcript
boundary and the bundle's determinism property.

**Holder's rebuttal.** `spec.md` permits "no durable transcript capture/export
*without an explicit product decision*" — RFC 0119 *is* that decision (F1's
Objection-5 rebuttal). And RFC 0072 already proved corpus export determinism is
preservable.

**Does it survive? Yes — narrowed to a mechanism gap, not a kill.** The RFC may
indeed be the authorizing decision, so the *mechanism* is acceptable in
principle. But three obligations remain unmet, and each is a live
durable-provenance hole: (a) RFC 0119 must explicitly supersede both the
enforced `validateCorpusSourcePath` denial *and* **D028** ("no transcript
capture by default," which accepted RFC 0072 `:357` lists as "**preserved**") —
RFC 0119 cites neither; (b) RFC 0072's determinism proof covered *artifact
bodies* (already-fixed bytes), not live free-form transcripts, so it does not
transfer to `lane_trajectory`; (c) ratification must require the
redaction/normalization-to-deterministic-bytes contract and the explicit
guardrail amendment, or drop the "deterministic" claim and keep the class out of
the bundle that `spec.md` calls "read-only durable provenance." Until then the
durable-provenance bundle's integrity is eroded by assertion.

## What I am NOT claiming (cut as non-load-bearing)

- **"`lane_trajectory` is a streaming/push surface."** The prompt invites this,
  but RFC D2 (`:86`) states it is "pull-only and deterministic … never
  streamed," consistent with `spec.md:1228-1230` ("Striatum does not stream
  runtime events to any external consumer"). I can rebut a streaming objection
  from the RFC text alone, so I cut it. (Same disposition as Falsifier 1.) The
  *export-class* erosion is determinism + the transcript guardrail, not push.

- **"Eviction breaks corpus-export determinism / defeats the metadata-only
  archive / moving bodies to PG erodes provenance."** Cut. Accepted RFC 0072
  (`:30-31`, `:236-242`, acceptance `:266-268`) already settled byte-stable
  corpus export for blob-backed bodies, and the metadata-only archive
  (`spec.md:1261-1263`) is the accepted post-0072 baseline, not an RFC 0119
  regression. RFC 0119's marginal erosion is the *kinds* it newly evicts
  (Objections 1–2), not the eviction transport.

- **"D1 index scope = everything copies durable provenance into the warm tier."**
  Indexing a read-only copy is augmentation; canonical authority stays git
  (Non-Goals RFC `:63-65`). Indexing ≠ relocating the canonical record, so it
  does not "move durable provenance out of git." I can rebut it on my own
  posture, so it is not load-bearing here. (I flag, out of scope, that "index
  scope = everything" copies *unredacted* git content into the warm tier without
  stating that the `corpus export` redaction contract applies to the index path
  — a confidentiality question for the adjudicator, not a durable-provenance
  erosion.)

## Verdict

The eviction policy **does** erode the `spec.md` durable-provenance boundary —
but not where a naïve attack would aim. The transport (PG/blobs) and the
corpus-export/archive determinism story were settled by accepted RFC 0072 and I
do not contest them. The surviving, load-bearing erosions are:

1. **(Decisive)** RFC 0119's only worked definition of "exhaust" evicts
   `operator_report`, which **accepted RFC 0072** (`:44-47`, `:59`) and
   `spec.md` (`:1217-1220`) both pin as git-tracked durable provenance —
   contradicting an accepted decision RFC 0119 cites as its own mechanism, and
   its own "durable provenance stays canonical in git" (RFC `:52`).
2. The `*_ledger` **glob** (RFC `:139`) abandons RFC 0072's enumerated,
   carve-out discipline and over-sweeps decisional ledgers RFC 0072 never
   classified — including this gate's own `collaboration_ledger`
   (`contracts.go:241-244`), a required gate-deciding artifact.
3. The `lane_trajectory` export class must weaken the enforced corpus
   transcript-denial guardrail (`redaction.go:24-29`, `:84-87`;
   `exports.go:408`) and cannot meet the bundle's byte-identical-re-export
   durable-provenance guarantee (`spec.md:1223-1226`) for free-form transcripts;
   its "deterministic" claim (RFC `:86`) is asserted, not shown, and it does not
   supersede the denial or **D028**.

`lane_trajectory` is **not** a streaming/push surface (cut).

Minimal conditions for the gate to clear honestly on the durable-provenance
posture:

- (a) D178 must define "exhaust" as an **explicit allow-list** that is a strict
  subset of RFC 0072's blob set, **never a glob**, and must provably exclude
  every RFC 0072 git-tracked kind (`decision`, `escalation`, `operator_brief`,
  `operator_report`, `work_plan`) and every front-matter–carrying decisional
  kind (add `collaboration_ledger`).
- (b) RFC 0119 must explicitly **reconcile with or supersede** accepted RFC 0072's
  boundary table and **D028**, not silently contradict them while citing
  "the RFC 0072 pattern."
- (c) `lane_trajectory` must specify the amendment to `validateCorpusSourcePath`
  and a **deterministic** redaction/normalization contract, or the determinism
  claim must be dropped and the class kept out of the `spec.md` "read-only
  durable provenance" bundle.
