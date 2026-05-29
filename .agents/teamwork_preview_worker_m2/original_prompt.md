## 2026-05-29T03:40:45Z
You are the specialist Worker agent compiled to write the expert systems architecture review of the Striatum codebase.

**Objective**: Generate a highly technical, dense, and grounded markdown report exactly at `~/git/striatum/STRIATUM_ARCHITECTURE_REVIEW_TEAMWORK_PREVIEW_2026-05-29.md`. The report length must be between 3,000 and 5,000 words.

**Key Inputs**:
You must read and synthesize the findings from the three Explorer agents:
- Explorer 1 (Codebase structure, directories, files inventory, domain model): `~/git/striatum/.agents/teamwork_preview_explorer_m1_1/analysis.md` and `~/git/striatum/.agents/teamwork_preview_explorer_m1_1/handoff.md`
- Explorer 2 (CLI, daemon, MCP, CLI/daemon boundaries, capability authorization): `~/git/striatum/.agents/teamwork_preview_explorer_m1_2/analysis.md` and `~/git/striatum/.agents/teamwork_preview_explorer_m1_2/handoff.md`
- Explorer 3 (Postgres state transition, scratch spaces, test posture): `~/git/striatum/.agents/teamwork_preview_explorer_m1_3/analysis.md` and `~/git/striatum/.agents/teamwork_preview_explorer_m1_3/handoff.md`

**Core Requirements**:
1. **Word Count**: Strictly between 3,000 and 5,000 words. Be technical, detailed, dense, and comprehensive.
2. **Structure**: 11 sections numbered 0 to 10 in this exact order:
   - **0. Files reviewed**: A flat list of absolute or repository-relative paths of all files read during this review.
   - **1. Executive summary**: 5–10 honest, fluff-free bullets.
   - **2. What the project is trying to be**: Goals, principles, domain model, and operating model, citing repository docs.
   - **3. Current architecture**: Components, runtime, state/storage, interfaces (CLI, API, daemon, web, MCP), test posture, and release posture. Note any discrepancies between documentation and actual implementation.
   - **4. Strengths**: Specific architectural decisions and abstractions worth preserving, with justifications.
   - **5. Concerns**: Critical architectural concerns ranked as Blocker, Serious, or Smell, each backed by specific code evidence.
   - **6. North-star architecture**: A greenfield architecture design tailored specifically to the project's real constraints (single operator, laptop/homelab runtime, no Kubernetes, no managed cloud, demo-stage maturity).
   - **7. Recommended changes**: A table mapping priority, change, rationale, benefit, risk, and rough effort (hours/days/weeks).
   - **8. Functionality I'd add**: A table mapping proposed feature, priority, rationale, benefit, risk, and rough effort.
   - **9. Execution roadmap**: Concrete first step (startable today), near-term (month), medium-term (quarter), and long-term milestones.
   - **10. Open questions**: Crucial architectural or design questions that could not be determined solely from the codebase.
3. **Tri-Voice Grounding**: Every non-trivial architectural claim in sections 3, 4, and 5 must be grounded in specific file paths and/or line ranges (e.g. `docs/reference/spec.md:20-50` or `go/pkg/rpc/server.go:120-150`). Grounding must be represented explicitly using the three voices:
   - **Stated**: Claims from docs/READMEs.
   - **Actual**: Implementation from the codebase.
   - **Mine**: Your expert judgment.
4. **Active Disagreement**: Do not use "describe-then-defer". Actively argue against principles or designs that are suboptimal or incorrect.
5. **No Vague Verbs**: Recommendation tables must not use vague verbs like "improve", "enhance", "consider", or "explore". Use precise names of changes.
6. **Local-First Grounding**: Absolutely no generic cloud-ops/SaaS-ops or cloud-scale advice (no "use Kubernetes", "add AWS RDS", or third-party cloud tools). Keep it laptop/homelab scale.
