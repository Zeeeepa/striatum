package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

const (
	laneUIDPoolEnv               = "STRIATUM_LANE_UID_POOL"
	laneUIDLeaseStateActive      = "active"
	laneUIDLeaseStateScrubbing   = "scrubbing"
	laneUIDLeaseStateQuarantined = "quarantined"
	laneUIDLeaseStateReturned    = "returned"

	laneUIDTaskReaped  = "reaped"
	laneUIDTaskLive    = "live"
	laneUIDTaskUnknown = "unknown"
)

type laneUIDPoolEntry struct {
	User string
	UID  int
	Home string
}

type laneUIDLease struct {
	LeaseID    string
	User       string
	UID        int
	Generation int64
}

var (
	laneUIDLookupUser = func(name string) (*user.User, error) { return user.Lookup(name) }
	laneUIDLookupID   = func(id string) (*user.User, error) { return user.LookupId(id) }
	laneUIDExec       = func(command string, args ...string) error { return exec.Command(command, args...).Run() }
	laneUIDRemoveAll  = os.RemoveAll
	laneUIDReadDir    = os.ReadDir
	laneUIDReadFile   = os.ReadFile
	laneUIDStat       = os.Stat
)

func configuredLaneUIDPool() ([]laneUIDPoolEntry, error) {
	raw := strings.TrimSpace(os.Getenv(laneUIDPoolEnv))
	if raw == "" {
		return nil, nil
	}
	seen := map[int]bool{}
	entries := []laneUIDPoolEntry{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var u *user.User
		var err error
		if allDigits(part) {
			u, err = laneUIDLookupID(part)
		} else {
			u, err = laneUIDLookupUser(part)
		}
		if err != nil {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("%s entry %q does not resolve to an OS user: %v", laneUIDPoolEnv, part, err), nil)
		}
		uid, err := strconv.Atoi(strings.TrimSpace(u.Uid))
		if err != nil || uid <= 0 {
			return nil, rpc.NewError("invalid_transition", fmt.Sprintf("%s entry %q resolved to invalid uid %q", laneUIDPoolEnv, part, u.Uid), nil)
		}
		if seen[uid] {
			continue
		}
		seen[uid] = true
		entries = append(entries, laneUIDPoolEntry{User: strings.TrimSpace(u.Username), UID: uid, Home: strings.TrimSpace(u.HomeDir)})
	}
	return entries, nil
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func allocateLaneUIDLeaseInTx(ctx context.Context, tx db.TxRunner, repositoryID string, config *supervisionStartConfig, supervisorID string) (*laneUIDLease, error) {
	pool, err := configuredLaneUIDPool()
	if err != nil {
		return nil, err
	}
	if len(pool) == 0 {
		return nil, nil
	}
	if err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "striatum:lane_uid_leases"); err != nil {
		return nil, err
	}
	rows, err := queryRows(ctx, tx, `
		SELECT pool_uid
		  FROM striatumd.lane_uid_leases
		 WHERE state IN ('active','scrubbing','quarantined')`,
	)
	if err != nil {
		return nil, err
	}
	held := map[int]bool{}
	for _, row := range rows {
		if uid, ok := intValueOptional(row["pool_uid"]); ok {
			held[uid] = true
		}
	}
	var chosen laneUIDPoolEntry
	found := false
	for _, candidate := range pool {
		if !held[candidate.UID] {
			chosen = candidate
			found = true
			break
		}
	}
	if !found {
		return nil, rpc.NewError("lane_uid_pool_exhausted", "no lane uid pool entry is available; active/scrubbing/quarantined leases occupy every configured uid", map[string]any{
			"pool_size": len(pool),
		})
	}
	var maxGeneration *int64
	if err := tx.QueryRow(ctx, `
		SELECT MAX(generation)
		  FROM striatumd.lane_uid_leases
		 WHERE pool_uid = $1`,
		chosen.UID,
	).Scan(&maxGeneration); err != nil {
		return nil, err
	}
	generation := int64(1)
	if maxGeneration != nil {
		generation = *maxGeneration + 1
	}
	leaseID, err := newID("luid")
	if err != nil {
		return nil, err
	}
	now := nowString()
	if err := tx.Exec(ctx, `
		INSERT INTO striatumd.lane_uid_leases (
		  repository_id, lease_id, pool_uid, pool_user, generation,
		  run_id, session_id, supervisor_id, state, leased_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'active',$9)`,
		repositoryID, leaseID, chosen.UID, chosen.User, generation,
		config.RunID, config.SessionID, supervisorID, now,
	); err != nil {
		return nil, err
	}
	config.RunAsUser = chosen.User
	config.LaneUIDLeaseID = leaseID
	config.LaneUID = chosen.UID
	config.LaneUIDGeneration = generation
	return &laneUIDLease{LeaseID: leaseID, User: chosen.User, UID: chosen.UID, Generation: generation}, nil
}

