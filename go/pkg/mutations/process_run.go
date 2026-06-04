package mutations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const (
	processRunDefaultTimeoutSeconds = 300
	processRunMaxTimeoutSeconds     = 1800
)

type processRunStartConfig struct {
	RepositoryID string
	ProcessID    string
	RunID        string
	JobID        string
	SessionID    string
	LeaseID      string
	PacketID     string
	RepoRoot     string
	Cwd          string
	LaneID       string
	Command      []string
	StartedAt    string
	Timeout      time.Duration
}

type processRunResult struct {
	State       string
	ExitCode    any
	PID         any
	StartedAt   string
	EndedAt     string
	StdoutHash  string
	StderrHash  string
	StdoutBytes int
	StderrBytes int
	TimedOut    bool
}

func HandleProcessRun(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID, err := requiredStringParam(envelope.Params, "session_id")
	if err != nil {
		return nil, err
	}
	jobID, err := requiredStringParam(envelope.Params, "job_id")
	if err != nil {
		return nil, err
	}
	leaseID, err := requiredStringParam(envelope.Params, "lease_id")
	if err != nil {
		return nil, err
	}
	command, err := processRunCommand(envelope)
	if err != nil {
		return nil, err
	}
	timeoutSeconds := intParam(envelope, "timeout_seconds", processRunDefaultTimeoutSeconds)
	if timeoutSeconds <= 0 {
		return nil, rpc.NewError("schema_invalid", "timeout_seconds must be positive", nil)
	}
	if timeoutSeconds > processRunMaxTimeoutSeconds {
		timeoutSeconds = processRunMaxTimeoutSeconds
	}
	if _, err := enforceSessionBinding(ctx, sessionID); err != nil {
		return nil, err
	}
	processID := strings.TrimSpace(stringParam(envelope, "process_id"))
	if processID == "" {
		processID, err = newID("proc")
		if err != nil {
			return nil, err
		}
	}

	started, err := processRunStart(ctx, runner, repositoryID, sessionID, jobID, leaseID, processID, command, time.Duration(timeoutSeconds)*time.Second)
	if err != nil {
		return nil, err
	}
	result := executeMediatedProcess(ctx, runner, started)
	if err := processRunFinish(ctx, runner, started, result); err != nil {
		return nil, err
	}
	return processRunResultMap(started, result), nil
}

func processRunCommand(envelope rpc.Envelope) ([]string, error) {
	command := stringSliceParam(envelope, "command", "command_json", "args")
	if len(command) == 0 {
		return nil, rpc.NewError("schema_invalid", "process.run requires a non-empty command array or args after --", nil)
	}
	for i, part := range command {
		if strings.TrimSpace(part) == "" {
			return nil, rpc.NewError("schema_invalid", fmt.Sprintf("command entry %d must be non-empty", i), nil)
		}
		command[i] = part
	}
	return command, nil
}

func processRunStart(ctx context.Context, runner db.Runner, repositoryID, sessionID, jobID, leaseID, processID string, command []string, timeout time.Duration) (processRunStartConfig, error) {
	cfg := processRunStartConfig{}
	_, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		started, err := processRunInsertStarting(ctx, tx, repositoryID, sessionID, jobID, leaseID, processID, command, timeout)
		if err != nil {
			return nil, err
		}
		cfg = started
		return map[string]any{"status": "starting", "process_id": processID}, nil
	})
	return cfg, err
}

