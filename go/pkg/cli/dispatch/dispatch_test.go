package dispatch

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeInvoker struct {
	calls []call
	err   error
}

type call struct {
	method string
	params map[string]any
}

func (f *fakeInvoker) Invoke(_ context.Context, method string, params map[string]any) (map[string]any, error) {
	f.calls = append(f.calls, call{method: method, params: params})
	if f.err != nil {
		return nil, f.err
	}
	if method == "repo.resolve" {
		return map[string]any{"repository_id": "repo_resolved"}, nil
	}
	if method == "supervise.trajectory" {
		return map[string]any{
			"content": "booted\nready\n",
			"trajectory_log": map[string]any{
				"status": "available",
			},
		}, nil
	}
	return map[string]any{"method": method}, nil
}

func TestDispatchRoutesReadThroughRPC(t *testing.T) {
	invoker := &fakeInvoker{}
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"--repo", "/repo", "status", "--run-id", "run_1"}, &stdout, &stderr, Options{
		Invoker:     invoker,
		ResolveRepo: true,
		Cwd:         "/cwd",
		Env:         []string{},
	})
	if exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if len(invoker.calls) != 2 {
		t.Fatalf("calls = %#v", invoker.calls)
	}
	if invoker.calls[0].method != "repo.resolve" || invoker.calls[0].params["path"] != "/repo" {
		t.Fatalf("resolve call = %#v", invoker.calls[0])
	}
	if invoker.calls[1].method != "status" || invoker.calls[1].params["repository_id"] != "repo_resolved" || invoker.calls[1].params["run_id"] != "run_1" {
		t.Fatalf("status call = %#v", invoker.calls[1])
	}
}

func TestDispatchRoutesMutationThroughRPC(t *testing.T) {
	invoker := &fakeInvoker{}
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"--repository-id", "repo_1", "run", "start", "run_1"}, &stdout, &stderr, Options{Invoker: invoker})
	if exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if invoker.calls[0].method != "run.start" || invoker.calls[0].params["run_id"] != "run_1" {
		t.Fatalf("call = %#v", invoker.calls[0])
	}
}

// #182: `daemon token-create` routes to the daemon.token.create RPC so an
// operator can mint an apply-capable token (run.integrate) without raw RPC.
// It is daemon_global, so no repository_id resolution happens, and repeated
// --capability flags collapse into a capabilities array.
func TestDispatchRoutesDaemonTokenCreate(t *testing.T) {
	invoker := &fakeInvoker{}
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"daemon", "token-create",
		"--capability", "apply", "--capability", "admin",
		"--display-name", "operator-apply"}, &stdout, &stderr, Options{Invoker: invoker, ResolveRepo: true})
	if exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if len(invoker.calls) != 1 {
		t.Fatalf("expected a single RPC call (no repo.resolve for daemon_global), got %#v", invoker.calls)
	}
	got := invoker.calls[0]
	if got.method != "daemon.token.create" {
		t.Fatalf("method = %q", got.method)
	}
	if _, ok := got.params["repository_id"]; ok {
		t.Fatalf("daemon_global route must not carry repository_id: %#v", got.params)
	}
	caps, ok := got.params["capabilities"].([]any)
	if !ok || len(caps) != 2 || caps[0] != "apply" || caps[1] != "admin" {
		t.Fatalf("capabilities = %#v (params=%#v)", got.params["capabilities"], got.params)
	}
	if got.params["display_name"] != "operator-apply" {
		t.Fatalf("display_name = %#v", got.params["display_name"])
	}
}

func TestDispatchUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"bogus"}, &stdout, &stderr, Options{Invoker: &fakeInvoker{}})
	if exit != 2 {
		t.Fatalf("exit = %d", exit)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestSuggestCommandRescuesTranspositionAndPlural(t *testing.T) {
	cases := map[string]string{
		"run list":      "list runs",     // transposition + plural
		"runs list":     "list runs",     // plural on first token
		"sessions list": "list sessions", // transposition + plural
		"list run":      "list runs",     // singular subcommand
	}
	for input, want := range cases {
		if got := suggestCommand(strings.Fields(input)); got != want {
			t.Fatalf("suggestCommand(%q) = %q, want %q", input, got, want)
		}
	}
	if got := suggestCommand([]string{"totally", "bogus"}); got != "" {
		t.Fatalf("expected no suggestion for nonsense, got %q", got)
	}
}

func TestDispatchUnknownCommandSuggests(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"run", "list"}, &stdout, &stderr, Options{Invoker: &fakeInvoker{}})
	if exit != 2 {
		t.Fatalf("exit = %d", exit)
	}
	if !strings.Contains(stderr.String(), "did you mean: striatum list runs") {
		t.Fatalf("stderr = %s", stderr.String())
	}
}

