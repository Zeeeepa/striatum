# Review Design Prompt (threat_model posture)

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter (JSON-encoded values; quote strings).

Review `docs/dogfood/033/DESIGN_SYNTHESIS.md` under the **threat_model** posture. Use only an accepting verdict (`accept` or `accept_with_findings`) if the plan enumerates the trust boundaries and attack surfaces the substrate rewrite introduces, and each is either acknowledged or mitigated.

Scope per RFC 0031 §Threat Model: defend against over-eager AI agents acting through documented interfaces, and against operator-mistake footguns. **Out of scope**: a malicious local-root operator who reads daemon files, kills the daemon, or impersonates the daemon process. Reviews relying on the malicious-local-root framing should be redirected to the post-implementation `devils_advocate` / `security` reviews (which run after this dogfood lands, not as a gate).

In scope for this review:

- **Daemon → Postgres trust boundary**: does the synthesis specify which Postgres role the daemon connects as, what privileges that role has, and what the daemon refuses when the role's privileges are wrong?
- **Schema version drift**: what happens when daemon binary is older than on-disk schema (refuse via exit code 9), and what happens when daemon binary is newer (apply forward migration). Are the failure modes specified?
- **Audit chain across cutover**: does the V1 SQLite → V2 Postgres cutover preserve hash anchors byte-equivalently? Is there a `daemon migrate --verify` path that catches a corrupted import before V1 reads are refused?
- **Append-only enforcement**: role privileges prevent UPDATE/DELETE on audit tables via daemon API. Does the synthesis name the role and the GRANT/REVOKE pattern? An operator who connects as DB superuser is out of scope (RFC 0031 threat model); a daemon bug that lets ordinary daemon code UPDATE audit rows is in scope.
- **Operator onboarding footguns**: malformed `STRIATUM_DAEMON_DB_URL`, two daemons against one DB, PG version drift, role with insufficient privileges. Does the synthesis make each one safe-to-fail rather than data-loss-by-accident?
- **Cross-platform reality**: macOS, Linux, Windows-via-WSL install paths are real concerns; is the operator-onboarding story consistent across them or does it silently assume Linux?
- **Concurrency under serializable isolation**: is deadlock avoidance specified for supervisor heartbeats, audit append, and capability lookup? Or is the synthesis hand-waving about MVCC?
- **Scope discipline**: does the synthesis stay inside RFC 0033's bounds (no bundled-binary code, no Go port design, no repo-local SQLite changes)?
- **Provenance overclaim**: does the synthesis claim cryptographic guarantees the substrate does not provide? Apply receipts (RFC 0031) and lane attestation (RFC 0026) keep their own scope; the substrate adds Postgres role-based append-only, not non-repudiation.

For `needs_revision`, list the minimum concrete changes needed before implementation may proceed. For `accept_with_findings`, the findings must be non-blocking and explicitly say so.

Stay inside the review write scope (`docs/dogfood/033/review/design/threat/`). Do not modify the synthesis. Do not call striatum CLI.
