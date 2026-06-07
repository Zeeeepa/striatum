package mutations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/agentloop"
	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/lanehealth"
	"github.com/halbritt/striatum/go/pkg/rpc"
	gosupervisor "github.com/halbritt/striatum/go/pkg/supervisor"
	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	supervisionTransportPipe      = "pipe"
	supervisionTransportPTYHelper = "pty_helper"

	stdinDeliveryPersistentFIFO = "persistent_fifo"
	stdinDeliveryOneShotEOF     = "one_shot_eof"

	agentLoopModeSelfDriving = "self_driving"
	supervisedLaneOSUserEnv  = "STRIATUM_LANE_OS_USER"
	// agentLoopModePush labels a supervised lane that does NOT use the agent loop:
	// a stdin-FIFO/push consumer that reads a delivered packet then runs the agent,
	// rather than a true self-driver that calls work.await_packet. Recording the
	// honest mode (#146) keeps sessionHasSelfDrivingSupervisor — and therefore the
	// claim-next hint — accurate: a push lane needs `supervise send`, so it must not
	// receive the self_driving "do not run supervise send" note.
	agentLoopModePush = "supervised_push"
)

type supervisionStartConfig struct {
	SessionID          string
	RepositoryID       string
	RunID              string
	LaneID             string
	RepoRoot           string
	WorkflowSnapshotID string
	Command            []string
	OriginalCommand    []string
	AgentLoopMode      string
	Transport          string
	StdinDelivery      string
	RequireTmux        bool
	RunAsUser          string
	// CapabilityToken is the session-BOUND MCP capability token minted for this
	// lane at supervise start (RFC 0096 V2 / #135). It is injected into the lane
	// env as STRIATUM_MCP_TOKEN so the lane authenticates as its OWN session and
	// the cross-session impersonation guard (enforceSessionBinding) bites in live
	// runs. The plaintext lives only in memory → lane env; it is never returned to
	// any RPC caller. Empty only in dev/test paths that do not mint (the lane then
	// gets no injected token, which fails loudly rather than silently inheriting
	// the daemon's shared operator override).
	CapabilityToken string
}

// adapterName returns the bare CLI adapter name of the lane (e.g. "claude"),
// derived from the RAW lane argv0 (OriginalCommand) rather than the possibly
// agent-loop–wrapped Command — so per-adapter env hardening
// (supervisedAdapterEnvEntries / #101) keys off the real child CLI, not the
// "striatumd -agent-loop" wrapper. Uses the same canonical argv→adapter mapping
// as the agent-loop wiring (agentloop.LaneAdapterName).
func (c supervisionStartConfig) adapterName() string {
	argv := c.OriginalCommand
	if len(argv) == 0 {
		argv = c.Command
	}
	if len(argv) == 0 {
		return ""
	}
	return agentloop.LaneAdapterName(argv[0])
}

type supervisorControlRow struct {
	SupervisorID       string
	RunID              string
	SessionID          string
	State              string
	ScratchPath        string
	StdinPipePath      string
	PID                int
	HasPID             bool
	PIDStartTime       string
	DaemonSupervisorID string
	Metadata           map[string]any
	EndedAt            any
	StopReason         any
}

type supervisionPacketRow struct {
	PacketID  string
	RunID     string
	JobID     string
	LeaseID   string
	SessionID string
	Packet    map[string]any
}

type supervisionLaunchResult struct {
	PID                 int
	PIDStartTime        string
	HelperPID           int
	HelperPIDStartTime  string
	Metadata            map[string]any
	InitialHelperEvents []map[string]any
	InitialHelperOffset int
}

var (
	supervisionMkfifo = func(path string) error {
		return syscall.Mkfifo(path, 0o600)
	}
	supervisionLaunch         = launchSupervisedProcess
	supervisionRebridgeLaunch = launchRebridgeHelper
	supervisionWrite          = writeSupervisorPayload
	supervisionTmuxRunner     = gosupervisor.DefaultTmuxRunner()
	signalProcessZeroLocal    = signalProcessZero
	errSupervisorPipeNoReader = errors.New("supervisor pipe has no reader")
)

type supervisorPipeNoReaderDeliveryError struct {
	supervisorID string
	metadata     map[string]any
	reason       string
}

func (e *supervisorPipeNoReaderDeliveryError) Error() string {
	return "supervisor delivery is degraded: " + e.reason
}

func (e *supervisorPipeNoReaderDeliveryError) Unwrap() error {
	return errSupervisorPipeNoReader
}

func HandleSuperviseStart(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID, err := requiredControlTextParam(envelope, "session_id", "supervise.start requires session_id")
	if err != nil {
		return nil, err
	}
	replace := boolParam(envelope, "replace")
	config, err := loadSupervisionStartConfig(ctx, runner, repositoryID, sessionID)
	if err != nil {
		return nil, err
	}
	supervisorID, err := newID("sup")
	if err != nil {
		return nil, err
	}
	daemonSupervisorID, err := newID("dsup")
	if err != nil {
		return nil, err
	}
	scratch := filepath.Join(config.RepoRoot, ".striatum", "scratch", supervisorID)
	pipePath := filepath.Join(scratch, "stdin.pipe")
	eventPath := filepath.Join(scratch, "helper-events.jsonl")
	if err := os.MkdirAll(scratch, 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(pipePath)
	if err := supervisionMkfifo(pipePath); err != nil {
		return nil, err
	}
	cleanupPipe := true
	defer func() {
		if cleanupPipe {
			_ = os.Remove(pipePath)
		}
	}()

	startedAt := nowString()
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if err := lockSuperviseStart(ctx, tx, repositoryID, sessionID); err != nil {
			return nil, err
		}
		if err := supersedeStaleSupervisorIfRequested(ctx, tx, repositoryID, sessionID, replace, startedAt); err != nil {
			return nil, err
		}
		// RFC 0096 V2 / #135: mint a session-BOUND capability token and inject it
		// into the lane env (below) so the lane authenticates as its own session,
		// not the shared operator override. Done inside the start transaction so the
		// token is committed atomically with the supervisor rows; the plaintext is
		// captured into config.CapabilityToken (in-memory → lane env only).
		boundToken, err := mintSessionBoundToken(ctx, tx, repositoryID, sessionID)
		if err != nil {
			return nil, err
		}
		if token, ok := boundToken["token"].(string); ok {
			config.CapabilityToken = token
		}
		if err := insertStartingsSupervisorRowsWithCleanError(ctx, tx, repositoryID, config, supervisorID, daemonSupervisorID, scratch, pipePath, eventPath, startedAt, sessionID); err != nil {
			return nil, err
		}
		payload := map[string]any{
			"supervisor_id":        supervisorID,
			"daemon_supervisor_id": daemonSupervisorID,
			"adapter":              "process",
			"transport":            config.Transport,
			"stdin_delivery":       config.StdinDelivery,
			"require_tmux":         config.RequireTmux,
			"agent_loop_mode":      config.AgentLoopMode,
			"stdin_pipe_path":      pipePath,
		}
		if config.Transport == supervisionTransportPTYHelper {
			payload["helper_events_path"] = eventPath
		}
		_, err = appendEvent(ctx, tx, repositoryID, config.RunID, "supervisor.starting", sessionID, nil, nil, nil, nil, payload)
		return nil, err
	}); err != nil {
		return nil, err
	}

	launch, err := supervisionLaunch(ctx, config, supervisorID, scratch, pipePath, eventPath)
	if err != nil {
		_ = markSupervisorLost(ctx, runner, repositoryID, supervisorID, config.RunID, sessionID, "start failed: "+err.Error(), 0, map[string]any{"phase": "start", "error": err.Error()})
		return nil, rpc.NewError("invalid_transition", "supervisor could not launch lane command: "+err.Error(), nil)
	}
	if launch.PIDStartTime == "" {
		launch.PIDStartTime, _ = processStartToken(launch.PID)
	}
	if launch.PIDStartTime == "" {
		launch.PIDStartTime = tmuxPaneStartTokenFromMetadata(launch.Metadata)
	}
	if !pidAliveLocal(launch.PID) {
		payload := failedAttachCleanupPayload(ctx, launch)
		_ = markSupervisorLostWithMetadata(ctx, runner, repositoryID, supervisorID, config.RunID, sessionID, "child exited before attach", launch.PID, launch.Metadata, payload)
		return nil, rpc.NewError("invalid_transition", "supervisor child exited before it could be attached", nil)
	}

	attachedAt := nowString()
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if len(launch.InitialHelperEvents) > 0 {
			for _, event := range launch.InitialHelperEvents {
				normalized, normErr := normalizeSuperviseReportEvent(event, "", supervisorID, 0)
				if normErr != nil {
					return nil, normErr
				}
				if _, recErr := recordSuperviseReportEvent(ctx, tx, repositoryID, normalized); recErr != nil {
					return nil, recErr
				}
			}
		}
		if len(launch.Metadata) > 0 || launch.InitialHelperOffset > 0 {
			metadata := copyMap(launch.Metadata)
			if launch.InitialHelperOffset > 0 {
				metadata["helper_events_offset"] = launch.InitialHelperOffset
			}
			if err := mergePointerMetadata(ctx, tx, repositoryID, supervisorID, metadata); err != nil {
				return nil, err
			}
		}
		if err := updateSupervisorState(ctx, tx, repositoryID, supervisorID, daemonSupervisorID, "attached", attachedAt, launch.PID, launch.PIDStartTime, attachedAt, nil, nil); err != nil {
			return nil, err
		}
		payload := map[string]any{
			"supervisor_id":        supervisorID,
			"daemon_supervisor_id": daemonSupervisorID,
			"pid":                  launch.PID,
			"transport":            config.Transport,
			"stdin_delivery":       config.StdinDelivery,
			"require_tmux":         config.RequireTmux,
			"agent_loop_mode":      config.AgentLoopMode,
			"stdin_pipe_path":      pipePath,
		}
		if config.Transport == supervisionTransportPTYHelper {
			payload["helper_pid"] = optionalPositiveInt(launch.HelperPID)
			payload["helper_events_path"] = eventPath
		}
		if tmux := objectOrNil(launch.Metadata["tmux"]); tmux != nil {
			payload["tmux"] = tmux
		}
		if runAsUser := metadataString(launch.Metadata["run_as_user"]); runAsUser != "" {
			payload["run_as_user"] = runAsUser
		}
		_, err := appendEvent(ctx, tx, repositoryID, config.RunID, "supervisor.started", sessionID, nil, nil, nil, nil, payload)
		return nil, err
	}); err != nil {
		return nil, err
	}
	cleanupPipe = false
	result := map[string]any{
		"supervisor_id":        supervisorID,
		"daemon_supervisor_id": daemonSupervisorID,
		"session_id":           sessionID,
		"run_id":               config.RunID,
		"pid":                  launch.PID,
		"pid_start_time":       nullableString(launch.PIDStartTime),
		"stdin_pipe_path":      pipePath,
		"state":                "attached",
		"transport":            config.Transport,
		"stdin_delivery":       config.StdinDelivery,
		"require_tmux":         config.RequireTmux,
		"agent_loop_mode":      config.AgentLoopMode,
		"helper_process":       helperProcessPayload(config.Transport, launch.HelperPID, launch.HelperPIDStartTime, eventPath),
		"lane_attestation":     laneAttestation(launch.PIDStartTime),
		"lane_id":              config.LaneID,
		"tmux":                 objectOrNil(launch.Metadata["tmux"]),
	}
	if runAsUser := metadataString(launch.Metadata["run_as_user"]); runAsUser != "" {
		result["run_as_user"] = runAsUser
	}
	// #115: a prepared/running run uses its FROZEN workflow snapshot, so on-disk
	// workflow.json edits are inert. Surface a warning when the lane just launched
	// from a snapshot that diverges from the current file, so the operator does not
	// burn time on a silent no-op (the fix is to prepare a new run).
	if w := snapshotDivergenceWarningForRun(ctx, runner, repositoryID, config.RepoRoot, config.WorkflowSnapshotID); w != "" {
		result["snapshot_divergence_warning"] = w
	}
	if config.AgentLoopMode == agentLoopModePush {
		result["auto_dispatch"] = autoDispatchPushSupervisor(ctx, runner, repositoryID, sessionID)
	}
	return result, nil
}

func autoDispatchPushSupervisor(ctx context.Context, runner db.Runner, repositoryID, sessionID string) map[string]any {
	result, err := withTxRetryOnDeadlock(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		claim, err := claimNextInTx(ctx, tx, repositoryID, sessionID, 3600)
		if err != nil {
			return nil, err
		}
		status, _ := claim["status"].(string)
		if status != "claimed" {
			out := map[string]any{"status": status}
			if status == "" {
				out["status"] = "no_work"
			}
			for _, key := range []string{"paused", "ineligible_reason", "workflow_job_id", "hint"} {
				if value, ok := claim[key]; ok {
					out[key] = value
				}
			}
			return out, nil
		}
		packetID, _ := claim["packet_id"].(string)
		if packetID == "" {
			return nil, rpc.NewError("invalid_transition", "claim-next did not return a packet_id for push auto-dispatch", nil)
		}
		delivery, err := deliverClaimedPacketToSupervisorInTx(ctx, tx, repositoryID, sessionID, packetID, "supervise.start.auto_dispatch")
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"status":    "delivered",
			"packet_id": packetID,
			"delivery":  delivery,
		}, nil
	})
	if err == nil {
		return result
	}
	var noReader *supervisorPipeNoReaderDeliveryError
	if errors.As(err, &noReader) {
		if _, markErr := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
			return map[string]any{}, markPointerDeliveryDegraded(ctx, tx, repositoryID, noReader.supervisorID, noReader.metadata, noReader.reason)
		}); markErr != nil {
			err = markErr
		}
	}
	out := map[string]any{
		"status": "failed",
		"error":  err.Error(),
	}
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		out["error_code"] = rpcErr.Code
		out["error"] = rpcErr.Message
		if len(rpcErr.Details) > 0 {
			out["error_details"] = rpcErr.Details
		}
	}
	return out
}

