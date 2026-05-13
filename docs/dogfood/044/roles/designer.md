# Designer Role (Dogfood 044)

Three fresh-design lanes (codex, claude, gemini) produce independent
perspectives on RFC 0040 V1.5 F1-F6. Synthesis picks one path. Cite the
existing code that your design changes — do not propose green-field
shapes.

Required citations (read these before designing):

- `docs/rfcs/0040-mcp-driven-dogfood-harness.md` — Proposal §1–§5.
- `docs/dogfood/040/OPERATOR_REPORT.md` — F1-F6 findings narrative.
- `src/striatum/daemon_pg/` — daemon MCP `tools/call` entry point and
  method-registry routing. F1 + F4 lifecycle hooks land here.
- `src/striatum/web/chat_tools.py` — composite tool entry points
  `dogfood.publish_on_behalf` and `dogfood.surgical_recovery`. F2/F3
  atomicity changes land here.
- `src/striatum/process_progress.py` — supervised-progress watcher
  module. F5 race + signal hardening lands here.
- `src/striatum/supervisor.py` — process supervisor; the watcher hook
  may land here if synthesis routes it that way.
- `src/striatum/cli/` — CLI surface (only the e2e-test-supporting
  pieces are in scope).

Address: F1 dispatch wiring (exact entry function + registry handle),
F2/F3 atomicity model (pick one), F4 watcher invocation point (named
lifecycle function), F5 race windows + guards, F6 e2e test plan.

**Backward compatibility for existing MCP tools is non-negotiable** —
`generate_workflow_preview`, `generate_workflow_write`, and every RPC-
thin tool must keep working. The daemon RPC envelope-v1 schema is
unchanged; the design must explicitly note this.

Out of scope: hosted services, RFC 0040 §6 future work, new tool
surface beyond F1-F6 wiring, frontend.
