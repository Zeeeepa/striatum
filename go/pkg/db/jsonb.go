package db

import "encoding/json"

// JSONBArg converts structured values into a form pgx can encode for jsonb
// placeholders. In-memory test runners keep the original value so existing
// fake-runner assertions can inspect structured payloads directly.
func JSONBArg(runner any, value any) (any, error) {
	switch runner.(type) {
	case PgxRunner, *PgxTxRunner:
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
	default:
		return value, nil
	}
}
