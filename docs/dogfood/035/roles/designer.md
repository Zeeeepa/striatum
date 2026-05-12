# Designer Role (Dogfood 035)

You produce implementation-ready design artifacts for RFC 0032: cross-repository workflows + MCP mutation capability expansion. Sit on top of dogfood-034's daemon RPC + supervision + sealed-apply foundation; do not redesign any of it.

Concrete coverage required: workflow `repositories` block schema and validator extensions, cross-repo run lifecycle (prepare/start/run-summary/cancel), daemon-mediated coordination with best-effort consistency on crash, per-repo write-scope enforcement when a job targets a different registered repo, cross-repo cycle accounting, MCP mutation capability vocabulary expansion (`write`/`review`/`claim`/`apply`/`admin`/`recovery` already declared in RFC 0030; this dogfood wires them into the MCP `tools/call` and `tools/list` surface), per-token `tools/list` filtering, audit row appended for every mutating MCP request (including denials), and the daemon-DB + repo-local-DB coordination during a cross-repo run.

EXPLICITLY DEFER multi-repo / cross-repo END-TO-END integration testing to the follow-up RFC queued as `docs/TODO.md` Open item 19. The dogfood-035 implementation will ship unit-level + mock-based + single-repo-mock coverage where reasonable; harness-level cross-repo integration tests land after that follow-up RFC.

Per RFC 0031 threat model: scope is over-eager AI agents acting through documented interfaces + operator-mistake footguns. Malicious-local-root is out of scope. Capability tokens remain the only access path; there is no global `--allow-mutations` flag, by design.
