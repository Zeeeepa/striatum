---
schema_version: "striatum.receipt.v1"
artifact_kind: "receipt"
check_id: "builtin:go-build"
argv: ["/home/halbritt/.local/bin/go", "build", "-o", "/tmp/verifier-scratch-1131078646/gobuild-out", "./..."]
binary_sha256: "69596b260ecc0682fec99928e521f57e6fd8e8dc02f90de08bb5db69ae079f93"
exit_code: 0
stdout_sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
cwd_tree_sha: "a94dff8c0346217e8348bdb682d6d20b52ae749a6ba457121dc1b7542e569156"
seal_digest: "16250e793f5697f8e11f8e5151ec08e393e34882dd2c9d35b897739e5dbe84b4"
created_at: "2026-06-20T14:05:27Z"
---

# Verifier receipt

- check_id: `builtin:go-build`
- exit_code: `0`
- sandbox_mechanism: `bubblewrap`
- sandbox_strict: `true`
- independent_reexecution_agreement: `true`
- builtin_id: `builtin:go-build`
- striatum_version: `unknown`
- negative_control_void: `false`
