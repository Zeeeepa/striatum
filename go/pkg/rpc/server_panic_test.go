package rpc

import (
	"bytes"
	"context"
	"io"
	"log"
	"strings"
	"testing"
)

type scriptedReadWriteCloser struct {
	reader *strings.Reader
	writes bytes.Buffer
}

func newScriptedReadWriteCloser(input string) *scriptedReadWriteCloser {
	return &scriptedReadWriteCloser{reader: strings.NewReader(input)}
}

func (c *scriptedReadWriteCloser) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *scriptedReadWriteCloser) Write(p []byte) (int, error) {
	return c.writes.Write(p)
}

func (c *scriptedReadWriteCloser) Close() error {
	return nil
}

func TestServeConnLogsMethodAndRepanicsOnHandlerPanic(t *testing.T) {
	var logs bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	defer log.SetOutput(originalWriter)
	defer log.SetFlags(originalFlags)

	server := NewServer()
	server.Register("status", func(context.Context, Envelope) (map[string]any, error) {
		panic("handler panic sentinel")
	})
	server.markHandshake("conn-panic")

	conn := newScriptedReadWriteCloser(`{"schema_version":1,"request_id":"req_panic","method":"status","params":{"repository_id":"repo_1"},"deadline_ms":0}` + "\n")

	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		if err := server.ServeConn(context.Background(), conn, "conn-panic"); err != nil && err != io.EOF {
			t.Fatalf("ServeConn returned unexpected error before panic: %v", err)
		}
	}()
	if recovered != "handler panic sentinel" {
		t.Fatalf("recovered panic = %#v, want handler panic sentinel", recovered)
	}
	logText := logs.String()
	for _, want := range []string{
		"daemon RPC ServeConn panic",
		`connection_id="conn-panic"`,
		`method="status"`,
		`request_id="req_panic"`,
		"handler panic sentinel",
		"runtime/debug.Stack",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("panic log missing %q:\n%s", want, logText)
		}
	}
}
