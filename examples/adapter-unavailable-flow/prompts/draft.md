# Draft

Draft the artifact at the expected path. Treat the lane as fully offline; the
workflow declares `network=forbidden` with required enforcement `enforced`.
This fixture is intentionally rejected at validation by the process adapter,
which can only provide `advisory_strict` for network.