func processRunInsertStarting(ctx context.Context, runner any, repositoryID, sessionID, jobID, leaseID, processID string, command []string, timeout time.Duration) (processRunStartConfig, error) {
	job, err := rowByID(ctx, runner, repositoryID, "jobs", "job_id", jobID, true)
	if err != nil {
		return processRunStartConfig{}, err
	}
	if _, err := activeLeaseFor(ctx, runner, repositoryID, leaseID, sessionID, jobID); err != nil {
		return processRunStartConfig{}, err
	}
	if currentLease := strings.TrimSpace(fmt.Sprint(job["current_lease_id"])); currentLease != "" && currentLease != leaseID {
		return processRunStartConfig{}, rpc.NewError("lease_error", "lease is not the job's current lease", nil)
	}
	runID := fmt.Sprint(job["run_id"])
	if !processRunAllowed(job) {
		escaped, err := processRunHasEscapeDecision(ctx, runner, repositoryID, runID, command)
		if err != nil {
			return processRunStartConfig{}, err
		}
		if !escaped {
			return processRunStartConfig{}, rpc.NewError("invalid_transition", "process.run requires capability_requirements.process_execution=true on the job or a matching escape decision", nil)
		}
	}
	run, err := rowByID(ctx, runner, repositoryID, "runs", "run_id", runID, false)
	if err != nil {
		return processRunStartConfig{}, err
	}
	packetID, err := processRunPacketID(ctx, runner, repositoryID, sessionID, jobID, leaseID)
	if err != nil {
		return processRunStartConfig{}, err
	}
	repoRoot := fmt.Sprint(run["repo_root"])
	cwd, err := processRunCwd(ctx, runner, repositoryID, jobID, repoRoot)
	if err != nil {
		return processRunStartConfig{}, err
	}
	laneID := ""
	if laneSelector := asMap(job["lane_selector_json"]); laneSelector != nil {
		laneID, _ = laneSelector["lane_id"].(string)
	}
	startedAt := nowString()
	commandArg, err := db.JSONBArg(runner, command)
	if err != nil {
		return processRunStartConfig{}, err
	}
	if err := runner.(interface {
		Exec(context.Context, string, ...any) error
	}).Exec(ctx, `
		INSERT INTO striatumd.process_executions (
		  repository_id, process_id, run_id, job_id, session_id, lease_id,
		  packet_id, adapter, command_json, cwd, scratch_path, stdin_mode,
		  stdio_mode, state, started_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,'none','suppressed','starting',$12)`,
		repositoryID, processID, runID, jobID, sessionID, leaseID, packetID,
		processAdapter(command), commandArg, cwd, processScratchPath(processID), startedAt,
	); err != nil {
		return processRunStartConfig{}, err
	}
	if _, err := appendEvent(ctx, runner.(db.TxRunner), repositoryID, runID, "process.started", sessionID, jobID, nil, nil, leaseID, map[string]any{
		"process_id":     processID,
		"command_sha256": commandHash(command),
	}); err != nil {
		return processRunStartConfig{}, err
	}
	return processRunStartConfig{
		RepositoryID: repositoryID,
		ProcessID:    processID,
		RunID:        runID,
		JobID:        jobID,
		SessionID:    sessionID,
		LeaseID:      leaseID,
		PacketID:     packetID,
		RepoRoot:     repoRoot,
		Cwd:          cwd,
		LaneID:       laneID,
		Command:      append([]string(nil), command...),
		StartedAt:    startedAt,
		Timeout:      timeout,
	}, nil
}

func processRunAllowed(job map[string]any) bool {
	requirements := asMap(job["capability_requirements_json"])
	return requirements["process_execution"] == true || requirements["test_execution"] == true
}

func processRunHasEscapeDecision(ctx context.Context, runner any, repositoryID, runID string, command []string) (bool, error) {
	actionText := strings.Join(command, " ")
	actionHash := "sha256:" + commandHash(command)
	rows, err := queryRows(ctx, runner, `
		SELECT 1
		  FROM striatumd.events
		 WHERE repository_id = $1
		   AND run_id = $2
		   AND event_type = 'decision.recorded'
		   AND payload_json->>'escape_decision' = 'true'
		   AND payload_json->>'escape_surface' = ANY($3)
		   AND payload_json->>'escape_action' = ANY($4)
		 LIMIT 1`,
		repositoryID,
		runID,
		[]string{"process.run", "shell_command"},
		[]string{actionText, actionHash},
	)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func processRunPacketID(ctx context.Context, runner any, repositoryID, sessionID, jobID, leaseID string) (string, error) {
	rows, err := queryRows(ctx, runner, `
		SELECT packet_id
		  FROM striatumd.work_packets
		 WHERE repository_id = $1
		   AND session_id = $2
		   AND job_id = $3
		   AND lease_id = $4
		 ORDER BY created_at DESC, packet_id DESC
		 LIMIT 1`, repositoryID, sessionID, jobID, leaseID)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", rpc.NewError("not_found", "process.run requires a delivered work packet for the active lease", nil)
	}
	return fmt.Sprint(rows[0]["packet_id"]), nil
}

func processRunCwd(ctx context.Context, runner any, repositoryID, jobID, repoRoot string) (string, error) {
	activeWorktree, err := activeWorktreeForJob(ctx, runner, repositoryID, jobID)
	if err != nil {
		return "", err
	}
	if activeWorktree == nil {
		return repoRoot, nil
	}
	worktreePath := strings.TrimSpace(fmt.Sprint(activeWorktree["worktree_path"]))
	if worktreePath == "" {
		return repoRoot, nil
	}
	if filepath.IsAbs(worktreePath) {
		return filepath.Clean(worktreePath), nil
	}
	return filepath.Clean(filepath.Join(repoRoot, worktreePath)), nil
}

func processScratchPath(processID string) string {
	return filepath.ToSlash(filepath.Join(".striatum", "scratch", "process-executions", processID))
}

func processAdapter(command []string) string {
	if len(command) == 0 {
		return "process"
	}
	name := strings.TrimSpace(filepath.Base(command[0]))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "process"
	}
	return name
}

