package mutations

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/db"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
)

func stopTmuxBackedLane(ctx context.Context, metadata map[string]any, identity gosupervisor.TmuxIdentity, panePID int, paneStartToken string) (signal any, note string, fallbackReason string, cleanupSkip string) {
	if strings.TrimSpace(identity.SessionName) == "" {
		if panePID > 0 {
			signal, cleanupSkip = terminateProcessWithStartToken(panePID, paneStartToken)
			return signal, "", "tmux_session_missing", cleanupSkip
		}
		return nil, "tmux_session_missing", "", ""
	}
	_, err := tmuxRunnerForSupervisorMetadata(metadata).Run(ctx, "kill-session", "-t", identity.SessionName)
	if err == nil || tmuxSessionAlreadyGone(err) {
		if err != nil {
			note = string(gosupervisor.TmuxLivenessSessionMissing)
		}
		return "tmux_kill_session", note, "", ""
	}
	if panePID > 0 {
		signal, cleanupSkip = terminateProcessWithStartToken(panePID, paneStartToken)
		return signal, "", string(gosupervisor.TmuxLivenessUnavailable), cleanupSkip
	}
	return nil, "", string(gosupervisor.TmuxLivenessUnavailable), ""
}

func tmuxSessionAlreadyGone(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "can't find session") ||
		strings.Contains(text, "can't find window") ||
		strings.Contains(text, "no server running") ||
		strings.Contains(text, "session not found")
}

// cleanupSupervisorLaneMCPConfig resolves the supervisor's working directory and
// removes/restores any per-launch lane operational files the lane wrote into the
// target work tree: the agy .gemini/settings.json bearer-token file (#62) and
// the Claude .claude/scheduled_tasks.lock (#129). Best-effort: a missing repo
// root or absent files leave nothing to do.
func cleanupSupervisorLaneMCPConfig(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string) {
	repoRoot := supervisorRepoRootFromDB(ctx, runner, repositoryID, supervisorID)
	if repoRoot == "" {
		return
	}
	agentloop.CleanupGeminiSettings(repoRoot, supervisorID)
	agentloop.CleanupClaudeScheduledTasksLock(repoRoot)
}

// supervisorRepoRootFromDB reads the supervisor's recorded working directory
// (cwd), falling back to deriving it from the scratch path layout
// (<repo>/.striatum/scratch/<supervisor_id>).
func supervisorRepoRootFromDB(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string) string {
	var cwd, scratchPath string
	if err := runner.QueryRow(ctx, `
		SELECT cwd, scratch_path
		  FROM striatumd.process_supervisors
		 WHERE repository_id = $1 AND supervisor_id = $2`,
		repositoryID, supervisorID,
	).Scan(&cwd, &scratchPath); err != nil {
		return ""
	}
	if strings.TrimSpace(cwd) != "" {
		return cwd
	}
	if strings.TrimSpace(scratchPath) != "" {
		// scratch_path is <repo>/.striatum/scratch/<supervisor_id>.
		return filepath.Dir(filepath.Dir(filepath.Dir(scratchPath)))
	}
	return ""
}

func terminateProcess(pid int) any {
	if pid <= 0 || !pidAliveLocal(pid) {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}
	signaled := "SIGTERM"
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return nil
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		reapIfChild(pid)
		if !pidAliveLocal(pid) {
			return signaled
		}
		time.Sleep(50 * time.Millisecond)
	}
	signaled = "SIGKILL"
	_ = proc.Signal(syscall.SIGKILL)
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		reapIfChild(pid)
		if !pidAliveLocal(pid) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return signaled
}

func terminateProcessWithStartToken(pid int, expectedStartToken string) (any, string) {
	if pid <= 0 {
		return nil, ""
	}
	expectedStartToken = strings.TrimSpace(expectedStartToken)
	if expectedStartToken == "" {
		return nil, "start_token_missing"
	}
	currentStartToken, ok := processStartToken(pid)
	if !ok || strings.TrimSpace(currentStartToken) == "" {
		return nil, "start_token_unavailable"
	}
	if currentStartToken != expectedStartToken {
		return nil, "start_token_mismatch"
	}
	return terminateProcess(pid), ""
}

func metadataString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func supervisorRunAsUser(metadata map[string]any) string {
	if runAsUser := strings.TrimSpace(metadataString(metadata["run_as_user"])); runAsUser != "" {
		return runAsUser
	}
	if tmux := asMap(metadata["tmux"]); len(tmux) > 0 {
		return strings.TrimSpace(metadataString(tmux["run_as_user"]))
	}
	return ""
}

func tmuxRunnerForSupervisorMetadata(metadata map[string]any) gosupervisor.TmuxRunner {
	runAsUser := supervisorRunAsUser(metadata)
	if runAsUser == "" {
		return supervisionTmuxRunner
	}
	env := []string{"PATH=" + supervisedPath()}
	env = append(env, laneUserIdentityEnv(runAsUser)...)
	return gosupervisor.RunAsTmuxRunner(runAsUser, env)
}

func pidAliveLocal(pid int) bool {
	if pid <= 0 {
		return false
	}
	if processZombieLocal(pid) {
		return false
	}
	err := signalProcessZeroLocal(pid)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func signalProcessZero(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.Signal(0))
}

func processZombieLocal(pid int) bool {
	if runtime.GOOS != "linux" || pid <= 0 {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	return linuxProcStatZombie(data)
}

func linuxProcStatZombie(data []byte) bool {
	text := string(data)
	idx := strings.LastIndex(text, ")")
	if idx < 0 || idx+1 >= len(text) {
		return false
	}
	fields := strings.Fields(text[idx+1:])
	return len(fields) > 0 && fields[0] == "Z"
}

func reapIfChild(pid int) {
	var status syscall.WaitStatus
	_, _ = syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
}
