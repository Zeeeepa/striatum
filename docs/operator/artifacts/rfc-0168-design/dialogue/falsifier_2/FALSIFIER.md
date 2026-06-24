# FALSIFIER - RFC 0168 P0 PostgreSQL pool-deny challenge

author: falsifier-reviewer-004

## Verdict

**needs_revision.** I credit the holder on the assigned hard core where it is
actually source-grounded: this host has Yama `ptrace_scope=1`; the current tmux
path is run through `sudo -n -u <RunAsUser> -- env -i ...`; bare tmux uses the
per-uid default socket; and this live lane's socket is under
`/tmp/tmux-994/default` with `/tmp/tmux-994` mode `0700` owned by
`striatum-lane`. If RFC 0168 really launches two lanes under distinct pool uids
and does not add a same-uid bridge, the `respawn-pane`, `0600` file, ptrace,
setns, and `/proc` secret-read claims are the right structural negative-control
shape.

The standing challenge is narrower and material: the holder's OQ4/A13 answer
says PostgreSQL isolation for the whole pool is handled by a group reject rule,
`local all %striatum-lanes reject`, but PostgreSQL HBA rules do not match Unix
OS group membership that way. HBA's `user` field matches the requested
PostgreSQL user or PostgreSQL role membership (`+role`), not the client's Unix
group. For TCP connections it does not know the client OS uid at all. Therefore
the proposed pool-level PG deny is not build-bearing, and A13 can falsely pass
if it only proves that the pool uids have no same-named DB role or no password
today.

This is not `reject`: the per-lane uid direction still looks like the right
structural move, and I am not re-opening D261. It is not `accept_with_findings`:
OQ4 explicitly requires ACL/PG isolation for every pool uid without widening
authority, and the holder's proposed PG deny primitive is not the primitive
PostgreSQL implements.

## Challenge: `pg_hba` Cannot Deny A Unix Pool Group As Written

### Precise Claim Attacked

The holder claims OQ4 is discharged by a DEFAULT group ACL for repo read,
per-uid private secrets, leasing-uid worktree write, and PostgreSQL isolation
for every pool uid. The PG part is stated as: "The PG isolation (`pg_hba`
reject) must cover **every** pool uid - handled by a group reject rule (`local
all %striatum-lanes reject`, the pool analogue of `lane-sandbox.md:75-80`)"
(`HOLDER.md:376-386`). A13 then requires `make lane-isolation-check` to show
**every** pool uid denied PostgreSQL over both the Unix socket and loopback TCP
(`HOLDER.md:387-394`).

That is the claim I attack: **a Unix pool group can be denied in `pg_hba.conf`
with `%striatum-lanes`, and that proves every pool OS uid is denied
PostgreSQL.**

### Source-Anchored Evidence

- The existing lane sandbox runbook requires two independent facts: the lane
  user has **no PostgreSQL role** and is denied by `pg_hba.conf`, so the only
  control plane is MCP (`docs/how-to/lane-sandbox.md:31-42`). The current
  concrete `pg_hba.conf` rows are explicit user rows for `striatum-lane`, over
  both local peer and loopback TCP (`docs/how-to/lane-sandbox.md:66-80`). There
  is no current source-backed OS-group HBA shortcut.
- The holder changes that to a pool group rule and writes `%striatum-lanes`,
  but PostgreSQL 17's HBA grammar says the `user` field is a database user name,
  a regex, or a group name preceded by `+`; the `+` group is PostgreSQL role
  membership, not a Unix group. See the official PostgreSQL 17 `pg_hba.conf`
  documentation: https://www.postgresql.org/docs/17/auth-pg-hba-conf.html.
- PostgreSQL peer auth obtains the client's OS user name only as part of local
  peer authentication, after a matching HBA record has been selected for the
  requested database user. It is not an HBA OS-group selector. See the official
  PostgreSQL 17 peer auth documentation:
  https://www.postgresql.org/docs/17/auth-peer.html.
- On this host, `psql --version` reports PostgreSQL 17.10, so the PostgreSQL 17
  HBA syntax is the relevant operational surface for the current design host.
- The holder's launch path and environment stripping are real (`pty.go:98-155`),
  but they do not prove the PG deny. Stripping `DATABASE_URL`/`PGPASSWORD`
  reduces accidental credential exposure; it is not an HBA rule that denies an
  OS uid over local and TCP paths.

### Concrete Failing Case

1. The operator provisions pool OS users `striatum-lane-001` and
   `striatum-lane-002`, both members of the Unix group `striatum-lanes`,
   exactly as RFC 0168's OQ4/OQ3 direction requires.
2. The operator follows the holder's PG recipe and installs pool-level HBA rows
   shaped like:

   ```
   local   all   %striatum-lanes                    reject
   host    all   %striatum-lanes   127.0.0.1/32     reject
   host    all   %striatum-lanes   ::1/128          reject
   ```

3. A lane process running as `striatum-lane-002` attempts a local or TCP
   PostgreSQL connection. PostgreSQL matches HBA records against the requested
   database user, not against the process's Unix group. `%striatum-lanes` is not
   PostgreSQL's group syntax; even if corrected to `+striatum-lanes`, it would
   mean "requested DB role is a member of PostgreSQL role `striatum-lanes`,"
   not "client OS uid is in Unix group `striatum-lanes`."
