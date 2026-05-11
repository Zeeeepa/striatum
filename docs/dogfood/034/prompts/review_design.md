# Review Design Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings). Example:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0030", "rfc-0031", "daemon", "design"]
---
```

Review `docs/dogfood/034/DESIGN_SYNTHESIS.md` under the **threat_model** posture. Use only an accepting verdict (`accept` or `accept_with_findings`) if the plan enumerates the trust boundaries and attack surfaces the paired RFC 0030 + RFC 0031 changes introduce, and each is either acknowledged or mitigated.

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads the daemon's signing key, kills the daemon, or impersonates the daemon process. Reviews relying on the malicious-local-root framing should be redirected to the post-implementation `devils_advocate` / `security` reviews (which run after this dogfood lands, not as a gate).

In scope for this review:

- **Daemon trust boundary**: does the synthesis specify which mutations route through RPC vs which still allow `--no-daemon`, and the refuse-on-version-mismatch behavior (no silent downgrade)?
- **Capability vocabulary and gating**: are all RPC routes mapped to required capabilities? Is the audit row appended for every mutating request including denials? Is the denial reason vocabulary documented?
- **Sealed-apply boundary**: are the refuse paths (digest mismatch, base-tree drift, reviewer-verdict-against-wrong-digest, missing signing key) specified with concrete exit codes? Does the receipt format avoid overclaiming non-repudiation? Does documentation reflect the AI-guardrail framing exactly?
- **Signing key custody**: OS keyring fallback to `0600` runtime file; daemon refuses sealed-mode without a loadable key; rotation RPC behavior; key never written to env vars, audit log, or repo files.
- **Supervisor reattach**: pid + pid_start_time verification; mismatch transitions to `lost`; tests exercise the daemon-crashed-with-live-children path.
- **Version skew**: `daemon.hello` / `daemon.welcome` refuses cleanly with exit code 10; no silent fallback to direct mode.
- **Operator onboarding footguns**: malformed `STRIATUM_DAEMON_DB_URL`, token leak, daemon connected as Postgres superuser. Do operator mistakes fail-safe rather than data-loss?
- **MCP mutation safety**: no `--allow-mutations` global flag; capability tokens are the only access path; `tools/list` filters by token capability; prompt-injection mitigation is "tokens, not trust claims."
- **Cross-platform reality**: are Unix socket and loopback HTTP transports specified for each supported platform, and is Windows-native-daemon honestly deferred?
- **Concurrency**: does the synthesis specify serializable-isolation audit append on the RFC 0033 substrate? Deadlock avoidance across capability checks, supervisor heartbeats, audit append?
- **Scope discipline**: does the synthesis stay inside RFC 0030 + RFC 0031 V2? No cross-repo, no MCP mutation capability expansion, no Go core, no bundled distribution.
- **Provenance overclaim**: does the synthesis claim cryptographic non-repudiation, model-token authorship proof, or malicious-local-root resistance? Per RFC 0031 threat model, none of those are in scope.

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, the findings must be non-blocking and explicitly say so.

Stay inside the review write scope (`docs/dogfood/034/review/design/threat/`). Do not modify the synthesis. Do not call striatum CLI; the operator publishes otherwise.
