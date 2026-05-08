# Synthesize V1 Design

Take the research handoff plus RFC 0012 itself and produce
docs/dogfood/006/DESIGN_SYNTHESIS.md with locked V1 contracts:

1. Server class (stdlib only); single-threaded vs threading.
2. Exact endpoint routing table.
3. Argv parsing inside POST /v1/invoke; mutation-detection rule with
   the explicit whitelist/blacklist.
4. SSE wire format; poll cadence; ?since and Last-Event-ID handling;
   end-of-stream signal.
5. Auth model: Unix-socket no-auth; HTTP optional --token; non-loopback
   refusal at startup with exit 8.
6. PID file path, single-instance enforcement, graceful shutdown.
7. CLI surface for striatum serve.
8. Exhaustive test plan covering all RFC 0012 acceptance criteria.
9. Doc update list.
10. Deferred items.

Do not write product code from a synthesis job.
