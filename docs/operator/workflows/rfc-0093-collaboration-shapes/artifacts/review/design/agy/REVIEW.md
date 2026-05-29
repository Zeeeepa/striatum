---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
---

# Design Review (agy) — Devil's Advocate
author: operator

**Verdict:** `accept_with_findings`
**Severity:** `low`

## Interrogation Rounds
* **Rounds Used:** 0
* **Reason for Stopping:** The `agy` lane was not granted the `interrogate` capability in its session capabilities (`capabilities_json: ["review"]`), preventing technical invocation of the `interrogation.open` method (returned `capability_denied`). As a result, we exited the interrogation loop immediately with 0 rounds, and proceeded to complete a rigorous, document-only, fresh-context review of [DESIGN_SYNTHESIS.md](file:///home/halbritt/git/striatum/docs/operator/workflows/rfc-0093-collaboration-shapes/artifacts/DESIGN_SYNTHESIS.md) under the assigned `devils_advocate` posture.

---

## Summary
The proposed design synthesis in [DESIGN_SYNTHESIS.md](file:///home/halbritt/git/striatum/docs/operator/workflows/rfc-0093-collaboration-shapes/artifacts/DESIGN_SYNTHESIS.md) is technically excellent, exceptionally cohesive, and represents a high-discipline approach to implementing RFC 0093 V1. The four-slice landing order minimizes rollout risks, and the decision to map the substance-gate onto the existing `phase_synthesis` job class avoids complex scheduling modifications.

However, as a Devil's Advocate, we have identified several load-bearing gaps, assumptions, and contested trade-offs that warrant explicit recognition and tracking:
1. **The Hollow-Triple Vulnerability in the Split Rubric:** The structural Go validator only checks for the existence of *at least one* `claim/challenge/rebuttal` triple. This is a very low bar that can still be satisfied by formalistic/hollow entries, relying heavily on the semantic model filter.
2. **Structural Reference Blindness:** Go validator shape-checks `dialogue:<seq>` references but does not resolve them against the database. Stale, invalid, or cross-topic references will pass Go validation.
3. **Infinite Feedback Theater on Checkpoints:** Gating on human checkpoints when revision iterations are exhausted could lead to operator fatigue or deadlocks in automated environments, replacing automated loops with manual overhead.
4. **Weak Same-Model Refusal Guardrails:** A narrow lint-only warning for same-model self-adjudication makes it too easy to bypass reviewer-independence in practice.

---

## Detailed Findings & Adversarial Analysis

### 1. Split Rubric Gaps & Hollow-Triple Risk
The core of the synthesizer's anti-theater design is the **split rubric**: a deterministic Go validator ensuring structure, and a semantic prompt-level model ensuring quality. 

While elegant, this introduces a **necessary but extremely weak** structural gate:
* **The Single-Triple Bypass:** The Go validator requires only one `claim`, one `challenge`, and one `rebuttal`. A rushed or low-capability lane can easily generate a single hollow triple (e.g., "This design is good" -> "Is it really good?" -> "Yes, it is") that complies with the schema. 
* **Model Demotion vs. Promotion:** The design ensures the model can *demote* a structurally valid draft, but it cannot *promote* an invalid one. This is correct, but it leaves the system highly dependent on the semantic quality of the adjudicator's prompt. If the adjudicator is slightly soft or has a high temperature, hollow triples will slip through.
* *Recommendation:* In V2, consider a structural rule requiring that `entries[]` has a count proportional to the dialogue length, or that text fields meet minimum character/token count thresholds to deter pure ritual compliance.

### 2. Lack of DB Reference Verification
To keep the `artifactcontracts` package clean and free of PostgreSQL/database dependencies, the Go validator only performs a string regex shape-check on `dialogue:<seq>` refs.
* **The Disconnect:** Because the validator does not assert that the reference exists in the dialogue database or corresponds to the active run, a lane could publish a ledger referencing non-existent turns (e.g., `dialogue:9999`) or turns belonging to a completely different thread.
* *Recommendation:* Keep the contract validator pure, but ensure that the adjudicator's publishing job itself (which *does* have database access) resolves and validates the references before it attempts to write/publish the ledger.

### 3. Cycle Router: Checkpoint vs. Hard Failure
When the cycle budget is exhausted (`max_iterations`), the synthesizer selected the Codex path: open the existing human checkpoint with a `cycle budget exhausted` reason.
* **The Risk of Infinite Manual Loops:** While this preserves operator agency, it enables "infinite recovery theater." An operator can repeatedly resolve checkpoints to keep an unproductive or low-cognitive loop spinning indefinitely, driving up token costs.
* **CI pipeline stalls:** In unattended CI/CD runs, opening a human checkpoint behaves like a silent stall or hang, whereas a hard terminal failure would immediately fail the build and signal the engineering team.
* *Recommendation:* Accept this path for V1 as it preserves agency, but monitor the friction log closely for manual override fatigue.

### 4. Weak Same-Model Adjudicator Refusal
Reviewer independence is handled by generating a lint error if the adjudicator's `lane_id` matches the holder/proposer's `lane_id`.
* **Audited Override theater:** Lint errors can easily be ignored, and generating an "audited override" artifact becomes a standard box-ticking exercise. Same-model adjudication is structurally compromised because the same underlying model weights are evaluating their own synthesis.
* *Recommendation:* Enforce same-model refusal as a **hard validator failure** rather than a lint warning, unless a strict command-line argument or environment variable explicitly authorizes the override at the scheduler level.