func HandleSuperviseSend(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID, err := requiredControlTextParam(envelope, "session_id", "supervise.send requires session_id")
	if err != nil {
		return nil, err
	}
	packetID, err := requiredControlTextParam(envelope, "packet_id", "supervise.send requires packet_id")
	if err != nil {
		return nil, err
	}

	result, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return deliverClaimedPacketToSupervisorInTx(ctx, tx, repositoryID, sessionID, packetID, "supervise.send")
	})
	if err == nil {
		return result, nil
	}
	var noReader *supervisorPipeNoReaderDeliveryError
	if !errors.As(err, &noReader) {
		return nil, err
	}
	if _, markErr := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return map[string]any{}, markPointerDeliveryDegraded(ctx, tx, repositoryID, noReader.supervisorID, noReader.metadata, noReader.reason)
	}); markErr != nil {
		return nil, markErr
	}
	return nil, rpc.NewError("invalid_transition", noReader.Error(), nil)
}

func deliverClaimedPacketToSupervisorInTx(ctx context.Context, tx db.TxRunner, repositoryID, sessionID, packetID, phase string) (map[string]any, error) {
	supervisor, err := requireActiveControlSupervisor(ctx, tx, repositoryID, sessionID, true)
	if err != nil {
		return nil, err
	}
	if err := drainHelperEvents(ctx, tx, repositoryID, supervisor.SupervisorID, 0); err != nil {
		return nil, err
	}
	supervisor, err = requireActiveControlSupervisor(ctx, tx, repositoryID, sessionID, true)
	if err != nil {
		return nil, err
	}
	if supervisor.State != "attached" {
		message := fmt.Sprintf("supervise send requires an attached supervisor (supervisor_id=%s, state=%s)", supervisor.SupervisorID, supervisor.State)
		if supervisor.State == "detached" {
			message = fmt.Sprintf("supervisor is detached; stop this supervisor and restart/reclaim before delivery (supervisor_id=%s)", supervisor.SupervisorID)
		}
		return nil, rpc.NewError("invalid_transition", message, nil)
	}
	packet, err := loadWorkPacket(ctx, tx, repositoryID, packetID)
	if err != nil {
		return nil, err
	}
	if packet.SessionID != sessionID {
		return nil, rpc.NewError("invalid_transition", fmt.Sprintf("work packet does not belong to this session: packet_session=%q, requested_session=%q", packet.SessionID, sessionID), nil)
	}
	if packet.RunID != supervisor.RunID {
		return nil, rpc.NewError("invalid_transition", "work packet run does not match supervisor run", nil)
	}
	if err := ensureActivePacketLease(ctx, tx, repositoryID, packet, sessionID); err != nil {
		return nil, err
	}
	if err := reconcileSupervisorForDelivery(ctx, tx, repositoryID, supervisor, phase); err != nil {
		return nil, err
	}
	if supervisor.StdinPipePath == "" {
		return nil, rpc.NewError("invalid_transition", "supervisor stdin pipe is missing: <unset>", nil)
	}
	if _, err := os.Stat(supervisor.StdinPipePath); err != nil {
		return nil, rpc.NewError("invalid_transition", "supervisor stdin pipe is missing: "+supervisor.StdinPipePath, nil)
	}
	payload, err := json.Marshal(packet.Packet)
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')
	delivery, err := supervisionWrite(ctx, tx, repositoryID, supervisor.SupervisorID, supervisor.StdinPipePath, payload)
	if err != nil {
		return nil, err
	}
	deliveredAt := nowString()
	if err := refreshSupervisorHeartbeat(ctx, tx, repositoryID, supervisor.SupervisorID, supervisor.DaemonSupervisorID, deliveredAt); err != nil {
		return nil, err
	}
	if err := drainHelperEvents(ctx, tx, repositoryID, supervisor.SupervisorID, 250*time.Millisecond); err != nil {
		return nil, err
	}
	_, err = appendEvent(ctx, tx, repositoryID, supervisor.RunID, "supervisor.packet_delivered", sessionID, nil, nil, nil, nil, map[string]any{
		"supervisor_id":            supervisor.SupervisorID,
		"packet_id":                packetID,
		"bytes_written":            delivery.BytesWritten,
		"stdin_delivery":           delivery.StdinDelivery,
		"stdin_closed_after_write": delivery.StdinClosedAfterWrite,
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"supervisor_id":            supervisor.SupervisorID,
		"packet_id":                packetID,
		"delivered_at":             deliveredAt,
		"bytes":                    delivery.BytesWritten,
		"stdin_delivery":           delivery.StdinDelivery,
		"stdin_closed_after_write": delivery.StdinClosedAfterWrite,
		"delivery_state":           "delivered_unacknowledged",
		"control_ack_expected":     true,
	}, nil
}

func HandleSuperviseStop(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID, err := requiredControlTextParam(envelope, "session_id", "supervise.stop requires session_id")
	if err != nil {
		return nil, err
	}
	reason, err := requiredControlTextParam(envelope, "reason", "supervise.stop requires reason")
	if err != nil {
		return nil, err
	}
	if err := requireSessionExists(ctx, runner, repositoryID, sessionID); err != nil {
		return nil, err
	}
	terminal, err := latestTerminalSupervisor(ctx, runner, repositoryID, sessionID)
	if err != nil {
		return nil, err
	}
	if terminal != nil {
		if terminal.State == "lost" {
			return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
				return stopSupervisorInTx(ctx, tx, repositoryID, sessionID, reason, *terminal)
			})
		}
		return map[string]any{
			"supervisor_id": terminal.SupervisorID,
			"session_id":    sessionID,
			"pid":           optionalIntValue(terminal.PID, terminal.HasPID),
			"state":         "stopped",
			"ended_at":      terminal.EndedAt,
			"stop_reason":   terminal.StopReason,
			"signal":        nil,
			"note":          "supervisor was already " + terminal.State,
		}, nil
	}

	return withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		supervisor, err := requireActiveControlSupervisor(ctx, tx, repositoryID, sessionID, true)
		if err != nil {
			return nil, err
		}
		return stopSupervisorInTx(ctx, tx, repositoryID, sessionID, reason, supervisor)
	})
}

func stopSupervisorInTx(ctx context.Context, tx db.TxRunner, repositoryID, sessionID, reason string, supervisor supervisorControlRow) (map[string]any, error) {
	_ = drainHelperEvents(ctx, tx, repositoryID, supervisor.SupervisorID, 0)
	var signaled any
	eventExtra := map[string]any{}
	stopNote := any(nil)
	if tmuxIdentity, ok := gosupervisor.TmuxIdentityFromMetadata(supervisor.Metadata); ok {
		signal, note, fallbackReason, cleanupSkip := stopTmuxBackedLane(ctx, supervisor.Metadata, tmuxIdentity, supervisor.PID, supervisor.PIDStartTime)
		signaled = signal
		if note != "" {
			stopNote = note
		}
		if fallbackReason != "" {
			eventExtra["tmux_kill_fallback_reason"] = fallbackReason
		}
		if cleanupSkip != "" {
			eventExtra["pane_pid_cleanup_skipped_reason"] = cleanupSkip
		}
	} else if supervisor.HasPID {
		signal, cleanupSkip := terminateProcessWithStartToken(supervisor.PID, supervisor.PIDStartTime)
		signaled = signal
		if cleanupSkip != "" {
			eventExtra["pid_cleanup_skipped_reason"] = cleanupSkip
		}
	}
	if helperPID, ok := intValueOptional(supervisor.Metadata["helper_pid"]); ok && (!supervisor.HasPID || helperPID != supervisor.PID) {
		helperSignal, cleanupSkip := terminateProcessWithStartToken(helperPID, metadataString(supervisor.Metadata["helper_pid_start_time"]))
		if helperSignal != nil {
			eventExtra["helper_signal"] = helperSignal
		}
		if cleanupSkip != "" {
			eventExtra["helper_pid_cleanup_skipped_reason"] = cleanupSkip
		}
	}
	if supervisor.StdinPipePath != "" {
		_ = os.Remove(supervisor.StdinPipePath)
	}
	endedAt := nowString()
	if err := updateSupervisorState(ctx, tx, repositoryID, supervisor.SupervisorID, supervisor.DaemonSupervisorID, "stopped", endedAt, 0, "", "", &endedAt, &reason); err != nil {
		return nil, err
	}
	// #50: a stopped supervisor must not leave its session reading as `active` —
	// that pollutes "find the latest active <role>/<lane> session" lookups
	// (interrogation targeting, reviewer prompts). Close the session in one
	// guarded UPDATE: only when it is still `active` AND holds no active lease
	// (mid-work sessions are left for explicit recovery). Done as a single
	// conditional statement so no extra row read is required.
	if err := tx.Exec(ctx, `
			UPDATE striatumd.sessions
			   SET state = 'closed', closed_at = $1, close_reason = $2
			 WHERE repository_id = $3 AND session_id = $4 AND state = 'active'
			   AND NOT EXISTS (
				 SELECT 1 FROM striatumd.leases l
				  WHERE l.repository_id = $3 AND l.owner_session_id = $4 AND l.state = 'active')`,
		endedAt, "supervisor stopped: "+reason, repositoryID, sessionID); err != nil {
		return nil, err
	}
	eventPayload := map[string]any{
		"supervisor_id":        supervisor.SupervisorID,
		"daemon_supervisor_id": nullableString(supervisor.DaemonSupervisorID),
		"pid":                  optionalIntValue(supervisor.PID, supervisor.HasPID),
		"reason":               reason,
		"signal":               signaled,
	}
	for key, value := range eventExtra {
		eventPayload[key] = value
	}
	_, err := appendEvent(ctx, tx, repositoryID, supervisor.RunID, "supervisor.stopped", sessionID, nil, nil, nil, nil, eventPayload)
	if err != nil {
		return nil, err
	}
	agentloop.CleanupGeminiSettings(supervisorWorkingDir(supervisor), supervisor.SupervisorID)
	agentloop.CleanupClaudeScheduledTasksLock(supervisorWorkingDir(supervisor))
	return map[string]any{
		"supervisor_id":        supervisor.SupervisorID,
		"daemon_supervisor_id": nullableString(supervisor.DaemonSupervisorID),
		"session_id":           sessionID,
		"pid":                  optionalIntValue(supervisor.PID, supervisor.HasPID),
		"state":                "stopped",
		"ended_at":             endedAt,
		"stop_reason":          reason,
		"signal":               signaled,
		"note":                 stopNote,
	}, nil
}

func failedAttachCleanupPayload(ctx context.Context, launch supervisionLaunchResult) map[string]any {
	payload := map[string]any{"phase": "start"}
	if tmuxIdentity, ok := gosupervisor.TmuxIdentityFromMetadata(launch.Metadata); ok {
		signal, note, fallbackReason, cleanupSkip := stopTmuxBackedLane(ctx, launch.Metadata, tmuxIdentity, launch.PID, launch.PIDStartTime)
		if signal != nil {
			payload["signal"] = signal
		}
		if note != "" {
			payload["cleanup_note"] = note
		}
		if fallbackReason != "" {
			payload["tmux_kill_fallback_reason"] = fallbackReason
		}
		if cleanupSkip != "" {
			payload["pane_pid_cleanup_skipped_reason"] = cleanupSkip
		}
		return payload
	}
	if launch.PID > 0 {
		signal, cleanupSkip := terminateProcessWithStartToken(launch.PID, launch.PIDStartTime)
		if signal != nil {
			payload["signal"] = signal
		}
		if cleanupSkip != "" {
			payload["pid_cleanup_skipped_reason"] = cleanupSkip
		}
	}
	return payload
}

