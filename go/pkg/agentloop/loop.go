package agentloop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
)

func Run(socketPath, repoRoot, runID, sessionID string, command []string) error {
	if sessionID == "" || runID == "" || repoRoot == "" {
		return fmt.Errorf("STRIATUM_SESSION_ID, STRIATUM_RUN_ID, and STRIATUM_REPO must be in environment")
	}
	if len(command) == 0 {
		return fmt.Errorf("agent command is required")
	}

	endpoint, err := ResolveMCPEndpoint(repoRoot, os.Environ())
	if err != nil {
		return err
	}
	repositoryID := os.Getenv(EnvRepositoryID)
	token, err := ResolveTokenMaterial(repoRoot, os.Environ())
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := runConfig{
		SocketPath:   socketPath,
		RepoRoot:     repoRoot,
		RepositoryID: repositoryID,
		RunID:        runID,
		SessionID:    sessionID,
		Endpoint:     endpoint,
		Token:        token,
		Command:      command,
		Env:          os.Environ(),
	}
	return runWithIO(ctx, cfg, os.Stdin, os.Stdout, os.Stderr)
}

type runConfig struct {
	SocketPath   string
	RepoRoot     string
	RepositoryID string
	RunID        string
	SessionID    string
	Endpoint     string
	Token        TokenMaterial
	Command      []string
	Env          []string
}

func runWithIO(ctx context.Context, cfg runConfig, stdin io.Reader, stdout, stderr io.Writer) error {
	log.Printf("Starting Striatum agent PTY for session %s on run %s", cfg.SessionID, cfg.RunID)
	log.Printf("Agent command: %v", cfg.Command)
	log.Printf("MCP endpoint: %s", cfg.Endpoint)

	childEnv := AgentEnvironment(cfg.Env, BootstrapContext{
		SocketPath:   cfg.SocketPath,
		RepoRoot:     cfg.RepoRoot,
		RepositoryID: cfg.RepositoryID,
		RunID:        cfg.RunID,
		SessionID:    cfg.SessionID,
		Endpoint:     cfg.Endpoint,
		Token:        cfg.Token,
	})
	prompt := BuildBootstrapPrompt(BootstrapContext{
		SocketPath:   cfg.SocketPath,
		RepoRoot:     cfg.RepoRoot,
		RepositoryID: cfg.RepositoryID,
		RunID:        cfg.RunID,
		SessionID:    cfg.SessionID,
		Endpoint:     cfg.Endpoint,
		Token:        cfg.Token,
	})

	cmd := exec.CommandContext(ctx, cfg.Command[0], cfg.Command[1:]...)
	cmd.Dir = cfg.RepoRoot
	cmd.Env = childEnv

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("agent-loop pty start: %w", err)
	}
	defer ptmx.Close()

	if inputFile, ok := stdin.(*os.File); ok {
		_ = pty.InheritSize(inputFile, ptmx)
	}

	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		_, _ = io.Copy(stdout, ptmx)
	}()

	if _, err := io.WriteString(ptmx, prompt); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("agent-loop bootstrap prompt: %w", err)
	}

	if stdin != nil {
		go func() {
			_, err := io.Copy(ptmx, stdin)
			if err != nil && !errors.Is(err, os.ErrClosed) {
				_, _ = fmt.Fprintf(stderr, "agent-loop stdin copy failed: %v\n", err)
			}
		}()
	}

	err = cmd.Wait()
	_ = ptmx.Close()
	<-outputDone
	if err != nil {
		return fmt.Errorf("agent command exited: %w", err)
	}
	return nil
}
