package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"
)

func Run(socketPath, repoRoot, runID, sessionID string, command []string) error {
	if sessionID == "" || runID == "" || repoRoot == "" {
		return fmt.Errorf("STRIATUM_SESSION_ID, STRIATUM_RUN_ID, and STRIATUM_REPO must be in environment")
	}
	if len(command) == 0 {
		return fmt.Errorf("agent command is required")
	}

	log.Printf("Starting MCP loop wrapper for session %s on run %s", sessionID, runID)
	log.Printf("Wrapped command: %v", command)

	client, err := NewClient(socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon socket: %w", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Handshake
	_, err = client.Call(ctx, "daemon.hello", map[string]any{
		"client": map[string]any{
			"name":               "striatum-mcp-loop-wrapper",
			"version":            "0.1.0",
			"supported_envelope": []int{1},
			"supported_framings": []string{"json"},
		},
	}, "")
	if err != nil {
		return fmt.Errorf("daemon.hello failed: %w", err)
	}

	token, err := ReadRuntimeToken(repoRoot)
	if err != nil {
		log.Printf("Warning: failed to read runtime token: %v", err)
	}

	for {
		// Check run summary
		runSummary, err := client.Call(ctx, "run.summary", map[string]any{
			"repository_id": repoRoot,
			"run_id":        runID,
		}, token)
		if err != nil {
			log.Printf("RPC Error fetching run.summary: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if runMap, ok := runSummary["run"].(map[string]any); ok {
			if state, _ := runMap["state"].(string); state != "running" {
				log.Printf("Run state is %s, exiting interactive loop.", state)
				break
			}
		}

		log.Printf("Waiting for packet...")
		res, err := client.Call(ctx, "work.await_packet", map[string]any{
			"repository_id": repoRoot,
			"session_id":    sessionID,
			"lease_seconds": 3600,
		}, token)

		if err != nil {
			log.Printf("RPC Error await_packet: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		status, _ := res["status"].(string)
		if status == "no_work" || status == "timeout" {
			time.Sleep(1 * time.Second)
			continue
		}

		if status != "claimed" {
			log.Printf("Unexpected await_packet status: %s", status)
			time.Sleep(1 * time.Second)
			continue
		}

		packet, _ := res["packet"].(map[string]any)
		if packet == nil {
			log.Printf("Claimed response is missing packet field")
			continue
		}

		packetID, _ := res["packet_id"].(string)
		leaseID, _ := res["lease_id"].(string)
		var jobID string
		if job, ok := packet["job"].(map[string]any); ok {
			jobID, _ = job["job_id"].(string)
		}

		log.Printf("Claimed packet %s (job %s). Invoking wrapped command...", packetID, jobID)

		packetBytes, err := json.Marshal(packet)
		if err != nil {
			log.Printf("Failed to marshal packet: %v", err)
			continue
		}

		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		cmd.Dir = repoRoot
		cmd.Env = os.Environ()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Printf("Failed to get stdin pipe: %v", err)
			continue
		}

		if err := cmd.Start(); err != nil {
			log.Printf("Failed to start command: %v", err)
			continue
		}

		// Write packet JSON to stdin
		go func() {
			defer stdin.Close()
			_, _ = stdin.Write(packetBytes)
			_, _ = io.WriteString(stdin, "\n")
		}()

		err = cmd.Wait()
		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			} else {
				exitCode = 1
			}
		}

		if exitCode == 0 {
			log.Printf("Command succeeded. Completing job %s...", jobID)
			_, err = client.Call(ctx, "work.complete", map[string]any{
				"repository_id": repoRoot,
				"session_id":    sessionID,
				"job_id":        jobID,
				"lease_id":      leaseID,
				"summary":       "Completed via striatumd agent-loop",
			}, token)
			if err != nil {
				log.Printf("RPC Error completing work: %v", err)
			}
		} else {
			log.Printf("Command failed with code %d. Releasing job %s...", exitCode, jobID)
			_, err = client.Call(ctx, "work.release", map[string]any{
				"repository_id": repoRoot,
				"session_id":    sessionID,
				"lease_id":      leaseID,
				"reason":        fmt.Sprintf("Wrapped command failed with exit code %d", exitCode),
			}, token)
			if err != nil {
				log.Printf("RPC Error releasing work: %v", err)
			}
		}
	}

	return nil
}
