# Gemini Design Prompt

Produce `docs/dogfood/034/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0030 + RFC 0031 with emphasis on cross-platform reality and operational lifecycle.

Your plan must cover:

- **Cross-platform daemon RPC**: Unix-domain socket default on macOS/Linux; named pipe or loopback TCP on Windows (or Windows-via-WSL only, deferred otherwise); owner-only permissions on each platform; per-platform socket discovery paths (`STRIATUM_DAEMON_SOCKET`, `${XDG_RUNTIME_DIR}/striatum/daemon.sock` on Linux, `~/Library/Application Support/striatum/daemon.sock` on macOS).
- **Cross-platform process supervision**: how the daemon spawns and re-attaches supervised lane processes on each platform; `pid_start_time` (Linux `/proc/<pid>/stat`) vs platform equivalents; refuse cleanly when no platform-stable token is available rather than silently downgrading.
- **Signing key custody**: OS keyring per platform (Keychain on macOS, libsecret/Secret Service on Linux, DPAPI/Credential Manager on Windows-via-WSL) with `0600` runtime fallback in `~/.local/state/striatum/daemon/signing_key`; daemon refuses to start sealed-mode runs without a loadable key; how rotation works per platform.
- **Packaging and distribution deltas**: the daemon RPC server is a real long-running process now (not just a foreground sweep). What platform-specific service install hooks are needed for daemon-managed lifecycle (launchd / systemd user units / Windows service)? Recommendation: defer service-manager install to a follow-up RFC; document the manual `striatum daemon start` workflow in V2.
- **Operator onboarding**: how a new operator gets from "I have a registered repo" to "I have a daemon RPC server running with a capability token I can use to call MCP mutations." Concrete platform-specific commands plus token bootstrap UX (admin token from `daemon start` or `repo add`).
- **Adversarial test cases**:
  - hostile MCP client requesting `tools/list` then `tools/call` with elevated args — capability gate should refuse, audit row records `capability_missing`;
  - prompt-injected MCP client claiming "trusted" identity — no `--allow-mutations` global flag; tokens are the only access path;
  - supervisor reattach after daemon crash with live children — daemon DB row + pid + pid_start_time must match;
  - sealed-apply with substituted patch digest — apply RPC refuses with documented exit code;
  - sealed-apply with mismatched base-tree — refuse;
  - sealed-apply with reviewer verdict against a different patch digest — refuse;
  - version skew: CLI built against daemon vN, daemon is vN+2 — refuse via `daemon.welcome` with `version_incompatible`;
  - capability token leaked to MCP prompt then used elsewhere — operator `daemon.token.revoke` plus audit chain shows the timeline;
  - signing key cannot be loaded — daemon refuses to start sealed-mode runs;
  - audit chain tamper attempt via daemon API — role-enforced append-only refuses; manual SQL UPDATE on `audit_log` by the daemon DB role is out of scope (RFC 0031 threat model).
- **Staged delivery**: which parts of RFC 0030 land first (envelope + handshake + capability registry + audit), which depend on existing RFC 0033 substrate, which require RFC 0031 supervisor migration before they are useful.

State which parts of the design require platform-specific work and which are cross-platform. Treat Windows-via-WSL as the only Windows path in V2; native Windows daemon support stays deferred.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.

- The byline must be a plain Markdown line with NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly.
- Correct: `author: designer-gemini-pro-001`
- Wrong: `**Author:** designer-gemini-pro-001` (this is what failed in dogfood-031 and dogfood-033 — do not repeat it)
- Wrong: `Author: designer-gemini-pro-001` (capital A)
- Wrong: `author: "designer-gemini-pro-001"` (quoted)

The `handoff` artifact kind does not require YAML front matter. If you produce schema-bearing artifacts in this run (synthesis, finding, decision), the file must start with a JSON-encoded `key: <value>` front matter block. Example for `finding`:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0030", "rfc-0031"]
---
```

The byline appears AFTER the front matter block and a blank line, not inside it.

Do not call striatum CLI; the operator publishes on your behalf otherwise.
