# CLI Reference

> This page is a copy-paste reference. It may lag the parser;
> `striatum --help` (and `striatum <verb> --help`) is
> authoritative.

## Core lifecycle

```text
striatum repo add <path> [--init]
striatum skills install [--profile <profile>] [--scope project|user]
striatum plugin install [--profile <profile>] [--scope project|user]
striatum workflow validate
striatum workflow generate
striatum workflow templates list
striatum workflow templates show
striatum run prepare
striatum branch confirm
striatum run start
striatum run drive
striatum operator bootstrap
striatum run summary
striatum archive create
striatum archive verify
```

`workflow generate` (below) is the way to scaffold a starter workflow tree;
`run prepare` requires an explicit `--workflow <path>` and creates no runtime
default. The generated tree uses a single `local` process lane as a valid
placeholder; edit lanes and job `lane_id` bindings for real agent runs.

`run drive --run-id <id> [--interval 15s] [--once] [--json]` is a local
operator loop over existing daemon RPC methods. It reads `run.detail` and
`list.sessions`, registers and supervises one fresh session per queued
role/lane as the DAG unblocks, adopts already-active matching sessions, and
stops terminal or superseded launched lanes before registering fresh reviewers.
It is not a daemon RPC method and does not call rescue verbs or force non-fresh
sessions.

`operator bootstrap [--operator-docs-root <path>] [--limit N]
[--markdown|--json]`
is a bounded AI-operator cold-start packet. It is a local read-only
composite over existing daemon reads (`repo.resolve`, `status`, `doctor`)
plus local git, `VERSION`, daemon-runtime-path, MCP-endpoint-path, and
`docs/operator/BRIEF.md` probes. It prints frontier state, doctor counts,
operator-brief freshness, skill drift, exact next commands, and a bounded
reading plan without embedding full status, doctor, session, verdict, or
historical run arrays. `--limit` caps expanded lists and is bounded to 20.
The command creates no live state and is not a daemon RPC method.

`workflow templates list [--kind shape|lane_set|role_pack|adversary_pack]`
and `workflow templates show <template_id>` read the bundled local
workflow-template catalog (embedded under `go/pkg/workflowtemplates`).

`workflow generate --shape <shape> [--lane-set <set>] [--workflow-id <id>]
[--scaffold-root <path>] [--artifact-root <path>] [--option key=value ...]`
compiles a concrete workflow tree from that catalog. It previews the planned
repo-relative files by default and writes them only with `--write`; `--json`
emits the structured envelope. The lane set defaults to the `local` fixture
lane (which needs no real lane command), so `workflow generate --shape
conversation --option topic="…"` scaffolds a valid starter out of the box; edit
in real lanes (e.g. `--lane-set author_reviewer` then supply lane commands in
the generated `workflow.json`) before a real run. `--option phases=…`
(`multi_phase`) and shape-specific `--option key=value` values are
accepted as the shape requires.

The Python-era `workflow init`, `workflow lint`, `workflow plan`,
`workflow graph`, `workflow upgrade`, and `workflow templates render-md`
verbs are not part of the current Go CLI (RFC 0078 ported `validate`,
`generate`, and `templates {list,show}`); workflow-authoring lint is enforced
at `validate`/generation time. A fuller CLI_REFERENCE audit against the Go
command surface is tracked separately.

Same-model-pairing lint is enforced by `workflow validate` (refuse unless
`--allow-same-model-pairing`); operational accepted-risk overrides are recorded
through the daemon `workflow accept-risk` / `workflow accepted-risks` commands.

`striatum repo add <path> [--init]` registers a target repository
with the daemon-owned PostgreSQL registry. Pass `--init` for a fresh
target repo so the daemon creates `.striatum/scratch` and adds
`.striatum/` to `.gitignore`.

`striatum skills install --profile <profile>` writes the agent skill
bundle for `claude_code` | `codex` | `agy` | `generic` | `all`.
Use `--scope user` to install once in the user's agent config
directory instead of a project tree.

`striatum plugin install --profile <profile>` writes agent plugin
bundles for `claude_code`, `codex`, or `agy`. Project-scope plugin
installs also write a local marketplace fixture when supported by the
profile.

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

Use `submit-review` for the normal single-call review path: publish the
finding artifact and record the verdict. Use `verdict` when the required
review artifact is already published for the current attempt, such as a
re-claimed review job after lease recovery.