func executeMediatedProcess(ctx context.Context, runner db.Runner, cfg processRunStartConfig) processRunResult {
	startedAt := cfg.StartedAt
	if strings.TrimSpace(startedAt) == "" {
		startedAt = nowString()
	}
	execCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	var stdout, stderr processOutputDigest
	cmd := exec.CommandContext(execCtx, cfg.Command[0], cfg.Command[1:]...)
	cmd.Dir = cfg.Cwd
	cmd.Env = supervisedEnv(processAdapter(cfg.Command), cfg.RepoRoot, cfg.RepositoryID, cfg.RunID, cfg.SessionID, cfg.ProcessID, cfg.LaneID, "")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	result := processRunResult{State: "failed", StartedAt: startedAt}
	if err := cmd.Start(); err != nil {
		result.EndedAt = nowString()
		fillProcessOutputDigests(&result, stdout, stderr)
		return result
	}
	pid := cmd.Process.Pid
	result.PID = pid
	if err := processRunMarkRunning(ctx, runner, cfg, pid); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		result.EndedAt = nowString()
		fillProcessOutputDigests(&result, stdout, stderr)
		return result
	}
	waitErr := cmd.Wait()
	result.EndedAt = nowString()
	timedOut := errors.Is(execCtx.Err(), context.DeadlineExceeded)
	result.TimedOut = timedOut
	switch {
	case timedOut:
		result.State = "timed_out"
	case waitErr == nil:
		result.State = "exited"
		result.ExitCode = 0
	default:
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			result.State = "exited"
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.State = "failed"
		}
	}
	fillProcessOutputDigests(&result, stdout, stderr)
	return result
}

func processRunMarkRunning(ctx context.Context, runner db.Runner, cfg processRunStartConfig, pid int) error {
	_, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if err := tx.Exec(ctx, `
			UPDATE striatumd.process_executions
			   SET state = 'running', pid = $1
			 WHERE repository_id = $2 AND process_id = $3 AND state = 'starting'`,
			pid, cfg.RepositoryID, cfg.ProcessID,
		); err != nil {
			return nil, err
		}
		return map[string]any{"status": "running", "process_id": cfg.ProcessID}, nil
	})
	return err
}

func processRunFinish(ctx context.Context, runner db.Runner, cfg processRunStartConfig, result processRunResult) error {
	_, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if err := tx.Exec(ctx, `
			UPDATE striatumd.process_executions
			   SET state = $1, exit_code = $2, ended_at = $3
			 WHERE repository_id = $4 AND process_id = $5`,
			result.State, result.ExitCode, result.EndedAt, cfg.RepositoryID, cfg.ProcessID,
		); err != nil {
			return nil, err
		}
		payload := map[string]any{
			"process_id":     cfg.ProcessID,
			"state":          result.State,
			"exit_code":      result.ExitCode,
			"timed_out":      result.TimedOut,
			"stdout_sha256":  result.StdoutHash,
			"stderr_sha256":  result.StderrHash,
			"stdout_bytes":   result.StdoutBytes,
			"stderr_bytes":   result.StderrBytes,
			"command_sha256": commandHash(cfg.Command),
		}
		if _, err := appendEvent(ctx, tx, cfg.RepositoryID, cfg.RunID, "process.completed", cfg.SessionID, cfg.JobID, nil, nil, cfg.LeaseID, payload); err != nil {
			return nil, err
		}
		return map[string]any{"status": result.State}, nil
	})
	return err
}

func processRunResultMap(cfg processRunStartConfig, result processRunResult) map[string]any {
	return map[string]any{
		"status":         result.State,
		"process_id":     cfg.ProcessID,
		"run_id":         cfg.RunID,
		"job_id":         cfg.JobID,
		"session_id":     cfg.SessionID,
		"lease_id":       cfg.LeaseID,
		"packet_id":      cfg.PacketID,
		"cwd":            cfg.Cwd,
		"exit_code":      result.ExitCode,
		"timed_out":      result.TimedOut,
		"stdout_sha256":  result.StdoutHash,
		"stderr_sha256":  result.StderrHash,
		"stdout_bytes":   result.StdoutBytes,
		"stderr_bytes":   result.StderrBytes,
		"command_sha256": commandHash(cfg.Command),
		"started_at":     result.StartedAt,
		"ended_at":       result.EndedAt,
	}
}

func commandHash(command []string) string {
	sum := sha256.Sum256([]byte(strings.Join(command, "\x00")))
	return hex.EncodeToString(sum[:])
}

func fillProcessOutputDigests(result *processRunResult, stdout, stderr processOutputDigest) {
	result.StdoutHash = stdout.HashHex()
	result.StderrHash = stderr.HashHex()
	result.StdoutBytes = stdout.Total
	result.StderrBytes = stderr.Total
}

type processOutputDigest struct {
	Total int
	Hash  hash.Hash
}

func (b *processOutputDigest) Write(p []byte) (int, error) {
	if b.Hash == nil {
		b.Hash = sha256.New()
	}
	_, _ = b.Hash.Write(p)
	b.Total += len(p)
	return len(p), nil
}

func (b processOutputDigest) HashHex() string {
	if b.Hash == nil {
		sum := sha256.Sum256(nil)
		return hex.EncodeToString(sum[:])
	}
	return hex.EncodeToString(b.Hash.Sum(nil))
}
