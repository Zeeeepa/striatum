# Interrogation Data Brief — Interrogation Chat Panel (2026-05-25)

Reconnaissance for the feature that will render a run's interrogation Q&A
thread as a chat-style transcript in the workflow-history web UI.

---

## 1. Data model

### Table: `striatumd.interrogations`

Defined in `go/pkg/db/sql/0016_interrogation_sessions.sql`.

| Column | Type | Notes |
|--------|------|-------|
| `repository_id` | text NOT NULL | composite PK part 1 |
| `interrogation_id` | text NOT NULL | composite PK part 2; prefix `intg` |
| `run_id` | text NOT NULL | indexed (`idx_interrogations_run`) |
| `interrogator_session_id` | text NOT NULL | session that opened the interrogation |
| `target_session_id` | text NOT NULL | session being questioned; indexed (`idx_interrogations_target`) |
| `topic` | text (nullable) | optional subject line |
| `state` | text NOT NULL | `'open'` or `'closed'` (CHECK constraint) |
| `turn_count` | integer NOT NULL | bumped on each ask or answer |
| `opened_at` | timestamptz NOT NULL | — |
| `closed_at` | timestamptz (nullable) | set on `interrogation.close` |

**No FK to `striatumd.runs`, `sessions`, or `repositories`** — referential
integrity is enforced in Go, not the DB, to respect the migration-ownership
rule (RFC 0079 §5). `0016_interrogation_sessions.sql:16`

### Turn storage: `striatumd.queue_messages`

Turns are **not a separate table**. They reuse the existing message bus.
Each turn is a row with:
- `kind = 'agent_message'`
- `payload_json` contains the structured turn fields (see below)
- `target_session_id` set to the recipient session

Turn correlation: `payload_json->>'interrogation_id'` links a message to its
interrogation. `mutations/interrogation.go:343-364`

**Turn payload fields** (written by `interrogationTurnMessage`,
`mutations/interrogation.go:333-365`):

| Field | Values | Notes |
|-------|--------|-------|
| `kind` | `"interrogation_question"` or `"interrogation_answer"` | the message_kind |
| `body` | string | authored text, never raw provider output (D028) |
| `interrogation_id` | string | links back to `striatumd.interrogations` |
| `turn` | `"question"` or `"answer"` | redundant shorthand for `kind` |
| `turn_index` | integer | zero-based, monotonically increasing per interrogation |

**Question state lifecycle:** Questions are inserted with `state = 'pending'`.
On answer, the daemon marks the matching question `state = 'completed'`.
`mutations/interrogation.go:186-198`

---

## 2. Read/query path

### RPC method: `interrogation.list`

**Handler:** `go/pkg/reads/interrogation.go:11` — `HandleInterrogationList`

**Registered at:** `go/pkg/reads/reads.go:160`
```go
server.Register("interrogation.list", makeHandler(runner, HandleInterrogationList))
```

**Params:** `{ repository_id, run_id }`

**Query:** `SELECT ... FROM striatumd.interrogations WHERE repository_id=$1 AND run_id=$2 ORDER BY opened_at ASC, interrogation_id ASC`
`reads/interrogation.go:20-26`

**Response shape:**
```json
{ "count": N, "run_id": "...", "items": [ { interrogation row } ] }
```

### RPC method: `interrogation.show`

**Handler:** `go/pkg/reads/interrogation.go:36` — `HandleInterrogationShow`

**Registered at:** `go/pkg/reads/reads.go:161`
```go
server.Register("interrogation.show", makeHandler(runner, HandleInterrogationShow))
```

**Params:** `{ repository_id, interrogation_id }`

**Query 1** (interrogation metadata): reads one row from `striatumd.interrogations`.
`reads/interrogation.go:45-54`

**Query 2** (ordered turns): `reads/interrogation.go:59-65`
```sql
SELECT message_id, target_session_id, payload_json, created_at
  FROM striatumd.queue_messages
 WHERE repository_id = $1
   AND kind = 'agent_message'
   AND payload_json->>'interrogation_id' = $2
 ORDER BY (payload_json->>'turn_index')::int ASC, created_at ASC, message_id ASC
```

**Turn projection** (per turn, `reads/interrogation.go:71-82`):
```json
{
  "message_id":        "...",
  "target_session_id": "...",
  "created_at":        "...",
  "turn":              "question" | "answer",
  "turn_index":        0,
  "kind":              "interrogation_question" | "interrogation_answer",
  "body":              "<authored text>"
}
```

