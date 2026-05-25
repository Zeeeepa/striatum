# Daemon Runbook

Operator reference for the Striatum Go daemon (`striatumd`): install,
lifecycle, runtime layout, the Postgres DSN, logs, and troubleshooting.
Formalized by [RFC 0079](../rfcs/0079-go-only-operability-and-install.md);
the Postgres bootstrap details live in
[POSTGRES_TRANSITION.md](../POSTGRES_TRANSITION.md).

Striatum is Go-only (RFC 0078). The daemon is a hard prerequisite for every
Striatum verb; there is no `--no-daemon` mode.

## Install / uninstall

`striatum daemon install` is a local bootstrap helper (it issues no daemon
RPC). It:

- renders `~/.config/systemd/user/striatumd.service` from an embedded
  template that uses the systemd `%h` (home) and `%t` (runtime dir)
  specifiers, so the unit is host-portable and contains no hardcoded paths;
- scaffolds a commented `~/.config/striatum/daemon.toml` **only when absent**
  (it never overwrites a real DSN);
- runs `systemctl --user daemon-reload` and `enable --now` unless
  `--no-start`;
- prints the resolved socket, token, and MCP-endpoint paths.

```bash
striatum daemon install              # render unit, scaffold config, enable --now
striatum daemon install --no-start   # render + enable, but do not start
striatum daemon install --print-unit # print the rendered unit, touch nothing
striatum daemon uninstall            # disable/stop/remove the unit (keeps config + data)
striatum daemon status               # unit state + runtime layout + doctor
```

`make install` chains binaries → `daemon install --no-start` →
`skills install` → best-effort start + health check; `make uninstall`
reverses it.

On a host without systemd user services, `daemon install` prints a
foreground run recipe instead of failing:

```bash
# 1. configure a Postgres DSN (see below)
# 2. run the daemon in the foreground:
striatumd -socket "${XDG_RUNTIME_DIR}/striatum/daemon-go.sock"
# 3. in another shell: striatum doctor
```

## Lifecycle (systemd user service)

```bash
systemctl --user start   striatumd      # start
systemctl --user stop    striatumd      # stop
systemctl --user restart striatumd      # restart (re-reads daemon.toml)
systemctl --user enable  striatumd      # start at login
systemctl --user is-active striatumd    # active | inactive | failed
```

After editing the unit by hand, run `systemctl --user daemon-reload`. To run
the user service without an active login session,
`loginctl enable-linger $USER` keeps it alive.

## Runtime layout

The daemon owns a per-user runtime directory,
`${XDG_RUNTIME_DIR}/striatum/` on Linux (typically `/run/user/<uid>/striatum/`)
and `~/Library/Caches/striatum/runtime/` on macOS. Override with
`STRIATUM_DAEMON_RUNTIME_DIR`. Contents:

| File | Purpose |
|---|---|
| `daemon-go.sock` | The canonical Unix-domain control socket. This is the single supported name; the retired Python launcher's `striatumd.sock` and the cutover symlink are gone (RFC 0079). |
| `client-token` | Capability token the CLI/MCP clients read to authenticate to the daemon. |
| `mcp-http-endpoint` | The resolved MCP HTTP endpoint URL (host:port + `/mcp`). |
| `striatumd.pid` | Daemon pidfile. |

Clients resolve the socket from `${XDG_RUNTIME_DIR}/striatum/daemon-go.sock`
by default; override with `--daemon-socket` or `STRIATUM_DAEMON_SOCKET`.

## Postgres DSN (`daemon.toml`)

The daemon refuses to bind a socket without a configured Postgres DSN
(D094 / RFC 0043). Resolution order (highest precedence first):

1. `--postgres-url <url>` flag (where accepted),
2. `STRIATUM_DAEMON_DB_URL` environment variable,
3. `postgres_url` in `~/.config/striatum/daemon.toml`
   (`$XDG_CONFIG_HOME/striatum/daemon.toml`).

`daemon install` scaffolds the file with a commented `postgres_url` example.
Set it to a reachable DSN, e.g.:

```toml
postgres_url = "postgres://striatum@localhost:5432/striatum?sslmode=disable"
```