For operator-authored or otherwise unattested recovery of an accepting fresh
review, record an accepting run-level decision with
`--escape-surface review_provenance`, then pass
`--review-provenance-decision-id <decision-id>` to `submit-review` or `verdict`.
The verdict event records the decision id and artifact id as durable override
evidence.

`publish-artifact` validates lease ownership, write scope, path
safety, artifact kind, front matter, and byline. Model-bylined
artifacts require lane evidence: a path-specific supervised
`artifact_observed` event when the wrapper reports one, or the legacy
clean `process_executions` fallback. Operators can explicitly bypass
missing lane evidence with `--allow-no-process-execution
--override-rationale <text>`; the rationale is stored on the artifact
row and in the provenance event.
For review jobs declaring `require_attested_lane: true`, `publish-artifact`
also refuses publication unless the session has an attached lane supervisor.
`decision record --mark-run-compromised` records an accepting decision and
transitions a completed run to `compromised` for provenance invalidation; V1
uses this replacement-run-only path for compromised completed review jobs.

Artifact listing, detail, summary, export, and dashboard JSON includes
`provenance.category` so operator-bylined artifacts can be separated from
attested supervised-lane, daemon auto-finalized, operator-on-behalf,
self-declared operator, recovery-authored, and run-level operator artifacts.

## Worktree (opt-in per lane via `worktree_isolation: per_job`)

```text
striatum worktree create
striatum worktree release
striatum worktree gc [--run-id <id>]
striatum worktree list
```

`worktree gc` removes only on-disk job worktrees whose jobs are terminal and
whose HEAD is reachable from the run branch or a `refs/striatum/` pin; skipped
rows are reported with reasons. Worktrees with no-blob published artifacts that
are not present in the worktree `HEAD` are skipped until the artifact content is
durable outside the per-job worktree.

## Supervisor (RFC 0009)

```text
striatum supervise start
striatum supervise send
striatum supervise stop
striatum supervise status
striatum supervise list
striatum supervise trajectory
```

## Dashboard

```text
striatum dashboard
striatum dashboard --all
```

`dashboard --all` (RFC 0028 V1) groups registered repositories and
reports daemon/Postgres-backed per-repository runs, blockers, claimable jobs,
stale leases, and degraded repositories. It uses daemon-owned repository
registration state and requires a daemon `read` capability token from the
runtime `client-token` file.

## Daemon and multi-repo registry (RFC 0028 V1)

```text
striatum daemon install [--no-start] [--print-unit]
striatum daemon status
striatum daemon uninstall
striatum daemon migrate-db [--admin-url <dsn>] [--json]
striatum daemon owner-ddl apply [--owner-url <dsn>] [--json]
striatum doctor [--first-run] [--verbose] [--json]
striatumd [daemon-start options]
systemctl --user start|stop|restart|status striatumd
striatum repo add <path> [--init] [--no-migrate compatibility flag]
striatum repo list
striatum repo remove <id>
striatum cross-repo list
striatum cross-repo describe <cross_repo_run_id>
striatum cross-repo why <cross_repo_run_id>
striatum cross-repo cancel <cross_repo_run_id> [--reason <text>]
```

`striatum daemon install` renders the systemd user unit, scaffolds
`daemon.toml` when absent, and enables/starts `striatumd` unless `--no-start`
is passed. Use `systemctl --user start|stop|restart striatumd` for service
lifecycle after installation. On hosts without systemd user services, run
`striatumd -socket "${XDG_RUNTIME_DIR}/striatum/daemon-go.sock"` directly as
described in the daemon runbook.

`striatumd` is the supported foreground daemon process. Per D094 / RFC 0043 the
daemon is a hard prerequisite for every Striatum verb; CLI verbs without a
reachable daemon refuse with exit code 11 (`daemon_unreachable`) and do not
fall back to direct mode.

On first successful startup, `striatumd` bootstraps a single admin token when
daemon-owned Postgres has no clients and writes a `0600` runtime
`client-token` file. Token secrets are never read from environment variables,
never logged to audit, and never stored in the registry.
Authorization uses the closed daemon method capability vocabulary:
`read`, `write`, `review`, `claim`, `apply`, `admin`, `recovery`, and
`surgical_recovery`.

`repo add` canonicalizes the repository root, refuses
symlink/path-traversal ambiguity, derives a realpath/inode-based
repository identity, and refuses active path re-occupation by a
different identity. Pass `--init` when no `.striatum/` directory
exists; it creates operational scratch only and does not create
`.striatum/retired-local-state`. If a pre-D094 repo-local SQLite source
exists, registration refuses and tells the operator to archive/remove
the legacy SQLite file before registering.

