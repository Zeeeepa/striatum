# Claude Code Design Prompt

Produce `docs/dogfood/034/design/claude_code/DESIGN.md`.

Design an implementation plan for RFC 0030 (daemon RPC server + version skew protocol) + RFC 0031 (daemon-owned supervision + sealed-apply boundary). Treat them as one architectural unit; RFC 0030 is the trust boundary, RFC 0031 flows over it.

Focus on:

- **Trust boundaries**: daemon process, RPC clients (CLI / MCP / web), supervised lane processes, operator. Per RFC 0031 §Threat Model the scope is over-eager AI agents acting through documented interfaces, and operator-mistake footguns; malicious-local-root operators are out of scope. Documentation must reflect this exactly.
- **Capability vocabulary and token lifecycle**: `read`/`write`/`review`/`claim`/`apply`/`admin`; how tokens are issued via `daemon.token.create` (admin-only), scoped to repository or daemon-global, expired, revoked, rotated; storage in OS keyring with `0600` runtime fallback; constant-time compare; refusal vocabulary (`capability_missing`, `token_revoked`, `token_expired`, `repo_not_registered`, `method_unknown`, `version_incompatible`).
- **Sealed-apply gate semantics**: `apply.reviewed_patch` workflow — load patch artifact, verify hash, load reviewer verdict, verify `patch_digest_hash` match, verify base-tree hash, apply to daemon-owned worktree, record receipt. The apply gate (`apply_gate: true` workflow field) refuses to mark a build job complete unless a downstream reviewer verdict references the patch digest. The full set of refuse paths with documented exit codes.
- **Apply receipt format and provenance limits**: receipt records patch digest + base-tree + post-apply tree + verdict id + signing key id + timestamp + daemon version + substrate version. The receipt is local evidence the AI did what its packet claimed, NOT cryptographic non-repudiation against a malicious operator. SPEC.md and README.md must say so explicitly.
- **Supervisor reattach across daemon restart**: daemon DB owns the supervisor row; repo-local SQLite holds a pointer. Daemon restart re-attaches by pid + pid_start_time verification (RFC 0026); mismatch transitions to `lost` and surfaces via `supervise.list`. Tests must exercise the daemon-crashed-with-live-children path explicitly.
- **V1 → V2 supervisor row migration**: where the repo-local rows go, what the pointer table holds, how `striatum supervise` CLI verbs route through daemon by default, when `--no-daemon` is honored during the transition.
- **MCP behavior**: V2 daemon MCP gains tool routes via the RPC method registry, but `tools/list` filters by token capability. No `tools/list` expansion beyond what RPC routes already exist. RFC 0032 owns wider MCP mutation policy.
- **Concrete touch points in `src/striatum/`**: `daemon.py`, `daemon_pg/*` (existing substrate), `mcp.py`, `service.py`, `supervisor.py`, `process_adapter.py`, plus new modules under `src/striatum/daemon_rpc/` and `src/striatum/daemon_apply/` (or whatever names the synthesis settles on).

Explicitly state what cannot be claimed even after this dogfood lands:
- model-token authorship proof;
- independent human decision provenance;
- adversarial local-root resistance;
- cross-repository transaction semantics (those are RFC 0032).

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim into the artifact title block.

- The byline must be a plain Markdown line with NO bold (`**`), NO italics, NO heading prefix (`#`), NO quotes around the value, NO trailing punctuation.
- The line must start with lowercase `author:` exactly.
- Correct: `author: designer-claude-opus-001`
- Wrong: `**Author:** ...`, `Author: ...`, `# author: ...`, `author: "..."`.

The `handoff` artifact kind does not require YAML front matter. If you produce schema-bearing artifacts in this run (synthesis, finding, decision), include the JSON-encoded `key: <value>` front matter block as shown in `synthesize_design.md`.

Do not call striatum CLI unless your harness profile permits it; the operator publishes on your behalf otherwise.
