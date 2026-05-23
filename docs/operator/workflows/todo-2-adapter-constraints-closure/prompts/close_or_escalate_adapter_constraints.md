# Close Or Escalate Adapter Constraints

Close TODO 2 for the current process-adapter scope if the source already
honestly enforces the accepted model. Add only a narrow guardrail test if it
helps preserve that boundary.

Do not invent a sandbox adapter, filesystem namespace, network namespace,
provider SDK integration, hosted service, telemetry, transcript capture, or
external persistence. If enforced network/filesystem isolation is still the
desired outcome, document that it requires a new RFC.
