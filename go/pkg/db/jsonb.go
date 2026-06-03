package db

import "encoding/json"

// JSONBTextEncoder is implemented by runner wrappers that delegate to a pgx
// runner (e.g. a SQL-recording test runner) and therefore want JSONBArg to
// marshal structured values to the pgx text-protocol JSON string, exactly as
// PgxRunner/PgxTxRunner do. A plain in-memory fake runner does not implement it,
// so it keeps receiving the original value.
type JSONBTextEncoder interface {
	EncodesJSONBAsText() bool
}

// JSONBArg converts structured values into a form pgx can encode for jsonb
// placeholders. In-memory test runners keep the original value so existing
// fake-runner assertions can inspect structured payloads directly; a wrapper
// over a pgx runner opts into the pgx text encoding via JSONBTextEncoder.
func JSONBArg(runner any, value any) (any, error) {
	pgxText := false
	switch runner.(type) {
	case PgxRunner, *PgxTxRunner:
		pgxText = true
	}
	if enc, ok := runner.(JSONBTextEncoder); ok && enc.EncodesJSONBAsText() {
		pgxText = true
	}
	if !pgxText {
		return value, nil
	}
	switch typed := value.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	case json.RawMessage:
		return string(typed), nil
	default:
		body, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return string(body), nil
	}
}
