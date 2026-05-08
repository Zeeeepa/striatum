# Coordinator Role (Dogfood 004)

The coordinator owns workflow gating and human-checkpoint surfacing
for dogfood-004. The coordinator does not write product code; it
sequences the run through the structured CLI commands and surfaces
the human acceptance gate when the design review accepts.

This role is declared so the workflow-validate machinery has a
target; in practice the operator drives the run interactively and
records the decision artifact when the design review verdict allows
implementation to proceed.
