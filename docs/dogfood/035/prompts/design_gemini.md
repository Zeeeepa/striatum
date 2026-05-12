# Gemini Design Prompt

Produce `docs/dogfood/035/design/gemini/DESIGN.md`.

Design an implementation plan for RFC 0032 (cross-repository workflows + MCP mutation capabilities) with emphasis on cross-platform reality and per-repo failure isolation.

Your plan must cover:

- **Cross-platform path and identity** for registered repositories: macOS realpath/inode-based identity, Linux ext4/btrfs paths, Windows-via-WSL paths under `/mnt/c/` and their interaction with the daemon's `repo_identity` derivation. Treat Windows-native daemon as deferred per dogfood-034 scope.
- **Per-repo failure isolation during a cross-repo run**: what happens if one participating repo's `.striatum/state.sqlite3` becomes unreachable mid-run (disk full, permissions changed, repository moved on disk)? The run should pause with a human checkpoint rather than corrupting the rest of the cross-repo state.
- **MCP capability tokens with short expiries** for mutation: recommended issuance pattern (`daemon.token.create --capability write --expires-in 1h --repo <id>`), operator UX for granting/revoking, audit trail across the token lifecycle.
- **MCP `tools/call` audit pattern**: every mutating call produces an audit row in the daemon DB with client_id, repo_id when scoped, method, params_hash, decision, denial_reason from the documented vocabulary, transport, hash chain continuity. Compare with the RPC audit shape from dogfood-034 (`rpc_request_log` + audit chain); reuse where structurally identical, extend where MCP needs distinct fields.
- **Operator UX for granting / revoking apply tokens** scoped to one repository within a cross-repo workflow: how does the operator preview which capabilities are needed for a cross-repo workflow before granting tokens? Recommendation: `striatum daemon describe --workflow <path>` listing required capabilities per participating repo.
- **Adversarial test cases**:
  - hostile MCP client requesting `tools/list` then `tools/call` with elevated args
  - prompt-injected MCP client claiming "trusted" identity
  - capability token leaked across repos (token scoped to repo A used against repo B → refuse + audit `capability_missing`)
  - daemon crash mid-cross-repo-prepare (reconciliation: complete or roll back)
  - one participating repo unregistered mid-run (pause + human checkpoint)
  - cross-repo cycle iteration accounting (max_iterations is global to the cycle)
  - per-repo write-scope bypass attempt (job targeting repo B writes to repo A path)
  - audit chain tamper attempt via daemon API (role-enforced append-only refuses)
- **Documentation for `docs/HOW_TO_HUMAN.md`**: cross-repo workflow walkthrough — register two repos, write a cross-repo workflow.json, prepare and start, dashboard --all to see both, cancel cleanly.

**Multi-repo / cross-repo END-TO-END integration testing is EXPLICITLY DEFERRED** to a follow-up RFC (`docs/TODO.md` Open item 19). Your design should specify cross-platform unit-level + mock-based coverage where applicable without authoring a multi-repo daemon harness.

State which parts of the design require platform-specific work and which are cross-platform. Treat Windows-native daemon as deferred per dogfood-034 scope.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim.

- Plain Markdown line, NO bold (`**`), NO italics, NO heading prefix, NO quotes.
- Lowercase `author:` exactly.
- Correct: `author: designer-gemini-pro-001`
- Wrong: `**Author:** designer-gemini-pro-001` (this is what failed in dogfood-031 and dogfood-033 — do not repeat it)
- Wrong: `Author: designer-gemini-pro-001` (capital A)
- Wrong: `author: "designer-gemini-pro-001"` (quoted)

If you produce schema-bearing artifacts (synthesis, finding), the file must start with a JSON-encoded `key: <value>` front matter block. Example for `finding`:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0032"]
---
```

The byline appears AFTER the front matter block and a blank line, not inside it.

Do not call striatum CLI; the operator publishes on your behalf otherwise.