func HandleSuperviseRebridge(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	sessionID, err := requiredControlTextParam(envelope, "session_id", "supervise.rebridge requires session_id")
	if err != nil {
		return nil, err
	}
	var supervisor supervisorControlRow
	var identity gosupervisor.TmuxIdentity
	var eventPath string
	if _, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		var err error
		supervisor, err = requireActiveControlSupervisor(ctx, tx, repositoryID, sessionID, true)
		if err != nil {
			return nil, err
		}
		if supervisor.State != "attached" {
			return nil, rpc.NewError("invalid_transition", "supervise.rebridge requires an attached supervisor", nil)
		}
		identity, err = requireRebridgeableTmuxPane(ctx, supervisor)
		if err != nil {
			return nil, err
		}
		eventPath = metadataString(supervisor.Metadata["helper_events_path"])
		if eventPath == "" {
			eventPath = filepath.Join(supervisor.ScratchPath, "helper-events.jsonl")
		}
		if supervisor.StdinPipePath == "" {
			return nil, rpc.NewError("invalid_transition", "supervise.rebridge requires a supervisor stdin pipe path", nil)
		}
		if err := ensureSupervisorFIFO(supervisor.StdinPipePath); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	}); err != nil {
		return nil, err
	}

	launch, err := supervisionRebridgeLaunch(ctx, supervisor, identity, eventPath)
	if err != nil {
		return nil, rpc.NewError("invalid_transition", "supervise.rebridge could not attach delivery bridge: "+err.Error(), nil)
	}
	rebridgedAt := nowString()
	result, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		// #67: a successful rebridge rebuilds the delivery transport (fresh helper
		// + persistent FIFO). The re-attached tmux attach-observer then re-emits a
		// benign attach_client_exited (#63 F7) — that is NOT a delivery failure, so
		// it must not keep the lane reported as degraded. Only a real transport
		// failure the helper reports on relaunch (helper_error / agent_exited)
		// preserves the degraded delivery_liveness block.
		hasRealDeliveryFailure := false
		if len(launch.InitialHelperEvents) > 0 {
			for _, event := range launch.InitialHelperEvents {
				normalized, normErr := normalizeSuperviseReportEvent(event, "", supervisor.SupervisorID, 0)
				if normErr != nil {
					return nil, normErr
				}
				switch normalized.EventType {
				case string(gosupervisor.HelperEventError), string(gosupervisor.HelperEventAgentExited):
					hasRealDeliveryFailure = true
				}
				if _, recErr := recordSuperviseReportEvent(ctx, tx, repositoryID, normalized); recErr != nil {
					return nil, recErr
				}
			}
		}
		current, err := pointerMetadata(ctx, tx, repositoryID, supervisor.SupervisorID)
		if err != nil {
			return nil, err
		}
		updated := copyMap(current)
		updated["helper_pid"] = launch.HelperPID
		updated["helper_pid_start_time"] = launch.HelperPIDStartTime
		updated["helper_events_path"] = eventPath
		updated["helper_events_offset"] = launch.InitialHelperOffset
		if !hasRealDeliveryFailure {
			delete(updated, "delivery_liveness")
		}
		if tmux := asMap(updated["tmux"]); len(tmux) > 0 {
			if !hasRealDeliveryFailure {
				delete(tmux, "delivery_liveness")
			}
			tmux["attach_client_pid"] = launch.Metadata["attach_client_pid"]
			tmux["last_rebridged_at"] = rebridgedAt
			if launchTmux := asMap(launch.Metadata["tmux"]); len(launchTmux) > 0 {
				for key, value := range launchTmux {
					tmux[key] = value
				}
				if !hasRealDeliveryFailure {
					delete(tmux, "delivery_liveness")
				}
			}
			updated["tmux"] = tmux
		}
		if err := replacePointerMetadata(ctx, tx, repositoryID, supervisor.SupervisorID, updated); err != nil {
			return nil, err
		}
		if err := refreshSupervisorHeartbeat(ctx, tx, repositoryID, supervisor.SupervisorID, supervisor.DaemonSupervisorID, rebridgedAt); err != nil {
			return nil, err
		}
		payload := map[string]any{
			"supervisor_id":     supervisor.SupervisorID,
			"session_id":        sessionID,
			"helper_pid":        launch.HelperPID,
			"attach_client_pid": launch.Metadata["attach_client_pid"],
			"tmux_liveness":     string(gosupervisor.TmuxLivenessOK),
		}
		_, err = appendEvent(ctx, tx, repositoryID, supervisor.RunID, "supervisor.rebridged", sessionID, nil, nil, nil, nil, payload)
		if err != nil {
			return nil, err
		}
		deliveryStateVal := "healthy"
		if hasRealDeliveryFailure {
			deliveryStateVal = "degraded"
		}
		return map[string]any{
			"supervisor_id":     supervisor.SupervisorID,
			"session_id":        sessionID,
			"run_id":            supervisor.RunID,
			"state":             "attached",
			"delivery_state":    deliveryStateVal,
			"helper_pid":        launch.HelperPID,
			"attach_client_pid": launch.Metadata["attach_client_pid"],
			"rebridged_at":      rebridgedAt,
			"tmux":              asMap(updated["tmux"]),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func requireRebridgeableTmuxPane(ctx context.Context, supervisor supervisorControlRow) (gosupervisor.TmuxIdentity, error) {
	identity, ok := gosupervisor.TmuxIdentityFromMetadata(supervisor.Metadata)
	if !ok {
		return gosupervisor.TmuxIdentity{}, rpc.NewError("invalid_transition", "supervise.rebridge requires a tmux-backed supervisor", nil)
	}
	live := gosupervisor.ProbeLaneLiveness(ctx, tmuxRunnerForSupervisorMetadata(supervisor.Metadata), supervisor.Metadata, supervisor.PID, supervisor.PIDStartTime)
	if live.Class == string(gosupervisor.TmuxLivenessUnavailable) {
		return gosupervisor.TmuxIdentity{}, rpc.NewError("invalid_transition", "supervise.rebridge cannot verify tmux pane liveness: "+live.Detail, nil)
	}
	if !live.Alive {
		return gosupervisor.TmuxIdentity{}, rpc.NewError("invalid_transition", "supervise.rebridge refused because pane liveness is "+live.Class+"; stop and restart or reclaim the lane", nil)
	}
	if live.Tmux != nil && live.Tmux.ObservedPanePID > 0 {
		identity.PanePID = live.Tmux.ObservedPanePID
	}
	if live.Tmux != nil && live.Tmux.ObservedStartTok != "" {
		identity.PaneStartToken = live.Tmux.ObservedStartTok
	}
	return identity, nil
}

func ensureSupervisorFIFO(path string) error {
	if info, err := os.Stat(path); err == nil {
		if info.Mode()&os.ModeNamedPipe == 0 {
			return fmt.Errorf("stdin path exists but is not a FIFO: %s", path)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return supervisionMkfifo(path)
}

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

func loadSupervisionStartConfig(ctx context.Context, runner db.Runner, repositoryID string, sessionID string) (supervisionStartConfig, error) {
	var config supervisionStartConfig
	config.RepositoryID = repositoryID
	var sessionState string
	err := runner.QueryRow(ctx, `
		SELECT s.session_id, s.run_id, s.lane_id, s.state,
		       r.workflow_snapshot_id, repo.repo_root
		  FROM striatumd.sessions s
		  JOIN striatumd.runs r
		    ON r.repository_id = s.repository_id AND r.run_id = s.run_id
		  JOIN striatumd.repositories repo
		    ON repo.repository_id = s.repository_id
		 WHERE s.repository_id = $1 AND s.session_id = $2`,
		repositoryID, sessionID,
	).Scan(&config.SessionID, &config.RunID, &config.LaneID, &sessionState, &config.WorkflowSnapshotID, &config.RepoRoot)
	if errors.Is(err, pgx.ErrNoRows) {
		return config, rpc.NewError("not_found", "session not found: "+sessionID, nil)
	}
	if err != nil {
		return config, err
	}
	if sessionState != "active" {
		return config, rpc.NewError("invalid_transition", "supervise start requires an active session", nil)
	}
	var workflowRaw any
	if err := runner.QueryRow(ctx, `
		SELECT workflow_json
		  FROM striatumd.workflow_snapshots
		 WHERE repository_id = $1 AND workflow_snapshot_id = $2`,
		repositoryID, config.WorkflowSnapshotID,
	).Scan(&workflowRaw); err != nil {
		return config, err
	}
	workflow := asMap(workflowRaw)
	lane := laneConfig(workflow, config.LaneID)
	// #199: last line of defense before the subprocess launches. validate/prepare
	// already refuse `claude --print`/`-p` lanes, but the lane here comes from a
	// frozen snapshot — refuse here too so a snapshot that somehow carries one
	// never burns API tokens (real money per packet after 2026-06-15). The
	// override is the inline lane option `allow_claude_print: true`.
	if err := workflowauthoring.RefuseClaudePrintLane(config.LaneID, lane); err != nil {
		return config, rpc.NewError("invalid_transition", err.Error(), nil)
	}
	command, err := commandArray(lane)
	if err != nil {
		return config, err
	}
	config.OriginalCommand = append([]string(nil), command...)
	if laneUsesAgentLoop(lane) {
		// #181: an agent-loop lane is driven by the self-driving bootstrap +
		// PTY-submit wiring, which only knows how to deliver the initial prompt
		// to codex, agy, and claude (agentloop.bootstrapDeliveryModeFor). Any
		// other argv0 would be wrapped by selfDrivingAgentLoopCommand and launched
		// as a self-driver that can never receive its bootstrap, so the lane sits
		// idle behind a healthy-looking supervisor. Refuse BEFORE inserting the
		// supervisor row so the operator gets a legible error instead of a wedged
		// lane (RFC 0111).
		if err := requireSupportedAgentLoopAdapter(command); err != nil {
			return config, err
		}
		config.AgentLoopMode = agentLoopModeSelfDriving
		// RFC 0088: wrap the raw lane command in `striatumd -agent-loop -- …`
		// so the agent-loop executor delivers the bootstrap prompt and submits
		// it over a PTY (interactive lanes), instead of launching the bare
		// command which blocks waiting for input it never receives.
		command, err = selfDrivingAgentLoopCommand(command)
		if err != nil {
			return config, err
		}
	} else {
		// #146: a lane that does NOT use the agent loop is a stdin-FIFO/push
		// consumer (it reads a delivered packet then runs the agent), not a true
		// self-driver that calls work.await_packet. Record the honest push mode so
		// sessionHasSelfDrivingSupervisor — and therefore claim-next — surfaces the
		// supervise_send hint instead of the misleading self_driving self_claim_note
		// ("do not run supervise send"), which sent operators down a dead path.
		config.AgentLoopMode = agentLoopModePush
	}
	// RFC 0088: resolve argv0 against the augmented supervised PATH so a lane
	// binary that lives only in ~/.local/bin (codex, claude, agy) launches even
	// when the daemon's own PATH lacks it. exec.Command resolves argv0 against
	// the launching process's PATH at construction time, before cmd.Env is
	// applied, so setting the child PATH alone is insufficient (the F44
	// path.conf-retirement regression).
	command = resolveSupervisedCommandBinary(command)
	transport, err := supervisionTransport(lane)
	if err != nil {
		return config, err
	}
	delivery, err := supervisionStdinDelivery(lane, transport)
	if err != nil {
		return config, err
	}
	requireTmux, err := supervisionRequireTmux(lane, transport)
	if err != nil {
		return config, err
	}
	config.Command = command
	config.Transport = transport
	config.StdinDelivery = delivery
	config.RequireTmux = requireTmux
	config.RunAsUser = configuredLaneRunAsUser()
	if config.RunAsUser != "" && runtime.GOOS == "windows" {
		return config, rpc.NewError("invalid_transition", supervisedLaneOSUserEnv+" is not supported on windows", nil)
	}
	return config, nil
}

func insertStartingSupervisorRows(ctx context.Context, runner db.TxRunner, repositoryID string, config supervisionStartConfig, supervisorID, daemonSupervisorID, scratch, pipePath, eventPath, startedAt string) error {
	commandJSON, err := json.Marshal(config.Command)
	if err != nil {
		return err
	}
	metadata := map[string]any{
		"source":             "go_supervision_control_handler",
		"daemon_instance_id": currentDaemonInstanceID(),
		"transport":          config.Transport,
		"stdin_delivery":     config.StdinDelivery,
		"require_tmux":       config.RequireTmux,
		"agent_loop_mode":    config.AgentLoopMode,
	}
	if config.RunAsUser != "" {
		metadata["run_as_user"] = config.RunAsUser
	}
	if config.Transport == supervisionTransportPTYHelper {
		metadata["helper_events_path"] = eventPath
		metadata["helper_events_offset"] = 0
	}
	commandArg, err := db.JSONBArg(runner, config.Command)
	if err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisors (
		  repository_id, supervisor_id, run_id, session_id, adapter,
		  command_json, cwd, scratch_path, stdin_pipe_path, state, started_at
		)
		VALUES ($1,$2,$3,$4,'process',$5::jsonb,$6,$7,$8,'starting',$9)`,
		repositoryID, supervisorID, config.RunID, config.SessionID, commandArg, config.RepoRoot, scratch, pipePath, startedAt,
	); err != nil {
		return err
	}
	metadataArg, err := db.JSONBArg(runner, metadata)
	if err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.process_supervisor_pointers (
		  repository_id, supervisor_id, daemon_supervisor_id, run_id,
		  session_id, state, updated_at, metadata_json
		)
		VALUES ($1,$2,$3,$4,$5,'starting',$6,$7::jsonb)`,
		repositoryID, supervisorID, daemonSupervisorID, config.RunID, config.SessionID, startedAt, metadataArg,
	); err != nil {
		return err
	}
	return runner.Exec(ctx, `
		INSERT INTO striatumd.daemon_supervisors (
		  daemon_supervisor_id, repository_id, run_id, session_id,
		  repo_supervisor_id, daemon_instance_id, adapter, command_json,
		  command_sha256, cwd, stdin_pipe_path, state, started_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,'process',$7::jsonb,$8,$9,$10,'starting',$11)`,
		daemonSupervisorID, repositoryID, config.RunID, config.SessionID,
		supervisorID, currentDaemonInstanceID(), commandArg, sha256Hex(commandJSON),
		config.RepoRoot, pipePath, startedAt,
	)
}

func lockSuperviseStart(ctx context.Context, runner db.TxRunner, repositoryID, sessionID string) error {
	key := "striatum:supervise_start:" + repositoryID + ":" + sessionID
	return runner.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, key)
}

// supersedeStaleSupervisorIfRequested is called inside the advisory-locked
// transaction for supervise.start. When replace=true and a stale active
// supervisor exists for the session, it is superseded (marked lost) so the
// subsequent INSERT does not collide with the unique partial index
// uq_active_daemon_supervisor_pointer_per_session. When replace=false and a
// stale supervisor exists, a clean actionable error is returned directing the
// operator to retry with --replace. When no stale supervisor exists in either
// case the call is a no-op.
func supersedeStaleSupervisorIfRequested(ctx context.Context, runner db.TxRunner, repositoryID, sessionID string, replace bool, now string) error {
	var supervisorID, runID, state string
	err := runner.QueryRow(ctx, `
		SELECT supervisor_id, run_id, state
		  FROM striatumd.process_supervisors
		 WHERE repository_id = $1 AND session_id = $2
		   AND state = ANY($3)
		 ORDER BY started_at DESC, supervisor_id DESC
		 LIMIT 1
		 FOR UPDATE`,
		repositoryID, sessionID, []string{"starting", "attached", "detached"},
	).Scan(&supervisorID, &runID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		// No active supervisor — proceed with INSERT.
		return nil
	}
	if err != nil {
		return err
	}
	// A stale supervisor exists.
	if !replace {
		return rpc.NewError("invalid_transition", fmt.Sprintf(
			"session already has an active supervisor: %s (state=%s); retry with --replace to supersede it",
			supervisorID, state,
		), nil)
	}
	// replace=true: supersede the stale supervisor by marking it lost so the
	// unique partial index allows the incoming INSERT.
	reason := "superseded by supervise.start --replace"
	return markSupervisorLostInTx(ctx, runner, repositoryID, supervisorID, runID, sessionID, reason, 0, map[string]any{"superseded_at": now})
}

// insertStartingsSupervisorRowsWithCleanError is a wrapper around
// insertStartingSupervisorRows that detects a Postgres unique-constraint
// violation (SQLSTATE 23505) on the process_supervisor_pointers partial index
// and converts it into an actionable rpc.Error instead of surfacing the raw
// database error to the operator. This guards the narrow race window between
// the advisory-locked SELECT and the INSERT.
func insertStartingsSupervisorRowsWithCleanError(ctx context.Context, runner db.TxRunner, repositoryID string, config supervisionStartConfig, supervisorID, daemonSupervisorID, scratch, pipePath, eventPath, startedAt, sessionID string) error {
	err := insertStartingSupervisorRows(ctx, runner, repositoryID, config, supervisorID, daemonSupervisorID, scratch, pipePath, eventPath, startedAt)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return rpc.NewError("invalid_transition", fmt.Sprintf(
			"session already has an active supervisor for session_id=%q; retry with --replace to supersede it",
			sessionID,
		), nil)
	}
	return err
}

