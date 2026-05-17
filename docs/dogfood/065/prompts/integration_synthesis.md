# Integration synthesis prompt - dogfood 065

Produce `docs/dogfood/065/synthesis/INTEGRATION_SYNTHESIS.md` as a synthesis
artifact with valid `striatum.synthesis.v1` front matter. Include the four
track handoffs and four track reviews in `inputs`.

Use a title block with `author: designer-codex-codex-001`.

Required sections:

1. Outcome by track: completed, accepted with findings, needs revision, or
   blocked.
2. Go daemon port status: what now works, what remains below production gate,
   and what evidence backs each claim.
3. SQLite status: production paths closed, remaining migration/fixture
   exceptions, and any still-blocking direct SQLite access.
4. Client/service/MCP status: daemon boundary changes, production MCP tool
   list changes, and local-authoring/legacy surfaces.
5. Docs/decision status: D105/D107, RFC 0068-0071, SPEC/TODO/ROADMAP alignment.
6. Verification commands and their results.
7. Required follow-up decisions or dogfoods.

Do not edit README.md, OPERATOR_REPORT.md, workflow, prompts, roles, build
handoffs, or review artifacts.
