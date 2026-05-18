package admin

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

// ShutdownFunc is the daemon-process hook for orderly shutdown. Tests can omit
// it to keep the handler's fail-closed behavior explicit.
type ShutdownFunc func(context.Context) error

// KeyRotateFunc is the sealed-apply signing-key rotation hook. Go daemon key
// management is not wired yet; keeping this injectable makes the handler real
// in tests/future wiring while defaulting to a precise refusal.
type KeyRotateFunc func(context.Context) (map[string]any, error)

func (s Service) Shutdown(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.ShutdownHook == nil {
		return nil, rpc.NewError(
			"shutdown_unavailable",
			"daemon.shutdown is not wired in this Go daemon process; stop the process with its service manager or signal",
			map[string]any{
				"operation":         "daemon.shutdown",
				"python_dependency": false,
				"sqlite_dependency": false,
			},
		)
	}
	if err := s.ShutdownHook(ctx); err != nil {
		return nil, err
	}
	return map[string]any{
		"status":             "shutting_down",
		"accepted":           true,
		"python_dependency":  false,
		"sqlite_dependency":  false,
		"source_import_mode": "none",
	}, nil
}

func (s Service) KeyRotate(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	if s.KeyRotateHook == nil {
		return nil, rpc.NewError(
			"key_rotation_unavailable",
			"daemon.key.rotate is not wired in this Go daemon build; sealed-apply signing keys were not modified",
			map[string]any{
				"operation":         "daemon.key.rotate",
				"key_kind":          "sealed_apply_signing_key",
				"python_dependency": false,
				"sqlite_dependency": false,
			},
		)
	}
	result, err := s.KeyRotateHook(ctx)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = map[string]any{}
	}
	result["python_dependency"] = false
	result["sqlite_dependency"] = false
	if _, ok := result["source_import_mode"]; !ok {
		result["source_import_mode"] = "none"
	}
	return result, nil
}
