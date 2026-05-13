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

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const daemonVersion = "go-dev"

func main() {
	var socketPath string
	var postgresURL string
	var migrate bool
	var describe bool
	flag.StringVar(&socketPath, "socket", defaultSocketPath(), "Unix socket path")
	flag.StringVar(&postgresURL, "postgres-url", "", "PostgreSQL connection URL")
	flag.BoolVar(&migrate, "migrate", true, "apply daemon PostgreSQL migrations before serving when a URL is configured")
	flag.BoolVar(&describe, "describe", false, "print daemon metadata and exit")
	flag.Parse()

	if describe {
		fmt.Printf("core=go envelope=%d framing=%s methods_etag=%s\n", rpc.SupportedEnvelopeVersion, rpc.DefaultFraming, rpc.MethodsETag())
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	substrateSchema := 0
	var recorder *db.AuditRecorder
	config := db.ResolveConfig(postgresURL)
	if config.URL != "" && migrate {
		pool, version, err := db.ConnectAndMigrate(ctx, config.URL, daemonVersion)
		if err != nil {
			log.Fatalf("daemon db migration failed: %v", err)
		}
		substrateSchema = version
		recorder = &db.AuditRecorder{Runner: pool.Runner, DaemonVersion: daemonVersion}
	}

	server := rpc.NewServer()
	server.DaemonVersion = daemonVersion
	server.SubstrateSchema = substrateSchema
	server.Authorizer = rpc.AllowAllAuthorizer{}
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
