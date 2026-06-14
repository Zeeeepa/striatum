package mutations

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/pgtest"
)

// completingSpawnExecutor is the hermetic stand-in for the production spawn path
// (register + supervise.start). It registers a session under the owner context
// the scheduler hands it (proving owner attribution), attests it the way
// supervise.start would (minus the OS subprocess — the only leg that needs live
// verification), then drives that lane's OWN work loop (claim -> ack -> complete)
// to completion. Those work.* calls are lane RPCs, not operator orchestration:
// the gate's property is that NO operator session.register / supervise.start
// happens after run.start — the daemon scheduler issues them, attributed to the
// captured run owner.
type completingSpawnExecutor struct {
	owners []string // owner principal attributed to each spawn (C1 evidence)
	spawns []string // role/lane spawned, in order
}

func (e *completingSpawnExecutor) Spawn(ctx context.Context, runner db.Runner, repositoryID, runID, role, lane string) (string, error) {
	e.owners = append(e.owners, db.AuthorityFromContext(ctx).PrincipalID)
	e.spawns = append(e.spawns, role+"/"+lane)

	reg, err := HandleRegisterSession(ctx, runner, schedulerEnvelope(repositoryID, map[string]any{
		"run_id": runID,
		"role":   role,
		"lane":   lane,
		"fresh":  true,
	}))
	if err != nil {
		return "", err
	}
	sessionID := fmt.Sprint(reg["session_id"])
	if err := attestSchedulerSession(ctx, runner, repositoryID, runID, sessionID, lane); err != nil {
		return sessionID, err
	}

	// Drive the lane's own work loop. A real supervised lane does exactly this;
	// here it runs inline so the hermetic DAG reaches terminal deterministically.
	claim, err := HandleClaimNext(ctx, runner, schedulerEnvelope(repositoryID, map[string]any{"session_id": sessionID}))
	if err != nil {
		return sessionID, err
	}
	if fmt.Sprint(claim["status"]) != "claimed" {
		// No work for this lane right now; the next sweep re-derives.
		return sessionID, nil
	}
	packet := asMap(claim["packet"])
	lease := asMap(packet["lease"])
	jobBlock := asMap(packet["job"])
	jobID := fmt.Sprint(jobBlock["job_id"])
	leaseID := fmt.Sprint(lease["lease_id"])
	messageID := fmt.Sprint(lease["message_id"])

	if _, err := HandleAckWork(ctx, runner, schedulerEnvelope(repositoryID, map[string]any{
		"session_id": sessionID,
		"message_id": messageID,
		"lease_id":   leaseID,
	})); err != nil {
		return sessionID, err
	}
	if _, err := HandleCompleteWork(ctx, runner, schedulerEnvelope(repositoryID, map[string]any{
		"session_id": sessionID,
		"job_id":     jobID,
		"lease_id":   leaseID,
		"summary":    "scheduler-driven lane complete",
	})); err != nil {
		return sessionID, err
	}
	return sessionID, nil
}