`repo remove` is idempotent, revokes live repo-scoped
capabilities, preserves audit rows, and never reuses
`repository_id`; re-adding allocates a fresh id.

The resident recovery sweep runs inside `striatumd`. Use the daemon-backed
`recovery auto` / `recovery watch` family for explicit recovery diagnostics
where applicable; there is no `striatum daemon sweep` CLI command.

RFC 0033 V2 accepts system PostgreSQL as the daemon-owned
storage substrate for daemon-global state. Configure it with
`STRIATUM_DAEMON_DB_URL`, daemon config, or an explicit
`--postgres-url` client surface. The daemon owns schema
migrations and roles, but it does not install, start, stop, or
upgrade PostgreSQL. Bundled, embedded, and Dockerized Postgres
distributions are deferred.

`striatum doctor` is the daemon-backed health check. `doctor --first-run`
verifies daemon socket reachability, PostgreSQL posture, runtime-token
presence, repo registration, MCP visibility, and a sample daemon read route.
`doctor --verbose` includes structured `problem_records` alongside the stable
string `problems` list.
`striatum daemon status` is the local bootstrap summary for unit state and
runtime paths; it folds in read-only doctor information when the daemon is
reachable.

`daemon migrate-db [--admin-url <dsn>] [--json]` (RFC 0079 §5) applies pending
daemon PostgreSQL schema migrations using an owner/admin DSN, so DDL the runtime
role (`striatumd_rw`) cannot perform — e.g. a migration that adds a foreign key
to an owner-held table — is applied before the daemon serves. The admin DSN is
resolved from `--admin-url`, then `STRIATUM_DAEMON_ADMIN_DB_URL`, then the normal
daemon DSN (flag/env/`daemon.toml`) as a fallback for additive migrations the
runtime role can apply itself. This is distinct from the retired SQLite-era
`daemon migrate` below.

`daemon migrate --from sqlite --to pg` and
`daemon migrate-repo-local --from sqlite --to pg --repo <path>` are fully
removed SQLite-era import spellings. They are no longer parseable compatibility
commands; stale automation receives an unknown-command parse failure. Use
`striatum doctor --first-run --json` and `striatum repo add <path> --init` for
current registration/cutover diagnostics.
CLI verbs against an unregistered repo refuse with exit code 12
(`repo_not_migrated`) and point operators to archive/remove legacy SQLite
files, then register with `repo add --init`.

RFC 0030/0031 add the daemon V2 RPC and supervision/apply foundation on
top of RFC 0033. The wire envelope is versioned JSON; `daemon.hello`
negotiates envelope/framing, `daemon.describe` publishes the method
registry and `methods_etag`, and incompatible clients refuse with exit
code 10. RFC 0048 completed the production handler-port work in
v1.49.0-v1.55.0: mapped production verbs are daemon/Postgres-backed and
fail closed without daemon reachability, repository registration, and
capability authorization. Legacy SQLite paths remain only for migration
sources, golden fixtures, and explicitly gated subprocess compatibility
tests under `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`.

## Daemon-required runtime

Production commands route through the daemon authority boundary by default.
There is no global `--daemon` flag in the Go CLI; daemon connectivity is
required and assumed.

```text
striatum status [--run-id <id>]
striatum doctor
striatum why <job-id>
striatum dashboard --all
striatum doctor --first-run
```

The V1 `--no-daemon` flag is retired (D094 / RFC 0043); parsing it
returns the standard argparse "unrecognized arguments" error and exit
code 2. Production mutation and read verbs do not fall back to direct
repo-local mode.

`doctor --first-run` is a bootstrap smoke check, not a normal
repo-state doctor run. It returns a
`striatum.first_run_diagnostic.v1` JSON report that verifies daemon socket
reachability, Go daemon binary provenance from `striatumd --describe`,
Postgres doctor status, runtime token presence, repository registration,
MCP tool visibility, one sample daemon read route, and the daemon authority
report.

Daemon RPC method capabilities use the closed vocabulary `read`,
`write`, `review`, `claim`, `apply`, `admin`, `recovery`, and
`surgical_recovery`.
`supervise.*` and `apply.*` are daemon RPC routes; sealed apply fails
closed unless a daemon signing key and `apply` capability are present.
The Go daemon rotates the local Ed25519 fallback signing key through the
admin RPC method `daemon.key.rotate`; there is not yet a stable
user-facing `striatum keys` CLI.

