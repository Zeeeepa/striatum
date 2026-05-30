# Adjudicated Constraint Extraction Design (agy Lane)

author: designer-gemini-3.5-flash-high-001

## 1. Problem Framing & Epistemic Rationale

### The Limitations of RFC 0093 "Honest Refusal"
RFC 0093 successfully introduced the `adjudicator` role and the `phase_synthesis`-class substance gate. This ensured that model-to-model dialogue did not degrade into empty compliance theater (e.g., asking hollow questions and receiving fluent non-answers) by gating down-stream commits on an active substance check over the `dialogue` trajectory. If the dialog failed its epistemic requirements, the adjudicator issued a `needs_revision` verdict, honest refusal blocked the commit, and a revision cycle was triggered.

However, a bare `needs_revision` verdict is **too weak** and introduces two critical failures:
1. **Frictional Dissipation (Objection Loss):** The adjudicator's load-bearing objections live only as prose in a review artifact or message logs. During the next cycle, the convener simply re-reads the prior synthesis along with the criticism, attempting to average the critiques back into a blander, compromise-heavy synthesis. Discrete, high-severity objections are not carried forward as trackable, binding obligations; they silently evaporate.
2. **Unverifiable Closure:** There is no structural verification that the specific issues identified during adjudication are ever resolved. The final review is forced to either re-litigate the entire design from scratch or trust the convener's rhetoric, with no automated assurance that every high-severity challenge actually resulted in testable text, invariant code, or appropriate gates.

### The Power of "Productive Refusal"
RFC 0098 transforms honest refusal into **productive refusal**. Under this paradigm, a refusal is not merely a stop signal—it is a **constraint-generating event**. An adjudicator cannot simply reject a proposal; they must compile their load-bearing objections into discrete, structured, and binding *constraints* (along with a posture-disposition matrix). 

These constraints are carried forward as first-class input to the next revision cycle. The loop is closed at the final review phase, which performs a structured, typecheck-like verification that every binding constraint is explicitly and successfully discharged.

---

## 2. Proposed Approach (Smallest Blast-Radius First)

We structure the implementation into three incremental slices:

### Slice 1: `collaboration_ledger.v1.1` & Productive-Refusal Gate (The Target)
The core substrate is extended with zero new daemon methods or RPC endpoints:
- **`collaboration_ledger.v1.1` Schema:** Additive fields `constraints[]` and `branches{}` are introduced. Any old `v1` ledger remains fully valid (no missing required fields for older runs).
- **The Productive-Refusal Gate:** The validation logic inside `publish-artifact` / `review.submit` is modified. If a ledger carries `verdict: needs_revision`, validation **fails closed** (exit code 6) unless `constraints[]` contains at least one entry.
- **Contract Mismatch Remediations:** The ledger contract is updated to accept all clearing verbs advertised in the spec (e.g., `accept`, `accept_with_findings`, `needs_revision`, `reject`, `blocked_pending_answer`, `defer_with_successor`) and natural markdown front matter, resolving #88 and #79.

### Slice 2: Shape Fixture & Generator (Stretch)
- Register `adjudicated_constraint_extraction` in the collaboration shape pack.
- `striatum workflow generate --shape adjudicated_constraint_extraction` emits the full 8-phase graph:
  $$\text{survey} \rightarrow \text{convener\_synthesis} \rightarrow \text{cross\_exam} \rightarrow \text{adjudication} \rightarrow \text{revision\_synthesis} \rightarrow \text{constraint\_discharge\_review} \rightarrow \text{spec\_publication} \rightarrow \text{final\_review}$$
