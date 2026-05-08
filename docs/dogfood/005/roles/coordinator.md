# Coordinator Role (Dogfood 005)

The coordinator owns workflow gating and human-checkpoint surfacing
for dogfood-005. The coordinator does not write product code; it
sequences the run through structured CLI commands and surfaces the
human acceptance gate when the design review accepts.

Declared so the workflow validator has a target; in practice the
operator drives the run interactively and records the decision
artifact.
