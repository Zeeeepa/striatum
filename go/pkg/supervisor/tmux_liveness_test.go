package supervisor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

type fakeTmuxRunner struct {
	responses []fakeTmuxResponse
	calls     [][]string
}

type fakeTmuxResponse struct {
	prefix []string
	out    string
	err    error
}

func (r *fakeTmuxRunner) Run(_ context.Context, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	for _, response := range r.responses {
		if argsPrefix(args, response.prefix) {
			return response.out, response.err
		}
	}
	return "", exec.ErrNotFound
}

func argsPrefix(args, prefix []string) bool {
	if len(args) < len(prefix) {
		return false
	}
	for i := range prefix {
		if args[i] != prefix[i] {
			return false
		}
	}
	return true
}

func TestProbeTmuxLivenessOK(t *testing.T) {
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}, out: ""},
		{prefix: []string{"display-message"}, out: "%4|48211|0|1748452211\n"},
	}}
	live := ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
		SessionName:    "striatum-run",
		PaneID:         "%4",
		PanePID:        48211,
		PaneStartToken: "1748452211",
	})
	if live.Class != TmuxLivenessOK || !live.Healthy || live.ObservedPanePID != 48211 {
		t.Fatalf("liveness = %#v", live)
	}
}

func TestProbeTmuxLivenessFailureClasses(t *testing.T) {
	tests := []struct {
		name      string
		responses []fakeTmuxResponse
		want      TmuxLivenessClass
	}{
		{
			name:      "session_missing",
			responses: []fakeTmuxResponse{{prefix: []string{"has-session"}, err: errors.New("can't find session")}},
			want:      TmuxLivenessSessionMissing,
		},
		{
			name: "pane_missing",
			responses: []fakeTmuxResponse{
				{prefix: []string{"has-session"}},
				{prefix: []string{"display-message"}, out: "%9|48211|0|1748452211\n"},
			},
			want: TmuxLivenessPaneMissing,
		},
		{
			name: "pane_dead",
			responses: []fakeTmuxResponse{
				{prefix: []string{"has-session"}},
				{prefix: []string{"display-message"}, out: "%4|48211|1|1748452211\n"},
			},
			want: TmuxLivenessPaneDead,
		},
		{
			name: "pid_mismatch",
			responses: []fakeTmuxResponse{
				{prefix: []string{"has-session"}},
				{prefix: []string{"display-message"}, out: "%4|48212|0|1748452211\n"},
			},
			want: TmuxLivenessPanePIDMismatch,
		},
		{
			name: "start_token_mismatch",
			responses: []fakeTmuxResponse{
				{prefix: []string{"has-session"}},
				{prefix: []string{"display-message"}, out: "%4|48211|0|999\n"},
			},
			want: TmuxLivenessPanePIDMismatch,
		},
		{
			name:      "tmux_not_found",
			responses: []fakeTmuxResponse{{prefix: []string{"has-session"}, err: exec.ErrNotFound}},
			want:      TmuxLivenessUnavailable,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeTmuxRunner{responses: tc.responses}
			live := ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
				SessionName:    "striatum-run",
				PaneID:         "%4",
				PanePID:        48211,
				PaneStartToken: "1748452211",
			})
			if live.Class != tc.want || live.Healthy {
				t.Fatalf("liveness = %#v, want class %s unhealthy", live, tc.want)
			}
		})
	}
}

func TestProbeTmuxUnavailableCarriesTypedFailure(t *testing.T) {
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}, err: exec.ErrNotFound},
	}}
	live := ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
		SessionName: "striatum-run",
		PaneID:      "%4",
		PanePID:     48211,
	})
	if live.Class != TmuxLivenessUnavailable || live.State != "degraded" || live.Healthy {
		t.Fatalf("liveness = %#v, want degraded tmux_unavailable", live)
	}
	if live.Failure == nil {
		t.Fatalf("missing typed failure record: %#v", live)
	}
	if live.Failure.FailureClass != TmuxLivenessUnavailable || live.Failure.Errno != "ENOENT" {
		t.Fatalf("failure = %#v, want tmux_unavailable ENOENT", live.Failure)
	}
	payload := TmuxLivenessPayload(live)
	if payload["state"] != "degraded" {
		t.Fatalf("payload = %#v, want degraded state", payload)
	}
	failure := payload["probe_failure"].(map[string]any)
	if failure["failure_class"] != string(TmuxLivenessUnavailable) || failure["errno"] != "ENOENT" {
		t.Fatalf("payload failure = %#v", failure)
	}
}

