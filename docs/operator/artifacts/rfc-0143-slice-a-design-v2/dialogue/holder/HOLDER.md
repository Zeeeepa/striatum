# HOLDER (v2 / REVISION) — RFC 0143 **Slice A** falsifiable implementation spec: the daemon-observed `session_unrecoverable_across_rotation` typed-exit floor

author: holder-author-001

> **This is the v2 REVISION of the Slice-A design.** The v1 run
> (`rfc-0143-slice-a-design`) returned `needs_revision` on two findings —
> `SA-ROTATION-UNDERFIRE` (the floor never fired on the real #512 rotation path) and
> `SA-C2-TMUX-FORGE` (the tmux `#{pane_dead_status}` carrier is forgeable by a same-uid
> sibling). This spec **revises** the v1 `HOLDER.md`, carrying its credited skeleton
> forward unregressed and fixing exactly those two gaps. The structural root of both is
> the v7 `BC1-W1-ORACLE`: **under the shared `striatum-lane` uid every lane-side signal
> (an exit code, a `0600` file, the tmux pane status) is forgeable by a same-uid
> sibling.** The v1 spec leaned on a **lane-side** reserved exit code as the primary
> floor producer, which both (a) is unreachable on the real session-bound rotation path
> and (b) is forgeable on the tmux carrier. **The v2 fix direction is to make a
> DAEMON-SIDE observation the primary, forge-resistant producer and demote the
> lane-side exit code to a direct-path corroborator.**
>
> **Scope (D261, unchanged).** Slice A ships now as **PURE daemon-side observability** —
> the Option-4 typed `session_unrecoverable_across_rotation` floor that **mints no
> credential, widens no token, and touches no trust model.** Slice B (the
> `CapabilityReseal` authority + the W1 connect-out channel) is **OUT OF SCOPE, blocked
> on [RFC 0168](../../../../../rfcs/0168-per-lane-security-principal.md) (#585).** This
> spec designs **none** of Slice B. The deliverable is the falsifiable implementation
> spec the `rfc-0143-slice-a-build` run executes contract-first (TDD): concrete
> file:line anchors (re-verified against current `main`), named Go tests, and a
> mechanically-derived classification.
>
> **Observability-only premise (load-bearing for FIX-2's residual).** Slice A is a
> **classification refinement** of the existing `agent_exited_unsealed` recovery class.
> It grants **NO new auto-seal authority**: the typed floor routes the **same**
> finalize-or-escalate path `agent_exited_unsealed` already takes, only with a distinct,
> legible reason. A lane still requires an operator requeue (or Slice B, later) to seal
> a complete-but-unpublished deliverable. So a *forged* typed class is **no more
> privileged than a forged `agent_exited_unsealed`** — which a same-uid child can
> already cause by killing its own lane (§5.4).

---

## §0 — Addressing the v1 findings (auditable resolution map)

Read this table first: it states, per finding and per credited carry-forward item,
exactly how v2 resolves or preserves it and the source mechanism a falsifier verifies.

| v1 finding / item | v1 status | v2 disposition | Where |
|---|---|---|---|
| **SA-ROTATION-UNDERFIRE** (the floor never fires on the real session-bound #512 rotation path) | OPEN, verdict-driving | **RESOLVED.** A new **daemon-side** producer **T1** records a durable `daemon.stale_epoch_rotation` observation when `validateBootEpoch` rejects a request as `stale_daemon_identity` (`http.go:166-169`,`:681-700`); the recovery sweep records the typed class for an owning session that was observed presenting a stale epoch **and** is lane-lost **and** has its required artifacts complete-on-disk. This fires on the exact #512 path **regardless of token source or exit code** and does **not** require the lane to reach step-3 of the resolver. The **pre-auth attribution sub-question is resolved** in §3.2. The codex wedge is routed (§3.4). | §3 |
| **SA-C2-TMUX-FORGE** (the tmux `#{pane_dead_status}` carrier is forgeable) | OPEN, verdict-driving | **RESOLVED.** The **trusted** carriers are forge-resistant: T1 (the daemon's own observation) and, on the direct path, the wrapper's own `agent_exited.exit_code` (`helper.go:433`→`supervision.go:425`). The tmux `#{pane_dead_status}==97` carrier is **NOT claimed forge-resistant**: it records the typed class **only when corroborated** by a forge-resistant signal (T1 for the owning session), else it is honestly scoped **RFC-0168-bounded** best-effort. New negative `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation`. | §4.4, §5 |
| **OBSERVABILITY-ONLY clarification** | requested | **STATED.** The typed floor grants no new auto-seal authority; its routing is held no-more-privileged than `agent_exited_unsealed` (§5.3–5.4). | §5.3, §5.4 |
| **§1 reserved code 97 + sentinel** (`SA-…`) | accepted | **CARRIED unregressed.** `ExitUnrecoverableAcrossRotation = 97`, `ErrUnrecoverableAcrossRotation`. | §1 |
| **§2 Spot-1 credential-chain NARROWING** (A1/A4) | accepted | **CARRIED unregressed**, with the explicit note that it is **NOT** the central producer (T1 is). Refuse-before-read in both resolvers; owner-mode guard retained. | §2 |
| **§3.2–3.4 exact-code-only classification** (A2) | accepted | **CARRIED.** The exact-code (`==97`) gate and FIRST interposition order are retained as producer **T2**; T1 is added alongside, both exact-attribution. | §4.2–4.3 |
| **§3.5 launch-handshake dissolution** | accepted | **CARRIED verbatim.** 97 is produced only after `agent_started`, so a genuine launch failure stays a raw `helper_error`. | §4.5 |
| **§3.6 relationship to `agent_exited_unsealed` + #292** | accepted (HOLDS) | **CARRIED**, strengthened by the observability-only no-new-authority statement. | §5.1–5.3 |
| **§4 direct-path C2** (A5) | accepted (`SA-C2-DIRECT-PATH`) | **CARRIED.** `normalizeAgentExitError` (`loop.go:371-379`) keeps a provider child's 97/98 from driving the reserved code; now framed as the forge-resistant **T2** carrier on the direct path, insufficient alone for the tmux carrier. | §6 |
| **no-widening invariant + additive `isNecrosisStallClass` growth** | accepted | **CARRIED.** No path widens admin-token exposure; the new class is an additive necrosis member. | §0 table below, §4.3 |

**Every HARD CONSTRAINT, verified.** (Falsifiers can check each against the right-column mechanism.)

| HARD CONSTRAINT | How v2 honors it |
|---|---|
| **1 — No token widening** (`reject` if violated) | T1's attribution **reads** the daemon's own token store to **identify** the bound session (`auth_pg.go:73-117` shape, read-only); it grants **no** capability — the request is still rejected. Spot 1 still **narrows** (refuse-before-read). No path adds a read of the admin runtime `client-token`; no minted credential carries `{admin, apply, recovery, surgical_recovery}`. |
| **2 — No new credential / no Slice B** | Only an exit code (97), a recovery class, a daemon-observation event row, and an additive tmux probe field. No `CapabilityReseal`, connect-out channel, kernel-token capture, reseal-token file, reseal-98, `resealInFlightJob`, or owner bundle 0021. |
| **3 — Daemon-side / process state + the daemon's own observation only** | T1 is the daemon observing **its own** `stale_daemon_identity` rejection (not an inbound authenticated frame). All recovery predicates read durable daemon state: `striatumd.events`, `verifyRequiredArtifacts` (`mutations.go:828`), `verifyRequiredArtifactReconstructable` (`artifact_reconstructability.go:65`), `/proc`+`kill(0)`+tmux liveness. |
| **4 — No over-fire / no raw-error leak / no silent rotation exit** | The floor FIRES on the real #512 path (T1). It does **not** fire on: an unattributable rejection (§3.2 — no observation recorded), a legitimately-relaunched lane (not lane-lost; the observation keys on the **current owning session**, §3.5), a healthy lane (not lane-lost), an ordinary unsealed exit (no observation, exit ≠ 97), or a bare same-uid `respawn-pane … exit 97` (no forge-resistant corroboration, §4.4). The launch-handshake raw-leak stays structurally absent (§4.5). |
| **5 — Default-off / additive** | New file `exitcodes.go`; a refusal branch ahead of an existing read; a new daemon event + stall class; one additive tmux probe field; one additive necrosis-domain member. The single disclosed existing-test change is `TestNecrosisDomainMatchesConfirmedDeadConstants` (additive domain growth). |
| **6 — Product-boundary-safe** | No hosted service, no durable transcript, no external persistence. State is the existing daemon PostgreSQL + local process/tmux observation. |

---

## §1 — The reserved agentloop exit code (Slice A owns ONLY this) — CARRIED from v1 §1

**New file `go/pkg/agentloop/exitcodes.go`:**

```go
package agentloop

import "errors"

// ExitUnrecoverableAcrossRotation is the reserved agentloop process exit code for
// the RFC 0143 Slice A floor (D261, Option 4). The supervised lane wrapper exits
// with this code when, ON ITS OWN MCP CLIENT PATH, it observes the daemon rejecting
// it as stale_daemon_identity across a boot-epoch rotation (#512), OR when its
// credential-resolution chain would fall through to the owner-only admin runtime
// client-token for a NON-OWNER lane. It is a DIRECT-PATH CORROBORATOR of the typed
// floor, not its primary producer: the forge-resistant primary producer is the
// daemon's own observation of the rejection (§3). The daemon reads the code from
// durable process state (agent_exited.exit_code on the direct path) and routes the
// typed session_unrecoverable_across_rotation recovery class.
//
// Slice A owns ONLY this floor code. The reseal-request code (98), resealInFlightJob,
// the connect-out channel, the kernel-token capture, the CapabilityReseal authority,
// and owner bundle 0021 are Slice B (RFC 0168 / #585) — NOT defined here.
const ExitUnrecoverableAcrossRotation = 97

// ErrUnrecoverableAcrossRotation is the typed sentinel the lane maps — and ONLY it —
// to a clean exit with ExitUnrecoverableAcrossRotation. It is returned by the
// credential resolver (NEVER a token, NEVER a read of the admin file) when a
// non-owner lane's chain reaches the owner-only admin runtime client-token, AND by
// the rotation watcher when the lane's own MCP client observes a stale_daemon_identity
// rejection (§3.4). An owner process never receives the resolver sentinel.
var ErrUnrecoverableAcrossRotation = errors.New("agentloop: session unrecoverable across daemon boot-epoch rotation")
```

`97` is well outside the 0/1/2 range adapters use for success/error/usage and is distinct
from any Slice-B code (reseal-request `98` is **not** introduced here). `97` is a process
status, never a credential.

---

## §2 — Spot 1: credential-chain narrowing — CARRIED from v1 §2 (NOT the central producer)

Spot 1 stays exactly as v1 credited it; it is a **narrowing**, never a widening. **It is
retained but is NOT the rotation-path producer** — the central producer is now T1 (§3).
Spot 1 still legibly handles the distinct case of a **non-owner** lane whose chain reaches
the admin runtime `client-token` (e.g. a misconfigured lane, or the CLI fallback path).

### 2.1 The two resolver sites (re-verified against `main`)

- **`ResolveTokenMaterial`** (`go/pkg/agentloop/token.go:18-53`): step 1 = `STRIATUM_MCP_TOKEN`
  env literal (`:19-21`); step 2 = `STRIATUM_MCP_TOKEN_FILE` (`:23-29`); **step 3 = the
  runtime `client-token`** (`:31-42`); step 4 = the repo `.striatum/capability_token`
  (`:44-52`).
- **`ResolveTokenMaterialFresh`** (`go/pkg/agentloop/endpoint.go:117-138`): the #323
  rotation-recovery reader; re-reads `STRIATUM_MCP_TOKEN_FILE` (`:119-124`) then the
  runtime `client-token` (`:125-136`).
- The runtime `client-token` is the daemon's full-authority bootstrap admin token
  (`bootstrapCapabilities = {admin, read, write, claim, review, apply, recovery,
  surgical_recovery}`, `go/pkg/admin/bootstrap.go`), written `0600` in a `0700`
  owner-only dir.
- `ReadTokenFile` (`token.go:75-92`) rejects any non-owner-mode file (`mode&0077 != 0`,
  `:82`) but does not check owner **identity**; for a non-owner lane the OS denies the
  `0600` read with `EACCES` (the misleading "permission denied" dead-end).

### 2.2 The narrowing (refuse — never a read)

Add an unexported predicate to `token.go`, applied at the runtime-`client-token` tier in
**both** resolvers **before** the existing read:

```go
// adminTokenReachedByNonOwner reports whether the resolution chain has reached the
// owner-only admin runtime client-token while the current process is NOT its owner.
// It NEVER reads the token contents. Detection is local process state only:
//   - the file exists and its owner uid != this process euid           -> refuse; or
//   - stat is denied with EACCES/EPERM on the file or its 0700 parent  -> refuse.
// A missing file (ENOENT) is NOT the floor (fall through to the next tier, unchanged).
// When the file owner uid == euid (the OWNER/operator process) it returns false and
// the caller reads the token exactly as today (owner unaffected).
func adminTokenReachedByNonOwner(path string) bool // os.Lstat + Stat_t.Uid vs Geteuid; EACCES/EPERM => true; ENOENT => false
```

Wiring at `token.go:31-42` and the structurally-identical tier at `endpoint.go:125-136`:
when the chain reaches the runtime `client-token` tier and `adminTokenReachedByNonOwner(path)`
is true, **return `("", ErrUnrecoverableAcrossRotation)`** instead of reading. Otherwise
byte-for-byte unchanged (owner reads normally; ENOENT falls through to step 4). This is a
**narrowing**: it removes a step for a non-owner; it adds **no** read path. `ReadTokenFile`'s
owner-mode rejection (`:82`) is retained (A4).

### 2.3 Caller mapping → clean reserved-code exit

- **Startup / in-process harness (`loop.go:24` `Run`, `:67` `RunContext`).** The sentinel
  propagates; the agent-loop entrypoint maps **only** it to `97`:

  **`go/cmd/striatumd/main.go:114-117`** (the `-agent-loop` subcommand) changes from a
  blanket `log.Fatalf` to:
  ```go
  if err := agentloop.Run(socketPath, repoRoot, runID, sessionID, flag.Args()); err != nil {
      if errors.Is(err, agentloop.ErrUnrecoverableAcrossRotation) {
          log.Printf("agent-loop: %v", err)
          os.Exit(agentloop.ExitUnrecoverableAcrossRotation) // 97 — reserved floor
      }
      log.Fatalf("agent-loop failed: %v", err) // unchanged: any other error -> exit 1
  }
  os.Exit(0)
  ```
  This is the **only** place the wrapper emits `97`, and **only** for the sentinel (C2, §6).

*(Why this alone under-fired in v1, and is now demoted: a normal supervised lane launches
with its session-bound token as `STRIATUM_MCP_TOKEN` (`supervision_env.go:341-343`), so
`ResolveTokenMaterial` returns at step 1 (`token.go:19-21`) and never reaches step 3 — so
Spot 1's sentinel is never produced for the ordinary rotation lock-out. T1 (§3) is the
reachable producer; Spot 1 is retained for the genuine non-owner-admin-token-reach case.)*

---

## §3 — FIX-1: the daemon-observed stale-epoch rejection is the forge-resistant primary producer (T1)

This is the new central mechanism. It makes the typed floor FIRE on the real #512 path —
a session-bound lane carrying `STRIATUM_MCP_TOKEN` that presents a **stale boot epoch** the
daemon rejects as `stale_daemon_identity` — **without** depending on the lane reaching a
particular resolver tier or emitting any exit code, and **forge-resistant** because the
trigger is the daemon's own observation of its own rejection, not a lane-side signal.

### 3.1 The real #512 path (re-verified against `main`)

After #316 a still-valid bearer is **not** the whole identity. The lane env carries
`STRIATUM_MCP_BOOT_EPOCH` (`supervision_env.go:344-354`), echoed by the lane's MCP client as
the `X-Striatum-Boot-Epoch` header (`mcp.HeaderBootEpoch`, `http.go:63`). On a mid-run
daemon restart the new daemon mints a fresh boot epoch (`daemonBootEpoch()`,
`main.go:733-738`; wired as `MCPBootEpoch`, `main.go:335`). The surviving lane (#141) still
presents the **launch-time** epoch. `ServeHTTP` runs `validateBootEpoch(r)` **before**
bearer validation and before dispatch (`http.go:159-169`); for a presented epoch that
disagrees with the live one it returns the distinct `stale_daemon_identity` localRequestError
(`http.go:681-700`, literal `:697`) and the request is rejected `403` (`http.go:166-169`).

The lane cannot self-heal: the #323 watcher re-resolves the endpoint from owner-only files a
non-owner lane often cannot read and **silently continues** (`loop.go:589-594`); codex cannot
reload its launch-time `-c` MCP url and the wedge path only writes an in-PTY prompt then
returns `nil` (`loop.go:625-646`). So the lane dies on a dead/refusing endpoint with **no 97**
— which is exactly why v1 under-fired. T1 closes this by making the **daemon's rejection** the
producer.

### 3.2 Pre-auth session attribution — the sub-question, resolved concretely

`validateBootEpoch` runs **pre-auth** (`http.go:159-169`), so we must attribute the rejection
to a session **without** authorizing the request and **without** widening any token.

**The attribution.** The rejected request carries the lane's **session-bound bearer** in the
`Authorization` header (on the rotation path the lane still holds a valid session-bound token;
its bearer is present even though the request is refused for the stale epoch). The
session-bound token is daemon-minted: `mintSessionBoundToken` writes a `striatumd.clients` row
(`token_id`, `token_hash = hmac(salt, secret)`) and `striatumd.client_capabilities` rows that
each carry `(repository_id, session_id)` (`go/pkg/mutations/session_token.go:60-99`). The
daemon already resolves a presented bearer to its bound session in `PostgresAuthorizer.Authorize`
(`go/pkg/rpc/auth_pg.go:49-157`): split `token_id.secret` (`:60`), look up `striatumd.clients`
by `token_id` (`:73-76`), constant-time-verify `hmac(salt, secret)` against `token_hash`
(`:83-84`), reject revoked/expired (`:87-92`), and read the **bound `session_id`** from
`client_capabilities` (`:104-117`, surfaced as `AuthContext.SessionID` `:149-156`).

**Add a read-only identity resolver** reusing that exact store, granting nothing:

```go
// IdentifyBoundSession verifies the presented bearer is a genuine daemon-minted,
// non-revoked, non-expired token and returns the (repositoryID, sessionID) bound into
// its narrowest session-scoped capability grant. It authorizes NOTHING — no capability
// is checked or granted; it is a pure identity read of the daemon's own token store
// (the same striatumd.clients + client_capabilities rows Authorize reads, auth_pg.go:73-117).
// ok=false when the bearer is absent, malformed, fails the HMAC, is revoked/expired, or
// carries no session binding (a session-unbound operator/coordinator grant). On ok=false
// the rejection is UNATTRIBUTABLE and NOTHING is recorded (no over-fire).
func (a *PostgresAuthorizer) IdentifyBoundSession(token string) (repositoryID, sessionID string, ok bool)
```

This is a **narrowing-safe identity read**, not an authorization: the request stays rejected;
the daemon merely consults its own store to learn *which session* the rejected lane belongs to.
**No token is widened** — nothing reads the admin runtime `client-token`, and no capability is
granted to the stale request.

**Forge-resistance of the attribution.** The session binding is established by the daemon at
mint time via an HMAC over a daemon-held salt; a process cannot fabricate a token bound to a
session it does not own. The only residual is the shared-uid one: a same-uid sibling could read
**another** lane's `STRIATUM_MCP_TOKEN` from `/proc/<pid>/environ` and present it to attribute a
stale-epoch observation to that session. This is the **same RFC-0168-bounded shared-uid oracle**
(`BC1-W1-ORACLE`) that makes Slice B unsolvable, and it is **observability-only bounded**: the
worst case is a misclassified recovery *reason* for a session that must **also** be confirmed
lane-lost with a complete-on-disk deliverable (§3.5) — never a seal, never a privilege (§5.4).
We **do not** claim T1 is forge-resistant against a same-uid sibling that can already read a
victim lane's env; we claim it is forge-resistant against any **unprivileged** (non-same-uid)
party, and that the shared-uid residual is RFC-0168-bounded and observability-only.

### 3.3 Recording the daemon observation (durable daemon-side state)

On the `validateBootEpoch` rejection branch in `ServeHTTP` (`http.go:166-169`), before writing
the `403`, attribute and record:

```go
if epochErr := h.validateBootEpoch(r); epochErr != nil {
    if repoID, sessID, ok := h.Service.IdentifyBoundSession(bearerRaw(r)); ok {
        // daemon-side, best-effort, idempotent; failure to record must NOT change the 403.
        _ = h.Service.RecordStaleEpochRejection(r.Context(), repoID, sessID)
    }
    writeJSONResponseStatus(w, http.StatusForbidden, errorResponse(nil, jsonrpcForbidden, epochErr.Message, errorData(epochErr.Code, nil)))
    return
}
```

- `bearerRaw(r)` extracts the raw bearer string (reusing the parse in `bearerToken`,
  `http.go:518-528`) without requiring it to be valid-for-authz.
- `Service.IdentifyBoundSession` is the read-only resolver (§3.2); `ok=false` ⇒ record nothing.
- `Service.RecordStaleEpochRejection(ctx, repositoryID, sessionID)` is a **new daemon mutation**
  that `appendEvent`s (`mutations.go:1706`) a durable `daemon.stale_epoch_rotation` row into
  `striatumd.events` keyed on `(repository_id, run_id, session_id)` for that session's run.
  It is **idempotent** (one standing observation per session is sufficient) and **best-effort**:
  a record failure never alters the `403` (the #316 protection is unchanged). This is the
  daemon's own observation of its own rejection — **no inbound authenticated frame** (HARD
  CONSTRAINT 3).

The `Service` interface (`go/pkg/mcp/tools.go`) gains the two read/record methods; the daemon
wiring backs them with `PostgresAuthorizer.IdentifyBoundSession` and the new mutation. Unit-test
handlers (no `Service.BootEpoch`) never reach the branch (`validateBootEpoch` no-ops when the
daemon holds no live epoch, `http.go:683-685`), so existing MCP HTTP tests are unaffected.

### 3.4 Route the codex wedge + the watcher to the floor (lane-side corroborator / fallback)

T1 fires regardless of what the lane does. We **additionally** route the lane-side paths so the
direct-path lane gets a forge-resistant **corroborator** (its own `agent_exited.exit_code==97`):

- **Codex wedge (`loop.go:625-646`).** Today `applyMCPEndpointRotation` for codex writes an
  in-PTY prompt and `return nil`. Change: when the codex wedge fires for a rotated/dead
  endpoint, the lane has provably lost its MCP path; map the condition to
  `ErrUnrecoverableAcrossRotation` so `runWithIO` returns it and `main.go:114-117` exits `97`
  (instead of `return nil` continuing on a dead endpoint). Test:
  `TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable`.
- **Watcher (`loop.go:572-614`).** When the lane's own MCP client (the daemon-receiver loop, or
  a reconnect attempt) observes a `stale_daemon_identity` response on its own client path,
  request the unrecoverable exit via a new `requestUnrecoverableExit()` callback — the structural
  twin of the existing idle-exit callback (`loop.go:328-334`: set an `atomic.Bool`, `SIGTERM`
  the child) — so `runWithIO` returns `ErrUnrecoverableAcrossRotation` (mapped to `97`), instead
  of the current silent-continue (`loop.go:589-594`). This is the **honestly-scoped lane-side
  fallback** the SEED permits: it maps **only** the daemon's `stale_daemon_identity` response
  (not an ordinary network/non-epoch MCP error) to the floor. Tests:
  `TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane` /
  `TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor`.

So on the **direct** path the typed class is carried by both T1 (daemon observation) and the
forge-resistant `agent_exited.exit_code==97` (T2, §4) — corroborating. On the **tmux** path the
lane's `97` lives only in the forgeable `#{pane_dead_status}`, so the trusted carrier there is
**T1 alone** (§4.4).

### 3.5 No over-fire (the negatives)

- **Unattributable rejection** (`IdentifyBoundSession` ok=false) ⇒ no observation ⇒ no class.
- **Legitimately-relaunched lane.** After an operator requeue the job's **owning session** is the
  new session; T1's observation is keyed on `session_id`, and the recovery predicate (§4.2) keys
  on the job's **current owning session**, so a stale observation for the old/closed session never
  drives the new session's job. And a relaunched lane is **not lane-lost** (live pane).
- **Healthy lane** ⇒ not lane-lost ⇒ no class.
- **Ordinary unsealed exit** (no stale-epoch observation, exit ≠ 97) ⇒ stays
  `agent_exited_unsealed` (`TestOrdinaryUnsealedExitStaysAgentExitedUnsealed`, kept).

---

## §4 — Spot 2: daemon observation + typed-class recovery routing (FIX-2)

### 4.1 Observing the carriers from durable state — three signals, two trusted

The supervised tmux pane runs the agent-loop wrapper itself, and `remain-on-exit on` is set
**before** the lane command runs (`pty.go:459`), so a dead pane stays queryable.

1. **T1 — daemon-observed stale-epoch rejection (TRUSTED, forge-resistant).** The
   `daemon.stale_epoch_rotation` event for the owning session (§3.3). The primary rotation-path
   carrier; uses no tmux signal at all.
2. **T2-direct — `agent_exited.exit_code` (TRUSTED, forge-resistant).** On the direct (plain-PTY)
   path `RunHelper` reaps the child and emits `agent_exited` with `exit_code` from
   `processExitCode(result.Cmd.Wait())` (`helper.go:427-439`,`:433-434`,`:499-507`); the daemon
   keeps `exit_code` durably (`curatedSuperviseReportPayload` `supervision.go:412-443`,`:425`).
   This is the wrapper's **own** exit status — a same-uid child cannot set it on the wrapper's
   `Cmd.Wait` (§6).
3. **T2-tmux — `#{pane_dead_status}` (NOT trusted on its own; see §4.4).** On the tmux path
   `result.Cmd` is the attach client, so the pane wrapper's `97` lives in tmux
   `#{pane_dead_status}`. Extend the `display-message` capture in `ProbeTmuxLiveness`
   (`tmux_liveness.go:228`) from `#{pane_id}|#{pane_pid}|#{pane_dead}|#{pane_start_time}` to
   additionally request `#{pane_dead_status}`, parse into a new additive
   `TmuxLiveness.PaneDeadStatus *int` (populated only when `pane_dead==1`), surfaced via
   `LaneLiveness`. The existing pane-dead/pid-mismatch/start-mismatch/healthy classification is
   unchanged.

### 4.2 The new typed stall class + exact-attribution predicate

Add to `go/pkg/mutations/recovery_decision_tree.go` (alongside `:176-185`):
```go
// stallClassSessionUnrecoverableAcrossRotation — a confirmed-dead supervised agent
// whose owning session was locked out of resealing across a daemon boot-epoch rotation
// (RFC 0143 Slice A / #512). A strict refinement of agent_exited_unsealed for the
// rotation case; it routes the SAME finalize-or-escalate path with a distinct, legible
// reason and remediation. It is exact-attribution: it refuses to infer the floor from
// complete-on-disk + lane-lost ALONE.
stallClassSessionUnrecoverableAcrossRotation = "session_unrecoverable_across_rotation"
```

```go
// deadAgentUnrecoverableAcrossRotation reports whether a confirmed-dead lane's owning
// session is unrecoverable across a boot-epoch rotation, from durable daemon-side state
// ONLY, via EITHER trusted carrier (exact attribution, no over-fire):
//   T1 (forge-resistant, primary): a daemon.stale_epoch_rotation observation exists for
//      this run's owning session (the daemon's own validateBootEpoch rejection, §3.3); OR
//   T2-direct (forge-resistant): the latest supervisor.agent_exited event exit_code for the
//      owning supervisor is exactly 97 (the wrapper's own Cmd.Wait); OR
//   T2-tmux (RFC-0168-bounded, CORROBORATED ONLY): PaneDeadStatus == 97 AND a T1 observation
//      exists for the owning session — a bare #{pane_dead_status}==97 with no T1 corroboration
//      returns false (§4.4).
// Returns false for any other state. No inbound frame.
func deadAgentUnrecoverableAcrossRotation(ctx context.Context, tx db.TxRunner, repositoryID, runID string, row map[string]any) bool
```

### 4.3 Wiring into `recoverStuckJobs` — additive, interposed FIRST

The dead-lane classification today (`recovery_decision_tree.go:1136-1140`) is:
```go
if deadAgentExitedUnsealed(activity) { stallClass = stallClassAgentExitedUnsealed } else { stallClass = stallClassAgentPIDDead }
```
Slice A interposes the typed check **first**, leaving both existing branches intact:
```go
if deadAgentUnrecoverableAcrossRotation(ctx, tx, repositoryID, runID, row) {
    stallClass = stallClassSessionUnrecoverableAcrossRotation
} else if deadAgentExitedUnsealed(activity) {
    stallClass = stallClassAgentExitedUnsealed
} else {
    stallClass = stallClassAgentPIDDead
}
```
The same typed gate is applied at the auto-finalize-unsealed branch (`:958-1027`, gated on
`deadAgentExitedUnsealed(activity)` `:959`): a rotation-locked lane with a complete-on-disk,
reconstructable deliverable is finalized via the **same** `tryFinalizeUnsealedFromDurableArtifact`
(`:960`,`:1751`) + `closeStalledOwningSession` (`:969`,`:1654`) path as `agent_exited_unsealed`,
only **tagged with the typed class** (so it is **no more privileged** — §5.3). When there is **no**
durable deliverable, it escalates with the distinct "session unrecoverable across rotation —
operator requeue required" reason instead of a silent unsealed exit.

`isNecrosisStallClass` (`:196-197`) gains the new member (a confirmed-dead class):
```go
return stallClass == stallClassAgentPIDDead ||
    stallClass == stallClassAgentExitedUnsealed ||
    stallClass == stallClassSessionUnrecoverableAcrossRotation
```
**Disclosed test change (the only one):** `TestNecrosisDomainMatchesConfirmedDeadConstants` pins
the necrosis domain to `{agent_pid_dead, agent_exited_unsealed, recovery_exhausted}` (`:191-192`);
it is **extended** to admit `session_unrecoverable_across_rotation` — an additive domain growth,
not a meaning change to any existing member (A6).

### 4.4 FIX-2: the tmux carrier is honestly scoped, not claimed forge-resistant

The v1 over-fire was that a same-uid provider child can `tmux respawn-pane -k -t <pane> -- sh -c
'exit 97'` (the production tmux surface runs as the shared `striatum-lane` uid — `pty.go:98-112`
`sudo -n -u <RunAsUser> -- env -i`, `:310-314` `RunAsTmuxRunner`; deterministic session name
`:418`; `respawn-pane -k` `:479-484`; `remain-on-exit` `:459`), driving `#{pane_dead_status}==97`
and forging the typed class. v2 closes it:

1. **The trusted carriers are forge-resistant:** T1 (the daemon's own observation) and T2-direct
   (the wrapper's own `agent_exited.exit_code`). The central rotation deliverable rides T1, which
   uses **no** tmux signal — so the production-lane floor is forge-resistant.
2. **`#{pane_dead_status}==97` is NOT trusted alone.** `deadAgentUnrecoverableAcrossRotation`
   records the typed class from `PaneDeadStatus==97` **only when corroborated** by a T1
   observation for the owning session. A bare same-uid respawn produces no T1 observation (the
   daemon never rejected that session for a stale epoch) and no direct-path `agent_exited.exit_code
   ==97` (the attach client's exit, not the wrapper's), so it **stays `agent_exited_unsealed`** —
   no forge. Under the shared uid, an uncorroborated `#{pane_dead_status}` is **RFC-0168-bounded**
   best-effort legibility, not a trust anchor; we **do not** claim it forge-resistant.
3. **Negative test `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation`:** a same-uid
   `respawn-pane … exit 97` with **no** T1 observation and **no** direct-path `agent_exited.exit_code
   ==97` asserts recovery does **NOT** record the typed class. **Keep**
   `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` (direct path), treating it as
   insufficient alone.

### 4.5 The launch-handshake path is NOT a floor carrier — CARRIED from v1 §3.5

Slice A has **no** capture boundary at launch (no W1, no kernel-token capture). The reserved code
is produced by the wrapper **after** `RunHelper` emits `agent_started`
(`HelperEventAgentStarted`, `helper_protocol.go:15`; emitted after `helperLaunch`,
`helper.go:161`, before the `result.Cmd.Wait()` reap goroutine `:197`), and
`waitForHelperAgentStart` returns success on the first `agent_started` it reads, which always
**precedes** any `agent_exited`. So the launch/attach `helper_error` phase `launch` path requires
**no** change and correctly stays a **raw** `helper_error` for genuine launch failures (exec /
tmux setup failure) — which are **not** the floor. The raw-error-leak path is **structurally
absent** (dissolving the v7 `BC1-W1-CAPTURE-FLOOR`), and a genuine launch failure is never
reclassified (`TestLaunchHandshakeFailureStaysHelperErrorNotFloor`, kept).

---

## §5 — Relationship to `agent_exited_unsealed` / #292, and the observability-only clarification

### 5.1 vs `agent_exited_unsealed`

The typed floor is a **strict refinement** for the rotation case. It does **not** change
`deadAgentExitedUnsealed` (`recovery_decision_tree.go:1916`) or the generic class; a
rotation-locked lane is simply tagged with the more precise class **before** the generic test
runs (§4.3).

### 5.2 vs `HandleRecoveryCompleteStalled` (#292, `recovery_complete_stalled.go:49`)

The floor **routes to / does not duplicate** it. When the lane's required artifacts are
complete-on-disk and reconstructable (`verifyRequiredArtifacts` `recovery_complete_stalled.go:180`
/ `mutations.go:828`; `verifyRequiredArtifactReconstructable` `recovery_complete_stalled.go:187` /
`artifact_reconstructability.go:65`), the typed-class path finalizes via the same
`finalizeStalledJob` (`:275`). When there is no durable deliverable, it escalates with the
distinct legible reason ("operator requeue required — the supported #512 recovery"), leaving the
existing #292 operator verb (`recovery complete-stalled`) available exactly as today. It never
**overrides** #292's verdict-capable refusal or liveness guard.

### 5.3 Observability-only: no new auto-seal authority

The typed floor is a **classification refinement**, not an authority. Its recovery routing is held
**no-more-privileged than `agent_exited_unsealed`**: it takes the **same**
`tryFinalizeUnsealedFromDurableArtifact` path, which only fires when the required-artifact **row**
is published and body-reconstructable. A lane that wrote a deliverable to its worktree but never
**published the artifact row** (the literal #512 `DESIGN.md`-on-disk-but-unsealed case) is **not**
auto-sealed by the typed floor any more than by `agent_exited_unsealed` — it escalates for the
operator requeue. The typed floor adds **no** new finalize path, **no** new seal trigger, **no**
new credential; it only makes the failure **legible**.

### 5.4 The forge residual is bounded by observability-only

Because the floor grants no new authority, a **forged** typed class is **no more privileged than a
forged `agent_exited_unsealed`** — which a same-uid child can already cause by killing its own lane
(the lane then auto-finalizes from its own durable artifact, or escalates). The tmux carrier's
residual forgeability is therefore **not a privilege escalation**: its worst case is an
honest-RFC-0168-bounded misclassified recovery *reason*, never an unwanted seal. (And §4.4 makes
even that require T1 corroboration.) This is why FIX-2's honest scoping is sufficient: the typed
floor cannot be weaponized beyond what the shared uid already permits.

---

## §6 — C2 forge-resistance — CARRIED from v1 §4, extended

The wrapper must never let a **provider child's** exit status drive the reserved floor code. In
`runWithIO` (`loop.go:220-369`) the inner agent CLI's exit is consumed by
`normalizeAgentExitError` (`:368`,`:371-379`), which returns a **generic** `"agent command exited"`
error — it does **not** propagate the child's numeric code. The reserved `97` is emitted **only**
by `main.go:114-117` and **only** for `errors.Is(err, ErrUnrecoverableAcrossRotation)`. A provider
child that exits `97`/`98` therefore produces a generic error → `log.Fatalf` → wrapper exit `1`,
**never** `97`, and never the sentinel. This makes T2-direct (`agent_exited.exit_code`)
forge-resistant on the direct path. **Test:**
`TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` (direct path) — **insufficient
alone**, paired with `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation` (§4.4) for
the tmux carrier.

---

## §7 — Falsifiable assertions (each paired with its named test)

- **A1 (Spot 1, narrowing + owner-unaffected).** A non-owner lane whose chain reaches the admin
  runtime `client-token` gets `ErrUnrecoverableAcrossRotation` → exit `97`, not a generic
  permission error and not a silent unsealed exit; an owner process resolves the admin token
  normally. **Tests:** `TestResolveRefusesRuntimeClientTokenForLane` (non-owner: stat-mismatch +
  EACCES → sentinel) and `TestResolveAdminTokenUnaffectedForOwner` (owner → normal read), for
  **both** `ResolveTokenMaterial` and `ResolveTokenMaterialFresh`; plus
  `TestAgentLoopMapsUnrecoverableSentinelToReservedExit` (the `main.go` mapping).
- **A2 (FIX-1, daemon-observed primary producer FIRES on the real rotation path).** A session-bound
  lane carrying `STRIATUM_MCP_TOKEN` that presents a stale boot epoch the daemon rejects as
  `stale_daemon_identity` (a) records a `daemon.stale_epoch_rotation` observation for its bound
  session via the pre-auth identity attribution, and (b) when that lane is confirmed dead with a
  complete-on-disk deliverable, the recovery sweep records
  `session_unrecoverable_across_rotation`. **Tests:**
  `TestStaleEpochRejectionRecordsUnrecoverableForPresentingSession` (daemon-side attribution +
  observation), `TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor` (the v1 under-fire is
  gone — the floor fires though `cfg.Token.Source == EnvMCPToken`),
  `TestRecoverySweepClassifiesDaemonObservedStaleEpochAsUnrecoverableAcrossRotation`.
- **A2-route (codex wedge + watcher routed).** **Tests:**
  `TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable`,
  `TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane` /
  `TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor`.
- **A2-attrib (pre-auth attribution, no over-fire, no widening).** An **unattributable** stale-epoch
  rejection (no/malformed/HMAC-failing/revoked/expired/session-unbound bearer) records **no**
  observation and **no** class; `IdentifyBoundSession` grants no capability and reads no admin
  token. **Tests:** `TestUnattributableStaleEpochRejectionRecordsNothing`,
  `TestIdentifyBoundSessionGrantsNoCapability`.
- **A3 (no over-fire).** An ordinary unsealed exit (complete-on-disk, exit `0`/`1`, no observation)
  stays `agent_exited_unsealed`; a healthy or relaunched lane is not lane-lost; a never-engaged
  crash stays `agent_pid_dead`. **Tests:** `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed`
  (kept), `TestRelaunchedLaneStaleObservationForClosedSessionDoesNotFireForNewSession`.
- **A4 (no widening).** No path reads the admin token from a lane; the resolver still refuses a
  non-owner read; `IdentifyBoundSession` is identity-only; no minted credential carries an elevated
  verb. **Tests:** `TestLaneNeverReadsAdminRuntimeToken`, plus A2-attrib's
  `TestIdentifyBoundSessionGrantsNoCapability`.
- **A5 (FIX-2, forge-resistant trusted carriers; tmux honestly scoped).** The trusted carriers (T1,
  T2-direct) are forge-resistant; the tmux `#{pane_dead_status}` carrier records the typed class
  **only** with T1 corroboration. **Tests:**
  `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation` (bare same-uid respawn → no
  class), `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` (direct path, kept),
  `TestTmuxPaneDeadStatus97WithoutDaemonObservationStaysUnsealed`.
- **A6 (no regression; additive; observability-only).** Existing recovery / supervise / agentloop
  suites pass unchanged; the new exit code, daemon event, stall class, tmux field, and necrosis
  member are additive; the single disclosed change is `TestNecrosisDomainMatchesConfirmedDeadConstants`
  (extended to `{agent_pid_dead, agent_exited_unsealed, session_unrecoverable_across_rotation,
  recovery_exhausted}`). The typed floor's routing is no-more-privileged than
  `agent_exited_unsealed`. **Tests:** the full `go/pkg/{agentloop,supervisor,mutations,mcp,rpc}`
  suites green; `TestTypedFloorRoutingNoMorePrivilegedThanUnsealed` (asserts the typed class takes
  the same finalize-or-escalate path and adds no new seal trigger).

---

## §8 — Build slices (contract-first order, smallest safe first) + Acceptance Criteria

1. **Slice A-1 — reserved code + sentinel (pure, no behavior change).** Add
   `go/pkg/agentloop/exitcodes.go`. *Tests:* compile + constant/identity. No live path touched.
2. **Slice A-2 — Spot 1 narrowing + caller mapping (CARRIED).** Add `adminTokenReachedByNonOwner`;
   wire the refusal at `token.go:31-42` and `endpoint.go:125-136`; map the sentinel in `loop.go`
   (`Run`/`RunContext`) and `main.go:114-117`. *Tests:* **A1, A4, C2-direct.** Files: `token.go`,
   `endpoint.go`, `loop.go`, `cmd/striatumd/main.go`.
3. **Slice A-3 — FIX-1 daemon observation (the central slice).** Add
   `PostgresAuthorizer.IdentifyBoundSession` (read-only, `auth_pg.go`), the `Service`
   `IdentifyBoundSession` + `RecordStaleEpochRejection` methods (`mcp/tools.go`), the
   `RecordStaleEpochRejection` mutation (`appendEvent` `daemon.stale_epoch_rotation`,
   `mutations.go`), and the attribution branch in `ServeHTTP` (`http.go:166-169`). *Tests:* **A2,
   A2-attrib, A4.** Files: `rpc/auth_pg.go`, `mcp/tools.go`, `mcp/http.go`, `mutations/*.go`.
4. **Slice A-4 — route the lane-side corroborator/fallback.** `requestUnrecoverableExit` twin in
   `runWithIO`; codex wedge → sentinel (`loop.go:625-646`); watcher stale_daemon_identity →
   sentinel (`loop.go:572-614`). *Tests:* **A2-route.** Files: `loop.go`.
5. **Slice A-5 — FIX-2 Spot 2 observation + typed-class routing.** Extend `ProbeTmuxLiveness`
   (`tmux_liveness.go:228`) with `PaneDeadStatus`; add `stallClassSessionUnrecoverableAcrossRotation`
   + `deadAgentUnrecoverableAcrossRotation` (T1 / T2-direct / corroborated-T2-tmux); interpose in
   `recoverStuckJobs` (`:1136-1140` and `:958-1027`); extend `isNecrosisStallClass` + the
   necrosis-domain test. *Tests:* **A3, A5, A6.** Files: `tmux_liveness.go`,
   `recovery_decision_tree.go` (+ test files).

**Acceptance Criteria (the build + verify run must meet — the rotation game-day and the forge negative):**

- **GD-1 (rotation game-day, FIX-1).** A **session-bound** supervised lane carrying
  `STRIATUM_MCP_TOKEN` that, after a daemon boot-epoch rotation, presents a **stale boot epoch**
  rejected as `stale_daemon_identity`, is confirmed dead with a complete-on-disk deliverable →
  the run records `session_unrecoverable_across_rotation` (via the daemon's own observation),
  **not** a silent unsealed exit and **not** a "permission denied" dead-end; an unattributable
  rejection records nothing.
- **GD-2 (forge negative + carriers, FIX-2).** A same-uid `tmux respawn-pane … exit 97` with **no**
  daemon-observed rejection and **no** direct-path `agent_exited.exit_code==97` does **NOT** record
  the typed class (stays `agent_exited_unsealed`); the trusted carriers (T1, direct-path
  `agent_exited.exit_code`) do; a genuine launch failure still surfaces a raw `helper_error`; an
  ordinary unsealed exit stays `agent_exited_unsealed`.

---

<sub>Holder spec (v2 / REVISION) for the RFC 0143 **Slice A** falsification-gate design run — the
**daemon-observed** Option-4 `session_unrecoverable_across_rotation` typed-exit floor. FIX-1
(`SA-ROTATION-UNDERFIRE`): a forge-resistant **daemon-side** producer (T1) records a durable
`daemon.stale_epoch_rotation` observation when `validateBootEpoch` rejects a request as
`stale_daemon_identity` (`http.go:166-169`,`:681-700`), attributed to the presenting session by a
**read-only, grant-nothing** bearer→session identity resolution reusing the daemon's own token
store (`auth_pg.go:73-117`); the recovery sweep records the typed class for an owning session
observed presenting a stale epoch + complete-on-disk + lane-lost — firing on the real #512
session-bound path regardless of token source or exit code, with the codex wedge
(`loop.go:625-646`) and watcher routed to the floor. FIX-2 (`SA-C2-TMUX-FORGE`): the trusted
carriers (T1, direct-path `agent_exited.exit_code`) are forge-resistant; the tmux
`#{pane_dead_status}` carrier is **not** claimed forge-resistant — it records the typed class only
when corroborated by T1, else honestly scoped RFC-0168-bounded, with
`TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation`. Slice A is observability-only:
the typed floor grants **no new auto-seal authority** (routing no-more-privileged than
`agent_exited_unsealed`), so a forged class is no more privileged than a forged unsealed exit. The
v1-credited skeleton (§1 reserved code+sentinel / §2 Spot-1 narrowing / §3.2-3.4 exact-code
classification / §3.5 launch dissolution / §3.6 #292 relationship / §4 direct-path C2 /
no-widening) is carried forward unregressed. Slice B (the `CapabilityReseal` authority +
connect-out channel) is OUT OF SCOPE, blocked on RFC 0168 (#585). This is the published claim the
falsifiers re-attack; the adjudicator's collaboration ledger gates the phase.</sub>
