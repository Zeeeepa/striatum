# Implementation Panel Flow

This RFC 0074 Phase A example is a local, provider-neutral workflow for choosing
between implementation approaches. It fans out three independent proposals,
scores each against fixed criteria, compiles the tradeoffs, arbitrates a
preferred path, runs one dissent review, and records the final decision.

Validate it from the repository root:

```bash
PYTHONPATH=src python3 -m striatum.cli workflow validate examples/implementation-panel-flow/workflow.json
```

The fixture intentionally uses local process lanes so it can be inspected
without provider credentials while still keeping proposal, review, arbitration,
and decision roles on distinct model-family identities. Operators can adapt the
lane definitions to real agent commands before running it against a target
repository.