func requireActiveControlSupervisor(ctx context.Context, runner any, repositoryID, sessionID string, forUpdate bool) (supervisorControlRow, error) {
	suffix := ""
	if forUpdate {
		suffix = " FOR UPDATE OF ps"
	}
	sql := `
		SELECT ps.supervisor_id, ps.run_id, ps.session_id, ps.state,
		       COALESCE(ps.scratch_path, ''), COALESCE(ps.stdin_pipe_path, ''), ps.pid, COALESCE(ps.pid_start_time, ''),
		       COALESCE(p.daemon_supervisor_id, ''), COALESCE(p.metadata_json, '{}'::jsonb)
		  FROM striatumd.process_supervisors ps
		  LEFT JOIN striatumd.process_supervisor_pointers p
		    ON p.repository_id = ps.repository_id AND p.supervisor_id = ps.supervisor_id
		 WHERE ps.repository_id = $1 AND ps.session_id = $2
		   AND ps.state = ANY($3)
		 ORDER BY ps.started_at DESC, ps.supervisor_id DESC
		 LIMIT 1` + suffix
	rower, ok := runner.(interface {
		QueryRow(context.Context, string, ...any) db.Row
	})
	if !ok {
		return supervisorControlRow{}, fmt.Errorf("runner does not support query row")
	}
	var row supervisorControlRow
	var pid *int
	var metadata any
	err := rower.QueryRow(ctx, sql, repositoryID, sessionID, []string{"starting", "attached", "detached"}).Scan(
		&row.SupervisorID, &row.RunID, &row.SessionID, &row.State,
		&row.ScratchPath, &row.StdinPipePath, &pid, &row.PIDStartTime,
		&row.DaemonSupervisorID, &metadata,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, rpc.NewError("invalid_transition", fmt.Sprintf("no active supervisor for session_id=%q", sessionID), nil)
	}
	if err != nil {
		return row, err
	}
	if pid != nil {
		row.PID = *pid
		row.HasPID = true
	}
	row.Metadata = asMap(metadata)
	return row, nil
}

func latestTerminalSupervisor(ctx context.Context, runner any, repositoryID, sessionID string) (*supervisorControlRow, error) {
	if _, err := requireActiveControlSupervisor(ctx, runner, repositoryID, sessionID, false); err == nil {
		return nil, nil
	} else {
		var rpcErr *rpc.Error
		if !errors.As(err, &rpcErr) || rpcErr.Code != "invalid_transition" {
			return nil, err
		}
	}
	rower, ok := runner.(interface {
		QueryRow(context.Context, string, ...any) db.Row
	})
	if !ok {
		return nil, fmt.Errorf("runner does not support query row")
	}
	var row supervisorControlRow
	var pid *int
	var metadata any
	err := rower.QueryRow(ctx, `
		SELECT ps.supervisor_id, ps.run_id, ps.session_id, ps.state,
		       COALESCE(ps.scratch_path, ''), COALESCE(ps.stdin_pipe_path, ''), ps.pid, COALESCE(ps.pid_start_time, ''),
		       COALESCE(p.daemon_supervisor_id, ''), COALESCE(p.metadata_json, '{}'::jsonb),
		       ps.ended_at, ps.stop_reason
		  FROM striatumd.process_supervisors ps
		  LEFT JOIN striatumd.process_supervisor_pointers p
		    ON p.repository_id = ps.repository_id AND p.supervisor_id = ps.supervisor_id
		 WHERE ps.repository_id = $1 AND ps.session_id = $2
		   AND ps.state = ANY($3)
		 ORDER BY ps.started_at DESC, ps.supervisor_id DESC
		 LIMIT 1`,
		repositoryID, sessionID, []string{"lost", "stopped"},
	).Scan(
		&row.SupervisorID, &row.RunID, &row.SessionID, &row.State,
		&row.ScratchPath, &row.StdinPipePath, &pid, &row.PIDStartTime,
		&row.DaemonSupervisorID, &metadata, &row.EndedAt, &row.StopReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if pid != nil {
		row.PID = *pid
		row.HasPID = true
	}
	row.Metadata = asMap(metadata)
	return &row, nil
}

func loadWorkPacket(ctx context.Context, runner db.TxRunner, repositoryID, packetID string) (supervisionPacketRow, error) {
	var row supervisionPacketRow
	if wrongKind, ok := wrongKindPacketID(packetID); ok {
		return row, rpc.NewError("not_found", fmt.Sprintf("%s is a %s id, not a work packet id; use data.packet_id (or data.packet.packet_id) from claim-next JSON for supervise send", packetID, wrongKind), nil)
	}
	var packetRaw any
	err := runner.QueryRow(ctx, `
		SELECT packet_id, run_id, job_id, lease_id, session_id, packet_json
		  FROM striatumd.work_packets
		 WHERE repository_id = $1 AND packet_id = $2`,
		repositoryID, packetID,
	).Scan(&row.PacketID, &row.RunID, &row.JobID, &row.LeaseID, &row.SessionID, &packetRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, rpc.NewError("not_found", fmt.Sprintf("could not find work packet for packet_id=%q", packetID), nil)
	}
	if err != nil {
		return row, err
	}
	row.Packet = asMap(packetRaw)
	return row, nil
}

func wrongKindPacketID(packetID string) (string, bool) {
	switch {
	case strings.HasPrefix(packetID, "msg_"):
		return "message", true
	case strings.HasPrefix(packetID, "lease_"):
		return "lease", true
	case strings.HasPrefix(packetID, "job_"):
		return "job", true
	case strings.HasPrefix(packetID, "sess_"):
		return "session", true
	case strings.HasPrefix(packetID, "sup_"):
		return "supervisor", true
	default:
		return "", false
	}
}

func ensureActivePacketLease(ctx context.Context, runner db.TxRunner, repositoryID string, packet supervisionPacketRow, sessionID string) error {
	var state, ownerSessionID, resourceID string
	var expiresAt any
	err := runner.QueryRow(ctx, `
		SELECT state, owner_session_id, resource_id, expires_at
		  FROM striatumd.leases
		 WHERE repository_id = $1 AND lease_id = $2
		 FOR UPDATE`,
		repositoryID, packet.LeaseID,
	).Scan(&state, &ownerSessionID, &resourceID, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return rpc.NewError("lease_error", "lease not found", nil)
	}
	if err != nil {
		return err
	}
	if state != "active" {
		return rpc.NewError("lease_error", "lease is not active", nil)
	}
	if ownerSessionID != sessionID {
		return rpc.NewError("lease_error", "lease is owned by another session", nil)
	}
	if resourceID != packet.JobID {
		return rpc.NewError("lease_error", "lease does not belong to the job", nil)
	}
	if expires, ok := asTime(expiresAt); ok && expires.UTC().Before(time.Now().UTC()) {
		return rpc.NewError("lease_error", "lease is expired", nil)
	}
	return nil
}

func reconcileSupervisorForDelivery(ctx context.Context, runner db.TxRunner, repositoryID string, supervisor supervisorControlRow, phase string) error {
	// 1. Transactional FOR UPDATE locks inside the mutation block prior to checker call
	var pointerState string
	var daemonSupervisorID *string
	err := runner.QueryRow(ctx, `
		SELECT state, daemon_supervisor_id
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND supervisor_id = $2
		 FOR UPDATE`,
		repositoryID, supervisor.SupervisorID,
	).Scan(&pointerState, &daemonSupervisorID)
	if errors.Is(err, pgx.ErrNoRows) {
		return rpc.NewError("invalid_transition", "supervisor requires operator reconciliation before delivery: pointer_missing", nil)
	}
	if err != nil {
		return err
	}
	if daemonSupervisorID != nil && *daemonSupervisorID != "" {
		var daemonState string
		if err := runner.QueryRow(ctx, `
			SELECT state
			  FROM striatumd.daemon_supervisors
			 WHERE repository_id = $1 AND daemon_supervisor_id = $2
			 FOR UPDATE`,
			repositoryID, *daemonSupervisorID,
		).Scan(&daemonState); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	// 2. Checker call
	checker := lanehealth.Checker{
		Probe: lanehealth.ProdProbe{Runner: supervisionTmuxRunner},
	}
	health, err := checker.Check(ctx, runner, repositoryID, supervisor.SessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rpc.NewError("invalid_transition", "supervisor requires operator reconciliation before delivery: pointer_missing", nil)
		}
		return err
	}

	if !supervisor.HasPID || health.Reason == lanehealth.ReasonPIDMissing {
		if err := markSupervisorLostInTx(ctx, runner, repositoryID, supervisor.SupervisorID, supervisor.RunID, supervisor.SessionID, "pid_missing observed by "+phase, 0, nil); err != nil {
			return err
		}
		return rpc.NewError("invalid_transition", "supervisor cannot accept delivery: pid_missing", nil)
	}

	// #63 F10: key the delivery gate purely on the reconciled LIVE probe
	// (health.Deliverable), not on stale supervisor metadata. The lanehealth
	// checker already reconciles the benign #63 F7 case — an exited tmux
	// attach-session OBSERVER client whose pane is alive and whose real
	// transport (persistent FIFO / pty helper) is healthy stays
	// health.Deliverable == true — while genuine transport failures
	// (helper_process_gone, stdin_reader_missing) keep health.Deliverable
	// false. Previously this gate ALSO required supervisorDeliveryDegraded to
	// observe a delivery_liveness metadata record, so a helper/transport that
	// died abruptly WITHOUT writing that record (main lane PID still alive)
	// slipped the gate and dispatched a packet to a dead FIFO. We now reject
	// whenever the live probe is not deliverable. supervisorDeliveryDegraded
	// survives only as a fallback reason source for the rare case where the
	// probe is non-deliverable but did not carry a DeliveryReason string.
	if !health.Deliverable {
		reason := health.DeliveryReason
		if reason == "" {
			if metaReason, _ := supervisorDeliveryDegraded(supervisor.Metadata); metaReason != "" {
				reason = metaReason
			}
		}
		if reason == "" {
			reason = "not_deliverable"
		}
		return rpc.NewError("invalid_transition", "supervisor delivery is degraded: "+reason, nil)
	}

	if !health.Alive {
		reason := health.LivenessClass
		if reason == "" {
			reason = "pid_gone"
		}
		live := gosupervisor.ProbeLaneLiveness(ctx, tmuxRunnerForSupervisorMetadata(supervisor.Metadata), supervisor.Metadata, supervisor.PID, supervisor.PIDStartTime)
		if reason == string(gosupervisor.TmuxLivenessUnavailable) {
			count := tmuxUnavailableCount(supervisor.Metadata) + 1
			metadata := tmuxProbeDegradedMetadata(supervisor.Metadata, live, count)
			if count >= gosupervisor.TmuxUnavailableLostThreshold() {
				payload := map[string]any{"phase": phase, "tmux_liveness": live.Class, "probe_unavailable_count": count}
				if live.Tmux != nil && live.Tmux.Failure != nil {
					payload["probe_failure"] = gosupervisor.TmuxProbeFailurePayload(*live.Tmux.Failure)
				}
				if err := markSupervisorLostInTx(ctx, runner, repositoryID, supervisor.SupervisorID, supervisor.RunID, supervisor.SessionID, "tmux_unavailable_persistent", supervisor.PID, payload); err != nil {
					return err
				}
				return rpc.NewError("invalid_transition", "supervisor cannot accept delivery: tmux_unavailable_persistent", nil)
			}
			if err := replacePointerMetadata(ctx, runner, repositoryID, supervisor.SupervisorID, metadata); err != nil {
				return err
			}
			return rpc.NewError("invalid_transition", "supervisor liveness is degraded: tmux_unavailable; "+live.Detail, nil)
		}

		lostPayload := map[string]any{"phase": phase, "reattach_reason": reason}
		if strings.HasPrefix(reason, "tmux_") {
			lostPayload["tmux_liveness"] = reason
			if live.Tmux != nil && live.Tmux.Failure != nil {
				lostPayload["probe_failure"] = gosupervisor.TmuxProbeFailurePayload(*live.Tmux.Failure)
			}
		}
		if err := markSupervisorLostInTx(ctx, runner, repositoryID, supervisor.SupervisorID, supervisor.RunID, supervisor.SessionID, reason, supervisor.PID, lostPayload); err != nil {
			return err
		}
		if strings.HasPrefix(reason, "tmux_") {
			return rpc.NewError("invalid_transition", "supervisor cannot accept delivery: "+reason, nil)
		}
		return rpc.NewError("invalid_transition", fmt.Sprintf("supervisor pid is gone: %s", supervisor.SupervisorID), nil)
	}

	// 3. Structural checks from checker results
	if health.Reason == lanehealth.ReasonPointerStateMismatch {
		return rpc.NewError("invalid_transition", "supervisor requires operator reconciliation before delivery: pointer_state_mismatch", nil)
	}
	if health.Reason == lanehealth.ReasonDaemonStateMismatch {
		return rpc.NewError("invalid_transition", "supervisor requires operator reconciliation before delivery: daemon_state_mismatch", nil)
	}
	if health.Reason == lanehealth.ReasonDaemonSupervisorMissing {
		return rpc.NewError("invalid_transition", "supervisor requires operator reconciliation before delivery: daemon_supervisor_missing", nil)
	}

	return nil
}

func supervisorDeliveryDegraded(metadata map[string]any) (string, bool) {
	tmux := asMap(metadata["tmux"])
	delivery := asMap(tmux["delivery_liveness"])
	if len(delivery) == 0 {
		delivery = asMap(metadata["delivery_liveness"])
	}
	if len(delivery) == 0 {
		return "", false
	}
	if healthy, ok := delivery["healthy"].(bool); ok && healthy {
		return "", false
	}
	class, _ := delivery["class"].(string)
	class = strings.TrimSpace(class)
	if class == "" {
		return "", false
	}
	reason, _ := delivery["reason"].(string)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = class
	}
	return reason, true
}

