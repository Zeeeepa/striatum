package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/jackc/pgx/v5"
)

const recoverySchedulerPanicEnv = "STRIATUM_TEST_RECOVERY_SCHEDULER_PANIC"

type panicQueryRunner struct{}

func (panicQueryRunner) Exec(context.Context, string, ...any) error {
	return errors.New("unexpected Exec")
}

func (panicQueryRunner) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("recovery scheduler panic sentinel")
}

func (panicQueryRunner) QueryRow(context.Context, string, ...any) db.Row {
	panic("unexpected QueryRow")
}

func (panicQueryRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errors.New("unexpected QueryScalar")
}

func (panicQueryRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errors.New("unexpected BeginTx")
}

func TestRecoverySchedulerLogsAndRepanicsOnPanic(t *testing.T) {
	if os.Getenv(recoverySchedulerPanicEnv) == "1" {
		log.SetFlags(0)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := startRecoveryScheduler(ctx, cancel, panicQueryRunner{}, 0.01, optionalIntFlag{Provided: true, Value: 1})
		select {
		case err := <-done:
			t.Fatalf("scheduler returned instead of panicking: %v", err)
		case <-time.After(2 * time.Second):
			t.Fatal("scheduler did not panic")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestRecoverySchedulerLogsAndRepanicsOnPanic$")
	cmd.Env = append(os.Environ(), recoverySchedulerPanicEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("child test succeeded; want panic failure. output:\n%s", output)
	}
	if ctx.Err() != nil {
		t.Fatalf("child test timed out: %v\n%s", ctx.Err(), output)
	}
	logText := string(output)
	for _, want := range []string{
		"recovery scheduler goroutine panic",
		"recovery scheduler panic sentinel",
		"runtime/debug.Stack",
		"panic: recovery scheduler panic sentinel",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("scheduler panic output missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "auto_spawn scheduler") {
		t.Fatalf("recovery scheduler panic output mentioned auto_spawn:\n%s", logText)
	}
}
