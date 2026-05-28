package supervisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type TmuxIdentity struct {
	SessionName    string
	WindowID       string
	PaneID         string
	PanePID        int
	PaneStartToken string
}

type TmuxLivenessClass string

const (
	TmuxLivenessOK              TmuxLivenessClass = "tmux_ok"
	TmuxLivenessSessionMissing  TmuxLivenessClass = "tmux_session_missing"
	TmuxLivenessPaneMissing     TmuxLivenessClass = "tmux_pane_missing"
	TmuxLivenessPaneDead        TmuxLivenessClass = "tmux_pane_dead"
	TmuxLivenessPanePIDMismatch TmuxLivenessClass = "tmux_pane_pid_mismatch"
	TmuxLivenessUnavailable     TmuxLivenessClass = "tmux_unavailable"
)

const (
	PIDLivenessOK                  = "pid_ok"
	PIDLivenessGone                = "pid_gone"
	PIDLivenessIdentityMismatch    = "pid_identity_mismatch"
	PIDLivenessIdentityUnavailable = "pid_identity_unavailable"
)

type TmuxLiveness struct {
	Class            TmuxLivenessClass
	Healthy          bool
	ObservedPanePID  int
	ObservedStartTok string
	Detail           string
}

type LaneLiveness struct {
	Backed      string
	Alive       bool
	Class       string
	Tmux        *TmuxLiveness
	ObservedPID int
	Detail      string
}

type TmuxRunner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

type execTmuxRunner struct {
	timeout time.Duration
}

func DefaultTmuxRunner() TmuxRunner {
	return execTmuxRunner{timeout: tmuxProbeTimeout()}
}

func (r execTmuxRunner) Run(ctx context.Context, args ...string) (string, error) {
	timeout := r.timeout
	if timeout <= 0 {
		timeout = tmuxProbeTimeout()
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if runCtx.Err() != nil {
		return stdout.String(), runCtx.Err()
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return stdout.String(), fmt.Errorf("%w: %s", err, detail)
	}
	return stdout.String(), nil
}

func tmuxProbeTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("STRIATUM_TMUX_PROBE_TIMEOUT"))
	if raw == "" {
		return 2 * time.Second
	}
	if duration, err := time.ParseDuration(raw); err == nil && duration > 0 {
		return duration
	}
	if seconds, err := strconv.ParseFloat(raw, 64); err == nil && seconds > 0 {
		return time.Duration(seconds * float64(time.Second))
	}
	return 2 * time.Second
}

func CaptureTmuxIdentity(ctx context.Context, r TmuxRunner, sessionName string) (TmuxIdentity, error) {
	out, err := r.Run(ctx, "display-message", "-p", "-t", sessionName, "#{window_id}|#{pane_id}|#{pane_pid}|#{pane_start_time}")
	if err != nil {
		return TmuxIdentity{}, err
	}
	parts := strings.Split(strings.TrimSpace(out), "|")
	if len(parts) < 3 {
		return TmuxIdentity{}, fmt.Errorf("tmux identity capture returned %d fields", len(parts))
	}
	panePID, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil || panePID <= 0 {
		return TmuxIdentity{}, fmt.Errorf("tmux identity capture returned invalid pane pid %q", parts[2])
	}
	token := ""
	if len(parts) >= 4 {
		token = verifiedStartToken(parts[3])
	}
	if token == "" {
		if fallback, ok := ProcessStartToken(panePID); ok {
			token = verifiedStartToken(fallback)
		}
	}
	return TmuxIdentity{
		SessionName:    sessionName,
		WindowID:       strings.TrimSpace(parts[0]),
		PaneID:         strings.TrimSpace(parts[1]),
		PanePID:        panePID,
		PaneStartToken: token,
	}, nil
}

