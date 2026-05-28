# Design — RFC 0088 P2+P3 (one of two parallel lanes)

Read docs/operator/workflows/rfc-0088-p2p3-agy-codex-cleanup/TASK.md and RFC
0088 (docs/rfcs/0088-deprecate-print-interactive-pty-lanes-agy-migration.md;
Decisions 1, 3, 4, 5). Ground yourself in the real code:
go/pkg/workflowtemplates/catalog.go (lines ~26-28: gemini_cli, agy,
antigravity tool families); go/pkg/workflowtemplates/catalog.json
(gemini_default + agy_default harness_profile_fragments);
go/pkg/installers/skills.go (SkillsAllProfilesOrder ~line 12) and
go/pkg/installers/plugins.go (PluginsAllProfilesOrder ~line 14);
go/pkg/installers/templates/{plugins,skills}/{claude_code,codex,gemini}/;
go/pkg/agentloop/mcpconfig.go injectLaneMCPConfig (claude case to mirror for
agy); go/pkg/agentloop/loop.go + turn_driver.go;
go/pkg/mutations/supervision_control.go (laneUsesTurnDriver,
turnDriverAgentLoopCommand, laneUsesAgentLoop, selfDrivingAgentLoopCommand);
go/cmd/striatumd/main.go (-turn-driver flag). P1 proven mechanism is in
docs/operator/workflows/rfc-0088-p1-pty-foundation/TASK.md (verification log
+ closed follow-ups).

You are one of two independent design lanes — do not coordinate. Produce
DESIGN.md in your lane dir covering: (1) P2 catalog/installer/MCP/workflow
sweep — exact files to touch; how `agy plugin import claude` reuse manifests
in the installer; the claude→agy mirror in injectLaneMCPConfig; the gemini
removal across catalog/profiles/installer/templates/example workflows. (2)
P3 codex agent_loop wiring — invocation shape that enters a persistent
codex session (bare `codex` vs `codex resume --last` vs something else; pick
+ justify); the per-adapter submit driver (claude needed CR after 750ms; how
to handle codex similarly AND handle startup dialogs like claude's bypass
gate without flaky workarounds). (3) The deletion plan — exact files, exact
removals, no dangling references; verification that nothing else consumed
turn_driver/single_shot/oneshot delivery. (4) Order of land: P2 first, P3
after each adapter is PTY-proven; suggest one or two commits. 2-3
alternatives where there's a real choice; risks (bricking dogfood lanes,
removing while a workflow.json still references it, codex submit
fragility); rollout. No code. Stay in your lane dir. Emit the submit-handoff
packet when done.
