package reads

import (
	"os"
	"os/user"
	"strings"
)

// laneOSUserEnv names the OS user supervised lanes are spawned as, when the
// operator has adopted the PG-less lane sandbox (docs/how-to/lane-sandbox.md).
// When unset (or equal to the daemon's own user) lanes run as the daemon's OS
// user and can reach the daemon's PostgreSQL directly.
const laneOSUserEnv = "STRIATUM_LANE_OS_USER"

// currentUsername is a var so tests can stub the daemon's resolved OS user.
var currentUsername = func() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if v := strings.TrimSpace(os.Getenv("USER")); v != "" {
		return v
	}
	return ""
}

// lookupOSUser is a var so tests can stub OS-user existence checks.
var lookupOSUser = func(name string) bool {
	_, err := user.Lookup(name)
	return err == nil
}

// laneSandboxDoctorBlock reports whether supervised lanes are isolated from the
// daemon's PostgreSQL by a dedicated PG-less lane OS user (#87 / RFC 0096 §2).
//
// Advisory only (like codexDoctorBlock): it emits a `lane_pg_reachable` warning,
// never a hard `problems` failure, and reads NO DSN/token value — only
// configuration posture. The warning fires because a lane that runs as the
// daemon's own OS user can open the daemon's Postgres directly via unix-socket
// peer auth, bypassing the artifact API (the #87 incident). The DSN-leak half is
// already closed (the supervised-lane env allowlist drops every DSN/PG* var);
// this is the residual same-OS-user reachability.
//
// The honest full close is the OS-user adoption in docs/how-to/lane-sandbox.md:
// run lanes as a dedicated unprivileged user that has NO PostgreSQL role and is
// denied by pg_hba. An operator who has adopted it sets STRIATUM_LANE_OS_USER to
// that (existing, distinct) user, which clears the warning. This is a
// best-effort configuration proxy, not a live PG probe — labeled as such.
func laneSandboxDoctorBlock() (map[string]any, []string) {
	daemonUser := currentUsername()
	laneUser := strings.TrimSpace(os.Getenv(laneOSUserEnv))
	block := map[string]any{
		"checked":     true,
		"daemon_user": daemonUser,
		"proxy":       "configuration posture (no live PostgreSQL probe)",
	}
	if laneUser != "" {
		block["lane_os_user"] = laneUser
	}

	// A configured lane user must actually exist and differ from the daemon's
	// user to count as isolation; otherwise it is not adopted (or misconfigured).
	if laneUser != "" && laneUser != daemonUser {
		if lookupOSUser(laneUser) {
			block["lane_pg_isolated"] = true
			return block, nil
		}
		block["lane_pg_isolated"] = false
		return block, []string{
			"lane_pg_reachable: " + laneOSUserEnv + "=" + laneUser +
				" but no such OS user exists; supervised lanes still run as the daemon's user (" + daemonUser +
				") and can open the daemon's PostgreSQL directly. See docs/how-to/lane-sandbox.md (#87).",
		}
	}

	block["lane_pg_isolated"] = false
	return block, []string{
		"lane_pg_reachable: supervised lanes run as the daemon's OS user (" + daemonUser +
			") and can open the daemon's PostgreSQL directly via unix-socket peer auth, bypassing the artifact API. " +
			"Adopt a dedicated PG-less lane OS user (set " + laneOSUserEnv +
			") per docs/how-to/lane-sandbox.md to close this (#87).",
	}
}
