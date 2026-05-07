# Contributing

Striatum is local-first orchestration software. Do not add hosted-service
dependencies, telemetry, transcript capture, or external persistence without an
explicit product decision.

Before proposing changes:

1. Run `make check`.
2. Run `make smoke` for fresh-clone CLI smoke coverage.
3. Update `docs/DECISION_LOG.md` for product or architecture decisions.
4. Keep examples generic unless a file is explicitly labeled as an external
   reference fixture.

Unless noted otherwise, contributions are licensed under the Apache License,
Version 2.0.
