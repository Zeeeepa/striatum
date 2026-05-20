package agentloop

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

type Client struct {
	mu   sync.Mutex
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

func NewClient(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	// We want no timeout by default because await_packet long-polls,
	// but we could set socket deadlines around reads if necessary.
	// For now, no deadline.

	return &Client{
		conn: conn,
		enc:  json.NewEncoder(conn),
		dec:  json.NewDecoder(bufio.NewReader(conn)),
	}, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Close()
}

func (c *Client) Call(ctx context.Context, method string, params map[string]any, capabilityToken string) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	reqID := make([]byte, 16)
	rand.Read(reqID)

	req := map[string]any{
		"schema_version": 1,
		"request_id":     hex.EncodeToString(reqID),
		"method":         method,
		"params":         params,
	}
	if method == "daemon.hello" {
		req["jsonrpc"] = "2.0"
	}
	if capabilityToken != "" {
		req["capability_token"] = capabilityToken
	}

	if err := c.enc.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// We don't want the context to directly cancel the Dial/Read because
	// net.Conn methods block. If we wanted perfect cancellation, we'd
	// set read deadlines periodically based on ctx.Done().
	// For simplicity, we just block on read.

	var resp struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}

	if err := c.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !resp.OK {
		// Try to extract error message
		code := "rpc_error"
		msg := "RPC call failed"
		if resp.Data != nil {
			if c, ok := resp.Data["code"].(string); ok {
				code = c
			}
			if m, ok := resp.Data["message"].(string); ok {
				msg = m
			}
		}
		return nil, fmt.Errorf("%s: %s", code, msg)
	}

	return resp.Data, nil
}
