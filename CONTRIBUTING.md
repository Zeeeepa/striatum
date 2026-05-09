# Contributing to striatum

striatum is a small repo. This file is the front door for human
contributors. The quick path:

```bash
git clone https://github.com/halbritt/striatum
cd striatum
make install          # creates .venv/, installs editable + dev deps
make lint             # ruff
make typecheck        # mypy --strict
make test             # pytest
```

Every PR must keep `lint`, `typecheck`, and `test` green. CI runs
all three on Ubuntu and macOS against Python 3.11 and 3.12, plus
the package smoke (`scripts/package_smoke.sh`) and the
fresh-clone smoke (`scripts/fresh_clone_smoke.sh`).

striatum is local-first orchestration software. Do not add
hosted-service dependencies, telemetry, transcript capture, or
external persistence without an explicit product decision (see
[`docs/DECISION_LOG.md`](docs/DECISION_LOG.md)).

## Where to put what

The doc system has explicit boundaries — see
[`docs/DOC_MAP.md`](docs/DOC_MAP.md). Briefly:

- Behavior changes edit [`docs/SPEC.md`](docs/SPEC.md) and add a
  one-sentence-per-cell row to
  [`docs/DECISION_LOG.md`](docs/DECISION_LOG.md).
- New concepts add a glossary entry to
  [`docs/UBIQUITOUS_LANGUAGE.md`](docs/UBIQUITOUS_LANGUAGE.md)
  *first*, validator + introspection second
  (see [`docs/DDD.md`](docs/DDD.md) § "Adding to the model").
- Significant design changes go through an RFC under
  [`docs/rfcs/`](docs/rfcs/) before implementation.
- README is first-contact material capped at 250 lines by a
  test; per-feature detail belongs under `docs/`.

## Sending a PR

1. Branch from `main`.
2. Keep the PR scoped to one concern. Big features land via an
   RFC + dogfood run; small fixes don't need that ceremony.
3. Update the relevant doc in the same PR — for behavior
   changes, that means `SPEC.md` + `CHANGELOG.md` + (if it's an
   accepted RFC's V1) `DECISION_LOG.md`.
4. Add or update tests for behavior changes.
5. Don't commit `.striatum/`, `.venv/`, caches, egg-info,
   transcripts, or private diagnostics.
6. Push the branch; open the PR against `main`.

## Working as an agent contributor

If you are an LLM agent (Claude Code, Codex, Gemini CLI, …)
working on this codebase, read [`AGENTS.md`](AGENTS.md). It
points at [`docs/HOW_TO_AGENT.md`](docs/HOW_TO_AGENT.md) for
how to drive striatum *as a runner inside a target repo* —
that's a different role from contributing to striatum's source.

## Releases

Releases are tagged on `main` with `vX.Y.Z` and shipped to PyPI
+ GitHub Releases automatically by
[`.github/workflows/release.yml`](.github/workflows/release.yml).

To cut a release:

```bash
# 1. Bump version in pyproject.toml + src/striatum/__init__.py.
# 2. Promote `## Unreleased` to `## X.Y.Z — YYYY-MM-DD` in CHANGELOG.md.
# 3. Commit on main. The release workflow's tag-vs-pyproject check
#    will reject a tag that doesn't match the pyproject version.
git commit -am "vX.Y.Z: <one-line summary>"
git push origin main

# 4. Tag and push the tag. The Release workflow fires on the tag push.
git tag -a vX.Y.Z -m "vX.Y.Z: <one-line summary>"
git push origin vX.Y.Z
```

The release workflow:

1. Verifies the tag's version matches `pyproject.toml`.
2. Builds the wheel + sdist; runs `twine check --strict`.
3. Publishes to PyPI via
   [trusted publishing](https://docs.pypi.org/trusted-publishers/) —
   no API token in the repo.
4. Creates a GitHub Release for the tag with the matching
   CHANGELOG slice as the body and the dist files attached.

### One-time PyPI setup

Trusted publishing needs configuration on the PyPI side, not
just the repo side:

1. Create the project on PyPI (publish a first release with an
   API token, or pre-register via the PyPI UI).
2. On the project's PyPI "Publishing" page, add a trusted
   publisher with:
   - Owner: `halbritt`
   - Repository: `striatum`
   - Workflow: `release.yml`
   - Environment: `pypi`
3. In the GitHub repo settings, create an environment named
   `pypi` (no secrets required for OIDC; optionally restrict
   to the `main` branch / `v*` tags as a safety belt).

After that, every `v*` tag pushed to GitHub publishes the
release end-to-end.

## Versioning policy

- `0.x.y`: V1 RFCs landing on top of the V1 MVP baseline.
- `1.0.0`: every V1 RFC accepted (cut after RFC 0016 V1).
- `1.x.0`: new RFC landed (V1, or a step that closes a deferred
  slice).
- `1.x.y`: polish / fix / docs / a non-RFC feature follow-up
  (e.g., `1.4.1` added the run-level artifact rollup on top of
  RFC 0013 step 7).

## License

Apache-2.0. See [`LICENSE`](LICENSE). Unless noted otherwise,
contributions are licensed under the Apache License, Version 2.0.