// attestSchedulerSession inserts the supervisor/pointer/daemon-supervisor rows a
// real supervise.start would create, so the claim backend gate (dbf2013b) admits
// the session. It mirrors the conformance harness's AfterRegister attestation,
// minus *testing.T (it runs inside the executor).
func attestSchedulerSession(ctx context.Context, runner db.Runner, repositoryID, runID, sessionID, lane string) error {
	pid := os.Getpid()
	supID := "sup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter, command_json, cwd,
		  scratch_path, pid, state, started_at
		) VALUES ($1,$2,$3,$4,$5,'[]'::jsonb,'/tmp','/tmp/scratch',$6,'attached',now())`,
		repositoryID, supID, runID, sessionID, lane, pid); err != nil {
		return err
	}
	dsupID := "dsup_" + sessionID
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id, session_id,
		  pid, pid_start_time, state, updated_at, metadata_json
		) VALUES ($1,$2,$3,$4,$5,$6,'','attached',now(),'{}'::jsonb)`,
		repositoryID, supID, dsupID, runID, sessionID, pid); err != nil {
		return err
	}
	return runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_supervisors (
		  daemon_supervisor_id, repository_id, run_id, session_id, repo_supervisor_id,
		  daemon_instance_id, adapter, command_json, command_sha256, cwd, pid,
		  pid_start_time, state, started_at, heartbeat_at
		) VALUES ($1,$2,$3,$4,$5,'inst',$6,'[]'::jsonb,'sha','/tmp',$7,'','attached',now(),now())`,
		dsupID, repositoryID, runID, sessionID, supID, lane, pid)
}

// TestSchedulerDrivesDAGToTerminalNoOperatorRPC is the RFC 0105 gate extension:
// a hermetic auto_spawn DAG is driven to terminal state ENTIRELY by the daemon
// scheduler — after run.start there is no operator session.register or
// supervise.start. The two-node linear DAG (author -> builder) exercises the
// full chain: scheduler spawns the root, the lane completes it, completion
// unblocks the downstream job, the next sweep spawns it, and the run finalizes to
// completed. Every spawn is attributed to the captured run owner (C1, C5).
func TestSchedulerDrivesDAGToTerminalNoOperatorRPC(t *testing.T) {
	runner := pgtest.Pool(t).Runner
	ctx := context.Background()
	repoID, runID := "repo_sched_gate", "run_sched_gate"
	owner := "client_owner_gate"
	t.Setenv(supervisedLaneOSUserEnv, "")

	intgSeedRepo(t, ctx, runner, repoID)
	intgSeedRun(t, ctx, runner, repoID, runID, map[string]any{
		"workflow_id": "wf",
		// Workflow-wide auto_spawn default: both lanes inherit it.
		"supervision": map[string]any{"auto_spawn": true},
		"roles":       map[string]any{"author": map[string]any{}, "builder": map[string]any{}},
		"lanes": map[string]any{
			"codex":  map[string]any{"capabilities": []any{"write"}},
			"claude": map[string]any{"capabilities": []any{"write"}},
		},
		"jobs": []any{
			map[string]any{"id": "draft", "type": "draft", "role_id": "author", "lane_id": "codex"},
			map[string]any{"id": "build", "type": "build", "role_id": "builder", "lane_id": "claude"},
		},
		"edges": []any{map[string]any{"from": "draft", "to": "build", "on": "completed"}},
	})
	if err := runner.Exec(ctx, `UPDATE striatumd.runs SET state='ready' WHERE repository_id=$1 AND run_id=$2`, repoID, runID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	// Root job (no deps) + downstream job blocked on it.
	insertSchedulerJob(t, ctx, runner, repoID, runID, "job_draft", "draft", "author", "codex", "blocked")
	insertSchedulerJob(t, ctx, runner, repoID, runID, "job_build", "build", "builder", "claude", "blocked")
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.job_dependencies (repository_id, job_id, depends_on_job_id, gate_json)
		VALUES ($1,'job_build','job_draft','{"on":"completed"}'::jsonb)`, repoID); err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	// The ONLY operator RPC: run.start, which enqueues the root and captures the
	// spawn-authorization grant.
	authCtx, env := ownerAuthEnv(repoID, owner, map[string]any{"run_id": runID})
	if _, err := HandleRunStart(authCtx, runner, env); err != nil {
		t.Fatalf("run.start: %v", err)
	}

	// From here on, ONLY the daemon scheduler acts — no operator session.register
	// or supervise.start.
	exec := &completingSpawnExecutor{}
	terminal := false
	for sweep := 0; sweep < 6 && !terminal; sweep++ {
		if _, err := schedulerSpawnOnce(ctx, runner, repoID, runID, "test", exec); err != nil {
			t.Fatalf("scheduler sweep %d: %v", sweep, err)
		}
		state, err := oneRow(ctx, runner, `SELECT state FROM striatumd.runs WHERE repository_id=$1 AND run_id=$2`, repoID, runID)
		if err != nil {
			t.Fatalf("read run state: %v", err)
		}
		terminal = fmt.Sprint(state["state"]) == "completed"
	}

	if !terminal {
		t.Fatalf("run did not reach completed via the scheduler; spawns=%v", exec.spawns)
	}
	// Both lanes were spawned by the scheduler, in dependency order.
	if len(exec.spawns) != 2 || exec.spawns[0] != "author/codex" || exec.spawns[1] != "builder/claude" {
		t.Fatalf("scheduler spawns = %v, want [author/codex builder/claude]", exec.spawns)
	}
	// Every spawn was attributed to the captured run owner (C1 / C5).
	for i, got := range exec.owners {
		if got != owner {
			t.Fatalf("spawn %d attributed to %q, want %q (C1)", i, got, owner)
		}
	}
	// The grant was revoked at terminal (C2 defense-in-depth, RFC 0122 §6).
	grant, err := oneRow(ctx, runner, `
		SELECT revoked_at FROM striatumd.spawn_authorization_grants
		 WHERE repository_id=$1 AND run_id=$2`, repoID, runID)
	if err != nil {
		t.Fatalf("read grant: %v", err)
	}
	if grant["revoked_at"] == nil {
		t.Fatalf("grant must be revoked once the run completed")
	}
}
