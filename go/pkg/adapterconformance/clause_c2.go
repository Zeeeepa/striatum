package adapterconformance

import (
	"fmt"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/mutations"
)

// Fixed, non-secret placeholder ids the C2 golden feeds to the production env
// builder. The values are irrelevant to the contract; only key presence/absence
// is asserted.
const (
	c2RepoRoot      = "/tmp/conformance-target-repo"
	c2RepositoryID  = "repo_conformance"
	c2RunID         = "run_conformance"
	c2SessionID     = "session_conformance"
	c2SupervisorID  = "sup_conformance"
	c2LaneID        = "lane_conformance"
	c2MCPURL        = "http://127.0.0.1:65535/mcp"
	c2MCPToken      = "conformance-bearer-not-a-real-secret"
	c2HomeValue     = "/home/conformance"
	c2DatabaseURL   = "postgres://leak@127.0.0.1:5432/striatumd"
	c2PostgresDB    = "striatumd"
	c2PgPassword    = "leaky-pg-password"
	c2StriatumDSN   = "host=127.0.0.1 dbname=striatumd"
	c2GenericSecret = "very-secret-cloud-token"
)

// c2RequiredKeys are the keys the supervised child env MUST carry (DESIGN §1.1
// C2 / §1.4): PATH, HOME, the MCP endpoint, the bearer token material, and the
// run/session/supervisor/repo/lane id vars.
var c2RequiredKeys = []string{
	"PATH",
	"HOME",
	"STRIATUM_MCP_URL",
	"STRIATUM_MCP_TOKEN", // bearer token material
	"STRIATUM_RUN_ID",
	"STRIATUM_SESSION_ID",
	"STRIATUM_SUPERVISOR_ID",
	"STRIATUM_REPO",
	"STRIATUM_REPOSITORY_ID",
	"STRIATUM_LANE_ID",
}

// c2BannedExactKeys are exact key names that must NEVER reach the child env.
var c2BannedExactKeys = []string{
	"DATABASE_URL",
}

// c2BannedSubstrings are substrings; ANY env key containing one (case-folded)
// is banned: PG* / *POSTGRES* / *DSN* and a generic daemon secret family. A
// match is an EnvSecretLeak.
var c2BannedSubstrings = []string{
	"POSTGRES",
	"DSN",
	"SECRET",
	"PASSWORD",
}

// c2BannedPrefixes are key prefixes that must never reach the child (PG*).
var c2BannedPrefixes = []string{
	"PG",
}

// assertC2 is the hermetic LaneEnvHardened golden (DESIGN §1.1 C2 / §1.4 / #87).
// It calls the production supervised env builder (mutations.SupervisedLaneEnv,
// the exported composition of supervisedEnvPassThrough + supervisedEnvEntries +
// mergeEnvReplacing) with a controlled base env that contains both the required
// pass-through vars AND a battery of banned secret-bearing vars, then asserts:
//
//   - every required key is present in the child env; and
//   - none of the banned keys (DATABASE_URL, PG*, *POSTGRES*, *DSN*, daemon
//     secrets) survives the allowlist filter.
//
// A surviving banned key is an EnvSecretLeak. No live daemon, CLI, or
// PostgreSQL is involved.
func assertC2(in AssertInput) ClauseResult {
	c := Clause{ID: C2}
	adapter := in.Adapter.Name

	base := in.BaseEnv
	if base == nil {
		base = c2GoldenBaseEnv()
	}

	child := mutations.SupervisedLaneEnv(
		base,
		c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID,
	)
	childKeys := envKeySet(child)

	// (1) required keys present.
	var missing []string
	for _, key := range c2RequiredKeys {
		if !childKeys[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return c.contractFail(adapter, EnvSecretLeak,
			fmt.Sprintf("supervised child env is missing required keys %v (a hardened env must still carry the lane's control-plane wiring)", missing),
			map[string]string{"missing_keys": strings.Join(missing, ",")})
	}

	// (2) no banned key survives.
	if leaked := bannedKeysIn(childKeys); len(leaked) > 0 {
		sort.Strings(leaked)
		return c.contractFail(adapter, EnvSecretLeak,
			fmt.Sprintf("banned secret-bearing key(s) %v reached the supervised child env (#87 allowlist regression)", leaked),
			map[string]string{"leaked_keys": strings.Join(leaked, ",")})
	}

	return c.pass(adapter,
		"supervised child env carries the required control-plane keys and no banned DSN/Postgres/secret var")
}

// c2GoldenBaseEnv builds the controlled base environment the C2 golden feeds to
// the production builder: the required pass-through vars plus a battery of
// banned secret-bearing vars that a leaky allowlist would forward.
func c2GoldenBaseEnv() []string {
	return []string{
		// allowlisted pass-through (must survive):
		"STRIATUM_MCP_URL=" + c2MCPURL,
		"STRIATUM_MCP_TOKEN=" + c2MCPToken,
		"HOME=" + c2HomeValue,
		"USER=conformance",
		"TERM=xterm-256color",
		"PATH=/usr/bin:/bin",
		// banned (must be dropped):
		"DATABASE_URL=" + c2DatabaseURL,
		"POSTGRES_DB=" + c2PostgresDB,
		"PGPASSWORD=" + c2PgPassword,
		"PGHOST=127.0.0.1",
		"STRIATUM_POSTGRES_DSN=" + c2StriatumDSN,
		"STRIATUM_PG_TEST_URL=" + c2DatabaseURL,
		"SOME_CLOUD_SECRET=" + c2GenericSecret,
		"AWS_SECRET_ACCESS_KEY=" + c2GenericSecret,
	}
}

// envKeySet returns the set of keys present in an env slice.
func envKeySet(env []string) map[string]bool {
	keys := make(map[string]bool, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			keys[key] = true
		}
	}
	return keys
}

// bannedKeysIn returns every key in the set that matches a banned exact name,
// prefix, or substring (case-folded for the substring family).
func bannedKeysIn(keys map[string]bool) []string {
	var leaked []string
	for key := range keys {
		if keyIsBanned(key) {
			leaked = append(leaked, key)
		}
	}
	return leaked
}

// keyIsBanned reports whether an env key matches any banned rule.
func keyIsBanned(key string) bool {
	for _, banned := range c2BannedExactKeys {
		if key == banned {
			return true
		}
	}
	for _, prefix := range c2BannedPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	upper := strings.ToUpper(key)
	for _, sub := range c2BannedSubstrings {
		if strings.Contains(upper, sub) {
			return true
		}
	}
	return false
}
