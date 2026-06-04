# Lane Sandbox Runbook — isolate supervised lanes from the daemon's PostgreSQL (#87 / RFC 0096 §2)

This runbook closes the residual half of the supervised-lane trust boundary
([RFC 0096](../rfcs/0096-supervised-lane-trust-boundary.md) V2, [#87](https://github.com/halbritt/striatum/issues/87)):
a supervised lane must not be able to reach the daemon's PostgreSQL directly or
otherwise bypass the artifact API.

Read [HOW_TO_HUMAN.md](how-to-human.md) for the broader operator playbook and
[the PostgreSQL Transition Runbook](postgres-transition.md) for daemon DB
provisioning.

## The gap

The daemon spawns supervised lanes as **its own OS user**. Two leak vectors
follow:

1. **DSN in the lane env / pane** — *already closed.* The supervised-lane
   environment is built from an explicit allowlist
   (`supervisedEnvAllowlistKeys`) that drops every `*DSN*` / `*POSTGRES*` /
   `PG*` / `DATABASE_URL` var, and `STRIATUM_MCP_TOKEN` is now the lane's *own*
   session-bound token, never the shared operator override
   ([#135](https://github.com/halbritt/striatum/issues/135)).
2. **Same-OS-user PostgreSQL reachability** — *the residual this runbook
   closes.* Even with no DSN in its environment, a lane running as the daemon's
   OS user can open the daemon's Postgres over the local unix socket via
   **peer authentication** (`psql "host=/var/run/postgresql dbname=striatumd"`),
   because peer auth keys off the process UID. The live #87 incident was a lane
   that, hitting an artifact-publish conflict, connected directly and tried to
   delete artifact rows and disable an append-only trigger.

Peer-auth reachability cannot be closed in Go from inside the daemon — it is an
**OS / PostgreSQL configuration** property. The fix is to run lanes as a
dedicated, unprivileged OS user that has **no PostgreSQL role** and is denied by
`pg_hba.conf`, so the lane's only control plane is the MCP surface.

`striatum doctor` reports this posture under `lane_sandbox` and emits a
`lane_pg_reachable` warning until the isolation is adopted (see
[Verify](#verify)). It is a configuration-posture proxy — it does **not** open a
PostgreSQL connection — and is best-effort by design.

## Adopt the PG-less lane OS user

> These steps mutate your machine's OS users and PostgreSQL host-based auth.
> They are an operator adoption step, not performed by the daemon.

### 1. Create a dedicated, login-less lane user

```sh
sudo useradd --system --no-create-home --shell /usr/sbin/nologin striatum-lane
```

### 2. Ensure it has no PostgreSQL role

The lane user must NOT have a Postgres role. Confirm none exists (this should
print nothing):

```sh
sudo -u postgres psql -tAc "SELECT rolname FROM pg_roles WHERE rolname = 'striatum-lane'"
```

If a role exists, drop it: `sudo -u postgres psql -c 'DROP ROLE "striatum-lane"'`.

### 3. Deny the lane user at `pg_hba.conf`

Add a **reject** rule for the lane user on the local socket, ABOVE the broader
local rules so it is matched first (path varies by distro, e.g.
`/etc/postgresql/16/main/pg_hba.conf`):

```
# Reject the supervised-lane OS user before any broader local rule (#87).
local   all   striatum-lane   reject
```

Reload PostgreSQL: `sudo systemctl reload postgresql`. The daemon's own role is
unaffected — it continues to authenticate as before.

### 4. Run supervised lanes as the lane user

Configure your process manager / supervisor to spawn supervised-lane processes
as `striatum-lane` (for a systemd-run daemon, this is the launch path that
drops privileges to the lane user before `exec`). The lane keeps access to the
target work tree and the MCP socket; it loses peer-auth access to Postgres.

### 5. Declare adoption to `doctor`

Set the lane OS user in the daemon's environment so `striatum doctor` confirms
the isolation and stops warning:

```sh
# e.g. in the daemon's systemd unit / environment:
STRIATUM_LANE_OS_USER=striatum-lane
```

`doctor` checks that this user exists and differs from the daemon's user.

### 6. Enable the secure-profile doctor gate

After adopting the lane OS user and protected PostgreSQL socket posture, enable
the RFC 0110 secure-profile gate:

```sh
STRIATUM_SECURITY_PG_SOCKET_HARDENED=1
```

With this flag, `doctor` treats `lane_pg_reachable` as a blocking problem
instead of an advisory warning. This flag does not create the lane user, alter
PostgreSQL socket permissions, or edit `pg_hba.conf`; it only makes the daemon
health check fail closed if the configured posture is still unsafe.

## Verify

```sh
striatum doctor --json | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['lane_sandbox']); print([w for w in d['warnings'] if 'lane_pg' in w])"
```

Adopted state:

- `lane_sandbox.lane_pg_isolated == true`
- `lane_sandbox.pg_socket_hardened == true` when the secure-profile gate is
  enabled
- no `lane_pg_reachable` warning.

To prove the close end-to-end, from a shell running **as the lane user**, a
direct connection must be refused:

```sh
sudo -u striatum-lane psql "host=/var/run/postgresql dbname=striatumd" -c 'SELECT 1'
# expected: FATAL: ... "striatum-lane" ... (pg_hba reject)
```

## Scope and non-goals

- This is OS-level isolation of a process Striatum spawns; it is **not** new
  daemon authority or forcible sandboxing of a process Striatum did not spawn
  (RFC 0103 W7 / RFC 0099 honest-limit framing).
- It adds no new persistence, hosted service, or telemetry (D094/D028/D151
  intact).
- The artifact-publish *conflict* that drove the #87 incident toward direct DB
  edits is addressed separately by the artifact-contract legibility work
  (RFC 0100 / RFC 0103 W5) — this runbook removes the lane's *ability* to reach
  the DB regardless.
