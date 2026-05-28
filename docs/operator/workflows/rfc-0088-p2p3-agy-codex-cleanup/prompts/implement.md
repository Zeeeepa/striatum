# Implementation — RFC 0088 P2+P3 (codex builder)

Read TASK.md, RFC 0088, .../artifacts/DESIGN_SYNTHESIS.md, and the design-
panel findings. Implement the synthesis in Go and docs/templates, in this
order (commit at each boundary):

P2 — agy + gemini removal:
1. catalog.go: drop antigravity family; remove gemini_cli family.
2. catalog.json: remove gemini_default profile; keep agy_default.
3. installers (skills.go, plugins.go): add agy to ALL_PROFILES_ORDER and
   handler switch (reuse the claude_code shape per RFC Decision 4); remove
   gemini entries; delete templates/{plugins,skills}/gemini/.
4. agentloop/mcpconfig.go: add `agy` case to injectLaneMCPConfig, mirroring
   the claude `--mcp-config <file> --strict-mcp-config` wiring; add
   focused tests.
5. Workflow templates / examples: replace gemini lanes with agy or drop
   them (notably docs/operator/workflows/conversation-3way/).
6. Commit "RFC 0088 P2: ...".

P3 — codex cutover + retirement:
7. Wire codex agent_loop: lane command + per-adapter submit driver/dialog
   handling. Prove with a one-job verify run (same shape as
   docs/operator/workflows/rfc-0088-p1-verify/) that codex publishes with
   the lane byline; record evidence in HANDOFF.md.
8. Delete the turn-driver: turn_driver.go + turn_driver_signal.go +
   turn_driver_test.go; laneUsesTurnDriver +
   turnDriverAgentLoopCommand + agentLoopModeTurnDriver references in
   supervision_control.go; -turn-driver flag in cmd/striatumd/main.go.
9. Delete the --print supervised wrapper path: launchPipeProcess raw-
   command shape + stdinDeliveryOneShotEOF if no consumer remains;
   single_shot capability references everywhere.
10. Commit "RFC 0088 P3: ...".

Throughout: `cd go && gofmt -l . && go vet ./... && go test ./...` must
stay green. Write .../artifacts/build/HANDOFF.md with what landed/deferred,
exact verification commands, and the two commit SHAs. Stay live for
build-review (NOT interrogable — implement is codex one-shot in this
dogfood; reviewers read the diff and HANDOFF). Emit submit-handoff when
done.