- Requires cycle-aware logical names (resolved by **#84**) so that republishing revised artifacts under `*_synthesis_${cycle}` does not collide with content-hash guards.

### Slice 3: Discharge-Verifying Final Review (Stretch)
- Implement a new `constraint_discharge` finding type.
- The `final_review` job executes a structured verification of the published spec against the binding constraint table. It fails closed if any `binding: true` constraint is `missing` or unaccepted `partial`, and passes only when all are `discharged` or listed as `accepted_risk` with owner and stage.

---

## 3. Concrete Implementation Blueprint

### Constraint Schema Location
The front-matter schema is defined in Go in the `go/pkg/artifactcontracts/contracts.go` file. We will upgrade the `collaboration_ledger` schema definition within the `Schemas` map:

```go
"collaboration_ledger": {
    Fields: map[string]Field{
        "schema_version": {true, equalsValue("striatum.collaboration_ledger.v1.1")},
        "artifact_kind":  {true, equalsValue("collaboration_ledger")},
        "shape":          {true, oneOfValue("falsification_gate", "cross_examination", "fog_of_war_review", "synaptic_prune", "adjudicated_constraint_extraction")},
        "topic":          {true, isNonEmptyStringValue},
        "participants":   {true, isNonEmptyStringListValue},
        "entries":        {true, isCollaborationLedgerEntriesValue},
        "verdict":        {true, oneOfValue("accept", "accept_with_findings", "needs_revision", "reject", "blocked_pending_answer", "defer_with_successor")},
        "rationale":      {true, isNonEmptyStringValue},
        // --- Added in v1.1 ---
        "constraints":    {false, isConstraintListValue}, // Optional for backwards-compatibility, required when verdict=needs_revision
        "branches":       {false, isPostureDispositionMatrixValue},
    },
},
```

### The Productive-Refusal Gate Hook
The validation gate is hooked inside the artifact validation loop in `go/pkg/artifactcontracts/contracts.go` inside the `validateCollaborationLedger` helper:

```go
func validateCollaborationLedger(parsed map[string]any) error {
    // ... existing entry/ref validation ...

    verdict := fmt.Sprint(parsed["verdict"])
    
    // Productive-Refusal Gate
    if verdict == "needs_revision" {
        constraints, ok := parsed["constraints"].([]any)
        if !ok || len(constraints) == 0 {
            return fmt.Errorf("collaboration_ledger with verdict 'needs_revision' must carry a non-empty constraints list")
        }
        
        // Validate constraints structure
        for idx, item := range constraints {
            c := asMap(item)
            if err := validateConstraintEntry(c, idx); err != nil {
                return err
            }
        }
    }
    
    // Accept natural clearing verbs
    // ...
    return nil
}
```

This enforces the productive-refusal gate during both `striatum publish-artifact` and `striatum review.submit` because both command paths call `ValidateFrontMatter` before accepting the file, completely avoiding the need for a new daemon RPC method.

### Additive Preservation
To ensure that every existing RFC 0093 V1 ledger continues to validate successfully:
1. `schema_version` validation is relaxed to accept both `striatum.collaboration_ledger.v1` and `striatum.collaboration_ledger.v1.1`.
2. `constraints` and `branches` are declared as optional fields (`Required: false` in the validator field definition). They are only validated structurally if present, and the empty check is only triggered when the verdict is exactly `"needs_revision"`.

---

## 4. Alternatives Considered

| Alternative | Description | Pros | Cons | Verdict |
|---|---|---|---|---|
| **Separate `v2` Schema** | Create a separate `collaboration_ledger.v2` kind and schema definition. | Clean separation of schemas; no polluting `v1` parser. | Violates additive D053-related conventions; requires modifying all existing review lane parsers. | **Rejected** (prefer `v1.1` additive extension). |
| **First-class Daemon Constraint Objects** | Introduce first-class `constraint.*` daemon entities, schema tables, and RPC methods. | Highly structured query-ability; run-level persistence independent of artifact files. | Extremely high blast radius; requires DB migrations, client-auth matrix modifications, and supervisor updates. | **Deferred** (deferred to Slice 4 post-V1). |
| **Simple prose-parsing in Synthesis** | Parse raw Markdown tables in the synthesis body using custom regex/LLM blocks. | No schema changes needed. | Extremely fragile; susceptible to prompt-injection or formatting errors; lacks structural validation. | **Rejected** (prefer typed front-matter validation). |

---

## 5. Risks, Unknowns, and What Could Go Wrong

### 1. Schema Additive Regressions
* **Risk:** A strict change to `contracts.go` might fail to compile or might reject existing valid V1 ledgers used in past runs.
* **Mitigation:** Retain full test coverage for V1 ledger fixtures inside `go/pkg/artifactcontracts/contracts_test.go`. Explicitly verify that passing a V1 payload lacking `constraints` and `branches` (with an `"accept"` verdict) passes validation without warning.

### 2. The `adjudication → revision_synthesis` Cross-Phase Edge Mismatch (#66)
* **Risk:** The Striatum scheduler's `run.prepare` validator rejects edges that cross phase boundaries or target a later phase's synthesis job.
* **Mitigation:** The `adjudicated_constraint_extraction` shape generator must declare the revision synthetic relationship within the allowed cycle routing framework. If the scheduler is overly restrictive, we must relax `run.prepare`'s edge-legality guard for jobs marked as cycle re-entry targets.

### 3. The #84 Republish Collision Deadlock
* **Risk:** If an agent attempts to publish a revised synthesis in a later cycle under the same logical name (e.g. `forum_synthesis`), the daemon's content-hash write guard will reject it as a duplicate or write-bypass, deadlocking the run.
* **Mitigation:** Slice 2 **must not** be enabled unless cycle-aware logical names are implemented (e.g. utilizing the `resolveExpectedArtifactCycles` mapper to substitute `_cycle_${cycle}` segments dynamically). If #84 is unresolved, Slice 2 must be deferred, and design validation must fallback to serial, distinct logical names manually configured.

---

## 6. Rollout Plan

1. **Step 1 (Immediate - Slice 1):**
   - Update `allowedKinds` and `Schemas["collaboration_ledger"]` in `go/pkg/artifactcontracts/contracts.go`.
   - Update `validateCollaborationLedger` to implement the productive-refusal gate.
   - Run the full suite using `STRIATUM_PG_TEST_URL=postgres:///postgres go test ./pkg/artifactcontracts/... ./pkg/mutations/...` to confirm Slice 1 is 100% compliant.
2. **Step 2 (Medium Term - Slice 2):**
   - Confirm the status of issue #84 (cycle-aware logical names) in the daemon.
   - Register the `adjudicated_constraint_extraction` shape in the generator catalog.
   - Scaffold the starter template in `examples/adjudicated-constraint-extraction-flow/`.
3. **Step 3 (Long Term - Slice 3):**
   - Implement the `constraint_discharge` finding shape.
   - Write the `final_review` verifier validation rules in the final review lane.