func tmuxUnavailableCount(metadata map[string]any) int {
	tmux := asMap(metadata["tmux"])
	count, ok := intValueOptional(tmux["probe_unavailable_count"])
	if !ok || count < 0 {
		return 0
	}
	return count
}

func tmuxProbeDegradedMetadata(metadata map[string]any, live gosupervisor.LaneLiveness, count int) map[string]any {
	updated := copyMap(metadata)
	tmux := asMap(updated["tmux"])
	if len(tmux) == 0 {
		tmux = map[string]any{}
	}
	tmux["liveness_state"] = "degraded"
	tmux["probe_skipped_at"] = nowString()
	tmux["probe_unavailable_count"] = count
	if live.Detail != "" {
		tmux["last_unavailable_detail"] = live.Detail
	}
	if live.Tmux != nil {
		tmux["liveness"] = gosupervisor.TmuxLivenessPayload(*live.Tmux)
	}
	updated["tmux"] = tmux
	return updated
}

type supervisorDeliveryResult struct {
	BytesWritten          int
	StdinDelivery         string
	StdinClosedAfterWrite bool
}

func writeSupervisorPayload(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID, pipePath string, payload []byte) (supervisorDeliveryResult, error) {
	metadata, err := pointerMetadata(ctx, runner, repositoryID, supervisorID)
	if err != nil {
		return supervisorDeliveryResult{}, err
	}
	stdinDelivery := metadataStdinDelivery(metadata)
	if stdinDelivery == stdinDeliveryOneShotEOF && metadata["stdin_delivery_consumed"] == true {
		return supervisorDeliveryResult{}, rpc.NewError("invalid_transition", "one-shot supervisor stdin has already been consumed", nil)
	}
	bytesWritten, err := writeToPipe(ctx, pipePath, payload)
	if err != nil {
		if errors.Is(err, errSupervisorPipeNoReader) {
			return supervisorDeliveryResult{}, &supervisorPipeNoReaderDeliveryError{
				supervisorID: supervisorID,
				metadata:     metadata,
				reason:       "stdin_reader_missing",
			}
		}
		return supervisorDeliveryResult{}, err
	}
	closed := stdinDelivery == stdinDeliveryOneShotEOF
	if closed {
		_ = os.Remove(pipePath)
		if err := mergePointerMetadata(ctx, runner, repositoryID, supervisorID, map[string]any{"stdin_delivery_consumed": true}); err != nil {
			return supervisorDeliveryResult{}, err
		}
	}
	return supervisorDeliveryResult{BytesWritten: bytesWritten, StdinDelivery: stdinDelivery, StdinClosedAfterWrite: closed}, nil
}

type NamedPipeBuffer struct {
	mu       sync.Mutex
	queue    [][]byte
	degraded bool
}

func (b *NamedPipeBuffer) Push(payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.degraded {
		return fmt.Errorf("buffer is degraded")
	}
	if len(b.queue) >= 10 {
		b.degraded = true
		return fmt.Errorf("buffer overflow, degraded")
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	b.queue = append(b.queue, cp)
	return nil
}

func (b *NamedPipeBuffer) PopAll() [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	q := b.queue
	b.queue = nil
	return q
}

func (b *NamedPipeBuffer) IsDegraded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.degraded
}

var (
	pipeBuffersMu sync.Mutex
	pipeBuffers   = make(map[string]*NamedPipeBuffer)
)

func getPipeBuffer(pipePath string) *NamedPipeBuffer {
	pipeBuffersMu.Lock()
	defer pipeBuffersMu.Unlock()
	buf, ok := pipeBuffers[pipePath]
	if !ok {
		buf = &NamedPipeBuffer{}
		pipeBuffers[pipePath] = buf
	}
	return buf
}

func writeToPipe(ctx context.Context, pipePath string, payload []byte) (int, error) {
	buf := getPipeBuffer(pipePath)
	if buf.IsDegraded() {
		return 0, errSupervisorPipeNoReader
	}

	fd, err := syscall.Open(pipePath, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.ENXIO) {
			if pushErr := buf.Push(payload); pushErr != nil {
				return 0, errSupervisorPipeNoReader
			}
			return len(payload), nil
		}
		return 0, err
	}
	file := os.NewFile(uintptr(fd), pipePath)
	defer func() { _ = file.Close() }()

	buffered := buf.PopAll()
	for _, pkt := range buffered {
		if _, err := writeAll(ctx, file, pkt); err != nil {
			return 0, err
		}
	}

	return writeAll(ctx, file, payload)
}

func writeAll(ctx context.Context, file *os.File, payload []byte) (int, error) {
	total := 0
	for total < len(payload) {
		n, err := file.Write(payload[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			if errors.Is(err, syscall.EPIPE) {
				return total, rpc.NewError("invalid_transition", "supervisor pipe is broken; child has closed stdin", nil)
			}
			if errors.Is(err, syscall.EAGAIN) {
				select {
				case <-ctx.Done():
					return total, ctx.Err()
				case <-time.After(20 * time.Millisecond):
					continue
				}
			}
			return total, err
		}
		if n == 0 {
			return total, rpc.NewError("invalid_transition", "supervisor pipe write returned zero bytes", nil)
		}
	}
	return total, nil
}

func markPointerDeliveryDegraded(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string, metadata map[string]any, reason string) error {
	updated := map[string]any{}
	for key, value := range metadata {
		updated[key] = value
	}
	delivery := map[string]any{
		"class":       "degraded",
		"healthy":     false,
		"reason":      reason,
		"observed_at": nowString(),
	}
	if tmux := asMap(updated["tmux"]); len(tmux) > 0 {
		tmux["delivery_liveness"] = delivery
		updated["tmux"] = tmux
	} else {
		updated["delivery_liveness"] = delivery
	}
	return mergePointerMetadata(ctx, runner, repositoryID, supervisorID, updated)
}

func launchSupervisedProcess(ctx context.Context, config supervisionStartConfig, supervisorID, scratch, pipePath, eventPath string) (supervisionLaunchResult, error) {
	// RFC 0103 W3 (#141): a supervised helper/lane process must OUTLIVE the daemon
	// so a `systemctl restart striatumd` does not orphan it. exec.CommandContext
	// SIGKILLs the spawned child when its context is Done, and the handler context
	// is canceled on daemon shutdown — so a restart would kill the helper even with
	// the unit's KillMode=process (which only stops systemd's own cgroup kill).
	// Detach the spawn from daemon-lifetime cancellation so the helper (and its
	// tmux-backed agent) survive a restart and the daemon re-binds them on startup;
	// teardown still terminates helpers explicitly by PID (supervise stop / kill).
	ctx = context.WithoutCancel(ctx)
	if config.Transport == supervisionTransportPTYHelper {
		return launchPTYHelper(ctx, config, supervisorID, scratch, pipePath, eventPath)
	}
	return launchPipeProcess(ctx, config, supervisorID, pipePath)
}

func launchPipeProcess(ctx context.Context, config supervisionStartConfig, supervisorID, pipePath string) (supervisionLaunchResult, error) {
	fd, err := syscall.Open(pipePath, syscall.O_RDWR, 0)
	if err != nil {
		return supervisionLaunchResult{}, fmt.Errorf("open stdin fifo: %w", err)
	}
	stdin := os.NewFile(uintptr(fd), "stdin.pipe")
	defer func() { _ = stdin.Close() }()
	laneEnv := supervisedLaneEnv(config, supervisorID)
	cmd := supervisedLaneCommandContext(ctx, config.Command, config.RepoRoot, config.RunAsUser, laneEnv)
	cmd.Stdin = stdin
	stdout, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stdout.Close() }()
	stderr, err := openSupervisedStderr()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stderr.Close() }()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return supervisionLaunchResult{}, fmt.Errorf("cmd.Start: %w", err)
	}
	start, _ := processStartToken(cmd.Process.Pid)
	go func() {
		_ = cmd.Wait()
	}()
	metadata := map[string]any{}
	if config.RunAsUser != "" {
		metadata["run_as_user"] = config.RunAsUser
	}
	return supervisionLaunchResult{PID: cmd.Process.Pid, PIDStartTime: start, Metadata: metadata}, nil
}

// startHelperReaper harvests a supervisor-helper child's exit status so it is
// never left as an unreapable zombie in the daemon's PID table (#204). A helper
// is spawned detached (Setsid + context.WithoutCancel per RFC 0103 W3 / #141) and
// is meant to outlive its launch RPC, so nothing on the request path ever called
// cmd.Wait(); when the helper later exited (e.g. its supervisor was marked lost
// during respawn churn) the kernel kept a <defunct> zombie until the daemon
// itself exited. Reaping the exited child does NOT affect restart survival: the
// reaper only collects the wait status AFTER the process has exited on its own —
// it never cancels the (WithoutCancel) context and never signals the process.
//
// The returned channel closes when the helper exits, so the agent_started
// handshake loops can detect an early helper exit deterministically instead of
// polling cmd.ProcessState (which the reaper now owns — concurrent reads of that
// field while cmd.Wait() writes it would be a data race).
func startHelperReaper(cmd *exec.Cmd) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return done
}

func launchPTYHelper(ctx context.Context, config supervisionStartConfig, supervisorID, scratch, pipePath, eventPath string) (supervisionLaunchResult, error) {
	helper, err := resolveSupervisorHelper()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if err := os.WriteFile(eventPath, nil, 0o600); err != nil {
		return supervisionLaunchResult{}, err
	}
	launchSpec := gosupervisor.HelperLaunchSpec{
		SchemaVersion:   gosupervisor.HelperLaunchSchemaVersion,
		SupervisorID:    supervisorID,
		ScratchDir:      filepath.Dir(scratch),
		Command:         config.Command,
		Env:             supervisedPTYHelperSpecEnv(config, supervisorID),
		WorkingDir:      config.RepoRoot,
		RunAsUser:       config.RunAsUser,
		PacketInputPath: pipePath,
		RequireTmux:     config.RequireTmux,
	}
	specBody, err := json.Marshal(launchSpec)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	specPath := filepath.Join(scratch, "helper-launch.json")
	if err := os.WriteFile(specPath, append(specBody, '\n'), 0o600); err != nil {
		return supervisionLaunchResult{}, err
	}
	eventFile, err := os.OpenFile(eventPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = eventFile.Close() }()
	cmd := exec.CommandContext(ctx, helper)
	cmd.Dir = config.RepoRoot
	cmd.Stdout = eventFile
	stderr, err := openSupervisedStderr()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stderr.Close() }()
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return supervisionLaunchResult{}, err
	}
	reaped := startHelperReaper(cmd)
	if _, err := stdin.Write(append(specBody, '\n')); err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	_ = stdin.Close()
	events, offset, err := waitForHelperAgentStart(reaped, eventPath, helperStartTimeout())
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	agentPID, err := agentPIDFromEvents(events)
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	agentStart, _ := processStartToken(agentPID)
	helperStart, _ := processStartToken(cmd.Process.Pid)
	metadata := map[string]any{
		"transport":               supervisionTransportPTYHelper,
		"helper_binary":           helper,
		"helper_pid":              cmd.Process.Pid,
		"helper_pid_start_time":   helperStart,
		"helper_launch_spec_path": specPath,
		"helper_events_path":      eventPath,
	}
	if config.RunAsUser != "" {
		metadata["run_as_user"] = config.RunAsUser
	}
	if tmux := tmuxMetadataFromHelperEvents(events); tmux != nil {
		metadata["tmux"] = tmux
		if agentStart == "" {
			if token, _ := tmux["pane_start_token"].(string); token != "" {
				agentStart = token
			}
		}
	}
	return supervisionLaunchResult{
		PID:                 agentPID,
		PIDStartTime:        agentStart,
		HelperPID:           cmd.Process.Pid,
		HelperPIDStartTime:  helperStart,
		InitialHelperEvents: events,
		InitialHelperOffset: offset,
		Metadata:            metadata,
	}, nil
}

func launchRebridgeHelper(ctx context.Context, supervisor supervisorControlRow, identity gosupervisor.TmuxIdentity, eventPath string) (supervisionLaunchResult, error) {
	// RFC 0103 W3 (#141): a rebridged helper must also outlive the daemon (see
	// launchSupervisedProcess) so it is not re-orphaned by the next restart.
	ctx = context.WithoutCancel(ctx)
	helper, err := resolveSupervisorHelper()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if eventPath == "" {
		return supervisionLaunchResult{}, fmt.Errorf("helper event path is missing")
	}
	if err := os.MkdirAll(filepath.Dir(eventPath), 0o700); err != nil {
		return supervisionLaunchResult{}, err
	}
	eventFile, err := os.OpenFile(eventPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = eventFile.Close() }()
	startOffset, err := eventFile.Seek(0, io.SeekEnd)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	spec := gosupervisor.HelperLaunchSpec{
		SchemaVersion:   gosupervisor.HelperLaunchSchemaVersion,
		SupervisorID:    supervisor.SupervisorID,
		ScratchDir:      filepath.Dir(supervisor.ScratchPath),
		Env:             rebridgeHelperEnv(supervisor),
		WorkingDir:      supervisorWorkingDir(supervisor),
		RunAsUser:       supervisorRunAsUser(supervisor.Metadata),
		PacketInputPath: supervisor.StdinPipePath,
		RebridgeTmux:    &identity,
	}
	specBody, err := json.Marshal(spec)
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	cmd := exec.CommandContext(ctx, helper)
	cmd.Dir = supervisorWorkingDir(supervisor)
	cmd.Stdout = eventFile
	stderr, err := openSupervisedStderr()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	defer func() { _ = stderr.Close() }()
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return supervisionLaunchResult{}, err
	}
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	}
	if err := cmd.Start(); err != nil {
		return supervisionLaunchResult{}, err
	}
	reaped := startHelperReaper(cmd)
	if _, err := stdin.Write(append(specBody, '\n')); err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	_ = stdin.Close()
	events, offset, err := waitForHelperAgentStartFromOffset(reaped, eventPath, helperStartTimeout(), int(startOffset))
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	agentPID, err := agentPIDFromEvents(events)
	if err != nil {
		_ = terminateProcess(cmd.Process.Pid)
		return supervisionLaunchResult{}, err
	}
	helperStart, _ := processStartToken(cmd.Process.Pid)
	metadata := map[string]any{
		"transport":             supervisionTransportPTYHelper,
		"helper_binary":         helper,
		"helper_pid":            cmd.Process.Pid,
		"helper_pid_start_time": helperStart,
		"helper_events_path":    eventPath,
	}
	if runAsUser := supervisorRunAsUser(supervisor.Metadata); runAsUser != "" {
		metadata["run_as_user"] = runAsUser
	}
	agentStart := identity.PaneStartToken
	if tmux := tmuxMetadataFromHelperEvents(events); tmux != nil {
		metadata["tmux"] = tmux
		if agentStart == "" {
			if token, _ := tmux["pane_start_token"].(string); token != "" {
				agentStart = token
			}
		}
		if attachPID, ok := intValueOptional(tmux["attach_client_pid"]); ok {
			metadata["attach_client_pid"] = attachPID
		}
	}
	return supervisionLaunchResult{
		PID:                 agentPID,
		PIDStartTime:        agentStart,
		HelperPID:           cmd.Process.Pid,
		HelperPIDStartTime:  helperStart,
		InitialHelperEvents: events,
		InitialHelperOffset: offset,
		Metadata:            metadata,
	}, nil
}

