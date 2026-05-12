# CLI Reference

> This page is a copy-paste reference. It may lag the parser;
> `striatum --help` (and `striatum <verb> --help`) is
> authoritative.

## Core lifecycle

```text
striatum init [--with-skills <profile>] [--with-ddd-layout]
              [--ddd-layout-force] [--ddd-layout-dry-run]
striatum workflow validate
striatum workflow plan
striatum workflow graph
striatum workflow init
striatum workflow templates list
striatum workflow templates show
striatum workflow generate
striatum run prepare
striatum branch confirm
striatum run start
striatum run summary
```

`workflow init [--style minimal|review|code-change] <path>`
scaffolds a starter workflow tree; when `--style` is omitted, the
scaffold style is `review`. This does not create a runtime default:
`run prepare` still requires an explicit `--workflow <path>`. The
generated tree uses a single `local` process lane as a valid
placeholder; edit lanes and job `lane_id` bindings for real agent
runs.

`workflow templates list [--kind shape|lane_set]` and
`workflow templates show <template_id>` expose the bundled local
workflow-template catalog. `workflow generate <path> --shape <shape>
--lane-set <lane_set> --artifact-root <path>` compiles a concrete
workflow tree from that catalog and immediately validates the generated
`workflow.json`. Add `--dry-run --json` to preview the full envelope
without writing files. Real lane sets require lane commands such as
`--lane-command author='["codex","exec"]'`; the `local` lane set keeps
the placeholder fixture command. V1 refuses overwrites and does not
run the workflow automatically.

`striatum init` creates `.striatum/` in the target repo. The
optional flags scaffold extra material:

- `--with-skills <profile>` (RFC 0015) — write the agent skill
  bundle for `claude_code` | `codex` | `gemini` | `generic` |
  `all`. Default profile is `claude_code`.
- `--with-ddd-layout` (RFC 0021) — scaffold the seven canonical
  human-facing DDD documents (`docs/SPEC.md`, `docs/PRD.md`,
  `docs/DECISION_LOG.md`, `docs/UBIQUITOUS_LANGUAGE.md`,
  `docs/DDD.md`, `docs/rfcs/README.md`,
  `docs/rfcs/0001-template.md`). Existing files are preserved.
- `--ddd-layout-force` (RFC 0021 V1.5) — overwrite existing
  regular-file targets with the template body. Records
  `prior_sha256` for audit. Non-regular-file targets
  (directories, broken symlinks) still error and are not
  touched.
- `--ddd-layout-dry-run` (RFC 0021 V1.5) — preview without
  writing. Per-file statuses use the `would_*` vocabulary.

## Agent / session work loop

```text
striatum register-session
striatum claim-next
striatum ack
striatum heartbeat
striatum release
striatum send
striatum block
striatum publish-artifact
striatum complete
striatum verdict
striatum submit-review
striatum decision record
```

## Worktree (opt-in per lane via `worktree_isolation: per_job`)

```text
striatum worktree create
striatum worktree release
striatum worktree list
```

## Supervisor (RFC 0009)

```text
striatum supervise start
striatum supervise send
striatum supervise stop
striatum supervise status
striatum supervise list
```

## Dashboard

```text
striatum dashboard
striatum dashboard --all
```

`dashboard --all` (RFC 0028 V1) groups registered repositories and
reports repo-local runs, blockers, claimable jobs, stale leases,
and degraded repositories. It is registry-backed and requires a
daemon `read` capability token (bootstrapped by `repo add` or
`daemon start`) even when `--daemon` is not passed.

## Daemon and multi-repo registry (RFC 0028 V1)

```text
striatum daemon start
striatum daemon status
striatum daemon stop
striatum daemon sweep
striatum daemon migrate --from sqlite --to pg [--dry-run]
                         [--keep-sqlite-readonly]
striatumd                     # console-script alias for `daemon start`
striatum repo add <path> [--init] [--no-migrate]
striatum repo list
striatum repo remove <path>
striatum cross-repo list
striatum cross-repo describe <cross_repo_run_id>
striatum cross-repo why <cross_repo_run_id>
striatum cross-repo cancel <cross_repo_run_id>
```

`striatum daemon start` / `striatumd` runs a foreground sweep
process: it does not host an RPC server for clients in V1; CLI and
MCP callers open the owner-only daemon registry SQLite directly
under token/capability checks. The Unix socket bound by
`striatumd` is a lifecycle marker, not a request router.

Both `daemon start` and the first `repo add` bootstrap a single
admin token when the registry has no clients and write a
`0600` runtime-fallback file. Token secrets are never read from
environment variables, never logged to audit, and never stored in
the registry. Authorization vocabulary in V1 is `read` and
`admin` only.

`repo add` canonicalizes the repository root, refuses
symlink/path-traversal ambiguity (including symlinked parent
components and state-database symlink escapes), derives a
realpath/inode-based repository identity, and refuses active
path re-occupation by a different identity. `--init` is required
when `.striatum/state.sqlite3` is absent; `--no-migrate` refuses
registration if repo-local migrations would be needed.

`repo remove` is idempotent, revokes live repo-scoped
capabilities, preserves audit rows, and never reuses
`repository_id`; re-adding allocates a fresh id.

