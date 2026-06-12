package rundrive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/cli/rpcclient"
	"github.com/halbritt/striatum/go/pkg/laneproviderauth"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

type Invoker interface {
	Invoke(context.Context, string, map[string]any) (map[string]any, error)
}

type Options struct {
	RepositoryID     string
	RunID            string
	RepoRoot         string
	Interval         time.Duration
	Once             bool
	JSON             bool
	Stdout           io.Writer
	Stderr           io.Writer
	Now              func() time.Time
	Sleep            func(context.Context, time.Duration) error
	PID              int
	ProviderAuthGate string
}

type Driver struct {
	invoker  Invoker
	options  Options
	launched map[slotKey]string
}

type Action struct {
	TS            string `json:"ts,omitempty"`
	Action        string `json:"action"`
	RunID         string `json:"run_id,omitempty"`
	JobID         string `json:"job_id,omitempty"`
	WorkflowJobID string `json:"workflow_job_id,omitempty"`
	Attempt       int    `json:"attempt,omitempty"`
	Role          string `json:"role,omitempty"`
	Lane          string `json:"lane,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
	Result        string `json:"result,omitempty"`
	Message       string `json:"message,omitempty"`
}

type TerminalError struct {
	RunID string
	State string
}

func (e TerminalError) Error() string {
	return fmt.Sprintf("run %s reached terminal state %s", e.RunID, e.State)
}

type ProviderAuthRefusalError struct {
	Code      string
	SessionID string
}

func (e ProviderAuthRefusalError) Error() string {
	if e.Code == "" {
		return "lane provider auth preflight refused launch"
	}
	return "lane provider auth preflight refused launch: " + e.Code
}

type slotKey struct {
	WorkflowJobID string
	Attempt       int
}

type roleLane struct {
	Role string
	Lane string
}

var AllowedMethods = map[string]bool{
	"run.detail":       true,
	"list.sessions":    true,
	"session.register": true,
	"session.close":    true,
	"supervise.start":  true,
	"supervise.stop":   true,
}

func New(invoker Invoker, options Options) *Driver {
	if options.Interval <= 0 {
		options.Interval = 15 * time.Second
	}
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = io.Discard
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.Sleep == nil {
		options.Sleep = func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		}
	}
	if options.PID == 0 {
		options.PID = os.Getpid()
	}
	if strings.TrimSpace(options.ProviderAuthGate) == "" {
		options.ProviderAuthGate = "auto"
	}
	return &Driver{
		invoker:  invoker,
		options:  options,
		launched: map[slotKey]string{},
	}
}

func Run(ctx context.Context, invoker Invoker, options Options) error {
	if strings.TrimSpace(options.RepositoryID) == "" {
		return fmt.Errorf("run drive requires repository_id")
	}
	if strings.TrimSpace(options.RunID) == "" {
		return fmt.Errorf("run drive requires run_id")
	}
	driver := New(invoker, options)
	cleanup := driver.claimAdvisoryMarker()
	defer cleanup()
	for {
		actions, state, terminal, err := driver.ReconcileOnce(ctx)
		for _, action := range actions {
			driver.emit(action)
		}
		if len(actions) == 0 && driver.options.Once && driver.options.JSON {
			driver.emit(Action{
				TS:     driver.timestamp(),
				Action: "tick",
				RunID:  driver.options.RunID,
				Result: state,
			})
		}
		if err != nil {
			return err
		}
		if terminal {
			if state == "completed" {
				return nil
			}
			return TerminalError{RunID: options.RunID, State: state}
		}
		if driver.options.Once {
			return nil
		}
		if err := driver.options.Sleep(ctx, driver.options.Interval); err != nil {
			return err
		}
	}
}

func (d *Driver) ReconcileOnce(ctx context.Context) ([]Action, string, bool, error) {
	detail, err := d.invoke(ctx, "run.detail", map[string]any{"run_id": d.options.RunID})
	if err != nil {
		return nil, "", false, err
	}
	run := asMap(detail["run"])
	state := stringValue(run["state"])
	if isTerminalRunState(state) {
		return []Action{d.action("terminal", Action{Result: state})}, state, true, nil
	}
	jobs := mapList(detail["jobs"])
	workflow := asMap(detail["workflow"])
	sessions, err := d.listSessions(ctx)
	if err != nil {
		return nil, state, false, err
	}
	actions := []Action{}
	closeActions := d.closeFinishedLaunched(ctx, jobs)
	actions = append(actions, closeActions...)
	if anyStopped(closeActions) {
		sessions, err = d.listSessions(ctx)
		if err != nil {
			return actions, state, false, err
		}
	}
	launchActions, err := d.launchQueued(ctx, jobs, workflow, sessions)
	actions = append(actions, launchActions...)
	if err != nil {
		return actions, state, false, err
	}
	return actions, state, false, nil
}

func (d *Driver) closeFinishedLaunched(ctx context.Context, jobs []map[string]any) []Action {
	actions := []Action{}
	seen := map[slotKey]bool{}
	for _, job := range jobs {
		key := jobSlot(job)
		seen[key] = true
		sessionID := d.launched[key]
		if sessionID == "" {
			continue
		}
		if !isLaunchedJobDone(stringValue(job["state"])) {
			continue
		}
		action := d.jobAction("supervise.stop", job, Action{
			SessionID: sessionID,
			Result:    "attempted",
			Message:   "lane finished its job; closing for fresh-policy",
		})
		action = d.stopLaunchedSession(ctx, key, sessionID, action, "lane finished its job; closing for fresh-policy")
		actions = append(actions, action)
	}
	for key, sessionID := range d.launched {
		if seen[key] {
			continue
		}
		action := d.action("supervise.stop", Action{
			WorkflowJobID: key.WorkflowJobID,
			Attempt:       key.Attempt,
			SessionID:     sessionID,
			Result:        "attempted",
			Message:       "job attempt was superseded; closing launched lane",
		})
		action = d.stopLaunchedSession(ctx, key, sessionID, action, "job attempt was superseded; closing launched lane")
		actions = append(actions, action)
	}
	return actions
}

func (d *Driver) stopLaunchedSession(ctx context.Context, key slotKey, sessionID string, action Action, reason string) Action {
	if _, err := d.invoke(ctx, "supervise.stop", map[string]any{
		"session_id": sessionID,
		"reason":     reason,
	}); err != nil {
		action.Result = "error"
		action.Message = err.Error()
		return action
	}
	action.Result = "stopped"
	delete(d.launched, key)
	return action
}

func (d *Driver) launchQueued(ctx context.Context, jobs []map[string]any, workflow map[string]any, sessions []map[string]any) ([]Action, error) {
	actions := []Action{}
	activeBySlot := activeSessionsBySlot(sessions)
	usedSessions := map[string]bool{}
	for _, sessionID := range d.launched {
		usedSessions[sessionID] = true
	}
	for _, job := range sortedJobs(jobs) {
		if stringValue(job["state"]) != "queued" {
			continue
		}
		key := jobSlot(job)
		if d.launched[key] != "" {
			continue
		}
		role := stringValue(job["role_id"])
		lane, ok := resolveLane(job, workflow)
		if !ok {
			actions = append(actions, d.jobAction("skip", job, Action{
				Role:    role,
				Result:  "ambiguous_lane",
				Message: "cannot resolve lane for queued job; register manually",
			}))
			continue
		}
		slot := roleLane{Role: role, Lane: lane}
		if sessionID := nextUnusedSession(activeBySlot[slot], usedSessions); sessionID != "" {
			d.launched[key] = sessionID
			usedSessions[sessionID] = true
			actions = append(actions, d.jobAction("adopt", job, Action{
				Role:      role,
				Lane:      lane,
				SessionID: sessionID,
				Result:    "adopted_existing_session",
			}))
			continue
		}
		sessionID, registerActions, err := d.registerLane(ctx, role, lane)
		actions = append(actions, registerActions...)
		if err != nil {
			actions = append(actions, d.jobAction("register", job, Action{
				Role:    role,
				Lane:    lane,
				Result:  "error",
				Message: err.Error(),
			}))
			continue
		}
		if sessionID == "" {
			actions = append(actions, d.jobAction("register", job, Action{
				Role:    role,
				Lane:    lane,
				Result:  "waiting",
				Message: "fresh reviewer registration is blocked by an author still holding or eligible for work",
			}))
			continue
		}
		startAction := d.jobAction("supervise.start", job, Action{
			Role:      role,
			Lane:      lane,
			SessionID: sessionID,
			Result:    "attempted",
		})
		if _, err := d.invoke(ctx, "supervise.start", map[string]any{
			"session_id":         sessionID,
			"provider_auth_gate": d.options.ProviderAuthGate,
		}); err != nil {
			if code, ok := providerAuthFailureCode(err); ok {
				startAction.Result = "blocked"
				startAction.Message = "lane provider auth preflight refused launch: " + code
				actions = append(actions, startAction)
				_, _ = d.invoke(ctx, "session.close", map[string]any{
					"session_id": sessionID,
					"reason":     "run drive provider auth preflight refused launch: " + code,
				})
				return actions, ProviderAuthRefusalError{Code: code, SessionID: sessionID}
			}
			startAction.Result = "error"
			startAction.Message = err.Error()
			actions = append(actions, startAction)
			_, _ = d.invoke(ctx, "session.close", map[string]any{
				"session_id": sessionID,
				"reason":     "run drive supervise start failed",
			})
			continue
		}
		startAction.Result = "started"
		actions = append(actions, startAction)
		d.launched[key] = sessionID
		usedSessions[sessionID] = true
	}
	return actions, nil
}

func (d *Driver) registerLane(ctx context.Context, role, lane string) (string, []Action, error) {
	actions := []Action{}
	for attempt := 0; attempt < 2; attempt++ {
		result, err := d.invoke(ctx, "session.register", map[string]any{
			"run_id": d.options.RunID,
			"role":   role,
			"lane":   lane,
			"fresh":  true,
		})
		if err == nil {
			sessionID := stringValue(result["session_id"])
			return sessionID, actions, nil
		}
		if attempt == 0 && isFreshPolicyRefusal(err) {
			closed, closeErr := d.closeCompletedSessions(ctx)
			actions = append(actions, closed...)
			if closeErr != nil {
				return "", actions, closeErr
			}
			continue
		}
		if isFreshPolicyRefusal(err) {
			return "", actions, nil
		}
		return "", actions, err
	}
	return "", actions, nil
}

func (d *Driver) closeCompletedSessions(ctx context.Context) ([]Action, error) {
	detail, err := d.invoke(ctx, "run.detail", map[string]any{"run_id": d.options.RunID})
	if err != nil {
		return nil, err
	}
	jobs := mapList(detail["jobs"])
	workflow := asMap(detail["workflow"])
	doneSlots := map[roleLane]bool{}
	for _, job := range jobs {
		if stringValue(job["state"]) == "completed" {
			lane, ok := resolveLane(job, workflow)
			if !ok {
				continue
			}
			doneSlots[roleLane{Role: stringValue(job["role_id"]), Lane: lane}] = true
		}
	}
	sessions, err := d.listSessions(ctx)
	if err != nil {
		return nil, err
	}
	actions := []Action{}
	for _, session := range sessions {
		slot := roleLane{Role: stringValue(session["role_id"]), Lane: stringValue(session["lane_id"])}
		if stringValue(session["state"]) != "active" || !doneSlots[slot] {
			continue
		}
		sessionID := stringValue(session["session_id"])
		action := d.action("supervise.stop", Action{
			Role:      slot.Role,
			Lane:      slot.Lane,
			SessionID: sessionID,
			Result:    "attempted",
			Message:   "completed-job lane closed for fresh-policy",
		})
		if _, err := d.invoke(ctx, "supervise.stop", map[string]any{
			"session_id": sessionID,
			"reason":     "completed-job lane closed for fresh-policy",
		}); err != nil {
			action.Result = "error"
			action.Message = err.Error()
			actions = append(actions, action)
			continue
		}
		action.Result = "stopped"
		actions = append(actions, action)
	}
	return actions, nil
}

func (d *Driver) listSessions(ctx context.Context) ([]map[string]any, error) {
	result, err := d.invoke(ctx, "list.sessions", map[string]any{"run_id": d.options.RunID})
	if err != nil {
		return nil, err
	}
	return mapList(result["items"]), nil
}

func (d *Driver) invoke(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	if !AllowedMethods[method] {
		return nil, fmt.Errorf("run drive attempted unsupported method %s", method)
	}
	next := map[string]any{"repository_id": d.options.RepositoryID}
	for key, value := range params {
		next[key] = value
	}
	return d.invoker.Invoke(ctx, method, next)
}

func (d *Driver) emit(action Action) {
	if action.TS == "" {
		action.TS = d.timestamp()
	}
	if d.options.JSON {
		encoded, _ := json.Marshal(action)
		_, _ = fmt.Fprintln(d.options.Stdout, string(encoded))
		return
	}
	switch action.Action {
	case "terminal":
		_, _ = fmt.Fprintf(d.options.Stdout, "run %s -> %s\n", d.options.RunID, action.Result)
	case "skip":
		_, _ = fmt.Fprintf(d.options.Stderr, "skip %s: %s\n", action.WorkflowJobID, action.Message)
	default:
		_, _ = fmt.Fprintf(d.options.Stdout, "%s %s/%s %s attempt %d -> %s\n",
			action.Action, action.Role, action.Lane, action.WorkflowJobID, action.Attempt, action.Result)
	}
}

func (d *Driver) action(kind string, action Action) Action {
	action.TS = d.timestamp()
	action.Action = kind
	action.RunID = d.options.RunID
	return action
}

func (d *Driver) jobAction(kind string, job map[string]any, action Action) Action {
	action = d.action(kind, action)
	action.JobID = stringValue(job["job_id"])
	action.WorkflowJobID = stringValue(job["workflow_job_id"])
	action.Attempt = intValue(job["attempt"])
	if action.Role == "" {
		action.Role = stringValue(job["role_id"])
	}
	return action
}

func (d *Driver) timestamp() string {
	return d.options.Now().UTC().Format(time.RFC3339)
}

func (d *Driver) claimAdvisoryMarker() func() {
	if d.options.RepoRoot == "" || d.options.RunID == "" {
		return func() {}
	}
	dir := filepath.Join(d.options.RepoRoot, ".striatum", "scratch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		_, _ = fmt.Fprintf(d.options.Stderr, "warning: could not create run drive marker dir: %v\n", err)
		return func() {}
	}
	path := filepath.Join(dir, "run-drive-"+safeFileComponent(d.options.RunID)+".pid")
	if prior, err := os.ReadFile(path); err == nil {
		fields := strings.Fields(string(prior))
		if len(fields) > 0 {
			if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 && pid != d.options.PID && processAlive(pid) {
				_, _ = fmt.Fprintf(d.options.Stderr, "warning: another run drive appears active for this run (pid %d); daemon session guards prevent duplicate claims\n", pid)
			}
		}
	}
	content := fmt.Sprintf("%d\n", d.options.PID)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		_, _ = fmt.Fprintf(d.options.Stderr, "warning: could not write run drive marker: %v\n", err)
		return func() {}
	}
	return func() {
		current, err := os.ReadFile(path)
		if err == nil && string(current) == content {
			_ = os.Remove(path)
		}
	}
}

func activeSessionsBySlot(sessions []map[string]any) map[roleLane][]string {
	result := map[roleLane][]string{}
	for _, session := range sessions {
		if stringValue(session["state"]) != "active" {
			continue
		}
		slot := roleLane{Role: stringValue(session["role_id"]), Lane: stringValue(session["lane_id"])}
		if slot.Role == "" || slot.Lane == "" {
			continue
		}
		result[slot] = append(result[slot], stringValue(session["session_id"]))
	}
	return result
}

func nextUnusedSession(sessions []string, used map[string]bool) string {
	for _, sessionID := range sessions {
		if sessionID != "" && !used[sessionID] {
			return sessionID
		}
	}
	return ""
}

func anyStopped(actions []Action) bool {
	for _, action := range actions {
		if action.Result == "stopped" {
			return true
		}
	}
	return false
}

func sortedJobs(jobs []map[string]any) []map[string]any {
	result := append([]map[string]any(nil), jobs...)
	sort.SliceStable(result, func(i, j int) bool {
		a := stringValue(result[i]["workflow_job_id"])
		b := stringValue(result[j]["workflow_job_id"])
		if a == b {
			return intValue(result[i]["attempt"]) < intValue(result[j]["attempt"])
		}
		return a < b
	})
	return result
}

func resolveLane(job map[string]any, workflow map[string]any) (string, bool) {
	if lane := stringValue(job["lane_id"]); lane != "" {
		return lane, true
	}
	if lane := stringValue(asMap(job["lane_selector_json"])["lane_id"]); lane != "" {
		return lane, true
	}
	workflowJobID := stringValue(job["workflow_job_id"])
	for _, item := range asList(workflow["jobs"]) {
		wfJob := asMap(item)
		if stringValue(wfJob["id"]) == workflowJobID {
			if lane := stringValue(wfJob["lane_id"]); lane != "" {
				return lane, true
			}
		}
	}
	return "", false
}

func jobSlot(job map[string]any) slotKey {
	return slotKey{WorkflowJobID: stringValue(job["workflow_job_id"]), Attempt: intValue(job["attempt"])}
}

func isTerminalRunState(state string) bool {
	switch state {
	case "completed", "failed", "canceled", "needs_operator", "waiting_human":
		return true
	default:
		return false
	}
}

func isLaunchedJobDone(state string) bool {
	switch state {
	case "completed", "failed", "canceled", "skipped", "waiting_human":
		return true
	default:
		return false
	}
}

func isFreshPolicyRefusal(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "reviewer_context_policy: fresh") ||
		strings.Contains(text, "--force-non-fresh")
}

func providerAuthFailureCode(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) && laneproviderauth.IsFailureClass(rpcErr.Code) {
		return rpcErr.Code, true
	}
	var clientErr *rpcclient.Error
	if errors.As(err, &clientErr) && laneproviderauth.IsFailureClass(clientErr.Code) {
		return clientErr.Code, true
	}
	for _, code := range []string{
		laneproviderauth.FailureAuthFailed,
		laneproviderauth.FailureBinaryMissing,
		laneproviderauth.FailureUnavailable,
		laneproviderauth.FailureTimeout,
		laneproviderauth.FailureLaunchFailed,
		laneproviderauth.FailureUnsupported,
		laneproviderauth.FailureUnexpectedResult,
	} {
		if strings.Contains(err.Error(), code) {
			return code, true
		}
	}
	return "", false
}

func safeFileComponent(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	if b.Len() == 0 {
		return "run"
	}
	return b.String()
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func asMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func asList(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return nil
	}
}

func mapList(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, asMap(item))
		}
		return result
	default:
		return nil
	}
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, _ := strconv.Atoi(typed)
		return parsed
	default:
		return 0
	}
}
