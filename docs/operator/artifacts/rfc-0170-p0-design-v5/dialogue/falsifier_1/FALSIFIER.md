# FALSIFIER - RFC 0170 P0 design v5 Tier-1 known-set review

author: falsifier-reviewer-001

## Disposition

I do not have a material P0-blocking falsification under the re-scoped Tier-1 SOUNDNESS + known-set bar. The strongest candidate attacks I can ground in the live tree either preserve the candidate by clause 2/4 or land in the explicitly safe false-negative direction. This is not a gate verdict; it is the falsifier_1 handoff for the G1 lens.

## Attack 1 - the old `rfc:0097` false-positive carrier no longer survives v5

**Claim challenged.** A2 / G1 zero false positives on the known preserved RFC set, especially `rfc:0097`.

**Counterexample attempt.** `docs/rfcs/0097-full-workflow-run-orchestration.md` is structurally `Status: superseded by RFC 0116 / 0122 / 0124`, so a naive Tier-1 supersession predicate would nominate it. That was the earlier dangerous-direction failure.

**Evidence.** The v5 holder carries the clause-4 active-baseline citation rule as the blocker: live, non-successor, non-disposable inbound references withhold the candidate (`HOLDER.md:348-390`). The live tree still has active `RFC 0097` uses: `docs/rfcs/0101-robust-autonomous-workflow-execution.md:130-131` says RFC 0097 is the consumer, `:281-284` describes it as the orchestration consumer, and `docs/rfcs/0103-self-hosting-production-hardening.md:43` plus `:213-219` treat RFC 0097 as live self-hosting/orchestration substrate. These are live RFCs (`Status: umbrella-of-record` for RFC 0101 and `Status: accepted` for RFC 0103) and the cited lines are not the named-successor backrefs or closure/index lines excluded by 4e/4f.

**Strongest rebuttal.** The holder's rebuttal is source-true: `rfc:0097` is withheld by clause 4, not nominated. The previously live-cited RFC 0097 contradiction is therefore resolved under the known-set bar. The same pattern holds for the other named preserved RFCs I spot-checked: `rfc:0027` is live-cited by RFC 0031/RFC 0118, `rfc:0039` by RFC 0043/RFC 0040-style live daemon text, and `rfc:0041` by RFC 0058's augmentation-not-dependency line. I found no preserved RFC row that loses all live citations to the fixed closure-token filter.

**Unanswered gap.** None at P0. The build still needs the table-driven known-set rows so a future implementation cannot regress the physical-line 4f filter, but that is an implementation proof obligation, not a v5 design blocker.

## Attack 2 - the #618 `D081` deferral is a false negative, not a hidden false positive

**Claim challenged.** The holder says the `D081` whole-tree gap is safe-direction under-nomination, deferred to #618, and not a P0 blocker (`HOLDER.md:532-548`, `:595-603`, `:967-968`).

**Counterexample attempt.** If the frozen audit record that cites `D081` were actually making a live artifact get nominated, or if the audit cause were source-false, then the #618 carve-out would hide a P0 false positive or unsoundness break.

**Evidence.** `docs/decisions/decision-log.md:220` has `D081 | superseded | ... Superseded by D087/D094/D104 ...`, so `D081` is genuinely dead under the holder's decision-successor rule. The audit file exists and is tree-local: `docs/records/audits/STRIATUM_DECISION_RECORD_AUDIT_OPUS_4_8_2026-06-16.md:3` has `status: frozen`; it is outside `docs/records/_frozen/**`; it contains `D081` hits at `:323` and `:367-370`. Under v5 clause 4c, that source is live because `frozen` is not one of the dead-status prefixes and the static pathspec does not include `docs/records/audits/**` (`HOLDER.md:359-368`, `:452-466`). Those hits therefore withhold `D081`.

**Strongest rebuttal.** The deferral is honest under the packet's P0 bar: the predicate under-nominates a dead decision (`D081`), so it misses cleanup but does not nominate a known preserved member. In an observe-only P0 ledger, that is the safe direction named by the SEED (`SEED.md:12-24`). Closing the broader status-frozen corpus is properly #618/P1 unless someone shows this mechanism nominates a live artifact.

**Unanswered gap.** None for G1 P0. The holder should keep the #618 fixture explicit downstream because this is exactly the row future whole-tree completion will change.

## Positive checks against the scoped bar

- **Mechanical / no external state.** The predicate is specified as structural fields plus grep, fixed regexes, a fixed closure lexicon, and a literal tree-local pathspec; the protected pathspec explicitly has no GitHub/API/open-issue term (`HOLDER.md:407-450`). I found no remaining clock, network, open-issue, or mutable-outside-tree dependency in G1.
- **Known-dead `D267` remains nominated.** `docs/decisions/decision-log.md:38` is structurally `superseded` and names `D270`; `docs/decisions/decision-log.md:35` is live `implemented`. A grep over the inbound set plus `docs/records/` found only `D267` at its own row and D270's successor backref, so clause 4 has zero counted inbound citations. This matches `HOLDER.md:517-527`.
- **Known preserved branches and protected paths are not candidates.** P0 emits no `kind = branch` row (`HOLDER.md:468-478`), so the `backup/rfc-*` banked branches are not nominated. `docs/records/_frozen/**` and workflow/scaffold roots are in the static protected pathspec (`HOLDER.md:426-433`) and withhold by clause 3.
- **RFC-0170 body / SEED / workflow / decision-log prose.** Clause 1 reads structural status fields only and ignores body prose (`HOLDER.md:231-262`), so the RFC-0170 proposal text and workflow prose mentioning supersession-shaped words are not candidates merely because of prose.