RFC 0032 adds cross-repo workflow schema and daemon MCP mutation
capability gating on the PostgreSQL daemon substrate. Cross-repo
workflow files declare `repositories`, `primary_repository`, and
per-job `repository` aliases. The daemon DB records canonical
`cross_repo_run_id` rows under participating repository scopes.
`cross-repo list|describe|why` inspect those daemon records according to
capability scope. `cross-repo cancel` is the `cross_repo.cancel` recovery
route: it cancels non-terminal participant runs through the PG-native
participant runner, skips terminal participants and preparing participants
that never created a local run, and returns `blocked` with diagnostics when a
participant cannot be canceled. Daemon MCP `tools/list` is filtered by each
token's effective capabilities and scope, and `tools/call` re-checks
authorization and audits denials.

RFC 0036 adds no new CLI verb. Regenerate agent-facing MCP guidance with
`striatum skills install` or `striatum plugin install`; chat workflow
generation uses the existing local service and RFC 0034 generator paths.

## Skills (RFC 0015)

```text
striatum skills install
```

## Daemon-Mounted HTTP Service (RFC 0012 / 0013 / 0085)

```text
striatumd -mcp-http-addr 127.0.0.1:0
BASE_URL=$(sed 's#/mcp$##' "$XDG_RUNTIME_DIR/striatum/mcp-http-endpoint")
TOKEN=$(cat "$XDG_RUNTIME_DIR/striatum/client-token")
curl -H "Authorization: Bearer $TOKEN" "$BASE_URL/v1/health"
```

### Web routes (RFC 0013 / 0022 / 0024 / 0038)

`striatumd` mounts the Go web service on the same localhost-only HTTP listener
as daemon MCP. The endpoint file includes `/mcp`; strip that suffix for web
routes. The loopback service requires `Authorization: Bearer <client-token>`;
there is no separate `striatum serve` command and no `serve --web` flag.
Mutations are read-only by default and require
`STRIATUM_DAEMON_WEB_ALLOW_MUTATIONS=1` on the daemon process before startup.

For ordinary browser access without bearer-header tooling, run the optional
read-only tailnet identity listener (`striatumd -web-tailscale` or
`STRIATUM_DAEMON_WEB_TAILSCALE=1`) and point
`tailscale serve --bg unix:$XDG_RUNTIME_DIR/striatum/web-ui.sock` at the
owner-only socket.

| Route | Surface |
| --- | --- |
| `/` | Run list (RFC 0013/0022/0037). |
| `/run?run_id=<id>` | Server-rendered run detail/status page. |
| `/v1/health` | Service health and mutation posture. |
| `/v1/runs` | Daemon `status` JSON. |
| `/v1/runs/<id>` | Daemon `status --run-id` JSON. |
| `/v1/runs/<id>/events` | Server-Sent Events over daemon `run.events`. |
| `/v1/runs/<id>/dashboard` | Dashboard DTO for the run. |
| `/v1/runs/<id>/why?id=<entity>` | Daemon `why` JSON. |
| `/v1/runs/<id>/artifacts` | Artifact list for the run. |
| `/v1/artifacts/<id>/raw` | Raw artifact content. |
| `/workflow-templates` | Workflow template catalog list. |
| `/workflow-templates/<id>` | Workflow template catalog entry. |
| `/workflows/generate/preview` | Workflow generator preview (`POST`). |
| `/workflows/generate` | Workflow generator write (`POST`; requires web mutations). |

## List (read-only enumeration)

```text
striatum list runs
striatum list sessions
striatum list jobs
striatum list artifacts
striatum list workflows
```

`list runs` includes the workflow identity triple for each run:
`workflow_id`, `workflow_version`, and `workflow_snapshot_id`. The web
run list uses the same snapshot identity to display the workflow name
and link back to the workflow detail when the source path is known.

## Inspection and recovery