func laneUIDMetadata(config supervisionStartConfig) map[string]any {
	if config.LaneUIDLeaseID == "" || config.LaneUIDGeneration <= 0 {
		return nil
	}
	return map[string]any{
		"lane_uid_lease_id":   config.LaneUIDLeaseID,
		"lane_uid":            config.LaneUID,
		"lane_uid_generation": config.LaneUIDGeneration,
	}
}

func addLaneUIDMetadata(metadata map[string]any, config supervisionStartConfig) {
	for key, value := range laneUIDMetadata(config) {
		metadata[key] = value
	}
}

func laneUIDGenerationEnv(config supervisionStartConfig) []string {
	if config.LaneUIDLeaseID == "" || config.LaneUIDGeneration <= 0 {
		return nil
	}
	return []string{
		"STRIATUM_LANE_UID_LEASE_ID=" + config.LaneUIDLeaseID,
		"STRIATUM_LANE_UID=" + strconv.Itoa(config.LaneUID),
		"STRIATUM_LANE_UID_GENERATION=" + strconv.FormatInt(config.LaneUIDGeneration, 10),
	}
}

func enforceLaneUIDLeaseFreshness(ctx context.Context, runner any, repositoryID string, supervisor supervisorControlRow) error {
	leaseID := metadataString(supervisor.Metadata["lane_uid_lease_id"])
	if leaseID == "" {
		return nil
	}
	wantGeneration, ok := int64ValueOptional(supervisor.Metadata["lane_uid_generation"])
	if !ok || wantGeneration <= 0 {
		return rpc.NewError("lane_uid_generation_missing", "active supervisor metadata is missing lane uid generation", map[string]any{
			"supervisor_id": supervisor.SupervisorID,
			"lease_id":      leaseID,
		})
	}
	row, err := oneRow(ctx, runner, `
		SELECT state, generation, supervisor_id, session_id
		  FROM striatumd.lane_uid_leases
		 WHERE repository_id = $1 AND lease_id = $2`,
		repositoryID, leaseID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return rpc.NewError("lane_uid_lease_missing", "active supervisor references a missing lane uid lease", map[string]any{
			"supervisor_id": supervisor.SupervisorID,
			"lease_id":      leaseID,
		})
	}
	if err != nil {
		return err
	}
	gotGeneration, _ := int64ValueOptional(row["generation"])
	if rowString(row, "state") != laneUIDLeaseStateActive ||
		gotGeneration != wantGeneration ||
		rowString(row, "supervisor_id") != supervisor.SupervisorID ||
		rowString(row, "session_id") != supervisor.SessionID {
		return rpc.NewError("lane_uid_generation_mismatch", "active supervisor lane uid lease is not the active matching generation", map[string]any{
			"supervisor_id":          supervisor.SupervisorID,
			"lease_id":               leaseID,
			"expected_generation":    wantGeneration,
			"observed_generation":    gotGeneration,
			"observed_state":         row["state"],
			"observed_session_id":    row["session_id"],
			"observed_supervisor_id": row["supervisor_id"],
		})
	}
	return nil
}