func TestDispatchHelpListsRequiredAndOptionalFlags(t *testing.T) {
	cases := map[string][]string{
		"supervise stop":       {"--session-id", "--reason", "required:"},
		"register-session":     {"--capability", "--fresh", "run-id", "role", "lane"},
		"checkpoint resolve":   {"continue|cancel", "--decision-id", "blocker-id"},
		"supervise start":      {"--session-id", "supervise.start"},
		"supervise send":       {"--packet-id", "supervise.send"},
		"supervise status":     {"--session-id", "supervise.status"},
		"supervise trajectory": {"--session-id", "--tail", "--tail-lines", "supervise.trajectory"},
		"repo add":             {"--init", "path", "repo.add"},
		"repo write":           {"session-id", "job-id", "lease-id", "--content", "repo.write"},
		"repo patch-preview":   {"session-id", "job-id", "lease-id", "--patch", "repo.patch_preview"},
		"process run":          {"session-id", "job-id", "lease-id", "--command-json", "process.run"},
		"decision record":      {"run-id", "outcome", "--escape-surface", "decision.record"},
	}
	for cmd, wants := range cases {
		invoker := &fakeInvoker{}
		var stdout, stderr bytes.Buffer
		args := append(strings.Fields(cmd), "--help")
		exit := Run(context.Background(), args, &stdout, &stderr, Options{Invoker: invoker})
		if exit != 0 {
			t.Fatalf("%s --help exit = %d stderr=%s", cmd, exit, stderr.String())
		}
		if len(invoker.calls) != 0 {
			t.Fatalf("%s --help contacted the daemon: %#v", cmd, invoker.calls)
		}
		out := stdout.String()
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Fatalf("%s --help output missing %q:\n%s", cmd, want, out)
			}
		}
	}
}

func TestDispatchSuperviseTrajectoryPrintsContent(t *testing.T) {
	invoker := &fakeInvoker{}
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"--repository-id", "repo_1", "supervise", "trajectory", "sess_1", "--tail-lines", "2"}, &stdout, &stderr, Options{Invoker: invoker})
	if exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if stdout.String() != "booted\nready\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if len(invoker.calls) != 1 || invoker.calls[0].method != "supervise.trajectory" {
		t.Fatalf("calls = %#v", invoker.calls)
	}
	params := invoker.calls[0].params
	if params["session_id"] != "sess_1" || params["tail_lines"] != 2 {
		t.Fatalf("params = %#v", params)
	}
}

func TestDispatchSessionRegisterAliasResolvesToSessionRegister(t *testing.T) {
	invoker := &fakeInvoker{}
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"--repository-id", "repo_1", "session", "register", "run_1", "author", "lane_a"}, &stdout, &stderr, Options{Invoker: invoker})
	if exit != 0 {
		t.Fatalf("exit = %d stderr=%s", exit, stderr.String())
	}
	if len(invoker.calls) != 1 || invoker.calls[0].method != "session.register" {
		t.Fatalf("calls = %#v", invoker.calls)
	}
	params := invoker.calls[0].params
	if params["run_id"] != "run_1" || params["role"] != "author" || params["lane"] != "lane_a" {
		t.Fatalf("params = %#v", params)
	}
}

func TestDispatchUsesExitCodeMapper(t *testing.T) {
	wantErr := errors.New("daemon down")
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{"repo", "list"}, &stdout, &stderr, Options{
		Invoker:  &fakeInvoker{err: wantErr},
		ExitCode: func(err error) int { return 11 },
	})
	if exit != 11 {
		t.Fatalf("exit = %d", exit)
	}
}