`daemon sweep` is admin-gated and runs the sweep loop manually
across registered active runs; the normal recovery sweep also
runs from the foreground daemon process.

RFC 0033 V2 accepts system PostgreSQL as the daemon-owned
storage substrate for daemon-global state. Configure it with
`STRIATUM_DAEMON_DB_URL`, daemon config, or an explicit
`--postgres-url` client surface. The daemon owns schema
migrations and roles, but it does not install, start, stop, or
upgrade PostgreSQL. Bundled, embedded, and Dockerized Postgres
distributions are deferred.

`daemon migrate --from sqlite --to pg --dry-run` reports the V1
registry rows that would be exported. Without `--dry-run`, it
writes the V2 daemon DB schema, imports the V1 registry rows,
replays the metadata-only audit chain, verifies hash continuity,
and writes a cutover marker. Once the marker exists, V1 registry
reads are refused. `--keep-sqlite-readonly` keeps the V1 SQLite
file as an audit tombstone while blocking V1 writes. Repo-local
`.striatum/state.sqlite3` is untouched.

RFC 0030/0031 add the daemon V2 RPC and supervision/apply foundation on
top of RFC 0033. The wire envelope is versioned JSON; `daemon.hello`
negotiates envelope/framing, `daemon.describe` publishes the method
registry and `methods_etag`, and incompatible clients refuse with exit
code 10. Direct repo-local mode remains the compatibility path while
daemon routing moves method by method.

## Daemon-routed read mode

Pass `--daemon` (or set `STRIATUM_DAEMON=1`) on a read verb to
explicitly route through the daemon registry under token
authorization:

```text
striatum --daemon status [--run-id <id>]
striatum --daemon doctor
striatum --daemon why <job-id>
striatum --daemon dashboard --all
```

V1 read surfaces supported under `--daemon`: `status`, `doctor`,
`why`, `dashboard --all`. Forced-daemon mutation verbs refuse
with capability-denied semantics; the CLI does not fall back to
direct repo-local mode. `--no-daemon` forces direct mode.

Daemon RPC method capabilities use the closed vocabulary `read`,
`write`, `review`, `claim`, `apply`, `admin`, and `recovery`.
`supervise.*` and `apply.*` are daemon RPC routes; sealed apply fails
closed unless a daemon signing key and `apply` capability are present.

RFC 0032 adds cross-repo workflow schema and daemon MCP mutation
capability gating on the PostgreSQL daemon substrate. Cross-repo
workflow files declare `repositories`, `primary_repository`, and
per-job `repository` aliases. The daemon DB records canonical
`cross_repo_run_id` rows and each participant repo keeps a local
`runs.cross_repo_run_id` pointer. `cross-repo list|describe|why`
read those daemon records; the full live two-repo daemon harness and
real cross-repo end-to-end progression are deferred to TODO Open item
19. Daemon MCP `tools/list` is filtered by each token's effective
capabilities and scope, and `tools/call` re-checks authorization and
audits denials.

RFC 0036 adds no new CLI verb. Regenerate agent-facing MCP guidance with
`striatum skills install` or `striatum plugin install`; chat workflow
generation uses the existing local service and RFC 0034 generator paths.

## Skills (RFC 0015)

```text
striatum skills install
```

## Service (RFC 0012 / 0013)

```text
striatum serve
```

## List (read-only enumeration)

```text
striatum list runs
striatum list sessions
striatum list jobs
striatum list artifacts
striatum list workflows
```

## Inspection and recovery

```text
striatum status
striatum why
striatum doctor
striatum evidence export
striatum run graph
striatum recovery auto
striatum recovery watch
striatum recovery stale-leases
striatum recovery requeue-stale
striatum recovery cancel-job
striatum recovery process-reconcile
striatum recovery resume
striatum checkpoint resolve
striatum override-verdict
```

`run graph --run-id <id> [--format mermaid|json|dot|ascii]`
renders the workflow graph for an existing run with each node
colored by current job state. Mermaid output appends
`classDef`/`class` lines; JSON adds `current_state`, `attempt`,
and a `latest_verdict` block on review nodes; `ascii` reuses the
dashboard's graph panel renderer (RFC 0016).

## Adapter

```text
striatum adapter run
```

## Session lifecycle

```text
striatum session close
```

## Stable exit codes

- `0`: success, including `claim-next` with `no_work`.
- `1`: generic / unhandled runtime error.
- `2`: CLI usage error (argparse).
- `3`: missing run, session, job, message, blocker, artifact,
  verdict, or session target.
- `4`: invalid state transition.
- `5`: lease expiry or ownership mismatch.
- `6`: artifact or write-scope violation.
- `7`: branch confirmation required before work can be claimed.
- `8`: workflow config rejected (also raised by `branch confirm`
  when a requested git operation cannot be performed).
- `9`: local SQLite schema is newer than this striatum install
  supports.
- `10`: daemon RPC transport, handshake, or version-skew refusal.

## See also

- [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) — the operator's playbook
  with examples per verb.
- [HOW_TO_AGENT.md](HOW_TO_AGENT.md) — the coding-agent
  companion to the RFC 0015 skill bundle.
- [SPEC.md](SPEC.md) — the implementation contract.