func laneUIDAttestationOverlay(ctx context.Context, runner any, repositoryID, sessionID string, base map[string]any) map[string]any {
	if base["attested"] != true {
		return base
	}
	supervisorID := strings.TrimSpace(fmt.Sprint(base["supervisor_id"]))
	if supervisorID == "" || supervisorID == "<nil>" {
		return base
	}
	supervisor := supervisorControlRow{SupervisorID: supervisorID, SessionID: sessionID}
	if rows, err := queryRows(ctx, runner, `
		SELECT COALESCE(metadata_json, '{}'::jsonb) AS metadata_json
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND session_id = $2 AND supervisor_id = $3
		 LIMIT 1`,
		repositoryID, sessionID, supervisorID,
	); err == nil && len(rows) > 0 {
		supervisor.Metadata = asMap(rows[0]["metadata_json"])
		if err := enforceLaneUIDLeaseFreshness(ctx, runner, repositoryID, supervisor); err != nil {
			out := copyMap(base)
			out["attested"] = false
			out["state"] = "unattested"
			out["reason"] = "lane_uid_generation_mismatch"
			var rpcErr *rpc.Error
			if errors.As(err, &rpcErr) {
				out["reason"] = rpcErr.Code
				out["error"] = rpcErr.Message
				if len(rpcErr.Details) > 0 {
					out["error_details"] = rpcErr.Details
				}
			} else {
				out["error"] = err.Error()
			}
			return out
		}
	}
	return base
}

func scrubLaneUIDLeasesForSession(ctx context.Context, runner db.Runner, repositoryID, sessionID, reason string) map[string]any {
	repoRoot, _ := activeRepositoryRoot(ctx, runner, repositoryID)
	rows, err := queryRows(ctx, runner, `
		SELECT lease_id, pool_uid, pool_user, generation, supervisor_id
		  FROM striatumd.lane_uid_leases
		 WHERE repository_id = $1 AND session_id = $2
		   AND state IN ('active','scrubbing','quarantined')
		 ORDER BY leased_at, lease_id`,
		repositoryID, sessionID,
	)
	if err != nil || len(rows) == 0 {
		out := map[string]any{"checked": err == nil, "scrubbed_count": 0}
		if err != nil {
			out["error"] = err.Error()
		}
		return out
	}
	items := []map[string]any{}
	for _, row := range rows {
		row["repo_root"] = repoRoot
		items = append(items, scrubLaneUIDLease(ctx, runner, repositoryID, row, reason))
	}
	return map[string]any{
		"checked":        true,
		"scrubbed_count": len(items),
		"leases":         items,
	}
}

func scrubLaneUIDLease(ctx context.Context, runner db.Runner, repositoryID string, row map[string]any, reason string) map[string]any {
	leaseID := rowString(row, "lease_id")
	poolUser := rowString(row, "pool_user")
	poolUID, _ := intValueOptional(row["pool_uid"])
	supervisorID := rowString(row, "supervisor_id")
	repoRoot := rowString(row, "repo_root")
	started := nowString()
	_, _ = withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return map[string]any{}, tx.Exec(ctx, `
			UPDATE striatumd.lane_uid_leases
			   SET state = 'scrubbing', scrub_started_at = $1
			 WHERE repository_id = $2 AND lease_id = $3
			   AND state IN ('active','quarantined')`,
			started, repositoryID, leaseID,
		)
	})
	proof := baseLaneUIDScrubProof(poolUID, poolUser, supervisorID)
	scrubErr := scrubLaneUIDArtifacts(poolUser, supervisorID, repoRoot, proof)
	proofErr := appendLaneUIDScrubProof(ctx, runner, repositoryID, leaseID, poolUID, poolUser, supervisorID, repoRoot, proof)
	if scrubErr == nil {
		scrubErr = proofErr
	} else if proofErr != nil {
		proof["postcondition_failure"] = proofErr.Error()
	}
	status := "clean"
	state := laneUIDLeaseStateReturned
	var failure any
	if scrubErr != nil {
		status = "failed"
		state = laneUIDLeaseStateQuarantined
		failure = scrubErr.Error()
		proof["failure"] = scrubErr.Error()
	}
	proof["reason"] = reason
	proof["scrub_started_at"] = started
	proof["scrub_finished_at"] = nowString()
	proofArg, _ := db.JSONBArg(runner, proof)
	_, _ = withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return map[string]any{}, tx.Exec(ctx, `
			UPDATE striatumd.lane_uid_leases
			   SET state = $1,
			       scrub_status = $2,
			       scrub_proof = $3::jsonb,
			       scrub_failure = $4,
			       returned_at = CASE WHEN $1 = 'returned' THEN $5 ELSE returned_at END
			 WHERE repository_id = $6 AND lease_id = $7`,
			state, status, proofArg, failure, proof["scrub_finished_at"], repositoryID, leaseID,
		)
	})
	return map[string]any{
		"lease_id":      leaseID,
		"pool_uid":      poolUID,
		"pool_user":     poolUser,
		"supervisor_id": supervisorID,
		"state":         state,
		"scrub_status":  status,
		"scrub_failure": failure,
		"proof":         proof,
	}
}

