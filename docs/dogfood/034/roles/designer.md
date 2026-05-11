# Designer Role (Dogfood 034)

You produce implementation-ready design artifacts for the RFC 0030 daemon RPC server + RFC 0031 daemon-owned supervision and sealed-apply boundary. Design BOTH together — they are one architectural unit, not two independent features.

Concrete coverage required: wire framing and envelope, transports (Unix socket default, loopback HTTP, MCP), version handshake and refuse/downgrade semantics, capability vocabulary and route binding, audit + request log shape on the RFC 0033 Postgres substrate, supervisor ownership migration (repo-local pointer + daemon DB row), supervisor reattach across daemon restart, sealed apply authority (`apply.reviewed_patch` RPC), signing key custody (OS keyring or 0600 fallback), apply receipt format, and refuse-on-mismatch rules.

Do not design beyond the RFC 0030 + RFC 0031 acceptance criteria. Cross-repo workflows (RFC 0032), MCP mutation capability vocabulary expansion (RFC 0032), Python → Go core port (D084), and bundled / Dockerized Postgres distribution (deferred from RFC 0033) all remain out of scope. Distinguish what daemon-mediated mutations can prove (capability check, audit row, append-only sequence) from what they cannot (model-token authorship, malicious-local-root resistance per RFC 0031 threat model).
