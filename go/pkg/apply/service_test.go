package apply

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestReviewedPatchFailsClosedWithoutKey(t *testing.T) {
	service := Service{KeyLoaded: func() bool { return false }}
	_, err := service.ReviewedPatch(context.Background(), rpc.Envelope{})
	if rpcErr, ok := err.(*rpc.Error); !ok || rpcErr.Code != "sealed_key_missing" {
		t.Fatalf("err = %#v, want sealed_key_missing", err)
	}
}

func TestReviewedPatchFailsClosedWithKey(t *testing.T) {
	service := Service{KeyLoaded: func() bool { return true }}
	_, err := service.ReviewedPatch(context.Background(), rpc.Envelope{})
	if rpcErr, ok := err.(*rpc.Error); !ok || rpcErr.Code != "apply_gate_unsatisfied" {
		t.Fatalf("err = %#v, want apply_gate_unsatisfied", err)
	}
}

func TestReceiptLookupRequiresSchema(t *testing.T) {
	service := Service{}
	_, err := service.ShowReceipt(context.Background(), rpc.Envelope{Params: map[string]any{}})
	if rpcErr, ok := err.(*rpc.Error); !ok || rpcErr.Code != "daemon_db_missing" {
		t.Fatalf("err = %#v, want daemon_db_missing", err)
	}
}