func baseLaneUIDScrubProof(poolUID int, poolUser, supervisorID string) map[string]any {
	return map[string]any{
		"pool_uid":      poolUID,
		"pool_user":     poolUser,
		"supervisor_id": supervisorID,
		"checks":        map[string]any{},
	}
}

func appendLaneUIDScrubProof(ctx context.Context, runner db.Runner, repositoryID, leaseID string, poolUID int, poolUser, supervisorID, repoRoot string, proof map[string]any) error {
	checks := proof["checks"].(map[string]any)
	observations, blocking, err := laneUIDProcessObservations(poolUID)
	checks["p1_processes_for_uid"] = map[string]any{
		"observed_count": len(observations),
		"blocking_count": len(blocking),
		"observations":   observations,
		"blocking":       blocking,
	}
	if err != nil {
		return err
	}
	if len(blocking) > 0 {
		return fmt.Errorf("uid %d still owns non-reaped or unknown processes: %v", poolUID, blocking)
	}
	if err := proveLaneUIDTmuxSocketAbsent(poolUID, checks); err != nil {
		return err
	}
	if err := proveLaneUIDHomeCleanup(poolUser, checks); err != nil {
		return err
	}
	if err := proveLaneUIDSupervisorScratchAbsent(supervisorID, repoRoot, checks); err != nil {
		return err
	}
	if err := proveLaneUIDWorkspaceCleanup(ctx, runner, repositoryID, leaseID, checks); err != nil {
		return err
	}
	checks["p5_complete_proof"] = true
	return nil
}

func scrubLaneUIDArtifacts(poolUser, supervisorID, repoRoot string, proof map[string]any) error {
	checks := proof["checks"].(map[string]any)
	var firstErr error
	recordErr := func(err error) {
		if firstErr == nil && err != nil {
			firstErr = err
		}
	}
	if poolUser != "" {
		if err := laneUIDExec("sudo", "-n", "-u", poolUser, "kill", "-KILL", "-1"); err != nil {
			checks["s1_kill_all"] = "failed: " + err.Error()
			recordErr(fmt.Errorf("kill lane uid process domain for %s: %w", poolUser, err))
		} else {
			checks["s1_kill_all"] = "ok"
		}
		if home := laneOSUserHome(poolUser); home != "" {
			paths := []string{
				filepath.Join(home, ".codex"),
				filepath.Join(home, ".claude"),
				filepath.Join(home, ".striatum"),
			}
			removed := []string{}
			for _, path := range paths {
				if err := laneUIDRemoveAll(path); err != nil {
					checks["s2_s3_remove_failed"] = map[string]any{"path": path, "error": err.Error()}
					recordErr(fmt.Errorf("remove lane uid home path %s: %w", path, err))
					continue
				}
				removed = append(removed, path)
			}
			checks["s2_s3_home_delete"] = removed
		}
	}
	if supervisorID != "" {
		checks["s3_private_scratch_supervisor_id"] = supervisorID
		if repoRoot != "" {
			path := filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)
			if err := laneUIDRemoveAll(path); err != nil {
				checks["s3_private_scratch_delete"] = "failed: " + err.Error()
				recordErr(fmt.Errorf("remove supervisor scratch %s: %w", path, err))
			} else {
				checks["s3_private_scratch_delete"] = path
			}
		}
	}
	return firstErr
}