```text
striatum status
striatum why
striatum doctor
striatum git snapshot
striatum git commit-apply
striatum evidence export
striatum run graph
striatum recovery auto
striatum recovery auto-publish
striatum recovery auto-finalize
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

`recovery resume` resolves remediated process-adapter blockers with the
preserved lease. For remediated write-scope dirty-path blockers, it validates
that the tree is clean, resolves the blocker, and requeues the same attempt for
a fresh claim before completion.

`git snapshot --json [--ancestry-limit N] [--no-ancestry]` emits the
daemon read-only `git.snapshot` projection for the registered target
repository: local branch, HEAD metadata, dirty counts, changed paths,
and bounded ancestry. It does not fetch, push, commit, read remote URLs,
or include diff hunks or commit bodies.

`git commit-apply <commit-request-path> --confirm --confirm-request-id <id>
--json` emits daemon method `git.commit_apply`. It creates only a local
commit from a `striatum.commit_request.v1` artifact whose
`confirmation_status` is already `operator_confirmed` or `human_confirmed`.
It refuses base-HEAD, branch, or dirty-path mismatches, disables repository
Git hooks for the commit invocation, and never pushes or calls hosted
providers.

`recovery auto` emits the daemon `recovery.sweep` method. The sweep
runs workflow-opt-in `recovery.auto_finalize` before lazy lease expiry,
then the existing stale-lease, process-reconcile, and review-only requeue
recovery pieces where policy allows. Timed-out human checkpoints execute
the configured `recovery_policy.escalation_hook` in live sweeps; dry-runs
report the hook kind without side effects, and hook failures are reported
inside `escalations[]`. `recovery auto-publish` emits the explicit
`recovery.auto_publish_stale_artifacts` method; the deprecated `recovery.auto`
alias is not emitted by the current CLI. `recovery watch` is a CLI-local
foreground scheduler that repeatedly calls daemon `recovery.sweep`; it is not
a registered daemon RPC method.

## Corpus export (RFC 0044 V1 / RFC 0057 contract)

```text
striatum corpus export --since <ref> --out <dir>
striatum corpus verify --bundle <dir>
```

`corpus export` emits a redacted JSONL bundle of Striatum's durable
provenance (RFCs, decision-log rows, operator reports, run summaries,
audit-chain entries, changelog entries, ubiquitous-language terms,
harness-friction patterns, recent commits) plus a verifying
`manifest.json` with explicit `state_authority` metadata. Re-running over
unchanged inputs produces byte-identical JSONL files and stable per-file
SHA-256s; only `generated_at` varies, and it is excluded from the bundle
digest.
`corpus verify` is a local read-only checker for an existing bundle; it
validates the manifest, per-file hashes and byte counts, JSONL row shape,
duplicate row ids, row/file `sub_kind` consistency, row counts, and the
implied V1 corpus contract version.

The bundle is operator-triggered local provenance, never streamed to any
external service. Optional consumers (Engram is the first reference under
RFC 0044) may ingest the bundle for retrieval, but Striatum does not call
them at runtime and runs identically when no consumer is configured. The
V2 contract decisions (multi-corpus identity, redaction-tier metadata,
incremental watermarks, optional context-injection policy) are scoped by
[RFC 0057](../rfcs/0057-corpus-contract-v2.md).

## Run archive

```text
striatum archive create --run-id <id> --out <dir>
striatum archive verify --bundle <dir> [--manifest-only] [--repo-root <path>]
striatum archive inspect --bundle <dir> [--repo-root <path>]
```

`archive create` is a daemon/Postgres-backed read command that writes a
local archive directory for one run. The V2 archive contains the run row,
workflow snapshot, run-scoped rows, artifact metadata, event metadata, and a
self-verifying `manifest.json`; it does not copy artifact contents,
transcripts, or `.striatum/` scratch. `archive verify` is local and
read-only against an existing archive bundle, and it runs offline semantic
replay by default. `--manifest-only` is the explicit fast path that skips
semantic replay; `--repo-root` also verifies artifact content hashes against
files in a local repository checkout. `archive inspect` is a read-only local
projection over the same verifier.

## Adapter

```text
striatum adapter run
```

`adapter run` is retired outside the explicit legacy test-fixture
compatibility environment. Use daemon-supervised process lanes instead.

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
- `9`: state schema is newer than this striatum install supports
  (daemon PostgreSQL in production; legacy SQLite only in fixture paths).
- `10`: daemon RPC transport, handshake, or version-skew refusal.
- `11`: `daemon_unreachable`. The CLI could not reach the daemon
  socket; stderr names the socket path and remediation. No SQLite
  fallback is attempted.
- `12`: `repo_not_migrated`. The target repository is not registered for
  daemon/PostgreSQL state or still has a legacy `.striatum/retired-local-state`;
  stderr and the `--json` hint tell the operator to archive/remove legacy
  SQLite files and register with `repo add --init`.

## See also

- [HOW_TO_HUMAN.md](../how-to/how-to-human.md) — the operator's playbook
  with examples per verb.
- [HOW_TO_AGENT.md](../how-to/how-to-agent.md) — the coding-agent
  companion to the RFC 0015 skill bundle.
- [SPEC.md](spec.md) — the implementation contract.
