# Striatum Run Retrospective — RFC 0142-P4 & RFC 0143 design chains

> author: operator-opus-4-8-001 · date: 2026-06-24 · kind: cross-run process retrospective (operator-authored, not a workflow-published finding)

**Target:** the two falsification_gate design-run chains that fed RFC 0142-P4 and RFC 0143
**Repo:** `/home/halbritt/git/striatum` (branch `main`)
**Question asked:** why has landing these two RFCs taken "all day"?
**Model doing audit:** claude-opus-4-8
**Date:** 2026-06-24
**Note on scope:** `RUN_RETROSPECTIVE.md` audits *one* run; the operator asked a *cross-run* timing question, so this applies the same evidence-tier discipline across all 17 runs of the two chains. It is a process/timing retrospective, not a product review of the designs.

---

## 0. Provenance inspected

- **Evidence tiers used:** `live` (authorized `striatum list runs` daemon read — run states, timestamps, stop reasons), `history` (git log/show of the bank+scaffold commits), `committed` (the `docs/operator/artifacts/rfc-0142-p4-design-v*/` and `rfc-0143-design-v*/` dialogue trees).
- **Artifact inventory:** every cycle has the falsification_gate quartet — `holder/HOLDER.md`, `falsifier_1/FALSIFIER.md`, `falsifier_2/FALSIFIER.md`, `adjudicator/COLLABORATION_LEDGER_cycle_1.md`; the final accepted 0142 cycle (v9) additionally has `commit/proposal/PROPOSAL.md` + `commit/final/FINAL_SUMMARY.md`. Provenance is complete.
- **Not deep-audited:** the substance of individual FALSIFIER/HOLDER artifacts (out of scope for a timing audit; sampled only for the defect-family labels, which the daemon stop-reasons already carry verbatim).

## 1. The runs that went into these two RFCs

**17 runs total**, two parallel falsification_gate chains, plus one build run now scaffolded.

### RFC 0142-P4 (9 design cycles → ratified → build scaffolded)

| ver | run_id (short) | started | "completed" | stop reason (defect surfaced) |
|----|----|----|----|----|
| v1 | 10ce3267 | 06-22 19:41 | 06-23 01:57 | gate found material findings → rigorous revision |
| v2 | e9c4a2fe | 06-23 02:14 | 06-23 04:27 | resolved C1+C2; C3+N1 remain |
| v3 | 036047ad | 06-23 04:38 | 06-23 06:33 | needs_revision N1 + N2 |
| v4 | 21d69b05 | 06-23 06:33 | 06-23 08:00 | needs_revision M1 + M2 |
| v5 | 042e87a9 | 06-23 08:00 | 06-23 09:17 | needs_revision M3 + M4 |
| **v6** | 692c7b94 | 06-23 09:17 | 06-23 **17:53** | needs_revision M5 — **8.6h wall, mostly idle** |
| v7 | 65d0ab53 | 06-23 18:09 | 06-23 19:12 | needs_revision M6 |
| v8 | 9809fc07 | 06-23 23:32 | 06-24 01:07 | operator_canceled |
| v9 | 365daa96 | 06-24 01:29 | 06-24 03:17 | **accept_with_findings → ratified D262** (e219c8a4) |
| build | 25182947 | 06-24 03:34 | — | **stuck `needs_branch_confirmation`** (waiting on operator) |

### RFC 0143 (7 design cycles → split-accept, no build run)

| ver | run_id (short) | started | "completed" | stop reason (defect surfaced) |
|----|----|----|----|----|
| v1 | 9efc0b74 | 06-22 19:40 | 06-23 01:57 | gate found material findings → rigorous revision |
| v2 | c6130c9b | 06-23 02:14 | 06-23 05:30 | needs_revision BC1–BC5; falsifier_2 wedged on worktree_publish_acl |
| v3 | 173726d4 | 06-23 05:30 | 06-23 07:15 | needs_revision BC1 (same-uid channel replay) + BC5 |
| v4 | 8273cab2 | 06-23 07:17 | 06-23 08:35 | needs_revision BC1 (channel-installation ground) |
| v5 | d21c3e9a | 06-23 08:36 | 06-23 09:52 | needs_revision BC1-W1-TOKEN (clock mismatch) |
| **v6** | 8101eb03 | 06-23 09:52 | 06-23 **17:53** | needs_revision BC1-W1-CAPTURE — **8.0h wall, mostly idle** |
| v7 | ad391a22 | 06-23 18:09 | 06-23 19:16 | **convergence STOP** → BC1-W1-ORACLE unsolvable → split-accept D261 |