func proveLaneUIDTmuxSocketAbsent(poolUID int, checks map[string]any) error {
	if poolUID <= 0 {
		checks["p2_tmux_socket_absent"] = "skipped_no_uid"
		return nil
	}
	path := filepath.Join(os.TempDir(), fmt.Sprintf("tmux-%d", poolUID), "default")
	if err := pathMustBeAbsent(path); err != nil {
		checks["p2_tmux_socket_absent"] = map[string]any{"path": path, "error": err.Error()}
		return fmt.Errorf("tmux socket for uid %d still reachable: %w", poolUID, err)
	}
	checks["p2_tmux_socket_absent"] = path
	return nil
}

func proveLaneUIDHomeCleanup(poolUser string, checks map[string]any) error {
	home := laneOSUserHome(poolUser)
	if home == "" {
		checks["p3_p4_home_cleanup"] = "skipped_no_home"
		return nil
	}
	paths := []string{
		filepath.Join(home, ".codex"),
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".striatum"),
		filepath.Join(home, ".striatum", "reseal-token"),
		filepath.Join(home, ".striatum", "reseal_token"),
	}
	for _, path := range paths {
		if err := pathMustBeAbsent(path); err != nil {
			checks["p3_p4_home_cleanup"] = map[string]any{"path": path, "error": err.Error()}
			return fmt.Errorf("lane uid home cleanup proof failed for %s: %w", path, err)
		}
	}
	checks["p3_p4_home_cleanup"] = map[string]any{
		"home":          home,
		"absent_paths":  paths,
		"reseal_absent": true,
	}
	return nil
}

func proveLaneUIDSupervisorScratchAbsent(supervisorID, repoRoot string, checks map[string]any) error {
	if strings.TrimSpace(supervisorID) == "" || strings.TrimSpace(repoRoot) == "" {
		checks["p4_supervisor_scratch_absent"] = "skipped"
		return nil
	}
	path := filepath.Join(repoRoot, ".striatum", "scratch", supervisorID)
	if err := pathMustBeAbsent(path); err != nil {
		checks["p4_supervisor_scratch_absent"] = map[string]any{"path": path, "error": err.Error()}
		return fmt.Errorf("supervisor scratch cleanup proof failed for %s: %w", path, err)
	}
	checks["p4_supervisor_scratch_absent"] = path
	return nil
}

func proveLaneUIDWorkspaceCleanup(ctx context.Context, runner db.Runner, repositoryID, leaseID string, checks map[string]any) error {
	if strings.TrimSpace(leaseID) == "" {
		checks["p5_workspace_acl_cleanup"] = "skipped_no_lease"
		return nil
	}
	rows, err := queryRows(ctx, runner, `
		WITH lane_lease AS (
			SELECT run_id, session_id
			  FROM striatumd.lane_uid_leases
			 WHERE repository_id = $1 AND lease_id = $2
		)
		SELECT 'worktree' AS kind, w.worktree_id AS id, w.worktree_path AS path, w.state
		  FROM striatumd.job_worktrees w
		  JOIN striatumd.leases l
		    ON l.repository_id = w.repository_id AND l.lease_id = w.lease_id
		  JOIN lane_lease ll
		    ON ll.run_id = w.run_id AND ll.session_id = l.owner_session_id
		 WHERE w.repository_id = $1
		   AND w.state NOT IN ('released','removed')
		UNION ALL
		SELECT 'workspace' AS kind, w.workspace_id AS id, w.workspace_path AS path, w.state
		  FROM striatumd.job_workspaces w
		  JOIN striatumd.leases l
		    ON l.repository_id = w.repository_id AND l.lease_id = w.lease_id
		  JOIN lane_lease ll
		    ON ll.run_id = w.run_id AND ll.session_id = l.owner_session_id
		 WHERE w.repository_id = $1
		   AND w.state NOT IN ('released','removed')
		 ORDER BY kind, id`,
		repositoryID, leaseID,
	)
	if err != nil {
		checks["p5_workspace_acl_cleanup"] = map[string]any{"error": err.Error()}
		return err
	}
	checks["p5_workspace_acl_cleanup"] = map[string]any{
		"lease_id":        leaseID,
		"unclean_count":   len(rows),
		"unclean_entries": rows,
	}
	if len(rows) > 0 {
		return fmt.Errorf("lane uid lease %s still has active or abandoned worktrees/workspaces", leaseID)
	}
	return nil
}