func TestProbeTmuxFailureRecordsPaneProcessLiveness(t *testing.T) {
	pid := os.Getpid()
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}, err: errors.New("can't find session")},
		{prefix: []string{"list-panes"}, out: "%4|" + strconv.Itoa(pid) + "\n"},
	}}
	live := ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
		SessionName: "striatum-run",
		PaneID:      "%4",
		PanePID:     pid,
	})
	if live.Class != TmuxLivenessSessionMissing || live.State != "lost" {
		t.Fatalf("liveness = %#v", live)
	}
	if live.Failure == nil || live.Failure.ObservedPanePID != pid || live.Failure.PaneProcessLive == nil || !*live.Failure.PaneProcessLive {
		t.Fatalf("failure did not record live pane process: %#v", live.Failure)
	}
	if len(runner.calls) != 2 || !argsPrefix(runner.calls[1], []string{"list-panes"}) {
		t.Fatalf("tmux calls = %#v, want list-panes fallback", runner.calls)
	}
}

func TestProbeTmuxLivenessEmptyStartTokenIsUnverifiedNotLost(t *testing.T) {
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|48211|0|1748452211\n"},
	}}
	live := ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
		SessionName: "striatum-run",
		PaneID:      "%4",
		PanePID:     48211,
	})
	if live.Class != TmuxLivenessOK || !live.Healthy || live.Detail != "start_token_unverified" {
		t.Fatalf("liveness = %#v", live)
	}
}

func TestProbeTmuxLivenessLiteralStartTokenIsUnverifiedNotLost(t *testing.T) {
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|48211|0|#{pane_start_time}\n"},
	}}
	live := ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
		SessionName:    "striatum-run",
		PaneID:         "%4",
		PanePID:        48211,
		PaneStartToken: "#{pane_start_time}",
	})
	if live.Class != TmuxLivenessOK || !live.Healthy || live.Detail != "start_token_unverified" {
		t.Fatalf("liveness = %#v", live)
	}
}

func TestProbeTmuxLivenessFallsBackToProcessStartTokenWhenTmuxTokenMissing(t *testing.T) {
	pid := os.Getpid()
	start, ok := ProcessStartToken(pid)
	if !ok || start == "" {
		t.Skip("current process start token unavailable")
	}
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|" + strconv.Itoa(pid) + "|0|\n"},
	}}
	live := ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
		SessionName:    "striatum-run",
		PaneID:         "%4",
		PanePID:        pid,
		PaneStartToken: start,
	})
	if live.Class != TmuxLivenessOK || !live.Healthy || live.Detail != "" || live.ObservedStartTok != start {
		t.Fatalf("liveness = %#v, want verified via process start token fallback", live)
	}
}

func TestCaptureTmuxIdentityIgnoresLiteralStartToken(t *testing.T) {
	pid := os.Getpid()
	fallback, _ := ProcessStartToken(pid)
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"display-message"}, out: "%1|%4|" + strconv.Itoa(pid) + "|#{pane_start_time}\n"},
	}}
	id, err := CaptureTmuxIdentity(context.Background(), runner, "striatum-run")
	if err != nil {
		t.Fatalf("CaptureTmuxIdentity: %v", err)
	}
	if id.PaneStartToken != verifiedStartToken(fallback) {
		t.Fatalf("PaneStartToken = %q, want sanitized fallback %q", id.PaneStartToken, verifiedStartToken(fallback))
	}
}

func TestProbeTmuxLivenessNeverReadsPaneText(t *testing.T) {
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|48211|0|1748452211\n"},
	}}
	_ = ProbeTmuxLiveness(context.Background(), runner, TmuxIdentity{
		SessionName: "striatum-run",
		PaneID:      "%4",
		PanePID:     48211,
	})
	forbidden := []string{"capture-pane", "pipe-pane", "save-buffer", "show-buffer", "copy-mode", "select-pane"}
	for _, call := range runner.calls {
		joined := strings.Join(call, " ")
		for _, term := range forbidden {
			if strings.Contains(joined, term) {
				t.Fatalf("probe issued pane-text command %q in call %#v", term, call)
			}
		}
	}
}

func TestProbeLaneLivenessUsesTmuxBackedMetadata(t *testing.T) {
	runner := &fakeTmuxRunner{responses: []fakeTmuxResponse{
		{prefix: []string{"has-session"}},
		{prefix: []string{"display-message"}, out: "%4|48211|0|1748452211\n"},
	}}
	live := ProbeLaneLiveness(context.Background(), runner, map[string]any{
		"tmux": map[string]any{
			"state":            "backed",
			"session_name":     "striatum-run",
			"pane_id":          "%4",
			"pane_pid":         48211,
			"pane_start_token": "1748452211",
		},
	}, 99999, "")
	if live.Backed != "tmux" || live.Class != string(TmuxLivenessOK) || !live.Alive {
		t.Fatalf("lane liveness = %#v", live)
	}
}