func ProbeTmuxLiveness(ctx context.Context, r TmuxRunner, id TmuxIdentity) TmuxLiveness {
	if r == nil {
		r = DefaultTmuxRunner()
	}
	if strings.TrimSpace(id.SessionName) == "" {
		return TmuxLiveness{Class: TmuxLivenessSessionMissing, Healthy: false, Detail: "tmux session name missing"}
	}
	if _, err := r.Run(ctx, "has-session", "-t", id.SessionName); err != nil {
		if tmuxProbeUnavailable(err) {
			return TmuxLiveness{Class: TmuxLivenessUnavailable, Healthy: false, Detail: tmuxProbeDetail(err)}
		}
		return TmuxLiveness{Class: TmuxLivenessSessionMissing, Healthy: false, Detail: tmuxProbeDetail(err)}
	}
	if strings.TrimSpace(id.PaneID) == "" {
		return TmuxLiveness{Class: TmuxLivenessPaneMissing, Healthy: false, Detail: "tmux pane id missing"}
	}
	out, err := r.Run(ctx, "display-message", "-p", "-t", id.PaneID, "#{pane_id}|#{pane_pid}|#{pane_dead}|#{pane_start_time}")
	if err != nil {
		if tmuxProbeUnavailable(err) {
			return TmuxLiveness{Class: TmuxLivenessUnavailable, Healthy: false, Detail: tmuxProbeDetail(err)}
		}
		return TmuxLiveness{Class: TmuxLivenessPaneMissing, Healthy: false, Detail: tmuxProbeDetail(err)}
	}
	parts := strings.Split(strings.TrimSpace(out), "|")
	if len(parts) < 3 {
		return TmuxLiveness{Class: TmuxLivenessPaneMissing, Healthy: false, Detail: "tmux pane query returned incomplete identity"}
	}
	observedPaneID := strings.TrimSpace(parts[0])
	if observedPaneID != id.PaneID {
		return TmuxLiveness{Class: TmuxLivenessPaneMissing, Healthy: false, Detail: "tmux pane id mismatch"}
	}
	observedPID, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || observedPID <= 0 {
		return TmuxLiveness{Class: TmuxLivenessPaneMissing, Healthy: false, Detail: "tmux pane pid missing"}
	}
	observedStart := ""
	if len(parts) >= 4 {
		observedStart = verifiedStartToken(parts[3])
	}
	expectedStart := verifiedStartToken(id.PaneStartToken)
	if observedStart == "" && expectedStart != "" {
		if fallback, ok := ProcessStartToken(observedPID); ok {
			observedStart = verifiedStartToken(fallback)
		}
	}
	if strings.TrimSpace(parts[2]) == "1" {
		return TmuxLiveness{Class: TmuxLivenessPaneDead, Healthy: false, ObservedPanePID: observedPID, ObservedStartTok: observedStart}
	}
	if observedPID != id.PanePID {
		return TmuxLiveness{Class: TmuxLivenessPanePIDMismatch, Healthy: false, ObservedPanePID: observedPID, ObservedStartTok: observedStart}
	}
	if expectedStart != "" && observedStart != "" && observedStart != expectedStart {
		return TmuxLiveness{Class: TmuxLivenessPanePIDMismatch, Healthy: false, ObservedPanePID: observedPID, ObservedStartTok: observedStart}
	}
	detail := ""
	if expectedStart == "" || observedStart == "" {
		detail = "start_token_unverified"
	}
	return TmuxLiveness{
		Class:            TmuxLivenessOK,
		Healthy:          true,
		ObservedPanePID:  observedPID,
		ObservedStartTok: observedStart,
		Detail:           detail,
	}
}

