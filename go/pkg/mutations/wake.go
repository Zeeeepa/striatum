package mutations

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type WakeEvent struct {
	Sequence       uint64 `json:"sequence"`
	RepositoryID   string `json:"repository_id"`
	RunID          string `json:"run_id,omitempty"`
	Kind           string `json:"kind"`
	MessageID      string `json:"message_id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
}

type wakeFilter struct {
	RepositoryID string
	RunID        string
}

type wakeSubscription struct {
	filter wakeFilter
	ch     chan WakeEvent
}

type wakeBroker struct {
	mu          sync.Mutex
	nextID      int
	nextSeq     uint64
	subscribers map[int]wakeSubscription
}

func newWakeBroker() *wakeBroker {
	return &wakeBroker{subscribers: map[int]wakeSubscription{}}
}

var wakeBus = newWakeBroker()

func (b *wakeBroker) Subscribe(filter wakeFilter) (<-chan WakeEvent, func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	id := b.nextID
	ch := make(chan WakeEvent, 1)
	b.subscribers[id] = wakeSubscription{filter: filter, ch: ch}
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.subscribers, id)
	}
}

func (b *wakeBroker) Publish(event WakeEvent) {
	if event.RepositoryID == "" || event.Kind == "" {
		return
	}
	b.mu.Lock()
	b.nextSeq++
	event.Sequence = b.nextSeq
	var targets []chan WakeEvent
	for _, sub := range b.subscribers {
		if !sub.matches(event) {
			continue
		}
		targets = append(targets, sub.ch)
	}
	b.mu.Unlock()

	for _, ch := range targets {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s wakeSubscription) matches(event WakeEvent) bool {
	if s.filter.RepositoryID != "" && s.filter.RepositoryID != event.RepositoryID {
		return false
	}
	if s.filter.RunID != "" && s.filter.RunID != event.RunID {
		return false
	}
	return true
}

func (b *wakeBroker) Wait(ctx context.Context, filter wakeFilter, timeout time.Duration) (WakeEvent, bool, error) {
	if timeout <= 0 {
		return WakeEvent{}, false, nil
	}
	ch, cancel := b.Subscribe(filter)
	defer cancel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return WakeEvent{}, false, ctx.Err()
	case event := <-ch:
		return event, true, nil
	case <-timer.C:
		return WakeEvent{}, false, nil
	}
}

type wakeRecorder interface {
	recordWake(WakeEvent)
}

type wakeCollector struct {
	events []WakeEvent
}

func (c *wakeCollector) add(event WakeEvent) {
	if event.RepositoryID == "" || event.Kind == "" {
		return
	}
	c.events = append(c.events, event)
}

func (c *wakeCollector) publish() {
	for _, event := range c.events {
		wakeBus.Publish(event)
	}
}

type wakeTx struct {
	db.TxRunner
	collector *wakeCollector
}

func newWakeTx(tx db.TxRunner) *wakeTx {
	return &wakeTx{TxRunner: tx, collector: &wakeCollector{}}
}

func (tx *wakeTx) recordWake(event WakeEvent) {
	tx.collector.add(event)
}

func (tx *wakeTx) publishWakeEvents() {
	tx.collector.publish()
}

func (tx *wakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	q, ok := tx.TxRunner.(queryer)
	if !ok {
		return nil, fmt.Errorf("runner does not support row queries")
	}
	return q.Query(ctx, sql, args...)
}

func (tx *wakeTx) ExecBound(ctx context.Context, sql string, args ...any) error {
	bound, ok := tx.TxRunner.(interface {
		ExecBound(context.Context, string, ...any) error
	})
	if !ok {
		return tx.Exec(ctx, sql, args...)
	}
	return bound.ExecBound(ctx, sql, args...)
}

// QueryRowBound forwards the extended-protocol single-row read to the wrapped
// runner so identity-projection reads (admin.ResolvePrincipalForClient ->
// queryIdentityRow) executed inside a mutation transaction take the bound
// SECURITY DEFINER path (secret in Bind, not query text) rather than falling back
// to the direct SQL that 42501s on a two-role deploy (RFC 0167 P0 run-origin
// stamp). Falls back to the unbound QueryRow when the runner is not bound-capable.
func (tx *wakeTx) QueryRowBound(ctx context.Context, sql string, args ...any) pgx.Row {
	bound, ok := tx.TxRunner.(interface {
		QueryRowBound(context.Context, string, ...any) pgx.Row
	})
	if !ok {
		return tx.QueryRow(ctx, sql, args...)
	}
	return bound.QueryRowBound(ctx, sql, args...)
}

func (tx *wakeTx) EncodesJSONBAsText() bool {
	if enc, ok := tx.TxRunner.(db.JSONBTextEncoder); ok {
		return enc.EncodesJSONBAsText()
	}
	switch tx.TxRunner.(type) {
	case *db.PgxTxRunner:
		return true
	default:
		return false
	}
}

func recordWake(runner any, event WakeEvent) {
	if recorder, ok := runner.(wakeRecorder); ok {
		recorder.recordWake(event)
		return
	}
	wakeBus.Publish(event)
}

func HandleWakeWait(ctx context.Context, _ db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	timeout := time.Duration(intParam(envelope, "timeout_ms", 0)) * time.Millisecond
	if timeout > time.Minute {
		timeout = time.Minute
	}
	event, ok, err := wakeBus.Wait(ctx, wakeFilter{RepositoryID: repositoryID, RunID: runID}, timeout)
	if err != nil {
		return nil, err
	}
	if !ok {
		return map[string]any{
			"status":        "timeout",
			"repository_id": repositoryID,
			"run_id":        runID,
			"timeout_ms":    int(timeout / time.Millisecond),
		}, nil
	}
	return map[string]any{
		"status": "notified",
		"event":  wakeEventMap(event),
	}, nil
}

func wakeEventMap(event WakeEvent) map[string]any {
	out := map[string]any{
		"sequence":      event.Sequence,
		"repository_id": event.RepositoryID,
		"kind":          event.Kind,
	}
	if event.RunID != "" {
		out["run_id"] = event.RunID
	}
	if event.MessageID != "" {
		out["message_id"] = event.MessageID
	}
	if event.ConversationID != "" {
		out["conversation_id"] = event.ConversationID
	}
	return out
}
