package supervisor

// TmuxMeta is the single, structured codec for supervisor pointer metadata blocks.
type TmuxMeta struct {
	AgentLoopMode    string                 `json:"agent_loop_mode,omitempty"`
	Tmux             TmuxMetaBlock          `json:"tmux,omitempty"`
	DeliveryLiveness *DeliveryLivenessBlock `json:"delivery_liveness,omitempty"`
}

type TmuxMetaBlock struct {
	State                  string                 `json:"state,omitempty"`
	SessionName            string                 `json:"session_name,omitempty"`
	WindowID               string                 `json:"window_id,omitempty"`
	PaneID                 string                 `json:"pane_id,omitempty"`
	PaneStartToken         string                 `json:"pane_start_token,omitempty"`
	AttachCommand          string                 `json:"attach_command,omitempty"`
	UnavailableReason      string                 `json:"unavailable_reason,omitempty"`
	CapturedAt             string                 `json:"captured_at,omitempty"`
	ProbeSkippedAt         string                 `json:"probe_skipped_at,omitempty"`
	LastOkAt               string                 `json:"last_ok_at,omitempty"`
	LastUnavailableDetail  string                 `json:"last_unavailable_detail,omitempty"`
	LivenessState          string                 `json:"liveness_state,omitempty"`
	PanePID                int                    `json:"pane_pid,omitempty"`
	AttachClientPID        int                    `json:"attach_client_pid,omitempty"`
	ProbeUnavailableCount  int                    `json:"probe_unavailable_count,omitempty"`
	DeliveryLiveness       *DeliveryLivenessBlock `json:"delivery_liveness,omitempty"`
	AttachClientLastExit   map[string]any         `json:"attach_client_last_exit,omitempty"`
	Liveness               map[string]any         `json:"liveness,omitempty"`
}

type DeliveryLivenessBlock struct {
	Class       string `json:"class,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ObservedAt  string `json:"observed_at,omitempty"`
	ReportedAt  string `json:"reported_at,omitempty"`
	Remediation string `json:"remediation,omitempty"`
	Healthy     bool   `json:"healthy,omitempty"`
}

// IsValidTmux validates whether the Tmux metadata shape conforms to a backed session.
func (m TmuxMeta) IsValidTmux() bool {
	if m.Tmux.State != "backed" {
		return true // Not backed, so not invalid tmux
	}
	return m.Tmux.PanePID > 0 && m.Tmux.SessionName != "" && m.Tmux.PaneID != ""
}
