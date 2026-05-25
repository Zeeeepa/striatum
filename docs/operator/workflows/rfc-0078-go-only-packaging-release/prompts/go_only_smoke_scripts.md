# Go-Only Smoke Scripts

Read RFC 0078, current `scripts/package_smoke.sh`,
`scripts/fresh_clone_smoke.sh`, `scripts/smoke_common.sh`, Makefile targets,
and release archive requirements. Produce
`docs/operator/artifacts/rfc-0078-go-only-packaging-release/smoke/HANDOFF.md`.

Replace the package and fresh-clone smoke path with Go-only scripts:

- run built `striatum`, `striatumd`, and `striatum-supervisor-helper`
  binaries from a release archive or local build output;
- verify version output, daemon doctor/bootstrap behavior, workflow validation,
  embedded assets, and Postgres-aware skips when local Postgres is unavailable;
- avoid `python`, `pip`, `venv`, `pytest`, `wheel`, `sdist`, and Python package
  data checks;
- keep target-repository Python commands allowed only when they are clearly
  target project commands, not Striatum runtime setup.

Prefer additive replacement scripts first. Do not remove the old smoke scripts
unless the workflow packet and current validation evidence make deletion safe.
