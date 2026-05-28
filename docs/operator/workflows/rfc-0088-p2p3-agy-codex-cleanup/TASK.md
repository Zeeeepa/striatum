# TASK — RFC 0088 P2+P3: agy lane + gemini removal + codex cutover + delete wrapper / turn-driver

Reference: `docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md`
(Decisions 1, 3, 4, 5; Phasing P2+P3). P1 substrate landed on
`striatum/rfc-0088-p1-pty-foundation` (commits `841f719`, `4d6833c`,
`c11a6cf`, `97fbb53`); read the P1 verification log in
`docs/operator/workflows/rfc-0088-p1-pty-foundation/TASK.md` for the proven
mechanism (agent-loop wrap + delayed bootstrap submit + ephemeral
`--mcp-config --strict-mcp-config` + owned-PTY byline via D080) and the
known operational config (`path.conf` stays).

## P2 — agy lane + gemini removal

### Canonical tool family

- **Drop `antigravity` from `go/pkg/workflowtemplates/catalog.go`** (line 27
  registers it as a second family alongside `agy`); keep `agy` as the single
  canonical family. The `agy_default` profile in `catalog.json` already uses
  `tool_family: "agy"` and `display_name: "Antigravity default profile"` —
  "Antigravity" stays as the human-readable display string, not a tool family.
- **Remove `gemini_cli` from `catalog.go`** and the **`gemini_default`
  profile** from `catalog.json`.

### Installer surface (RFC Decision 4 — reuse the claude bundle)

- Add `agy` to `SkillsAllProfilesOrder` and `PluginsAllProfilesOrder` in
  `go/pkg/installers/skills.go` / `go/pkg/installers/plugins.go`; remove
  `gemini` from both.
- The `agy` profile **reuses the claude_code template tree** rather than
  authoring a parallel one (`agy plugin import claude` works wholesale per
  RFC §4). Generate the claude_code shape into agy's config dir; only thin
  agy-specific wrappers where the path differs.
- Delete `go/pkg/installers/templates/plugins/gemini/` and
  `go/pkg/installers/templates/skills/gemini/` and the gemini cases in
  `plugins.go` / `skills.go`.

### MCP-config injection covers agy (RFC Decision 5)

`go/pkg/agentloop/mcpconfig.go` `injectLaneMCPConfig` currently switches on
`laneAdapterName` and handles `claude`. Add an **`agy` case** that mirrors
claude (Antigravity is claude-shaped: `--mcp-config <file>` +
`--strict-mcp-config` per the binary; `agy plugin import claude` confirms
shared shape). Same ephemeral 0600 file under `.striatum/scratch`; same
fresh-at-launch / never-persist invariant.

### Workflow template + example sweep

- Replace gemini lanes with agy in any workflow.json under `docs/`,
  `examples/`, or `go/pkg/workflowtemplates/` that still references gemini —
  notably `docs/operator/workflows/conversation-3way/workflow.json` (lane
  `gemini` with `single_shot: true` is retired by P3 anyway; for the P2 step
  here, either delete that lane or replace it with an agy `agent_loop` lane).

## P3 — codex cutover + delete wrapper / turn-driver

### Codex over PTY (agent_loop)

- codex CLI has `resume --last` / `fork` and a bare interactive mode — same
  shape as claude. Wire a codex `agent_loop` lane: command `["codex"]` (bare
  interactive) or whatever invocation cleanly enters a persistent session;
  `adapter_capabilities.agent_loop: true`; same wrap path (`striatumd
  -agent-loop -- codex`).
- Per-adapter **submit driver**: P1 used a global default CR after a 750ms
  delay (`agentLoopSubmitDelay`/`agentLoopSubmitSequence`). codex may need a
  different submit key-sequence or a screen-detect step (claude's bypass
  dialog issue suggests detecting+answering startup dialogs is a generic
  P2/P3 surface — implement it once, here). Read the P1 close-out note in
  `docs/operator/workflows/rfc-0088-p1-pty-foundation/TASK.md` for context.
- Prove codex over PTY end-to-end the same way P1 proved claude (a one-job
  verify run with the byline landing); attestation comes for free via the
  existing D080 supervised path.

### Retirement (only after both adapters are PTY-proven)

- **Delete the turn-driver and `single_shot` capability**:
  - `go/pkg/agentloop/turn_driver.go`, `turn_driver_signal.go`,
    `turn_driver_test.go`
  - `agentLoopModeTurnDriver` / `laneUsesTurnDriver` /
    `turnDriverAgentLoopCommand` in `go/pkg/mutations/supervision_control.go`
  - the `-turn-driver` flag handling in `go/cmd/striatumd/main.go`
  - any `adapter_capabilities.single_shot` reference in catalog or examples
- **Delete the `--print` supervised wrapper path** — the
  `launchPipeProcess`-with-raw-command, claim→`supervise send`,
  one-shot-EOF delivery shape. All lanes are agent_loop-only after this.
  Remove `stdinDeliveryOneShotEOF` if no remaining consumer; keep the helper
  PTY path that the agent-loop already uses.
- F45 (gemini turn-driver slowness) is closed by removal.

## Verification

- `cd go && gofmt -l . && go vet ./... && go test ./...` clean.
- Live proof: a one-job verify run on **claude** (already proven in P1) and
  on **agy** and **codex** — each lane is launched via `striatumd -agent-loop
  -- <bin>` over a PTY, receives + submits its bootstrap, publishes with a
  lane byline, completes.
- Removal proof: `go build` after deleting turn-driver/wrapper; tests still
  green; no remaining references to `single_shot`, `turn_driver_*`,
  `agentLoopModeTurnDriver`, or `stdinDeliveryOneShotEOF` (if removed).
- Update `docs/SPEC.md` (per `AGENTS.md` "fix the doc if it disagrees with
  current source") and add `D148/D149/D150` entries to `docs/DECISION_LOG.md`
  marking RFC 0088 accepted.

## Out of scope

- New attestation code (P1 closed D149 via the existing D080 supervised path).
- Promoting the RFC to `accepted` status — leave for the operator on
  completion. The decision-log entries are added in this run; the RFC status
  line gets flipped at landing.
