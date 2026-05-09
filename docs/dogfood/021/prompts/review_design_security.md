Security-posture review: API-key handling (env-only, never
SQLite, never logs); SSRF (operator picks the endpoint, but
what if they paste a localhost SSRF target? same-host or
loopback warnings?); path traversal on /view/<path>;
sanitization of model output as Markdown; CSP impact; scratch
JSONL world-readable on shared hosts.
