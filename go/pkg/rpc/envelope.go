package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	SupportedEnvelopeVersion = 1
	DefaultFraming           = "json"
)

type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
	// Suggestion is the in-band remediation an agent can act on (RFC 0111
	// P2). Call sites may set it explicitly; when empty, ErrorResponse fills
	// the catalog's per-code default (error_catalog.go) centrally.
	Suggestion string `json:"suggestion,omitempty"`
	ExitCode   int    `json:"-"`
}

func (e *Error) Error() string {
	return e.Message
}

func NewError(code, message string, details map[string]any) *Error {
	return &Error{Code: code, Message: message, Details: details, ExitCode: 10}
}

type Envelope struct {
	SchemaVersion   int            `json:"schema_version"`
	RequestID       string         `json:"request_id"`
	Method          string         `json:"method"`
	Params          map[string]any `json:"params"`
	CapabilityToken string         `json:"capability_token,omitempty"`
	DeadlineMS      int            `json:"deadline_ms"`
}

type Response struct {
	SchemaVersion int            `json:"schema_version"`
	RequestID     string         `json:"request_id"`
	OK            bool           `json:"ok"`
	Data          map[string]any `json:"data"`
	AuditID       string         `json:"audit_id,omitempty"`
}

func DecodeEnvelope(body []byte) (Envelope, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return Envelope{}, NewError("schema_invalid", "daemon RPC body is not valid JSON", nil)
	}
	if len(raw) == 0 {
		return Envelope{}, NewError("schema_invalid", "daemon RPC body must be a JSON object", nil)
	}
	return EnvelopeFromMap(raw)
}

func EnvelopeFromMap(payload map[string]any) (Envelope, error) {
	version, ok := intValue(payload["schema_version"])
	if !ok || version != SupportedEnvelopeVersion {
		return Envelope{}, NewError("version_incompatible", "daemon RPC envelope version is not supported", map[string]any{
			"supported_envelope": []int{SupportedEnvelopeVersion},
			"received":           payload["schema_version"],
		})
	}
	requestID, ok := payload["request_id"].(string)
	if !ok || requestID == "" {
		return Envelope{}, NewError("schema_invalid", "daemon RPC request_id must be a non-empty string", nil)
	}
	method, ok := payload["method"].(string)
	if !ok || method == "" {
		return Envelope{}, NewError("schema_invalid", "daemon RPC method must be a non-empty string", nil)
	}
	params, ok := payload["params"].(map[string]any)
	if !ok {
		return Envelope{}, NewError("schema_invalid", "daemon RPC params must be a JSON object", nil)
	}
	token := ""
	if value, exists := payload["capability_token"]; exists && value != nil {
		var tokenOK bool
		token, tokenOK = value.(string)
		if !tokenOK {
			return Envelope{}, NewError("schema_invalid", "daemon RPC capability_token must be a string when present", nil)
		}
	}
	deadline := 0
	if value, exists := payload["deadline_ms"]; exists {
		var deadlineOK bool
		deadline, deadlineOK = intValue(value)
		if !deadlineOK || deadline < 0 {
			return Envelope{}, NewError("schema_invalid", "daemon RPC deadline_ms must be a non-negative integer", nil)
		}
	}
	return Envelope{
		SchemaVersion:   SupportedEnvelopeVersion,
		RequestID:       requestID,
		Method:          method,
		Params:          params,
		CapabilityToken: token,
		DeadlineMS:      deadline,
	}, nil
}

func (e Envelope) Encode() ([]byte, error) {
	return json.Marshal(e)
}

func OKResponse(requestID string, data map[string]any, auditID string) Response {
	if data == nil {
		data = map[string]any{}
	}
	return Response{
		SchemaVersion: SupportedEnvelopeVersion,
		RequestID:     requestID,
		OK:            true,
		Data:          data,
		AuditID:       auditID,
	}
}

func ErrorResponse(requestID string, err error, auditID string) Response {
	rpcErr := &Error{}
	if !errors.As(err, &rpcErr) {
		rpcErr = NewError("internal_error", err.Error(), nil)
	}
	data := map[string]any{
		"code":    rpcErr.Code,
		"message": rpcErr.Message,
		"details": rpcErr.Details,
	}
	// RFC 0111 P2: fill the remediation centrally so the 165+ NewError call
	// sites stay untouched; an explicit call-site Suggestion wins over the
	// catalog default, and the key is omitted when neither exists.
	suggestion := rpcErr.Suggestion
	if suggestion == "" {
		suggestion = DefaultSuggestion(rpcErr.Code)
	}
	if suggestion != "" {
		data["suggestion"] = suggestion
	}
	return Response{
		SchemaVersion: SupportedEnvelopeVersion,
		RequestID:     requestID,
		OK:            false,
		Data:          data,
		AuditID:       auditID,
	}
}

func (r Response) Encode() ([]byte, error) {
	return json.Marshal(r)
}

func intValue(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if v == float64(int(v)) {
			return int(v), true
		}
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func DecodeResponse(body []byte) (Response, error) {
	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	return response, nil
}