Provisioning the Postgres role and database is out of scope for `daemon
install`; see [POSTGRES_TRANSITION.md](../POSTGRES_TRANSITION.md) for the
role/grant runbook and `striatum doctor`'s repair guidance.

## Logs

```bash
journalctl --user -u striatumd            # full log
journalctl --user -u striatumd -f         # follow
journalctl --user -u striatumd -n 100     # last 100 lines
```

In the foreground recipe the daemon logs to stderr.

## Troubleshooting

- **`daemon_unreachable` (exit 11).** The daemon is not running or the socket
  path is wrong. Check `striatum daemon status` and
  `systemctl --user is-active striatumd`; confirm the socket exists at
  `${XDG_RUNTIME_DIR}/striatum/daemon-go.sock`.
- **Daemon won't start / no DSN.** `journalctl --user -u striatumd` shows the
  daemon refusing to bind without `postgres_url`. Set the DSN (above) and
  `systemctl --user restart striatumd`.
- **`repo_not_migrated` (exit 12).** The target repo isn't registered with
  the daemon. Run `striatum repo add <path> --init` (see
  [POSTGRES_TRANSITION.md](../POSTGRES_TRANSITION.md)).
- **Socket / token mismatch.** A stale `client-token` from a prior daemon can
  cause auth failures. Stop the daemon, remove the runtime directory contents
  (`daemon-go.sock`, `client-token`, `mcp-http-endpoint`), and restart; the
  daemon regenerates them.
- **Migration on start.** The daemon applies pending schema migrations on
  startup; a slow first start after an upgrade is normal. Watch the log.
- **Unit references a deleted launcher.** If you upgraded from a pre-RFC-0078
  install, the old unit may point at the retired Python launch path. Run
  `striatum daemon install` to overwrite it with the current Go unit.

## Conversation trajectories and tmux

[RFC 0081](../rfcs/0081-conversation-trajectories.md) introduces real-time
observable trajectories. Use `trajectory watch` to follow a run's dialogue or
lifecycle in a dedicated tmux pane.

```bash
# 1. Start a watch in a background pane or new shell:
striatum trajectory watch --run-id <run-id> --profile dialogue

# 2. To follow with a tmux 'tail -f' feel:
# (in a 80x24 pane, showing only curated chat text)
striatum trajectory watch --run-id <run-id> | jq -r '.body.text // ""'
```

Trajectories are read-only projections. They are constrained by D028: they
never contain raw provider transcripts (process stdout/stderr).

| Profile | Contents | Use case |
|---|---|---|
| `dialogue` | Curated chat messages and artifact references. | Pure conversation view. |
| `provenance` | Full lifecycle (claim, ack, verdict, blocker) + dialogue. | Operational audit view. |

Export a stable JSONL manifest for reproducibility:

```bash
striatum trajectory export --run-id <run-id> --profile provenance > manifest.jsonl
```

## Interrogation sessions

[RFC 0082](../rfcs/0082-interrogation-sessions.md) lets a reviewer interrogate a
builder's *preserved* context after the builder finishes an `interrogable` job.
Such a builder session does not close on `work.complete`; it enters the
`awaiting_interrogation` phase (visible as the `session.awaiting_interrogation`
event) and stays live until the interrogation is closed or it is recovered.

```bash
# list / inspect interrogations on a run
striatum interrogation list --run-id <run-id>
striatum interrogation show --interrogation-id <id>
```

Operational notes:

- Interrogations require the MCP agent-loop executor — the fresh-per-packet
  supervised wrapper cannot preserve context and is not interrogable.
- A builder left in `awaiting_interrogation` widens the lease/resource window.
  If an interrogation is never closed, the bounded idle timeout / stale-lease
  recovery sweep (RFC 0020/0077) reclaims the session; an operator can also run
  `striatum session close --session-id <builder> --reason "..."` once no lease
  is held. `interrogation.close` closes the target automatically when it holds
  no active lease and no other interrogation is open against it.
- Interrogation turns are curated (D028) and surface in the `dialogue`
  trajectory; they are never raw provider stdout/stderr. The
  `striatumd.interrogations` table (migration 0016) is ownership-safe: it is a
  plain runtime-role table with no `ALTER`/FK to owner-held tables.