func pathMustBeAbsent(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	_, err := laneUIDStat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	return fmt.Errorf("%s still exists", path)
}

func laneUIDProcessObservations(uid int) ([]map[string]any, []map[string]any, error) {
	if uid <= 0 {
		return nil, nil, nil
	}
	entries, err := laneUIDReadDir("/proc")
	if err != nil {
		return nil, nil, fmt.Errorf("read /proc: %w", err)
	}
	observations := []map[string]any{}
	blocking := []map[string]any{}
	for _, entry := range entries {
		if !entry.IsDir() || !allDigits(entry.Name()) {
			continue
		}
		pid, _ := strconv.Atoi(entry.Name())
		statusPath := filepath.Join("/proc", entry.Name(), "status")
		body, err := laneUIDReadFile(statusPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return observations, blocking, fmt.Errorf("read %s: %w", statusPath, err)
		}
		if procStatusHasUID(body, uid) {
			observation := map[string]any{
				"pid": pid,
				"uid": uid,
			}
			classification, state, stateErr := classifyPoolUIDTaskState(entry.Name())
			if state != "" {
				observation["state"] = state
			}
			observation["classification"] = classification
			if stateErr != nil {
				observation["error"] = stateErr.Error()
			}
			observations = append(observations, observation)
			if classification != laneUIDTaskReaped {
				blocking = append(blocking, observation)
			}
		}
	}
	return observations, blocking, nil
}

func procStatusHasUID(body []byte, uid int) bool {
	prefix := []byte("Uid:")
	for _, line := range strings.Split(string(body), "\n") {
		if !strings.HasPrefix(line, string(prefix)) {
			continue
		}
		fields := strings.Fields(line)
		for _, field := range fields[1:] {
			if got, err := strconv.Atoi(field); err == nil && got == uid {
				return true
			}
		}
	}
	return false
}

func classifyPoolUIDTaskState(pid string) (string, string, error) {
	body, err := laneUIDReadFile(filepath.Join("/proc", pid, "stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return laneUIDTaskReaped, "", nil
		}
		return laneUIDTaskUnknown, "", fmt.Errorf("read /proc/%s/stat: %w", pid, err)
	}
	state, err := parseProcStatState(string(body))
	if err != nil {
		return laneUIDTaskUnknown, "", err
	}
	classification, err := classifyProcState(state)
	if err != nil {
		return laneUIDTaskUnknown, string(state), err
	}
	return classification, string(state), nil
}

func parseProcStatState(stat string) (byte, error) {
	end := strings.LastIndex(stat, ")")
	if end < 0 || end+2 >= len(stat) {
		return 0, fmt.Errorf("malformed proc stat")
	}
	state := stat[end+2]
	if state == ' ' && end+3 < len(stat) {
		state = stat[end+3]
	}
	return state, nil
}

func classifyProcState(state byte) (string, error) {
	switch state {
	case 'Z', 'X', 'x':
		return laneUIDTaskReaped, nil
	case 'R', 'S', 'D', 'T', 't', 'I', 'P':
		return laneUIDTaskLive, nil
	default:
		return laneUIDTaskUnknown, fmt.Errorf("unrecognized /proc state %q", state)
	}
}

