# Plan TODO 60 Read-Only Git Snapshot

Produce the expected synthesis artifact only. Do not edit source in this job.

Focus on D127: Striatum core may add read-only local Git snapshots first, but
must not autonomously commit, push, call hosted providers, import provider
SDKs, or add telemetry. The plan must include:

- a daemon read method shape for local Git snapshot data;
- exact fields for branch, HEAD, dirty status, changed paths, and ancestry;
- capability, repository-scope, and audit behavior;
- CLI/MCP/UI read-only surfaces;
- tests proving no mutation, no hosted-provider call, and no provider SDK
  dependency;
- how later commit-request or PR-request artifacts stay separate;
- a small first implementation slice with disjoint write scope.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
