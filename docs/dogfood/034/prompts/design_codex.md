# Codex Design Prompt

Produce `docs/dogfood/034/design/codex/DESIGN.md`.

Design an implementation plan for the V2 paired slice of RFC 0030 (daemon RPC server + version skew protocol) AND RFC 0031 (daemon-owned supervision + sealed-apply boundary). These two RFCs ship as one architectural unit: RPC is the trust boundary; supervision and sealed apply flow over it.

Your plan must cover:

**RFC 0030:**
- the canonical wire envelope (`schema_version`, `request_id`, `method`, `params`, `capability_token`, `deadline_ms`) and how it survives a future Python → Go port (D084);
- transports: Unix-domain socket default with owner-only permissions, loopback HTTP opt-in, MCP over socket on a documented sub-path;
- the `daemon.hello` / `daemon.welcome` version handshake, framing negotiation, methods-etag caching, refuse/downgrade rules (exit code 10);
- the method registry shape: route name → required capability mapping, `daemon.describe` semantics, etag invalidation;
- capability vocabulary (`read`, `write`, `review`, `claim`, `apply`, `admin`) — `apply` introduced here for sealed mode (RFC 0031);
- audit shape per request (canonicalized params hash, decision, denial reason, hash chain) and request log retention on the RFC 0033 substrate;
- routing: which CLI verbs default to daemon, which still allow `--no-daemon`, which are admin-only;
- migration from V1 direct-registry-reads to daemon-mediated RPC routing.

**RFC 0031:**
- supervisor ownership migration: daemon DB `process_supervisors` table + repo-local `process_supervisor_pointers` table;
- daemon-mediated `supervise.start / send / stop / status / list` RPC methods with capability bindings;
- supervisor reattach across daemon restart using pid + pid_start_time verification;
- sealed apply RPC (`apply.reviewed_patch`): patch digest verification, base-tree hash check, reviewer verdict binding, apply against a daemon-owned worktree, refuse on mismatch with documented exit codes;
- signing key custody: Ed25519 keypair in OS keyring with `0600` runtime fallback file, daemon refuses to start sealed-mode runs without a loadable key, `daemon.key.rotate` admin RPC;
- apply receipt format: patch digest + base-tree hash + post-apply tree hash + reviewer verdict id + signing key id + timestamp + daemon version + substrate version, stored append-only on the substrate plus a Markdown evidence artifact under the run's evidence path;
- workflow-level fields: `require_daemon: true`, `apply_gate: true`, `sealed_patch_provider: refuse` (debugging aid);
- sealed-mode `run start` gating: refuse without daemon + signing key + `apply` token.

**Cross-cutting:**
- how the RPC server logs every mutating request to the audit chain on the substrate;
- how MCP mutation tools route through the same RPC + capability gating (no `tools/list` expansion in this dogfood; that's RFC 0032);
- test infrastructure: per-test daemon harness, per-test substrate isolation, version-skew injection, supervisor restart tests, sealed-apply refuse-on-mismatch tests.

Explicitly mark as deferred (do NOT design):
- cross-repository workflows + MCP mutation capability expansion (RFC 0032);
- Python → Go core port (D084);
- bundled / Dockerized Postgres distribution (RFC 0033 follow-up);
- retirement of direct repo-local CLI mode (separate future RFC).

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.

- The byline must be a plain Markdown line with NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly.
- Correct: `author: designer-codex-gpt-5.5-001`
- Wrong: `**Author:** ...`, `Author: ...`, `# author: ...`, `author: "..."`.

The `handoff` artifact kind does not require YAML front matter. Synthesis and finding artifacts in this dogfood will, with the JSON-encoded `key: <value>` block shown in `synthesize_design.md` and `review_design.md`.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