**Calendar span of the whole effort:** 06-22 19:40 → 06-24 03:34 ≈ **32 hours**. RFC 0143 stopped at a split-accept on 06-23 evening; RFC 0142-P4 only reached "build-ready" at 06-24 03:21 and its build run is still blocked on operator confirmation.

## 2. Process verdict

**Verdict: PROCESS_SOUND_WITH_FINDINGS.** Confidence: **high** (timing/state evidence is `live`+`history`; nothing material is unknowable for the timing question).

The gate did its job — it caught a *new, build-breaking* defect on every cycle and never rubber-stamped; one chain even reached a correct *negative* result (an architectural wall). The slowness is **not** a process failure of falsification. It is the predictable cost of three compounding factors, one of which (operator-paced wall-clock) is the dominant and the most addressable.

## 3. Why it took all day — three compounding causes

### Cause A — The gate genuinely converged, one new defect per cycle (rigor, not waste)

This was the operator's explicit choice: v1's stop reason on both chains is *"gate found material findings; re-driving a proper revision (operator chose rigorous path)."* The findings are **distinct each cycle**, not the same finding re-surfacing:

- **0142-P4 defect families:** C1/C2 → C3/N1 → N1/N2 → M1/M2 → M3/M4 → M5 → M6. Seven different families (already-applied byte-verify gaps, revoke-bundle self-heal reachability, complete-cursor legacy mutate, owner-watermark dimension split, …), each a real build-breaker caught *before* code.
- **0143 defect families:** BC1–BC5 → BC1/BC5 → BC1 → BC1-W1-TOKEN → BC1-W1-CAPTURE → BC1-W1-ORACLE.

Per-cycle *compute* is modest — roughly **60–135 min** for the cycles that ran while the operator was present (v2–v5, v7). A 7–9 cycle convergence at ~90 min each is ~10–13h of legitimate gate compute. That alone is most of a working day.

### Cause B — RFC 0143 spent five cycles proving a wall, not fixing a bug

0143's last five cycles all re-attack the **same core (BC1: same-uid channel replay)** from progressively narrower angles: channel-installation ground → TOKEN (peer-cred clock mismatch) → CAPTURE (pane-liveness boundary) → ORACLE (post-launch query can't bind to the daemon-launched wrapper). v7's stop reason records the conclusion: the same-uid tmux oracle **"needs a different primitive."** That is the gate doing expensive falsification to reach a **negative** result — which is correct and valuable, but it means 0143 was never going to fully land in one sitting: it forced the **split-accept (D261)** — Slice A ships now, Slice B is blocked on a brand-new prerequisite **RFC 0168 (per-lane OS uid, #585)**. Half of "RFC 0143" is, by design, deferred to another RFC's whole lifecycle.

### Cause C (dominant for wall-clock) — operator-paced human-in-the-loop ratchet with long absence gaps

The falsification_gate revision loop is **not auto-driven**. Every cycle, the operator must (1) bank the dialogue artifacts to `main` and (2) scaffold a fresh `-vN` run (stop reasons say *"superseded by proper revision rfc-…-vN"*; this matches the standing lesson "proper-revision = fresh `-vN` run, hands off the auto-driver on revision cycles"). That is **~16 manual bank+scaffold turns** across the two chains, each gated on the operator being at the keyboard.

The git history exposes how much of the 32h is *waiting on the operator*, not computing:

