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

// lanePGSocketHardenedEnv is the current env-backed form of RFC 0110's
// security.pg_socket_hardened flag. When enabled, doctor treats lane PG
// reachability as a blocking problem instead of an advisory warning.
const lanePGSocketHardenedEnv = "STRIATUM_SECURITY_PG_SOCKET_HARDENED"

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
// By default this is advisory like codexDoctorBlock: it emits a
// `lane_pg_reachable` warning and reads NO DSN/token value — only configuration
// posture. Under the RFC 0110 secure profile, it returns a hard problem so
// doctor fails closed. The finding fires because a lane that runs as the
// daemon's own OS user can open the daemon's Postgres directly via unix-socket
// peer auth, bypassing the artifact API (the #87 incident). The DSN-leak half
// is already closed (the supervised-lane env allowlist drops every DSN/PG*
// var); this is the residual same-OS-user reachability.
//
// The honest full close is the OS-user adoption in docs/how-to/lane-sandbox.md:
// run lanes as a dedicated unprivileged user that has NO PostgreSQL role and is
// denied by pg_hba. An operator who has adopted it sets STRIATUM_LANE_OS_USER to
// that (existing, distinct) user, which clears the warning. This is a
// best-effort configuration proxy, not a live PG probe — labeled as such.
func laneSandboxDoctorBlock() (map[string]any, []string, []string) {
	daemonUser := currentUsername()
	laneUser := strings.TrimSpace(os.Getenv(laneOSUserEnv))
	hardened := envBool(lanePGSocketHardenedEnv)
	block := map[string]any{
		"checked":                    true,
		"daemon_user":                daemonUser,
		"proxy":                      "configuration posture (no live PostgreSQL probe)",
		"pg_socket_hardened":         hardened,
		"enforcement":                "advisory",
		"launch_user_env":            laneOSUserEnv,
		"daemon_launch_enforced":     false,
		"daemon_launch_mechanism":    "same_os_user",
		"requires_passwordless_sudo": false,
	}
	if hardened {
		block["enforcement"] = "blocking"
	}
	if laneUser != "" {
		block["lane_os_user"] = laneUser
	}

	// A configured lane user must actually exist and differ from the daemon's
	// user to count as isolation; otherwise it is not adopted (or misconfigured).
	if laneUser != "" && laneUser != daemonUser {
		if lookupOSUser(laneUser) {
			block["lane_pg_isolated"] = true
			block["daemon_launch_enforced"] = true
			block["daemon_launch_mechanism"] = "sudo -n -u " + laneUser
			block["requires_passwordless_sudo"] = true
			return block, nil, nil
		}
		block["lane_pg_isolated"] = false
		message := "lane_pg_reachable: " + laneOSUserEnv + "=" + laneUser +
			" but no such OS user exists; supervised lanes still run as the daemon's user (" + daemonUser +
			") and can open the daemon's PostgreSQL directly. See docs/how-to/lane-sandbox.md (#87)."
		return block, laneSandboxWarning(message, hardened), laneSandboxProblem(message, hardened)
	}

	block["lane_pg_isolated"] = false
	message := "lane_pg_reachable: supervised lanes run as the daemon's OS user (" + daemonUser +
		") and can open the daemon's PostgreSQL directly via unix-socket peer auth, bypassing the artifact API. " +
		"Adopt a dedicated PG-less lane OS user (set " + laneOSUserEnv +
		") per docs/how-to/lane-sandbox.md to close this (#87)."
	return block, laneSandboxWarning(message, hardened), laneSandboxProblem(message, hardened)
}

func laneSandboxWarning(message string, hardened bool) []string {
	if hardened {
		return nil
	}
	return []string{message}
}

func laneSandboxProblem(message string, hardened bool) []string {
	if !hardened {
		return nil
	}
	return []string{message}
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
