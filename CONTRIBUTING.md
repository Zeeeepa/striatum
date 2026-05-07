# Contributing

Striatum is local-first orchestration software. Do not add hosted-service
dependencies, telemetry, transcript capture, or external persistence without an
explicit product decision.

Before proposing changes:

1. Run `make test`.
2. Run `scripts/fresh_clone_smoke.sh` for packaging and CLI smoke coverage.
3. Update `docs/DECISION_LOG.md` for product or architecture decisions.
4. Keep examples generic unless a file is explicitly labeled as an external
   reference fixture.

The current copyright status is all rights reserved pending an explicit owner
license decision.
