# Task: build the slice and publish the claim ledger

Build the assigned slice in your write scope. Then publish
`CLAIM_LEDGER.md` recording **every capability a downstream consumer would
rely on**, as a table:

| claim | status | witness |
|---|---|---|
| e.g. "analysis cannot import the source layer (C1)" | VERIFIED | `pytest tests/test_architecture_boundary.py` exits 0 |
| e.g. "mm + inch is a type error (C6)" | VERIFIED | `mypy --strict path/to/trap.py` reports the operand error |
| e.g. "bracketed verdict-agreement on cliff analyses" | DESIGNED | none yet — deferred |

Rules:
- `status` ∈ `VERIFIED` (witness exists and you ran it green) | `ASSERTED`
  (believed, no runnable witness) | `DESIGNED` (not built).
- Anything above `DESIGNED` MUST name a witness: a test id, a `grep`, a CLI
  command + expected output, or a `mypy` invocation.
- Do NOT use completion language for a claim above the status its witness
  earns. If you deferred something, give it a `DESIGNED` row — visible, not
  buried in prose.

Follow your role definition (`roles/builder.md`).
