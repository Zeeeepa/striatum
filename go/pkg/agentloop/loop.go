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
	"strconv"
	"strings"
	"syscall"
	"time"

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

// agentLoopSubmitSequence returns the key-sequence written after the bootstrap
// prompt to submit it to an interactive agent. Defaults to a single carriage
// return (Enter), which submits the input line in the TUIs we drive; override
// via STRIATUM_AGENT_LOOP_SUBMIT_SEQUENCE using Go-style escapes (\r, \n) for
// adapters that need a different submit (e.g. bracketed paste). An explicitly
// empty override disables the submit (for headless adapters that EOF instead).
func agentLoopSubmitSequence() string {
	raw, ok := os.LookupEnv("STRIATUM_AGENT_LOOP_SUBMIT_SEQUENCE")
	if !ok {
		return "\r"
	}
	return decodeSubmitSequence(raw)
}

func decodeSubmitSequence(raw string) string {
	replacer := strings.NewReplacer(`\r`, "\r", `\n`, "\n", `\t`, "\t", `\\`, `\`)
	return replacer.Replace(raw)
}

// agentLoopSubmitDelay is how long to wait after writing the bootstrap prompt
// before sending the submit key-sequence, so a TUI line editor finishes
// ingesting the multi-line paste. Override via STRIATUM_AGENT_LOOP_SUBMIT_DELAY_MS.
func agentLoopSubmitDelay() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("STRIATUM_AGENT_LOOP_SUBMIT_DELAY_MS")); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms >= 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 750 * time.Millisecond
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

	// RFC 0088 Decision 5: give the lane CLI a striatum MCP server pointed at
	// the live endpoint + token, generated fresh into ephemeral scratch and
	// removed on exit (never persist the rotating port).
	laneCommand, cleanupMCP, err := injectLaneMCPConfig(cfg.Command, cfg.RepoRoot, cfg.Endpoint, cfg.Token)
	if err != nil {
		return fmt.Errorf("agent-loop mcp config: %w", err)
	}
	defer cleanupMCP()

	cmd := exec.CommandContext(ctx, laneCommand[0], laneCommand[1:]...)
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

	// Debug-only: tee the PTY output to a file so the operator can see an
	// interactive TUI agent's screen while tuning the submit sequence. Off by
	// default (D028: no transcript capture); enabled via
	// STRIATUM_AGENT_LOOP_DEBUG_LOG=<path> for live submit debugging only.
	var sink io.Writer = stdout
	if dbg := strings.TrimSpace(os.Getenv("STRIATUM_AGENT_LOOP_DEBUG_LOG")); dbg != "" {
		if f, ferr := os.OpenFile(dbg, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); ferr == nil {
			defer f.Close()
			fmt.Fprintf(f, "\n===== agent-loop session %s @ %s, command=%v =====\n", cfg.SessionID, cfg.RunID, laneCommand)
			sink = io.MultiWriter(stdout, f)
		}
	}

	outputDone := make(chan struct{})
	go func() {
		defer close(outputDone)
		_, _ = io.Copy(sink, ptmx)
	}()

	// Write the bootstrap prompt, then — after a short delay so the TUI line
	// editor finishes ingesting the (multi-line) paste — send the submit
	// key-sequence as a SEPARATE write so an interactive TUI agent actually
	// submits it instead of leaving it buffered in the input line (the RFC 0088
	// / D140 "buffers unsubmitted" blocker). Concatenating the CR to the prompt
	// does not submit: the editor absorbs it into the multi-line input. Headless
	// agents read the prompt as input and the later CR is harmless.
	if _, err := io.WriteString(ptmx, prompt); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("agent-loop bootstrap prompt: %w", err)
	}
	if submit := agentLoopSubmitSequence(); submit != "" {
		time.Sleep(agentLoopSubmitDelay())
		if _, err := io.WriteString(ptmx, submit); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return fmt.Errorf("agent-loop bootstrap submit: %w", err)
		}
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
