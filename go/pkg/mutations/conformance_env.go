package mutations

// This file exposes a tight, read-only accessor so the hermetic
// adapter-conformance harness (go/pkg/adapterconformance) can assert the
// supervised lane-env hardening (RFC 0096 §2 / #87) over THIS package's real
// allowlist + builder, without duplicating it. It adds NO behavior: it is a
// thin projection of supervisedEnvPassThrough + supervisedEnvEntries +
// mergeEnvReplacing, the exact composition the production supervisedEnv uses.
// RFC 0101 Layer 2, Slice 1 (C2 LaneEnvHardened golden).

// SupervisedLaneEnv builds the full environment a supervised lane process would
// receive, given an explicit base environment (the daemon's os.Environ() in
// production) and the bare CLI adapter name (e.g. "claude"). The conformance C2
// golden passes a base that includes banned secret-bearing vars (DATABASE_URL,
// PG*, *POSTGRES*, *DSN*) and asserts that:
//
//   - the required keys survive (PATH, HOME, STRIATUM_MCP_URL, the bearer token
//     material, and the run/session/supervisor/repo/lane id vars);
//   - the per-adapter hardening keys survive (e.g. the #101 claude
//     welcome/update-nag suppression: DISABLE_AUTOUPDATER,
//     CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC); and
//   - none of the banned keys survive — a leak is an EnvSecretLeak.
//
// It is exported solely for hermetic assertion: it composes the production
// supervisedEnvPassThrough allowlist filter with supervisedEnvEntries via
// mergeEnvReplacing exactly as supervisedEnv does, but takes the base env and
// adapter as parameters (instead of reading os.Environ() / config) so the test
// controls them deterministically. Behavior is unchanged from supervisedEnv.
func SupervisedLaneEnv(adapter string, base []string, repoRoot, repositoryID, runID, sessionID, supervisorID, laneID string) []string {
	entries := supervisedEnvEntries(adapter, repoRoot, repositoryID, runID, sessionID, supervisorID, laneID)
	return mergeEnvReplacing(supervisedEnvPassThrough(base), entries)
}