4. The reject row therefore does not establish the claimed invariant. A later
   broad row such as `local all all peer`, `host all all 127.0.0.1/32
   scram-sha-256`, or a deployment-specific allow rule is now the operative rule
   for any requested DB user not literally matched by the reject entry.
5. If the lane can request a DB role for which it has credentials, an auth map,
   trust, or accidentally hydrated password material, it can reach PostgreSQL
   despite being a pool uid. That is a direct violation of the lane-sandbox
   boundary: the lane is no longer confined to MCP and can read or mutate
   daemon-owned state directly.

This is not hypothetical grammar nitpicking. The packet specifically asks
whether ACLs and related isolation grant exactly the authority each pool uid
needs, and the holder's A13 says every pool uid is denied over both PostgreSQL
routes. The proposed HBA rule does not test that property.

### Why The Holder's Rebuttal Is Not Enough

The strongest rebuttal is that pool OS users should have no PostgreSQL roles and
should not receive DB passwords, so the connection fails anyway. That is
necessary, but it is not the claim the holder made. "No role/no password
happened to be present" is a negative credential inventory, not a PostgreSQL
deny rule covering every pool uid. It also does not survive ordinary drift: an
operator can create a same-named role during debugging, a future feature can add
a mapped role, or credential hydration can accidentally expose a DB password.
The sandbox runbook explicitly treats loopback TCP as dangerous when a
role/password exists; RFC 0168 cannot replace that with an invalid group
shortcut.

The second rebuttal is to use PostgreSQL role grouping, e.g.
`+striatum_lanes`, instead of `%striatum-lanes`. That is a different design. It
requires creating and maintaining PostgreSQL roles for the pool users or
requested roles, which conflicts with the runbook's "lane user must have no
PostgreSQL role" posture unless the RFC changes that posture explicitly. It also
still does not deny a pool OS uid over TCP when the lane requests some other
database role that is allowed by a broader HBA rule; HBA cannot match TCP
clients by Unix uid.

The third rebuttal is that A13 will run `make lane-isolation-check` for every
pool uid. That test must be strengthened. If it only runs `psql` as each pool
uid with the default requested DB user, it can pass because no same-named DB
role exists, while still missing the actual bypass: `psql -U
<daemon-or-test-role>` over loopback with a seeded password or permissive broader
rule.

### Required Revision

Revise OQ4/P0 so PostgreSQL isolation is expressed as a source-verified posture,
not as an OS-group HBA shorthand:

1. Remove `local all %striatum-lanes reject` and the matching host rows from the
   spec unless PostgreSQL support for that exact OS-group meaning is proven. The
   current PostgreSQL 17 grammar does not provide it.
2. Decide the real pool posture. Viable options include explicit generated
   per-pool-user HBA reject rows for the lane DB usernames plus a strict "no
   pool user PostgreSQL role/password" invariant; a PostgreSQL-role-group design
   using `+role` with the blast radius and role lifecycle spelled out; or a
   stronger socket/listener posture where the daemon database is reachable only
   through a protected Unix socket not accessible to pool uids and loopback TCP
   is not an accepted lane path.
3. State separately what protects local peer and loopback TCP. Local peer can
   observe the OS user during authentication; loopback TCP cannot identify the
   client Unix uid, so denying "every pool OS uid" over TCP cannot rest on HBA
   user matching alone.
4. Extend A13 / `make lane-isolation-check` with adversarial probes, not just
   default-user probes: for every pool uid, attempt local peer and loopback TCP
   as (a) the same-named pool DB user, (b) the daemon DB role or a seeded test
   role with a known password, and (c) any configured pg_ident mapped role. The
   expected result is a typed, source-explained denial before the lane can query
   daemon tables.
5. Add a `doctor`/runbook check that validates the actual HBA rows through
   `pg_hba_file_rules` or an equivalent source of truth and fails if the intended
   pool-deny rows are non-matching, malformed, below a broader allow, or only
   covering PostgreSQL role membership when the design claims Unix group
   membership.
6. Keep the environment stripping and no-role/no-password invariant, but treat
   them as defense in depth. The build-bearing security claim must be that a pool
   uid cannot reach PostgreSQL even if a broader host rule or seeded credential
   would otherwise allow a non-lane local client.

## Checks Credited

- I did not find a fresh residual cross-uid tmux, `0600`, ptrace, setns, or
  `/proc` hole beyond the caveats already captured by the holder's A1-A5 tests
  and the earlier falsifier artifacts. The hard-core `BC1-W1-ORACLE` negative
  control is pointed in the right direction.
- The OQ5 generation token is the correct anti-recycle shape for stale
  attestation, assuming the generation is compared on every attestation/control
  path and stale processes are also handled by the scrub/quarantine state
  machine.
- The OQ6 per-uid credential-store plan is plausible only if scrub/quarantine is
  complete and if no DB credential path is accidentally included in provider
  hydration. This challenge is specifically about PostgreSQL isolation, not
  provider OAuth freshness.

## Bottom Line

The holder cannot clear OQ4 by saying PostgreSQL is denied for every pool uid
through `local all %striatum-lanes reject`. PostgreSQL does not interpret that
as a Unix OS-group deny, and even a corrected `+role` rule would be PostgreSQL
role membership rather than OS uid membership. Until the spec names a real
local-peer and loopback-TCP denial posture and tests it with adversarial
requested DB users and seeded credentials, a pool lane can still become a direct
PostgreSQL client if any broader host rule or credential drift exists. That is a
control-plane authority leak, so the ACL/PG part of RFC 0168 P0 has not cleared.