func recoverLaneUIDLeases(ctx context.Context, runner db.Runner, repositoryID, runID string, dryRun bool, retryQuarantined bool) map[string]any {
	if repositoryID == "" {
		return map[string]any{"checked": false, "skipped": "repository_id required"}
	}
	rows, err := queryRows(ctx, runner, `
		SELECT l.lease_id, l.pool_uid, l.pool_user, l.generation, l.supervisor_id,
		       l.session_id, l.state, l.scrub_started_at,
		       CASE
		         WHEN l.state = 'scrubbing' THEN 'stuck_scrubbing'
		         ELSE 'leaked_active'
		       END AS recovery_reason
		  FROM striatumd.lane_uid_leases l
		  LEFT JOIN striatumd.sessions s
		    ON s.repository_id = l.repository_id AND s.session_id = l.session_id
		  LEFT JOIN striatumd.process_supervisors ps
		    ON ps.repository_id = l.repository_id AND ps.supervisor_id = l.supervisor_id
		 WHERE l.repository_id = $1
		   AND ($2 = '' OR l.run_id = $2)
		   AND (
		     l.state = 'scrubbing'
		     OR (
		       l.state = 'active'
		       AND (
		         s.state IS NULL
		         OR s.state <> 'active'
		         OR ps.state IS NULL
		         OR ps.state NOT IN ('starting','attached','detached')
		       )
		     )
		   )
		 ORDER BY l.leased_at, l.lease_id`,
		repositoryID, runID,
	)
	if err != nil {
		return map[string]any{"checked": true, "error": err.Error()}
	}
	quarantined, qerr := queryRows(ctx, runner, `
		SELECT lease_id, pool_uid, pool_user, generation, supervisor_id,
		       session_id, state, scrub_started_at, scrub_failure
		  FROM striatumd.lane_uid_leases
		 WHERE repository_id = $1
		   AND ($2 = '' OR run_id = $2)
		   AND state = 'quarantined'
		 ORDER BY leased_at, lease_id`,
		repositoryID, runID,
	)
	if qerr != nil {
		return map[string]any{"checked": true, "error": qerr.Error()}
	}
	items := []map[string]any{}
	repoRoot, _ := activeRepositoryRoot(ctx, runner, repositoryID)
	for _, row := range rows {
		if dryRun {
			items = append(items, map[string]any{
				"lease_id": row["lease_id"],
				"state":    row["state"],
				"reason":   row["recovery_reason"],
				"action":   "would_scrub",
			})
			continue
		}
		row["repo_root"] = repoRoot
		items = append(items, scrubLaneUIDLease(ctx, runner, repositoryID, row, "recovery.auto: "+rowString(row, "recovery_reason")))
	}
	quarantineItems := []map[string]any{}
	for _, row := range quarantined {
		if retryQuarantined && !dryRun {
			row["repo_root"] = repoRoot
			quarantineItems = append(quarantineItems, scrubLaneUIDLease(ctx, runner, repositoryID, row, "recovery.operator_retry: quarantined"))
			continue
		}
		quarantineItems = append(quarantineItems, map[string]any{
			"lease_id":      row["lease_id"],
			"pool_uid":      row["pool_uid"],
			"pool_user":     row["pool_user"],
			"supervisor_id": row["supervisor_id"],
			"state":         row["state"],
			"scrub_failure": row["scrub_failure"],
			"action":        "operator_retry_required",
			"retry_hint":    "call recovery.sweep with retry_quarantined_lane_uids=true to rerun P1-P5; the uid returns only on clean proof",
		})
	}
	return map[string]any{
		"checked":           true,
		"count":             len(items),
		"leases":            items,
		"quarantined_count": len(quarantineItems),
		"quarantined":       quarantineItems,
	}
}

func int64ValueOptional(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case float64:
		return int64(v), true
	case json.Number:
		i, err := v.Int64()
		return i, err == nil
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}
