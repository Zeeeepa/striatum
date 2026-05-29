package webservice

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/websse"
)

// dialogueEventSource implements websse.Source to stream enqueued dialogue messages.
type dialogueEventSource struct {
	handler *Handler
	runID   string
}

func (s dialogueEventSource) Fetch(ctx context.Context, since int, limit int) (websse.Batch, error) {
	data, _, code, message := s.handler.call(ctx, "trajectory.watch", map[string]any{
		"run_id":    s.runID,
		"profile":   "dialogue",
		"since_seq": since,
	})
	if code != "" {
		return websse.Batch{}, fmt.Errorf("%s: %s", code, message)
	}

	batch := websse.Batch{}
	statusData, _, scode, _ := s.handler.call(ctx, "status", map[string]any{
		"run_id": s.runID,
	})
	if scode == "" {
		if run, ok := statusData["run"].(map[string]any); ok {
			state, _ := run["state"].(string)
			batch.RunState = state
			if state == "completed" || state == "failed" || state == "canceled" {
				batch.Terminal = true
			}
		}
	}

	records, _ := data["records"].([]any)
	for _, raw := range records {
		record, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		seq := intFromAny(record["seq"])
		batch.Events = append(batch.Events, websse.Event{
			ID:   seq,
			Type: "striatum.dialogue",
			Data: record,
		})
	}
	return batch, nil
}

func (h *Handler) resolveRepoRoot(ctx context.Context, runID string) (string, error) {
	data, _, code, message := h.call(ctx, "list.runs", map[string]any{})
	if code != "" {
		return "", fmt.Errorf("list.runs failed: %s: %s", code, message)
	}
	items, _ := data["items"].([]any)
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if fmt.Sprint(item["run_id"]) == runID {
			return fmt.Sprint(item["repo_root"]), nil
		}
	}
	return "", fmt.Errorf("run not found: %s", runID)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

func (h *Handler) streamLiveDialogue(w http.ResponseWriter, r *http.Request, runID string) {
	if !h.sse.acquire(runID) {
		writeJSON(w, http.StatusTooManyRequests, errorPayload("sse_limit_exceeded", "too many concurrent SSE streams for run"))
		return
	}
	defer h.sse.release(runID)

	source := dialogueEventSource{handler: h, runID: runID}
	websse.Stream(r.Context(), w, source, websse.Since(r), websse.Options{PollInterval: h.config.SSEPollInterval, Limit: 100})
}

func (h *Handler) streamLivePTY(w http.ResponseWriter, r *http.Request, sessionID string) {
	// 1. Fetch supervisor status to get supervisor_id and run_id.
	data, _, code, message := h.call(r.Context(), "supervise.status", map[string]any{"session_id": sessionID})
	if code != "" {
		writeJSON(w, http.StatusNotFound, errorPayload("supervisor_not_found", fmt.Sprintf("failed to get supervisor: %s: %s", code, message)))
		return
	}

	supervisor, _ := data["supervisor"].(map[string]any)
	if supervisor == nil {
		writeJSON(w, http.StatusNotFound, errorPayload("supervisor_not_found", "no supervisor details found"))
		return
	}

	supervisorID := fmt.Sprint(supervisor["supervisor_id"])
	runID := fmt.Sprint(supervisor["run_id"])
	if supervisorID == "" || runID == "" {
		writeJSON(w, http.StatusNotFound, errorPayload("supervisor_not_found", "missing supervisor_id or run_id"))
		return
	}

	// 2. Resolve repo root.
	repoRoot, err := h.resolveRepoRoot(r.Context(), runID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorPayload("repo_root_error", err.Error()))
		return
	}

	ptyLogPath := filepath.Join(repoRoot, ".striatum", "scratch", supervisorID, "pty.log")

	// Set SSE Headers
	header := w.Header()
	header.Set("Content-Type", "text/event-stream")
	header.Set("Cache-Control", "no-cache, no-transform")
	header.Set("Connection", "keep-alive")
	header.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	// Try to open the file, wait if it doesn't exist yet
	var file *os.File
	retryTicker := time.NewTicker(500 * time.Millisecond)
	defer retryTicker.Stop()

	openTimeout := time.After(10 * time.Second)
	opened := false

	for !opened {
		select {
		case <-r.Context().Done():
			return
		case <-openTimeout:
			_ = websse.WriteEvent(w, "striatum.pty_error", 0, map[string]any{"message": "timeout waiting for pty.log"})
			if flusher != nil {
				flusher.Flush()
			}
			return
		case <-retryTicker.C:
			var err error
			file, err = os.Open(ptyLogPath)
			if err == nil {
				opened = true
			} else {
				_ = websse.WriteEvent(w, "striatum.pty_waiting", 0, map[string]any{"message": "Waiting for terminal log..."})
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	}
	defer func() { _ = file.Close() }()

	// Seek to end minus 32KB for some initial history, or start from 0 if small.
	fi, err := file.Stat()
	if err == nil {
		if fi.Size() > 32768 {
			_, _ = file.Seek(fi.Size()-32768, io.SeekStart)
		}
	}

	buf := make([]byte, 4096)
	readTicker := time.NewTicker(100 * time.Millisecond)
	defer readTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-readTicker.C:
			n, err := file.Read(buf)
			if n > 0 {
				escaped := htmlEscape(string(buf[:n]))
				writeErr := websse.WriteEvent(w, "striatum.pty", 0, map[string]any{"text": escaped})
				if writeErr != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil && err != io.EOF {
				return
			}
		}
	}
}