- **The two "8-hour" v6 runs are an artifact of operator absence.** There are **zero commits between `06-23 09:50:33` and `06-23 17:27:47`** — a ~7.5h gap. Both v6 runs (0142 and 0143) report `completed_at` of **exactly `17:53:27`**, seconds after the operator returned and banked v6 of 0142 at `17:49:36` (commit `06904942`). The runs finished their gate cycle in the morning and then *sat* until the operator came back; their 516m/481m "durations" are mostly idle wall-clock, not work.
- **Evening gap:** 0142-v7 and 0143-v7 finished ~19:12–19:16; the next run (0142-v8) didn't start until `23:32` — a ~4h gap.
- **Overnight pacing:** v8 (canceled 01:07) → v9 (01:29 → 03:17) → ratify (03:21) → build scaffold (03:34, now stuck at `needs_branch_confirmation`). Even the final step is parked waiting on the operator.

In short: the **compute** wanted ~10–13h; the **calendar** consumed ~32h because roughly half of it was the loop idling between cycles for a human to bank-and-rescaffold.

## 4. Findings (process severity order)

1. **[High, addressable] The revision loop is operator-gated at every cycle, with no overnight progress.** ~16 manual bank/scaffold turns and a 7.5h + 4h + overnight set of dead gaps dominate wall-clock. *Fix direction:* allow the falsification_gate to **auto-bank + auto-scaffold the next revision** when a cycle returns `needs_revision` with carry-forwards intact and no operator decision is required (reserve manual routing only for genuine adjudication forks — e.g. the split-accept). This is the single biggest lever; it would have collapsed the two v6 idle blocks and the overnight gaps. (Tension to respect: the standing lesson "hands off the auto-driver on revision cycles" exists because co-driving caused attempt-collision wedges — so the fix is a *gate-native* sequential auto-revise, not the generic auto-driver.)
2. **[Medium] No convergence budget / stop rule was set up front.** 0142 ran to 9 cycles and 0143 to 7 before a STOP; 0143's wall (BC1) was visible by ~v3–v4 (BC1 recurring) but wasn't called as "architectural, needs a new primitive" until v7. *Fix direction:* a **"same-core recurs N times → escalate to architecture decision"** rule would have surfaced the RFC 0168 split 3–4 cycles earlier and saved ~4 cycles on 0143.
3. **[Medium] Two chains run in lockstep against one operator.** 0142 and 0143 were scaffolded and banked in interleaved pairs, so each chain waited on the other's operator turn. *Fix direction:* if auto-revise lands, the two chains stop competing for operator attention; until then, consider sequencing rather than interleaving so one can finish.
4. **[Low] `completed_at` on superseded runs measures operator latency, not work.** The 516m/481m durations are misleading for capacity planning. *Fix direction:* record an `agent_work_completed_at` distinct from the supersession timestamp so retrospectives can separate compute from wait.

## 5. What is *not* wrong

- **Review substance is strong.** Every cycle's gate produced distinct falsifier attacks that landed concrete, named, build-breaking findings (the M-series byte-verify/owner-watermark defects on 0142; the W1 clock/capture/oracle drill-down on 0143). No rubber-stamp `accept` appears until 0142-v9, which is `accept_with_findings` with a final proposal+summary — i.e. earned.
- **The negative result on 0143 is a success of the process, not a failure.** Proving same-uid no-replay unsolvable and forcing a clean split (Slice A now / Slice B → RFC 0168) is exactly what a falsification gate is for.

## 6. Bottom line

It took "all day" because you ran **16 falsification cycles across two RFCs on the rigorous path**, the gate found a real defect every time (good), one RFC hit an architectural wall that split it (necessary), and — the dominant wall-clock cost — **the revision loop is human-paced**: roughly half the 32-hour span was the loop sitting idle between cycles waiting for you to bank artifacts and scaffold the next `-vN` run, including a 7.5h daytime gap and an overnight one. The compute was ~10–13h; the calendar was ~32h. The highest-leverage fix is a gate-native auto-bank/auto-rescaffold for `needs_revision` cycles that carry forward cleanly, plus a "same-core-recurs → escalate to architecture" stop rule.
