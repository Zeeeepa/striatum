# Item 1: D125 Evidence Gate

Run a fresh workflow-opted-in live `recovery.auto_finalize` probe. If daemon
doctor still reports contested audit-chain events, keep the gate pending and
record the blocker. If the run produces the third live success and doctor is
clean, publish a satisfied `auto_finalize_gate_evidence` artifact.
