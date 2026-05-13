package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const daemonVersion = "go-dev"

func main() {
	var socketPath string
	var postgresURL string
	var migrate bool
	var describe bool
	var migrationsSHASource string
	flag.StringVar(&socketPath, "socket", defaultSocketPath(), "Unix socket path")
	flag.StringVar(&postgresURL, "postgres-url", "", "PostgreSQL connection URL")
	flag.BoolVar(&migrate, "migrate", true, "apply daemon PostgreSQL migrations before serving when a URL is configured")
	flag.BoolVar(&describe, "describe", false, "print daemon metadata and exit")
	flag.StringVar(&migrationsSHASource, "migrations-sha-source", "", "verify embedded migration SHAs against SQL files at this path before serving")
	flag.Parse()

	if describe {
		fmt.Printf("core=go envelope=%d framing=%s methods_etag=%s\n", rpc.SupportedEnvelopeVersion, rpc.DefaultFraming, rpc.MethodsETag())
		return
	}

	if migrationsSHASource != "" {
		if err := db.VerifyMigrationsSHASource(migrationsSHASource); err != nil {
			log.Fatalf("migrations sha source check failed: %v", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	substrateSchema := 0
	var recorder *db.AuditRecorder
	var authorizer rpc.Authorizer = rpc.AllowAllAuthorizer{}
	config := db.ResolveConfig(postgresURL)
	if config.URL != "" {
		var pool *db.Pool
		var err error
		if migrate {
			var version int
			pool, version, err = db.ConnectAndMigrate(ctx, config.URL, daemonVersion)
			if err != nil {
				log.Fatalf("daemon db connect/migrate failed: %v", err)
			}
			substrateSchema = version
		} else {
			pool, err = db.Connect(ctx, config.URL, daemonVersion)
			if err != nil {
				log.Fatalf("daemon db connect failed: %v", err)
			}
		}
		defer pool.Close()
		recorder = &db.AuditRecorder{Runner: pool.Runner, DaemonVersion: daemonVersion}
		authorizer = &rpc.PostgresAuthorizer{Runner: pool.Runner, Clock: time.Now}
	}

	server := rpc.NewServer()
	server.DaemonVersion = daemonVersion
	server.SubstrateSchema = substrateSchema
	server.Authorizer = authorizer
	if recorder != nil {
		server.AuditRecorder = recorder
	}

	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		log.Fatalf("create socket directory: %v", err)
	}
	listener, err := rpc.ListenUnix(socketPath)
	if err != nil {
		log.Fatalf("listen on %s: %v", socketPath, err)
	}
	log.Printf("striatumd-go listening on %s", socketPath)
	if err := server.Serve(ctx, listener); err != nil && ctx.Err() == nil {
		log.Fatalf("serve: %v", err)
	}
}

func defaultSocketPath() string {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	return filepath.Join(runtimeDir, "striatum", "daemon-go.sock")
}
