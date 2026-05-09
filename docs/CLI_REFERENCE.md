# CLI Reference

> This page is a copy-paste reference. It may lag the parser;
> `striatum --help` (and `striatum <verb> --help`) is
> authoritative.

## Core lifecycle

```text
striatum init
striatum workflow validate
striatum workflow plan
striatum workflow graph
striatum workflow init
striatum run prepare
striatum branch confirm
striatum run start
striatum run summary
```

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
```

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
striatum recovery stale-leases
striatum recovery requeue-stale
striatum recovery cancel-job
striatum recovery process-reconcile
striatum checkpoint resolve
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

## See also

- [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) — the operator's playbook
  with examples per verb.
- [HOW_TO_AGENT.md](HOW_TO_AGENT.md) — the coding-agent
  companion to the RFC 0015 skill bundle.
- [SPEC.md](SPEC.md) — the implementation contract.
