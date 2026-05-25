# Gate B — Installers: port skills/plugin to Go, retire scaffold

You are the implementer for RFC 0078 Gate B. Read first:
`docs/operator/plans/rfc-0078-remaining-work.md` (Gate B),
`docs/rfcs/0078-go-only-runtime-and-python-removal.md`,
`src/striatum/skills/install.py`, `src/striatum/skills/context.py`,
`src/striatum/plugins/install.py`, the template trees under
`src/striatum/skills/templates/` and `src/striatum/plugins/templates/`,
and how the Go CLI registers local (non-daemon) commands in
`go/pkg/cli/localcommands/` and `go/cmd/striatum/`.

## Decision (already made — do not re-litigate)

`striatum skills install` / `plugin install` are **documented live features**
(README, CLI_REFERENCE, GETTING_STARTED, HOW_TO_AGENT; `striatum doctor` emits
the invocation for stale bundles). They have **no Go command** today and must
be **ported to Go**. `scaffold` (only a `--scaffold` flag on `init`, no current
docs, no Go route) is **retired**.

## Steps

1. Implement `striatum skills install` and `striatum plugin install` as Go
   local commands with flag parity: `--profile {claude_code,codex,gemini,all}`,
   `--scope {user,project}`, `--force`, `--json`. Match the Python installer's
   output shape, especially the **version-stamp** that `striatum doctor`
   already reads to flag stale bundles (search Go for the doctor bundle-staleness
   check and align the stamp format).
2. Embed the template files (the 65 files under the two `templates/` trees) as
   Go assets (`embed.FS`, mirroring `go/pkg/webassets`). Drop the Python
   `__init__.py` package-data markers — they are not ported. Keep the actual
   skill/command/hook template content byte-equivalent.
3. Remove the `scaffold` behavior: drop the `--scaffold` flag and its handler
   from the Go `init` surface (and from the Python `cli/parser.py` /
   `cli/dispatch.py` if present in your scope).
4. Delete the Python installer source: `src/striatum/skills/*.py` and
   `src/striatum/plugins/*.py` (including the `templates/**/__init__.py`
   markers) once the Go path is wired. Leave Gate D's pytest targets alone.

## Constraints

- Local helpers must not open PostgreSQL except through accepted
  admin/bootstrap paths. `skills install` writes to the user/project skills
  dir, not daemon state.
- Stay within `write_scope.allowed_paths`.

## Validate

```bash
cd go && go test ./pkg/cli/... ./cmd/striatum
striatum skills install --profile all --scope user --force --json >/dev/null && echo install-ok
striatum doctor --json | grep -o '"problems":\[[^]]*\]'
```

## Required artifact

Publish `docs/operator/artifacts/rfc-0078-closure/installers/SUMMARY.md`
(`artifact_kind: synthesis`) listing: the new Go install command(s) + package,
how templates are embedded, the doctor version-stamp alignment, scaffold
removal, Python files deleted, and validation output. Use your packet byline.
