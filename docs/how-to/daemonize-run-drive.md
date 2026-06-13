# How-to: Daemonize `run drive` to take orchestration off the operator-model loop

`striatum run drive` is a deterministic Go reconcile loop over existing daemon
RPC methods (RFC 0116 / D175). It registers a session and supervises a lane for
each role/lane as the run's DAG unblocks, stops superseded or terminal lanes,
and blocks until the run reaches a terminal state — escalating loud on refusal
(RFC 0105). Crucially, **it spends zero operator-model tokens**: the mechanical
"spawn the next lane" work that an AI operator would otherwise hand-drive is
done by the binary, not by a frontier model.

Running it under systemd makes that token relief *standing* instead of
something the operator model has to babysit in-session. This is the recommended
way to drive long runs when the operator is an expensive model. It is also the
exact operational evidence that issue #212 (`supervision.auto_spawn`) names as
its first revisit trigger — "operators routinely run `run drive` as a
daemonized background process anyway."

> This how-to introduces no product-boundary change. Every spawn is still a
> capability-authenticated `supervise.start` RPC carrying the operator
> principal — the service just owns the loop instead of an interactive
> operator session. The residual it does *not* remove (a standing process
> holding the operator's capability for the run's lifetime) is precisely what
> [RFC 0122](../rfcs/0122-scheduler-principal-auto-spawn.md) would close.

## Prerequisites

- The daemon is running (`systemctl --user status striatumd`).
- The target repository is registered and a run is prepared and started
  (`striatum --repo <target> run prepare …` → `run start --run-id <id>`).
- A runtime client-token exists for the daemon (minted on daemon start under
  `$XDG_RUNTIME_DIR/striatum/client-token`); `run drive` discovers it the same
  way every other CLI verb does.
- Provider auth (if any lane needs it) is satisfied, or you pass an explicit
  `--provider-auth-gate` mode (RFC 0121).

## The unit (user service, one instance per run)

`run drive` is per-run (`--run-id` is required; there is no `--all`), so model
it as a systemd *template* instanced by run id. Install as a **user** unit so it
shares the operator's `XDG_RUNTIME_DIR` (where the client-token lives) and runs
as the operator OS user — the same principal an interactive `supervise start`
would carry.

`~/.config/systemd/user/striatum-run-drive@.service`:

```ini
[Unit]
Description=striatum run drive for run %i
# Driver is a client of the daemon; it should come up after it and stop trying
# if the daemon is gone (the loop will escalate loud rather than spin silently).
After=striatumd.service
Wants=striatumd.service

[Service]
Type=simple
# Resolve the target repository and the run from a per-run env file you write
# once at run-start time (see below). Keeps the unit generic across repos/runs.
EnvironmentFile=%h/.config/striatum/run-drive/%i.env
# ~/.local/bin holds the striatum CLI; daemon-spawned lanes only get this on
# PATH, so be explicit here too.
Environment=PATH=%h/.local/bin:/usr/local/bin:/usr/bin:/bin
WorkingDirectory=%h
ExecStart=%h/.local/bin/striatum --repo ${STRIATUM_TARGET_REPO} run drive \
  --run-id %i \
  --interval ${STRIATUM_DRIVE_INTERVAL} \
  --provider-auth-gate ${STRIATUM_DRIVE_AUTH_GATE} \
  --json
# run drive exits 0 when the run reaches a terminal state and non-zero on a
# loud refusal it could not resolve with normal lifecycle verbs. Do NOT restart
# on terminal success; do surface a refusal in the journal rather than masking
# it behind a respawn loop.
Restart=no

[Install]
WantedBy=default.target
```

Per-run env file `~/.config/striatum/run-drive/<run-id>.env`:

```sh
STRIATUM_TARGET_REPO=/abs/path/to/target/repo
STRIATUM_DRIVE_INTERVAL=15s
STRIATUM_DRIVE_AUTH_GATE=auto
```

## Drive a run

```sh
RUN=run_abcd1234
mkdir -p ~/.config/striatum/run-drive
cat > ~/.config/striatum/run-drive/$RUN.env <<EOF
STRIATUM_TARGET_REPO=$HOME/git/your-target-repo
STRIATUM_DRIVE_INTERVAL=15s
STRIATUM_DRIVE_AUTH_GATE=auto
EOF

systemctl --user daemon-reload
systemctl --user start striatum-run-drive@$RUN.service   # detached; no model in the loop
journalctl --user -u striatum-run-drive@$RUN.service -f   # watch reconcile actions / loud escalations
```

Because `run drive` is idempotent and holds no durable state of its own, a
restart of the service simply re-reconciles from daemon reads — it will not
double-spawn lanes that already have live sessions. Stopping the service mid-run
is safe; restart it to resume driving.

```sh
systemctl --user stop striatum-run-drive@$RUN.service     # safe to stop; re-drive resumes
```

When the run reaches a terminal state the loop exits and the unit goes
`inactive (dead)` with success — that is the expected end state, not a failure.
A non-zero exit means `run drive` hit a refusal it surfaced loudly (RFC 0105);
read the journal and resolve the underlying job, then restart the service to
re-drive.

## What this does and does not buy you

- **Token burn:** removed for the mechanical spawn/supervise/stop loop — the
  operator model only re-engages for genuine judgment or escalation, never to
  advance a ready job.
- **Latency:** `run drive` already blocks on the RFC 0120 wake bus rather than a
  fixed sleep where available, so the `--interval` is a fallback ceiling, not
  the steady-state reaction time.
- **Control surface (partial):** the operator model no longer needs to *exercise*
  register-session / supervise-start to advance the DAG, but the unit still
  holds the operator's capability token for the run's lifetime. Fully removing
  the standing operator credential (so the *daemon* owns spawn authority and the
  operator model needs no spawn capability at all) is the subject of
  [RFC 0122](../rfcs/0122-scheduler-principal-auto-spawn.md).

## Related

- [RFC 0116](../rfcs/0116-zero-operator-touch-dag.md) — `run drive` design and
  the `supervision.auto_spawn` deferral.
- [RFC 0122](../rfcs/0122-scheduler-principal-auto-spawn.md) — scheduler
  principal model that would let the daemon spawn directly.
- [Daemon runbook](daemon-runbook.md) — daemon lifecycle and token minting.
