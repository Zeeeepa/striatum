---
schema_version: "striatum.receipt.v1"
artifact_kind: "receipt"
check_id: "builtin:go-test"
argv: ["/home/halbritt/.local/bin/go", "test", "./..."]
binary_sha256: "828c23df6ad53159a6c964c186622d5d1fba8a01a840136b74b08a7a0759a0ab"
exit_code: 1
stdout_sha256: "6b12c575c83dcd4b88fa5135864150c88f5715a451d228d1f66b11a94724bd5a"
cwd_tree_sha: "b25ba44ce5f56405283114d22a26388459ca03bcfa812504a22fd8393f2dae13"
seal_digest: "05f65a4c957a6ce5f395b0388f51642d5a3216b1c8e9756a637bed52996f280c"
created_at: "2026-06-21T17:57:08Z"
---

# Verifier receipt

- check_id: `builtin:go-test`
- exit_code: `1`
- sandbox_mechanism: `bubblewrap`
- sandbox_strict: `true`
- independent_reexecution_agreement: `false`
- builtin_id: `builtin:go-test`
- striatum_version: `unknown`
- negative_control_void: `false`
- working_subdir: `go`
