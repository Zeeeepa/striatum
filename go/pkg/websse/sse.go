package websse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Event struct {
	ID   int
	Type string
	Data map[string]any
}

type Batch struct {
	Events   []Event
	Terminal bool
	RunState string
}

type Source interface {
	Fetch(ctx context.Context, since int, limit int) (Batch, error)
}

type Options struct {
	PollInterval time.Duration
	Limit        int
	Heartbeat    time.Duration
}

func Since(r *http.Request) int {
	if value := strings.TrimSpace(r.Header.Get("Last-Event-ID")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			return parsed
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("since")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			return parsed
		}
	}
	return 0
}

func Stream(ctx context.Context, w http.ResponseWriter, source Source, since int, opts Options) {
	poll := opts.PollInterval
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	heartbeat := opts.Heartbeat
	if heartbeat <= 0 {
		heartbeat = 30 * time.Second
	}
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
	lastID := since
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	heartbeatTicker := time.NewTicker(heartbeat)
	defer heartbeatTicker.Stop()
	for {
		batch, err := source.Fetch(ctx, lastID, limit)
		if err != nil {
			_ = WriteEvent(w, "striatum.error", lastID, map[string]any{"message": err.Error()})
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		for _, event := range batch.Events {
			eventID := event.ID
			if eventID <= 0 {
				eventID = lastID
			}
			eventType := event.Type
			if eventType == "" {
				eventType = "striatum.event"
			}
			if err := WriteEvent(w, eventType, eventID, event.Data); err != nil {
				return
			}
			lastID = eventID
		}
		if batch.Terminal && len(batch.Events) == 0 {
			_ = WriteEvent(w, "striatum.run_terminal", lastID, map[string]any{"state": batch.RunState})
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		case <-ticker.C:
		}
	}
}

func WriteEvent(w io.Writer, event string, id int, data map[string]any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\nid: %d\n", event, id); err != nil {
		return err
	}
	for _, line := range strings.Split(string(encoded), "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "\n")
	return err
}