func waitForHelperAgentStartFromOffset(reaped <-chan struct{}, eventPath string, timeout time.Duration, startOffset int) ([]map[string]any, int, error) {
	deadline := time.Now().Add(timeout)
	var lastEvents []map[string]any
	lastOffset := startOffset
	for time.Now().Before(deadline) {
		events, offset, err := readHelperEventsFromFile(eventPath, startOffset)
		if err != nil {
			return nil, 0, err
		}
		if len(events) > 0 {
			lastEvents = events
			lastOffset = offset
			for _, event := range events {
				switch event["event_type"] {
				case gosupervisor.HelperEventAgentStarted:
					return events, offset, nil
				case gosupervisor.HelperEventError:
					return nil, 0, fmt.Errorf("PTY helper failed before rebridge: %v", event["payload"])
				case gosupervisor.HelperEventAgentExited:
					return nil, 0, fmt.Errorf("PTY helper agent exited before rebridge")
				}
			}
		}
		if helperExited(reaped) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("PTY helper did not report rebridge agent_started before timeout (events=%d, offset=%d)", len(lastEvents), lastOffset)
}

// helperExited reports whether the reaper goroutine has already collected the
// helper child's exit status (the channel is closed by startHelperReaper after
// cmd.Wait() returns). A non-nil channel that has not closed means the helper is
// still running; a nil channel (test callers that do not spawn a real process)
// is treated as "not exited" so the handshake loop falls through to its timeout.
func helperExited(reaped <-chan struct{}) bool {
	if reaped == nil {
		return false
	}
	select {
	case <-reaped:
		return true
	default:
		return false
	}
}

func supervisorWorkingDir(supervisor supervisorControlRow) string {
	if dir := metadataString(supervisor.Metadata["repo_root"]); dir != "" {
		return dir
	}
	if supervisor.ScratchPath != "" {
		return filepath.Dir(filepath.Dir(filepath.Dir(supervisor.ScratchPath)))
	}
	return "."
}

func tmuxMetadataFromHelperEvents(events []map[string]any) map[string]any {
	for _, event := range events {
		if event["event_type"] != gosupervisor.HelperEventAgentStarted {
			continue
		}
		payload := asMap(event["payload"])
		metadata := asMap(payload["metadata"])
		tmux := asMap(metadata["tmux"])
		if len(tmux) > 0 {
			if lastExit := attachClientExitMetadataFromHelperEvents(events); len(lastExit) > 0 {
				tmux["attach_client_last_exit"] = lastExit
				if delivery := asMap(lastExit["delivery_liveness"]); len(delivery) > 0 {
					tmux["delivery_liveness"] = delivery
				}
			}
			return tmux
		}
	}
	return nil
}

func attachClientExitMetadataFromHelperEvents(events []map[string]any) map[string]any {
	var last map[string]any
	for _, event := range events {
		if event["event_type"] != gosupervisor.HelperEventAttachExited {
			continue
		}
		payload := asMap(event["payload"])
		if len(payload) == 0 {
			continue
		}
		out := map[string]any{}
		if observedAt := metadataString(event["timestamp"]); observedAt != "" {
			out["observed_at"] = observedAt
		}
		if observedAt := metadataString(payload["observed_at"]); observedAt != "" {
			out["observed_at"] = observedAt
		}
		if tmuxLiveness := metadataString(payload["tmux_liveness"]); tmuxLiveness != "" {
			out["tmux_liveness"] = tmuxLiveness
		}
		if pid, ok := intValueOptional(payload["attach_client_pid"]); ok {
			out["attach_pid"] = pid
		} else if pid, ok := intValueOptional(payload["attach_pid"]); ok {
			out["attach_pid"] = pid
		}
		if exitCode, ok := intValueOptional(payload["attach_exit_code"]); ok {
			out["attach_exit_code"] = exitCode
		} else if exitCode, ok := intValueOptional(payload["exit_code"]); ok {
			out["attach_exit_code"] = exitCode
		}
		if panePID, ok := intValueOptional(payload["pid"]); ok {
			out["pane_pid"] = panePID
		}
		delivery := asMap(payload["delivery_liveness"])
		if len(delivery) == 0 && payload["delivery_degraded"] == true {
			observedAt := metadataString(out["observed_at"])
			delivery = map[string]any{
				"class":       "degraded",
				"healthy":     false,
				"reason":      "attach_client_exited",
				"observed_at": observedAt,
			}
		}
		if len(delivery) > 0 {
			out["delivery_liveness"] = delivery
		}
		last = out
	}
	if last == nil {
		return nil
	}
	return last
}

func tmuxPaneStartTokenFromMetadata(metadata map[string]any) string {
	tmux := asMap(metadata["tmux"])
	token, _ := tmux["pane_start_token"].(string)
	return token
}

func objectOrNil(value any) map[string]any {
	object := asMap(value)
	if len(object) == 0 {
		return nil
	}
	return object
}

func waitForHelperAgentStart(reaped <-chan struct{}, eventPath string, timeout time.Duration) ([]map[string]any, int, error) {
	deadline := time.Now().Add(timeout)
	var lastEvents []map[string]any
	lastOffset := 0
	for time.Now().Before(deadline) {
		events, offset, err := readHelperEventsFromFile(eventPath, 0)
		if err != nil {
			return nil, 0, err
		}
		if len(events) > 0 {
			lastEvents = events
			lastOffset = offset
			for _, event := range events {
				switch event["event_type"] {
				case gosupervisor.HelperEventAgentStarted:
					return events, offset, nil
				case gosupervisor.HelperEventError:
					return nil, 0, fmt.Errorf("PTY helper failed before attach: %v", event["payload"])
				case gosupervisor.HelperEventAgentExited:
					return nil, 0, fmt.Errorf("PTY helper agent exited before attach")
				}
			}
		}
		if helperExited(reaped) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, 0, fmt.Errorf("PTY helper did not report agent_started before timeout (events=%d, offset=%d)", len(lastEvents), lastOffset)
}

func agentPIDFromEvents(events []map[string]any) (int, error) {
	for _, event := range events {
		if event["event_type"] != gosupervisor.HelperEventAgentStarted {
			continue
		}
		pid, ok := intValueOptional(asMap(event["payload"])["pid"])
		if !ok {
			return 0, fmt.Errorf("PTY helper did not report agent pid")
		}
		return pid, nil
	}
	return 0, fmt.Errorf("PTY helper did not report agent_started")
}

func drainHelperEvents(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string, wait time.Duration) error {
	metadata, err := pointerMetadata(ctx, runner, repositoryID, supervisorID)
	if err != nil {
		return err
	}
	if metadata["transport"] != supervisionTransportPTYHelper {
		return nil
	}
	path, _ := metadata["helper_events_path"].(string)
	if path == "" {
		return nil
	}
	offset, _ := intValueOptional(metadata["helper_events_offset"])
	deadline := time.Now().Add(wait)
	var events []map[string]any
	newOffset := offset
	for {
		events, newOffset, err = readHelperEventsFromFile(path, offset)
		if err != nil {
			return err
		}
		if len(events) > 0 || time.Now().After(deadline) || wait <= 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if len(events) > 0 {
		for _, event := range events {
			normalized, normErr := normalizeSuperviseReportEvent(event, "", supervisorID, 0)
			if normErr != nil {
				return normErr
			}
			if _, recErr := recordSuperviseReportEvent(ctx, runner, repositoryID, normalized); recErr != nil {
				return recErr
			}
		}
	}
	if newOffset != offset {
		return mergePointerMetadata(ctx, runner, repositoryID, supervisorID, map[string]any{"helper_events_offset": newOffset})
	}
	return nil
}

func readHelperEventsFromFile(path string, offset int) ([]map[string]any, int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, offset, nil
	}
	if err != nil {
		return nil, offset, err
	}
	if offset >= len(data) {
		return nil, len(data), nil
	}
	chunk := data[offset:]
	if len(chunk) == 0 {
		return nil, offset, nil
	}
	complete := chunk
	newOffset := len(data)
	if chunk[len(chunk)-1] != '\n' {
		last := strings.LastIndexByte(string(chunk), '\n')
		if last < 0 {
			return nil, offset, nil
		}
		complete = chunk[:last+1]
		newOffset = offset + last + 1
	}
	if strings.TrimSpace(string(complete)) == "" {
		return nil, newOffset, nil
	}
	events, err := parseHelperJSONL(string(complete))
	return events, newOffset, err
}

func pointerMetadata(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string) (map[string]any, error) {
	var raw any
	err := runner.QueryRow(ctx, `
		SELECT metadata_json
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND supervisor_id = $2`,
		repositoryID, supervisorID,
	).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	return asMap(raw), nil
}

func mergePointerMetadata(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string, metadata map[string]any) error {
	current, err := pointerMetadata(ctx, runner, repositoryID, supervisorID)
	if err != nil {
		return err
	}
	for key, value := range metadata {
		if key == "tmux" {
			currentTmux := asMap(current["tmux"])
			nextTmux := asMap(value)
			if len(currentTmux) > 0 && len(nextTmux) > 0 {
				merged := copyMap(currentTmux)
				for tmuxKey, tmuxValue := range nextTmux {
					merged[tmuxKey] = tmuxValue
				}
				current[key] = merged
				continue
			}
		}
		current[key] = value
	}
	metadataArg, err := db.JSONBArg(runner, current)
	if err != nil {
		return err
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET metadata_json = $1::jsonb
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		metadataArg, repositoryID, supervisorID,
	)
}

func replacePointerMetadata(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID string, metadata map[string]any) error {
	metadataArg, err := db.JSONBArg(runner, metadata)
	if err != nil {
		return err
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET metadata_json = $1::jsonb
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		metadataArg, repositoryID, supervisorID,
	)
}

func updateSupervisorState(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID, daemonSupervisorID, state, updatedAt string, pid int, pidStartTime, heartbeatAt string, endedAt *string, stopReason *string) error {
	pidArg := any(nil)
	if pid > 0 {
		pidArg = pid
	}
	pidStartArg := nullableString(pidStartTime)
	heartbeatArg := nullableString(heartbeatAt)
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisors
		   SET state = $1,
		       pid = COALESCE($2, pid),
		       pid_start_time = COALESCE($3, pid_start_time),
		       heartbeat_at = COALESCE($4, heartbeat_at),
		       ended_at = $5,
		       stop_reason = $6
		 WHERE repository_id = $7 AND supervisor_id = $8`,
		state, pidArg, pidStartArg, heartbeatArg, nullableStringPointer(endedAt), nullableStringPointer(stopReason), repositoryID, supervisorID,
	); err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET state = $1,
		       pid = COALESCE($2, pid),
		       pid_start_time = COALESCE($3, pid_start_time),
		       updated_at = $4
		 WHERE repository_id = $5 AND supervisor_id = $6`,
		state, pidArg, pidStartArg, updatedAt, repositoryID, supervisorID,
	); err != nil {
		return err
	}
	// Issue #62: any terminal supervisor transition — supervise.stop / tmux kill,
	// signal, lost, failed — must remove the per-launch .gemini/settings.json
	// (rotating MCP bearer token) the lane wrote. Centralizing this at the
	// state-transition choke point keeps teardown cleanup path-independent.
	// CleanupGeminiSettings is idempotent (no-ops once its scratch markers are
	// gone), so a redundant call from another path is harmless.
	if supervisorTerminalStates[state] {
		cleanupSupervisorLaneMCPConfig(ctx, runner, repositoryID, supervisorID)
	}
	if daemonSupervisorID == "" {
		return nil
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.daemon_supervisors
		   SET state = $1,
		       pid = COALESCE($2, pid),
		       pid_start_time = COALESCE($3, pid_start_time),
		       heartbeat_at = COALESCE($4, heartbeat_at),
		       ended_at = $5,
		       stop_reason = $6
		 WHERE repository_id = $7 AND daemon_supervisor_id = $8`,
		state, pidArg, pidStartArg, heartbeatArg, nullableStringPointer(endedAt), nullableStringPointer(stopReason), repositoryID, daemonSupervisorID,
	)
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

func refreshSupervisorHeartbeat(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID, daemonSupervisorID, updatedAt string) error {
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisors
		   SET heartbeat_at = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		updatedAt, repositoryID, supervisorID,
	); err != nil {
		return err
	}
	if err := runner.Exec(ctx, `
		UPDATE striatumd.process_supervisor_pointers
		   SET updated_at = $1
		 WHERE repository_id = $2 AND supervisor_id = $3`,
		updatedAt, repositoryID, supervisorID,
	); err != nil {
		return err
	}
	if daemonSupervisorID == "" {
		return nil
	}
	return runner.Exec(ctx, `
		UPDATE striatumd.daemon_supervisors
		   SET heartbeat_at = $1
		 WHERE repository_id = $2 AND daemon_supervisor_id = $3`,
		updatedAt, repositoryID, daemonSupervisorID,
	)
}

func markSupervisorLost(ctx context.Context, runner db.Runner, repositoryID, supervisorID, runID, sessionID, reason string, pid int, payload map[string]any) error {
	_, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		return nil, markSupervisorLostInTx(ctx, tx, repositoryID, supervisorID, runID, sessionID, reason, pid, payload)
	})
	return err
}

func markSupervisorLostWithMetadata(ctx context.Context, runner db.Runner, repositoryID, supervisorID, runID, sessionID, reason string, pid int, metadata map[string]any, payload map[string]any) error {
	_, err := withTx(ctx, runner, func(tx db.TxRunner) (map[string]any, error) {
		if len(metadata) > 0 {
			if mergeErr := mergePointerMetadata(ctx, tx, repositoryID, supervisorID, metadata); mergeErr != nil {
				if payload == nil {
					payload = map[string]any{}
				}
				payload["metadata_persist_error"] = mergeErr.Error()
			}
		}
		return nil, markSupervisorLostInTx(ctx, tx, repositoryID, supervisorID, runID, sessionID, reason, pid, payload)
	})
	return err
}

func markSupervisorLostInTx(ctx context.Context, runner db.TxRunner, repositoryID, supervisorID, runID, sessionID, reason string, pid int, payload map[string]any) error {
	now := nowString()
	daemonSupervisorID := ""
	_ = runner.QueryRow(ctx, `
		SELECT daemon_supervisor_id
		  FROM striatumd.process_supervisor_pointers
		 WHERE repository_id = $1 AND supervisor_id = $2`,
		repositoryID, supervisorID,
	).Scan(&daemonSupervisorID)
	if err := updateSupervisorState(ctx, runner, repositoryID, supervisorID, daemonSupervisorID, "lost", now, pid, "", "", &now, &reason); err != nil {
		return err
	}
	eventPayload := map[string]any{
		"supervisor_id":        supervisorID,
		"daemon_supervisor_id": nullableString(daemonSupervisorID),
		"pid":                  optionalPositiveInt(pid),
		"reason":               reason,
	}
	for key, value := range payload {
		eventPayload[key] = value
	}
	_, err := appendEvent(ctx, runner, repositoryID, runID, "supervisor.lost", sessionID, nil, nil, nil, nil, eventPayload)
	return err
}

func requireSessionExists(ctx context.Context, runner db.Runner, repositoryID, sessionID string) error {
	var found string
	err := runner.QueryRow(ctx, `
		SELECT session_id
		  FROM striatumd.sessions
		 WHERE repository_id = $1 AND session_id = $2
		 LIMIT 1`,
		repositoryID, sessionID,
	).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return rpc.NewError("not_found", "session not found: "+sessionID, nil)
	}
	return err
}

func laneConfig(workflow map[string]any, laneID string) map[string]any {
	lanes := asMap(workflow["lanes"])
	return asMap(lanes[laneID])
}

func commandArray(lane map[string]any) ([]string, error) {
	raw, ok := lane["command"]
	if !ok {
		return nil, rpc.NewError("invalid_transition", "process lane command must be a non-empty array", nil)
	}
	items := asList(raw)
	if len(items) == 0 {
		return nil, rpc.NewError("invalid_transition", "process lane command must be a non-empty array", nil)
	}
	command := make([]string, 0, len(items))
	for _, item := range items {
		part, ok := item.(string)
		if !ok || part == "" {
			return nil, rpc.NewError("invalid_transition", "process lane command entries must be non-empty strings", nil)
		}
		command = append(command, part)
	}
	if lane["adapter"] != "process" {
		return nil, rpc.NewError("invalid_transition", "supervise start requires a process-adapter lane", nil)
	}
	return command, nil
}

func boolLaneValue(values map[string]any, key string) (bool, bool) {
	value, exists := values[key]
	if !exists {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		if normalized == "true" || normalized == "1" {
			return true, true
		}
		if normalized == "false" || normalized == "0" {
			return false, true
		}
	}
	return false, false
}

// laneUsesAgentLoop reports whether a lane opts into the daemon-owned
// agent-loop PTY session (RFC 0088): the command is wrapped in
// `striatumd -agent-loop -- …` and driven over a PTY with a submitted
// bootstrap prompt. Opt-in via lane `agent_loop: true` or
// `adapter_capabilities.agent_loop: true`; default false preserves the
// raw-launch / one-shot-delivery behavior for existing lanes.
func laneUsesAgentLoop(lane map[string]any) bool {
	if value, ok := boolLaneValue(lane, "agent_loop"); ok {
		return value
	}
	capabilities := asMap(lane["adapter_capabilities"])
	if value, ok := boolLaneValue(capabilities, "agent_loop"); ok {
		return value
	}
	return false
}

// requireSupportedAgentLoopAdapter refuses an agent-loop lane whose argv0 is not
// a self-driving-capable adapter (codex / agy / claude). The predicate is
// agentloop.BootstrapDeliveryModeFor — the canonical bootstrap-delivery contract,
// pinned by the conformance C0 golden — so this guard cannot drift from the
// agent-loop wiring when an adapter is added or removed. The refusal names the
// offending argv0 and the supported adapters so the operator can fix the lane
// command (#181, RFC 0111 legibility).
func requireSupportedAgentLoopAdapter(command []string) error {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return rpc.NewError("invalid_transition", "agent-loop lane command must be non-empty", nil)
	}
	if agentloop.BootstrapDeliveryModeFor(command) == agentloop.BootstrapDeliveryArgv {
		return nil
	}
	adapter := agentloop.LaneAdapterName(command[0])
	return rpc.NewError("invalid_transition", fmt.Sprintf(
		"supervise start refuses agent-loop lane: adapter %q (argv0 %q) cannot run the self-driving loop; supported agent-loop adapters are codex, agy, claude. Set the lane command to one of those, or drop adapter_capabilities.agent_loop so the lane runs as a stdin-FIFO push consumer.",
		adapter, command[0]), nil)
}

// selfDrivingAgentLoopCommand wraps a raw lane command in the agent-loop
// executor so the daemon-owned PTY session delivers the bootstrap prompt.
func selfDrivingAgentLoopCommand(command []string) ([]string, error) {
	if len(command) == 0 {
		return nil, rpc.NewError("invalid_transition", "self-driving lane command must be non-empty", nil)
	}
	if agentLoopFlagIndex(command) >= 0 {
		return append([]string(nil), command...), nil
	}
	return append([]string{agentLoopExecutable(), "-agent-loop", "--"}, command...), nil
}

// resolveSupervisedCommandBinary rewrites command[0] to an absolute path found
// on the augmented supervised PATH, so the lane binary resolves regardless of
// the daemon's own PATH. A no-op when argv0 is already a path or cannot be
// resolved (the launch will then surface the original not-found error).
func resolveSupervisedCommandBinary(command []string) []string {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return command
	}
	bin := command[0]
	if strings.ContainsRune(bin, os.PathSeparator) {
		return command
	}
	for _, dir := range filepath.SplitList(supervisedPath()) {
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, bin)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			out := append([]string(nil), command...)
			out[0] = candidate
			return out
		}
	}
	return command
}

func agentLoopFlagIndex(command []string) int {
	limit := len(command)
	if limit > 4 {
		limit = 4
	}
	for i := 0; i < limit; i++ {
		if command[i] == "-agent-loop" || command[i] == "--agent-loop" {
			return i
		}
	}
	return -1
}

func agentLoopExecutable() string {
	if override := strings.TrimSpace(os.Getenv("STRIATUM_AGENT_LOOP_BINARY")); override != "" {
		return override
	}
	if executable, err := os.Executable(); err == nil && executable != "" {
		return executable
	}
	return "striatumd"
}

func supervisionTransport(lane map[string]any) (string, error) {
	supervision, err := laneSupervision(lane)
	if err != nil {
		return "", err
	}
	if supervision == nil {
		if laneUsesAgentLoop(lane) {
			return supervisionTransportPTYHelper, nil
		}
		return supervisionTransportPipe, nil
	}
	transport, _ := supervision["transport"].(string)
	if transport == "" {
		if laneUsesAgentLoop(lane) {
			return supervisionTransportPTYHelper, nil
		}
		return supervisionTransportPipe, nil
	}
	if transport == supervisionTransportPipe {
		return supervisionTransportPipe, nil
	}
	if transport == supervisionTransportPTYHelper {
		return supervisionTransportPTYHelper, nil
	}
	return "", rpc.NewError("invalid_transition", "lane supervision.transport must be 'pipe' or 'pty_helper'", nil)
}

func supervisionStdinDelivery(lane map[string]any, transport string) (string, error) {
	supervision, err := laneSupervision(lane)
	if err != nil {
		return "", err
	}
	if supervision == nil {
		return stdinDeliveryPersistentFIFO, nil
	}
	mode, _ := supervision["stdin_delivery"].(string)
	if mode == "" {
		return stdinDeliveryPersistentFIFO, nil
	}
	if mode != stdinDeliveryPersistentFIFO && mode != stdinDeliveryOneShotEOF {
		return "", rpc.NewError("invalid_transition", "lane supervision.stdin_delivery must be 'persistent_fifo' or 'one_shot_eof'", nil)
	}
	if mode == stdinDeliveryOneShotEOF && transport != supervisionTransportPipe {
		return "", rpc.NewError("invalid_transition", "lane supervision.stdin_delivery='one_shot_eof' requires supervision.transport='pipe'", nil)
	}
	return mode, nil
}

func supervisionRequireTmux(lane map[string]any, transport string) (bool, error) {
	supervision, err := laneSupervision(lane)
	if err != nil {
		return false, err
	}
	if supervision == nil {
		return false, nil
	}
	raw, ok := supervision["require_tmux"]
	if !ok {
		return false, nil
	}
	requireTmux, ok := raw.(bool)
	if !ok {
		return false, rpc.NewError("invalid_transition", "lane supervision.require_tmux must be a boolean", nil)
	}
	if requireTmux && transport != supervisionTransportPTYHelper {
		return false, rpc.NewError("invalid_transition", "lane supervision.require_tmux=true requires supervision.transport='pty_helper'", nil)
	}
	return requireTmux, nil
}

func laneSupervision(lane map[string]any) (map[string]any, error) {
	raw, ok := lane["supervision"]
	if !ok || raw == nil {
		return nil, nil
	}
	supervision, ok := raw.(map[string]any)
	if ok {
		return supervision, nil
	}
	return nil, rpc.NewError("invalid_transition", "lane supervision must be an object when provided", nil)
}

func metadataStdinDelivery(metadata map[string]any) string {
	value, _ := metadata["stdin_delivery"].(string)
	if value == stdinDeliveryOneShotEOF || value == stdinDeliveryPersistentFIFO {
		return value
	}
	return stdinDeliveryPersistentFIFO
}

func currentDaemonInstanceID() string {
	if value := os.Getenv("STRIATUM_DAEMON_INSTANCE_ID"); value != "" {
		return value
	}
	return "go-pg-handler"
}

// supervisedEnv builds the full environment for a supervised lane process.
//
// #87 (RFC 0096 §2): the lane env is constructed from an EXPLICIT ALLOWLIST,
// never from os.Environ(). Handing the lane the daemon's entire environment
// leaked any secret in the daemon env — a Postgres DSN, future cloud creds —
// into the lane and its visible pane (the live incident). supervisedEnvEntries
// already emits the explicit STRIATUM_* + PATH base with NO inheritance; this
// wrapper adds only the small, named pass-through set the adapters genuinely
// need (agent-loop MCP bootstrap vars + a handful of OS basics). Everything
// else — including every *DSN*/*POSTGRES*/PG*/DATABASE_URL var — is dropped.
//
// adapter is the bare CLI adapter name (agentloop.LaneAdapterName of the raw
// lane argv0, e.g. "claude"); it scopes per-adapter env hardening such as the
// #101 Claude Code welcome/update-nag suppression. It is the OriginalCommand
// adapter, not the agent-loop wrapper ("striatumd"), so the keys reach the real
// child CLI regardless of agent-loop wrapping.
func supervisedEnv(adapter, repoRoot, repositoryID, runID, sessionID, supervisorID, laneID, boundToken string) []string {
	entries := supervisedEnvEntries(adapter, repoRoot, repositoryID, runID, sessionID, supervisorID, laneID, boundToken)
	return mergeEnvReplacing(supervisedEnvPassThrough(os.Environ()), entries)
}

func supervisedLaneEnv(config supervisionStartConfig, supervisorID string) []string {
	adapter := config.adapterName()
	if strings.TrimSpace(config.RunAsUser) == "" {
		return supervisedEnv(adapter, config.RepoRoot, config.RepositoryID, config.RunID, config.SessionID, supervisorID, config.LaneID, config.CapabilityToken)
	}
	entries := supervisedEnvEntries(adapter, config.RepoRoot, config.RepositoryID, config.RunID, config.SessionID, supervisorID, config.LaneID, config.CapabilityToken)
	base := supervisedRunAsPassThrough(os.Environ(), config.RunAsUser)
	if endpoint, err := agentloop.ResolveMCPEndpoint(config.RepoRoot, os.Environ()); err == nil && strings.TrimSpace(endpoint) != "" {
		base = mergeEnvReplacing(base, []string{"STRIATUM_MCP_URL=" + endpoint})
	}
	return mergeEnvReplacing(base, entries)
}

func supervisedPTYHelperSpecEnv(config supervisionStartConfig, supervisorID string) []string {
	if strings.TrimSpace(config.RunAsUser) == "" {
		return supervisedEnvEntries(config.adapterName(), config.RepoRoot, config.RepositoryID, config.RunID, config.SessionID, supervisorID, config.LaneID, config.CapabilityToken)
	}
	return supervisedLaneEnv(config, supervisorID)
}

func rebridgeHelperEnv(supervisor supervisorControlRow) []string {
	runAsUser := supervisorRunAsUser(supervisor.Metadata)
	if runAsUser == "" {
		return []string{"PATH=" + supervisedPath()}
	}
	env := []string{"PATH=" + supervisedPath()}
	env = append(env, laneUserIdentityEnv(runAsUser)...)
	return env
}

func supervisedRunAsPassThrough(base []string, runAsUser string) []string {
	out := make([]string, 0, len(supervisedEnvAllowlistKeys))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		switch key {
		case "STRIATUM_MCP_URL",
			"STRIATUM_MCP_ADDR",
			"STRIATUM_MCP_PORT",
			"STRIATUM_DAEMON_MCP_HTTP_URL",
			"STRIATUM_DAEMON_MCP_HTTP_ENDPOINT_FILE",
			"STRIATUM_DAEMON_MCP_HTTP_ADDR",
			"STRIATUM_DAEMON_MCP_HTTP_PORT",
			"STRIATUM_DAEMON_RUNTIME_DIR",
			"TERM",
			"COLORTERM",
			"LANG",
			"LANGUAGE",
			"TZ":
			out = append(out, entry)
		default:
			if strings.HasPrefix(key, "LC_") {
				out = append(out, entry)
			}
		}
	}
	return mergeEnvReplacing(out, laneUserIdentityEnv(runAsUser))
}

func laneUserIdentityEnv(runAsUser string) []string {
	runAsUser = strings.TrimSpace(runAsUser)
	if runAsUser == "" {
		return nil
	}
	entries := []string{
		"USER=" + runAsUser,
		"LOGNAME=" + runAsUser,
	}
	if home := laneOSUserHome(runAsUser); home != "" {
		entries = append(entries, "HOME="+home)
	}
	return entries
}

var laneOSUserHome = func(name string) string {
	u, err := user.Lookup(name)
	if err != nil || strings.TrimSpace(u.HomeDir) == "" {
		return ""
	}
	return u.HomeDir
}

func configuredLaneRunAsUser() string {
	laneUser := strings.TrimSpace(os.Getenv(supervisedLaneOSUserEnv))
	if laneUser == "" {
		return ""
	}
	daemonUser := currentOSUsername()
	if daemonUser != "" && sameOSUsername(laneUser, daemonUser) {
		return ""
	}
	return laneUser
}

func currentOSUsername() string {
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		return strings.TrimSpace(u.Username)
	}
	return strings.TrimSpace(os.Getenv("USER"))
}

func sameOSUsername(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	return strings.HasSuffix(b, `\`+a)
}

func supervisedLaneCommandContext(ctx context.Context, command []string, workingDir, runAsUser string, env []string) *exec.Cmd {
	if strings.TrimSpace(runAsUser) == "" {
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Dir = workingDir
		cmd.Env = env
		return cmd
	}
	args := []string{"-n", "-u", strings.TrimSpace(runAsUser), "--", "env", "-i"}
	args = append(args, supervisedRunAsExecEnv(env)...)
	args = append(args, command...)
	cmd := exec.CommandContext(ctx, "sudo", args...)
	cmd.Dir = workingDir
	return cmd
}

func supervisedRunAsExecEnv(env []string) []string {
	seen := map[string]int{}
	for i, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			seen[key] = i
		}
	}
	out := make([]string, 0, len(seen))
	hasPath := false
	for i, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || seen[key] != i {
			continue
		}
		if key == "PATH" {
			hasPath = true
		}
		out = append(out, entry)
	}
	if !hasPath {
		out = append([]string{"PATH=" + supervisedPath()}, out...)
	}
	return out
}

func supervisedEnvEntries(adapter, repoRoot, repositoryID, runID, sessionID, supervisorID, laneID, boundToken string) []string {
	entries := []string{
		"PATH=" + supervisedPath(),
		"STRIATUM_REPOSITORY_ID=" + repositoryID,
		"STRIATUM_RUN_ID=" + runID,
		"STRIATUM_SESSION_ID=" + sessionID,
		"STRIATUM_SUPERVISOR_ID=" + supervisorID,
		"STRIATUM_REPO=" + repoRoot,
		"STRIATUM_LANE_ID=" + laneID,
	}
	// RFC 0096 V2 / #135: inject the lane's OWN session-bound capability token as
	// STRIATUM_MCP_TOKEN. It is set as an explicit entry (winning over any
	// inherited value via mergeEnvReplacing) and is the FIRST source
	// agentloop.ResolveTokenMaterial consults, so claude (inline --mcp-config),
	// codex (env), and agy (.gemini/settings.json) all authenticate with it. The
	// shared-token pass-through has been removed from supervisedEnvAllowlistKeys,
	// so when no bound token is provided the lane gets none (fails loudly) rather
	// than silently inheriting the daemon's operator override.
	if strings.TrimSpace(boundToken) != "" {
		entries = append(entries, "STRIATUM_MCP_TOKEN="+boundToken)
	}
	return append(entries, supervisedAdapterEnvEntries(adapter)...)
}

// supervisedAdapterEnvEntries returns the per-adapter, non-secret operational
// env knobs a supervised lane needs so its CLI acts on the work packet instead
// of parking on a startup splash. It is the env sibling of the per-adapter
// command/config hardening in agentloop.injectLaneMCPConfig (and of the agy
// usageStatisticsEnabled:false survey-suppression, #76).
//
// claude (#101 / RFC 0101 L2 lane-env hardening): a daemon-spawned Claude Code
// lane otherwise parks on the auto-updater "a new version is available" nag /
// onboarding splash and never acts on its packet — the single most common live
// dogfood wedge (the implement-lane stall behind #121). These are the
// authoritative env switches (Claude Code docs, code.claude.com/docs/en/env-vars,
// confirmed present in the installed claude 2.1.159 binary):
//
//   - DISABLE_AUTOUPDATER=1: disable the auto-updater + its "update available"
//     check; per the docs it takes precedence over the autoUpdates config.
//   - CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1: the bundle switch — equivalent
//     to setting DISABLE_AUTOUPDATER, DISABLE_FEEDBACK_COMMAND,
//     DISABLE_ERROR_REPORTING, and DISABLE_TELEMETRY together. We set it
//     alongside the explicit DISABLE_AUTOUPDATER so the update-nag stays
//     suppressed even if a future build narrows the bundle.
//
// Both keys are claude-namespaced / claude-read and harmless to other adapters,
// but we scope them per-adapter to keep the lane env minimal and the intent
// auditable. The first-run onboarding/theme splash is gated on the ~/.claude.json
// hasCompletedOnboarding config flag rather than an env var; we deliberately do
// NOT write the operator's ~/.claude.json (see report), so env covers the
// update-nag (the live #101 wedge) but not a never-onboarded profile.
func supervisedAdapterEnvEntries(adapter string) []string {
	switch adapter {
	case "claude":
		return []string{
			"DISABLE_AUTOUPDATER=1",
			"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		}
	default:
		return nil
	}
}

// supervisedEnvAllowlistKeys are the EXACT daemon-env var names a supervised
// lane is allowed to inherit (#87 / RFC 0096 §2). The list is deliberately
// explicit and easy to audit: if a new var must reach a lane, add it here on
// purpose. Anything not named here — every DSN/Postgres/credential var — is
// dropped. PATH and the STRIATUM_REPOSITORY_ID/RUN_ID/SESSION_ID/SUPERVISOR_ID/
// REPO/LANE_ID vars are NOT listed here because supervisedEnvEntries always
// sets them freshly (overriding any inherited value via mergeEnvReplacing).
var supervisedEnvAllowlistKeys = map[string]bool{
	// Agent-loop MCP bootstrap (go/pkg/agentloop endpoint.go / token.go /
	// bootstrap.go AgentEnvironment): the `striatumd -agent-loop` subprocess
	// resolves the live MCP endpoint from its own environment, so the daemon must
	// pass the endpoint vars through for a lane to reach the control plane.
	//
	// RFC 0096 V2 / #135: STRIATUM_MCP_TOKEN and STRIATUM_MCP_TOKEN_FILE are
	// DELIBERATELY NOT allowlisted here. The lane must authenticate with its OWN
	// session-bound token (minted at supervise start and injected explicitly by
	// supervisedEnvEntries), never the daemon's shared operator-override token.
	// Allowlisting the shared token would let the per-session impersonation guard
	// be bypassed in live runs (the spoof would be treated as an honest operator
	// override). With both removed, the only token a lane can carry is the
	// injected bound one.
	"STRIATUM_MCP_URL":                       true,
	"STRIATUM_MCP_ADDR":                      true,
	"STRIATUM_MCP_PORT":                      true,
	"STRIATUM_DAEMON_MCP_HTTP_URL":           true,
	"STRIATUM_DAEMON_MCP_HTTP_ENDPOINT_FILE": true,
	"STRIATUM_DAEMON_MCP_HTTP_ADDR":          true,
	"STRIATUM_DAEMON_MCP_HTTP_PORT":          true,
	"STRIATUM_DAEMON_RUNTIME_DIR":            true,
	"STRIATUM_DAEMON_SOCKET":                 true,
	// Supervised-lane operational knobs (diagnostics + PATH augmentation), set
	// by the operator, not secrets.
	"STRIATUM_SUPERVISED_STDERR_LOG": true,
	"STRIATUM_SUPERVISED_PATH_DIRS":  true,
	"STRIATUM_SUPERVISOR_HELPER":     true,
	// OS basics every interactive CLI adapter (claude/codex/agy) needs.
	"HOME":            true,
	"USER":            true,
	"LOGNAME":         true,
	"TERM":            true,
	"COLORTERM":       true,
	"LANG":            true,
	"LANGUAGE":        true,
	"TZ":              true,
	"TMPDIR":          true,
	"XDG_RUNTIME_DIR": true,
	"XDG_CONFIG_HOME": true,
	"XDG_DATA_HOME":   true,
	"XDG_CACHE_HOME":  true,
	"SSH_AUTH_SOCK":   true, // git over ssh for lanes that push/fetch
}

// supervisedEnvPassThrough filters a base environment down to the explicit
// allowlist (#87). LC_* locale vars are matched by prefix because they are a
// small open family (LC_ALL, LC_CTYPE, LC_MESSAGES, …) that adapters expect for
// correct text handling; none of them carry secrets.
func supervisedEnvPassThrough(base []string) []string {
	out := make([]string, 0, len(supervisedEnvAllowlistKeys))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if supervisedEnvAllowlistKeys[key] || strings.HasPrefix(key, "LC_") {
			out = append(out, entry)
		}
	}
	return out
}

func mergeEnvReplacing(base []string, updates []string) []string {
	keys := map[string]bool{}
	for _, entry := range updates {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key != "" {
			keys[key] = true
		}
	}
	out := make([]string, 0, len(base)+len(updates))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" || keys[key] {
			continue
		}
		out = append(out, entry)
	}
	return append(out, updates...)
}

// openSupervisedStderr returns the stderr sink for a supervised lane: by
// default /dev/null (D028 no-capture), but if STRIATUM_SUPERVISED_STDERR_LOG
// is set, it's appended to that path. Used to surface agent-loop / lane
// failures that would otherwise be silent — debug only.
func openSupervisedStderr() (*os.File, error) {
	if path := strings.TrimSpace(os.Getenv("STRIATUM_SUPERVISED_STDERR_LOG")); path != "" {
		return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	}
	return os.OpenFile(os.DevNull, os.O_WRONLY, 0)
}

func supervisedPath() string {
	current := os.Getenv("PATH")
	entries := filepath.SplitList(current)
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry != "" {
			seen[entry] = true
		}
	}
	for _, dir := range supervisedPathDirs() {
		if seen[dir] {
			continue
		}
		entries = append(entries, dir)
		seen[dir] = true
	}
	return strings.Join(entries, string(os.PathListSeparator))
}

func supervisedPathDirs() []string {
	rawDirs := []string{}
	if override := strings.TrimSpace(os.Getenv("STRIATUM_SUPERVISED_PATH_DIRS")); override != "" {
		rawDirs = filepath.SplitList(override)
	} else if home := supervisedHomeDir(); home != "" {
		rawDirs = []string{
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".npm-global", "bin"),
		}
	}
	dirs := make([]string, 0, len(rawDirs))
	seen := map[string]bool{}
	for _, raw := range rawDirs {
		dir := filepath.Clean(strings.TrimSpace(raw))
		if dir == "." || dir == "" || !filepath.IsAbs(dir) || seen[dir] {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		dirs = append(dirs, dir)
		seen[dir] = true
	}
	return dirs
}

func supervisedHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	return strings.TrimSpace(os.Getenv("HOME"))
}

func resolveSupervisorHelper() (string, error) {
	if override := os.Getenv("STRIATUM_SUPERVISOR_HELPER"); override != "" {
		return override, nil
	}
	if found, err := exec.LookPath("striatum-supervisor-helper"); err == nil {
		return found, nil
	}
	repoHelper := filepath.Join("go", "bin", "striatum-supervisor-helper")
	if _, err := os.Stat(repoHelper); err == nil {
		abs, _ := filepath.Abs(repoHelper)
		return abs, nil
	}
	return "", fmt.Errorf("striatum-supervisor-helper not found; set STRIATUM_SUPERVISOR_HELPER or build go/bin/striatum-supervisor-helper")
}

func helperStartTimeout() time.Duration {
	raw := os.Getenv("STRIATUM_SUPERVISOR_HELPER_START_TIMEOUT")
	if raw == "" {
		return 5 * time.Second
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil || seconds < 0.1 {
		return 5 * time.Second
	}
	return time.Duration(seconds * float64(time.Second))
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

func requiredControlTextParam(envelope rpc.Envelope, key string, message string) (string, error) {
	value, ok := envelope.Params[key]
	if !ok || value == nil {
		return "", rpc.NewError("schema_invalid", message, nil)
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return "", rpc.NewError("schema_invalid", message, nil)
	}
	return text, nil
}

func helperProcessPayload(transport string, helperPID int, helperStart, eventPath string) any {
	if transport != supervisionTransportPTYHelper {
		return nil
	}
	return map[string]any{
		"pid":            optionalPositiveInt(helperPID),
		"pid_start_time": nullableString(helperStart),
		"events_path":    eventPath,
	}
}

func laneAttestation(pidStartTime string) string {
	if pidStartTime == "" {
		return "unattested"
	}
	return "attested"
}

func nullableStringPointer(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func optionalPositiveInt(value int) any {
	if value <= 0 {
		return nil
	}
	return value
}

func optionalIntValue(value int, ok bool) any {
	if !ok {
		return nil
	}
	return value
}

func intValueOptional(value any) (int, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case int:
		return typed, true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err == nil
	case string:
		if typed == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	}
	return 0, false
}

func copyMap(input map[string]any) map[string]any {
	output := map[string]any{}
	for key, value := range input {
		output[key] = value
	}
	return output
}