**Response shape:**
```json
{
  "interrogation": { ...interrogation row... },
  "turns": [ ...ordered turn objects... ],
  "turn_count": N
}
```

### HTTP access from the web UI

Neither `interrogation.list` nor `interrogation.show` has a dedicated HTTP
route in the web service. They must be called via:

```
POST /v1/invoke
Content-Type: application/json
Authorization: Bearer <service_token>

{"method": "interrogation.show", "params": {"interrogation_id": "intg-..."}}
```

or via the GET route shorthand for reads through `callAndWrite` — but no
shorthand exists yet. **The UI feature will need to call `POST /v1/invoke`**
or the implementer must add a dedicated GET route such as
`/v1/runs/{runID}/interrogations` → `interrogation.list` and
`/v1/runs/{runID}/interrogations/{id}` → `interrogation.show`, following the
existing pattern in `routeRunGET` (`service.go:107-135`).

### RPC registry entries

Both methods are registered as `RequiredCapability: CapabilityRead`:
`go/pkg/rpc/registry_methods.go:12-13`
```go
{Method: "interrogation.list", RequiredCapability: CapPtr(CapabilityRead), ...}
{Method: "interrogation.show", RequiredCapability: CapPtr(CapabilityRead), ...}
```
They are not mutation-guarded; a read-only capability token is sufficient.

---

## 3. Trajectory dialogue export

**Handler:** `go/pkg/reads/trajectory.go:12` — `HandleTrajectoryExport`

**Registered at:** (not shown inline, same reads package)
`go/pkg/rpc/registry_methods.go:6`
```go
{Method: "trajectory.export", RequiredCapability: CapPtr(CapabilityRead), ...}
```

`trajectory.export` with `profile = "dialogue"` (the default) builds a UNION
query across `queue_messages`, `artifacts`, and the other provenance tables,
ordered by a derived `ROW_NUMBER() OVER (ORDER BY ts ASC, src ASC, tiebreak ASC)`.
`reads/trajectory.go:72-158`

Interrogation turns appear in the dialogue trajectory as `kind = "agent_message"`
rows from `queue_messages`. The curation pass at `reads/trajectory.go:166-207`
detects `payload_json->>'interrogation_id'` and surfaces these extra fields
on the projected body:

```json
{
  "message_kind":      "interrogation_question" | "interrogation_answer",
  "text":              "<authored body>",
  "interrogation_id":  "intg-...",
  "turn":              "question" | "answer",
  "turn_index":        0
}
```

`reads/trajectory.go:197-206`

**When to use `trajectory.export` vs `interrogation.show`:**
- `trajectory.export` interleaves interrogation turns with the full run
  dialogue (artifacts, other messages). Use it to show a run's complete
  timeline with Q&A in chronological context.
- `interrogation.show` returns a single interrogation's Q&A thread in
  isolation with full metadata (interrogator/target session IDs, topic,
  state, timestamps). Use it to render a focused chat panel for one
  interrogation.

For a chat-style panel scoped to one interrogation, **`interrogation.show`
is the correct call**; it returns turns already ordered by `turn_index`
with `kind`/`body` fields ready for rendering. For a run-history view that
shows interrogation threads in context with other events,
`trajectory.export` (or `trajectory.watch` for live tailing) is the right
source.

---

## 4. Summary: what the UI feature should reuse

| Concern | Existing function | File:line |
|---------|------------------|-----------|
| List interrogations for a run | `HandleInterrogationList` | `reads/interrogation.go:11` |
| Get ordered Q&A turns for one interrogation | `HandleInterrogationShow` | `reads/interrogation.go:36` |
| Ordered turn query | SQL inside `HandleInterrogationShow` | `reads/interrogation.go:59` |
| Full run dialogue with interrogation in context | `HandleTrajectoryExport` | `reads/trajectory.go:12` |
| Live-tail run dialogue | `HandleTrajectoryWatch` | `reads/trajectory.go:43` |
| RPC registry entries (read-only, no mutation gate) | `methodEntries` | `rpc/registry_methods.go:12-13` |

The data model and query logic are complete. No new tables, migrations, or
RPC methods are required to build a read-only chat panel. The only gaps are
(a) a dedicated HTTP route (or use of `POST /v1/invoke`) and (b) the F36
wiring of the React bundle into the Go embed tree.
