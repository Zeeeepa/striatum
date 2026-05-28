package supervisor

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestIntegrationAttachClientExitDoesNotKillTmuxProbe(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sessionName := "striatum-test-attach-" + strings.ReplaceAll(t.Name(), "/", "-")
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	t.Cleanup(func() { _ = exec.Command("tmux", "kill-session", "-t", sessionName).Run() })
	if err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "sleep", "600").Run(); err != nil {
		t.Fatalf("new-session: %v", err)
	}
	identity, err := CaptureTmuxIdentity(ctx, DefaultTmuxRunner(), sessionName)
	if err != nil {
		t.Fatalf("capture identity: %v", err)
	}
	attach := exec.Command("tmux", "attach-session", "-t", sessionName)
	_ = attach.Run()
	live := ProbeTmuxLiveness(ctx, DefaultTmuxRunner(), identity)
	if live.Class != TmuxLivenessOK || !live.Healthy {
		t.Fatalf("liveness after attach exit = %#v", live)
	}
}

func TestIntegrationSessionKillIsSessionMissing(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sessionName := "striatum-test-kill-" + strings.ReplaceAll(t.Name(), "/", "-")
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()
	if err := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "sleep", "600").Run(); err != nil {
		t.Fatalf("new-session: %v", err)
	}
	identity, err := CaptureTmuxIdentity(ctx, DefaultTmuxRunner(), sessionName)
	if err != nil {
		t.Fatalf("capture identity: %v", err)
	}
	if err := exec.Command("tmux", "kill-session", "-t", sessionName).Run(); err != nil {
		t.Fatalf("kill-session: %v", err)
	}
	live := ProbeTmuxLiveness(ctx, DefaultTmuxRunner(), identity)
	if live.Class != TmuxLivenessSessionMissing {
		t.Fatalf("liveness after kill-session = %#v", live)
	}
}
