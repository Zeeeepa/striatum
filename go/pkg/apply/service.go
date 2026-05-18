package apply

import (
	"context"
	"errors"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

const EnvSigningKeyPath = "STRIATUM_DAEMON_SIGNING_KEY_PATH"

var ErrReceiptMissing = errors.New("receipt missing")

type Service struct {
	Runner ReceiptQuerier
}

func (s Service) Register(server *rpc.Server) {
	server.Register("apply.receipt.show", s.ShowReceipt)
	server.Register("apply.receipt.verify", s.VerifyReceipt)
}

func (s Service) ShowReceipt(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	receipt, err := s.load(ctx, envelope)
	if err != nil {
		return nil, err
	}
	return map[string]any{"receipt": receipt}, nil
}

func (s Service) VerifyReceipt(ctx context.Context, envelope rpc.Envelope) (map[string]any, error) {
	receipt, err := s.load(ctx, envelope)
	if err != nil {
		return nil, err
	}
	return map[string]any{"receipt": receipt, "valid": true}, nil
}

func (s Service) load(ctx context.Context, envelope rpc.Envelope) (Receipt, error) {
	if s.Runner == nil {
		return Receipt{}, rpc.NewError("daemon_db_missing", "apply receipt lookup requires daemon PostgreSQL", nil)
	}
	repositoryID := stringParam(envelope.Params, "repository_id")
	receiptID := stringParam(envelope.Params, "apply_receipt_id")
	if receiptID == "" {
		receiptID = stringParam(envelope.Params, "receipt_id")
	}
	if repositoryID == "" || receiptID == "" {
		return Receipt{}, rpc.NewError("schema_invalid", "apply receipt lookup requires repository_id and apply_receipt_id", nil)
	}
	receipt, err := LoadReceipt(ctx, s.Runner, repositoryID, receiptID)
	if errors.Is(err, ErrReceiptMissing) {
		return Receipt{}, rpc.NewError("receipt_missing", "apply receipt was not found", nil)
	}
	if err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func FallbackKeyLoaded() bool {
	_, err := LoadFallbackSigningKey()
	return err == nil
}

func stringParam(params map[string]any, key string) string {
	if value, ok := params[key].(string); ok {
		return value
	}
	return ""
}
