package rpcclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/admin"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

const (
	EnvDaemonSocket    = "STRIATUM_DAEMON_SOCKET"
	EnvDaemonToken     = "STRIATUM_DAEMON_TOKEN"
	EnvDaemonTokenFile = "STRIATUM_DAEMON_TOKEN_FILE"
	DefaultDeadlineMS  = 30000
)

type Config struct {
	SocketPath string
	Token      string
	TokenFile  string
	DeadlineMS int
}

type Client struct {
	Config Config
}

func ResolveConfig(env []string, socketPath string, token string, tokenFile string, deadlineMS int) (Config, error) {
	values := envMap(env)
	if socketPath == "" {
		socketPath = values[EnvDaemonSocket]
	}
	if socketPath == "" {
		socketPath = defaultSocketPath(values)
	}
	if token == "" {
		token = values[EnvDaemonToken]
	}
	if tokenFile == "" {
		tokenFile = values[EnvDaemonTokenFile]
	}
	if token == "" && tokenFile == "" {
		var err error
		tokenFile, err = admin.RuntimeTokenPath()
		if err != nil {
			return Config{}, err
		}
	}
	if deadlineMS == 0 {
		deadlineMS = DefaultDeadlineMS
	}
	return Config{SocketPath: socketPath, Token: token, TokenFile: tokenFile, DeadlineMS: deadlineMS}, nil
}

func (c Client) Invoke(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	tokens, err := c.resolveInvocationTokens()
	if err != nil {
		return nil, err
	}
	data, err := c.invokeWithToken(ctx, method, params, tokens.primary)
	if err == nil {
		return data, nil
	}
	if tokens.fallback == "" || tokens.fallback == tokens.primary || !isTokenAuthFailure(err) {
		return nil, err
	}
	data, fallbackErr := c.invokeWithToken(ctx, method, params, tokens.fallback)
	if fallbackErr != nil {
		return nil, fallbackErr
	}
	if tokens.repairTokenFile != "" {
		_ = admin.WriteRuntimeToken(tokens.repairTokenFile, tokens.fallback)
	}
	return data, nil
}

type invocationTokens struct {
	primary         string
	fallback        string
	repairTokenFile string
}

func (c Client) resolveInvocationTokens() (invocationTokens, error) {
	if c.Config.Token != "" {
		return invocationTokens{primary: c.Config.Token}, nil
	}
	if c.Config.TokenFile == "" {
		return invocationTokens{}, nil
	}
	fallback, _ := runtimeDiscoveryToken(c.Config.TokenFile)
	body, err := os.ReadFile(c.Config.TokenFile)
	if err != nil {
		if fallback != "" {
			return invocationTokens{primary: fallback, repairTokenFile: c.Config.TokenFile}, nil
		}
		return invocationTokens{}, &Error{Code: "token_unavailable", Message: fmt.Sprintf("read daemon capability token: %v", err), ExitCode: 11}
	}
	return invocationTokens{
		primary:         strings.TrimSpace(string(body)),
		fallback:        fallback,
		repairTokenFile: c.Config.TokenFile,
	}, nil
}

func (c Client) invokeWithToken(ctx context.Context, method string, params map[string]any, token string) (map[string]any, error) {
	conn, err := net.DialTimeout("unix", c.Config.SocketPath, 5*time.Second)
	if err != nil {
		return nil, &Error{Code: "daemon_unreachable", Message: fmt.Sprintf("daemon unreachable at %s: %v", c.Config.SocketPath, err), ExitCode: 11}
	}
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	if _, err := send(ctx, conn, reader, rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     requestID("hello"),
		Method:        "daemon.hello",
		Params: map[string]any{"client": map[string]any{
			"name":               "striatum-go-cli",
			"supported_envelope": []int{rpc.SupportedEnvelopeVersion},
			"supported_framings": []string{rpc.DefaultFraming},
		}},
		DeadlineMS: c.Config.DeadlineMS,
	}); err != nil {
		return nil, err
	}
	response, err := send(ctx, conn, reader, rpc.Envelope{
		SchemaVersion:   rpc.SupportedEnvelopeVersion,
		RequestID:       requestID("cli"),
		Method:          method,
		Params:          params,
		CapabilityToken: token,
		DeadlineMS:      c.Config.DeadlineMS,
	})
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}

func runtimeDiscoveryToken(tokenFile string) (string, bool) {
	if filepath.Base(tokenFile) != "client-token" {
		return "", false
	}
	discoveryPath := filepath.Join(filepath.Dir(tokenFile), "discovery.json")
	info, err := os.Stat(discoveryPath)
	if err != nil || info.Mode().Perm()&0o077 != 0 {
		return "", false
	}
	body, err := os.ReadFile(discoveryPath)
	if err != nil {
		return "", false
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return "", false
	}
	token, _ := data["client_token"].(string)
	token = strings.TrimSpace(token)
	return token, token != ""
}

func isTokenAuthFailure(err error) bool {
	var clientErr *Error
	if errors.As(err, &clientErr) {
		switch clientErr.Code {
		case "token_missing", "token_malformed", "token_invalid", "token_revoked", "token_expired",
			"capability_missing", "capability_scope_mismatch", "capability_expired":
			return true
		}
	}
	return false
}

func send(ctx context.Context, conn net.Conn, reader *bufio.Reader, envelope rpc.Envelope) (rpc.Response, error) {
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	encoded, err := envelope.Encode()
	if err != nil {
		return rpc.Response{}, err
	}
	if _, err := conn.Write(append(encoded, '\n')); err != nil {
		return rpc.Response{}, &Error{Code: "daemon_unreachable", Message: fmt.Sprintf("write daemon RPC request: %v", err), ExitCode: 11}
	}
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return rpc.Response{}, &Error{Code: "daemon_unreachable", Message: fmt.Sprintf("read daemon RPC response: %v", err), ExitCode: 11}
	}
	response, err := rpc.DecodeResponse(line)
	if err != nil {
		return rpc.Response{}, &Error{Code: "schema_invalid", Message: err.Error(), ExitCode: 10}
	}
	if !response.OK {
		return rpc.Response{}, rpcError(response)
	}
	return response, nil
}

type Error struct {
	Code     string
	Message  string
	ExitCode int
}

func (e *Error) Error() string {
	return e.Message
}

func rpcError(response rpc.Response) error {
	code, _ := response.Data["code"].(string)
	message, _ := response.Data["message"].(string)
	if code == "" {
		code = "daemon_error"
	}
	if message == "" {
		message = code
	}
	return &Error{Code: code, Message: message, ExitCode: exitCode(code)}
}

func ExitCode(err error) int {
	var clientErr *Error
	if errors.As(err, &clientErr) && clientErr.ExitCode != 0 {
		return clientErr.ExitCode
	}
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return exitCode(rpcErr.Code)
	}
	return 1
}

func exitCode(code string) int {
	switch code {
	case "artifact_error", "write_scope_drift":
		return 6
	case "version_incompatible":
		return 10
	case "daemon_unreachable":
		return 11
	case "repo_not_registered":
		return 12
	case "token_missing", "token_malformed", "token_invalid", "token_revoked", "token_expired", "capability_missing", "capability_scope_mismatch", "capability_expired":
		return 13
	case "schema_invalid", "method_unknown":
		return 2
	default:
		return 1
	}
}

func requestID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func envMap(env []string) map[string]string {
	values := map[string]string{}
	if env == nil {
		env = os.Environ()
	}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func defaultSocketPath(values map[string]string) string {
	runtimeDir := values["XDG_RUNTIME_DIR"]
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	return filepath.Join(runtimeDir, "striatum", "daemon-go.sock")
}
