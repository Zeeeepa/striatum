// Package admin contains daemon-global administrative RPC handlers.
package admin

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

type Service struct {
	Runner        db.Runner
	DaemonVersion string
}

func (s Service) Register(server *rpc.Server) {
	if s.Runner == nil {
		return
	}
	server.Register("daemon.migrate", s.Migrate)
}

func (s Service) Migrate(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	versionLabel := s.DaemonVersion
	if versionLabel == "" {
		versionLabel = "unknown"
	}
	version, err := db.ApplyMigrations(ctx, s.Runner, versionLabel)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status":             "migrated",
		"substrate":          "postgres",
		"substrate_schema":   version,
		"latest_schema":      db.LatestDaemonDBVersion,
		"migration_count":    db.LatestDaemonDBVersion,
		"daemon_version":     versionLabel,
		"sqlite_dependency":  false,
		"python_dependency":  false,
		"source_import_mode": "none",
	}, nil
}