func ProbeLaneLiveness(ctx context.Context, r TmuxRunner, metadata map[string]any, pid int, expectedStartToken string) LaneLiveness {
	if os.Getenv("STRIATUM_TMUX_PROBE_DISABLE") != "1" {
		if id, ok := TmuxIdentityFromMetadata(metadata); ok {
			tmux := ProbeTmuxLiveness(ctx, r, id)
			return LaneLiveness{
				Backed:      "tmux",
				Alive:       tmux.Healthy,
				Class:       string(tmux.Class),
				Tmux:        &tmux,
				ObservedPID: tmux.ObservedPanePID,
				Detail:      tmux.Detail,
			}
		}
	}
	alive, currentStart, reason := PIDLiveWithStartToken(pid, expectedStartToken)
	class := PIDLivenessOK
	if !alive {
		class = reason
	}
	return LaneLiveness{
		Backed:      "plain_pty",
		Alive:       alive,
		Class:       class,
		ObservedPID: pid,
		Detail:      currentStart,
	}
}

func PIDLiveWithStartToken(pid int, expectedStart string) (bool, string, string) {
	if !pidSignalable(pid) {
		return false, "", PIDLivenessGone
	}
	currentStart := ""
	if expectedStart != "" {
		var ok bool
		currentStart, ok = ProcessStartToken(pid)
		if !ok || currentStart == "" {
			return false, currentStart, PIDLivenessIdentityUnavailable
		}
		if currentStart != expectedStart {
			return false, currentStart, PIDLivenessIdentityMismatch
		}
	}
	return true, currentStart, ""
}

func TmuxIdentityFromMetadata(metadata map[string]any) (TmuxIdentity, bool) {
	tmux := objectValue(metadata["tmux"])
	if strings.TrimSpace(stringValue(tmux["state"])) != "backed" {
		return TmuxIdentity{}, false
	}
	panePID, ok := intValue(tmux["pane_pid"])
	if !ok || panePID <= 0 {
		return TmuxIdentity{}, false
	}
	id := TmuxIdentity{
		SessionName:    stringValue(tmux["session_name"]),
		WindowID:       stringValue(tmux["window_id"]),
		PaneID:         stringValue(tmux["pane_id"]),
		PanePID:        panePID,
		PaneStartToken: verifiedStartToken(stringValue(tmux["pane_start_token"])),
	}
	return id, id.SessionName != "" && id.PaneID != ""
}

func verifiedStartToken(raw string) string {
	token := strings.TrimSpace(raw)
	if token == "" {
		return ""
	}
	if _, err := strconv.ParseUint(token, 10, 64); err != nil {
		return ""
	}
	return token
}

func TmuxLivenessLost(class string) bool {
	switch TmuxLivenessClass(class) {
	case TmuxLivenessOK, TmuxLivenessUnavailable:
		return false
	default:
		return strings.HasPrefix(class, "tmux_")
	}
}

func TmuxLivenessPayload(live TmuxLiveness) map[string]any {
	payload := map[string]any{
		"class":     string(live.Class),
		"healthy":   live.Healthy,
		"detail":    nil,
		"backed_by": "tmux",
	}
	if live.ObservedPanePID > 0 {
		payload["observed_pane_pid"] = live.ObservedPanePID
	}
	if live.ObservedStartTok != "" {
		payload["observed_pane_start"] = live.ObservedStartTok
	}
	if live.Detail != "" {
		payload["detail"] = live.Detail
	}
	return payload
}

func tmuxProbeUnavailable(err error) bool {
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, context.DeadlineExceeded)
}

func tmuxProbeDetail(err error) string {
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return "tmux: command not found"
	case errors.Is(err, context.DeadlineExceeded):
		return "probe_timeout"
	default:
		return strings.TrimSpace(err.Error())
	}
}

func pidSignalable(pid int) bool {
	if pid <= 0 || processZombie(pid) {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func processZombie(pid int) bool {
	if pid <= 0 {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	text := string(data)
	idx := strings.LastIndex(text, ")")
	if idx < 0 || idx+1 >= len(text) {
		return false
	}
	fields := strings.Fields(text[idx+1:])
	return len(fields) > 0 && fields[0] == "Z"
}

func objectValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			out[key] = value
		}
		return out
	default:
		return map[string]any{}
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
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
	case string:
		if typed == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(typed)
		return parsed, err == nil
	}
	return 0, false
}
