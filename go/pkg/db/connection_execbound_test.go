package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestBoundExecArgsUsesExtendedProtocol is the unit core of G-PRELUDE-MODE
// (RFC 0110 C-EXTENDED-AUTH-PRELUDE): the bound-exec arg builder must select the
// extended protocol (QueryExecModeExec) and must never fall back to the simple
// protocol, under which bound values interpolate into pg_stat_activity query
// text. It also preserves caller args in order after the mode marker.
func TestBoundExecArgsUsesExtendedProtocol(t *testing.T) {
	got := boundExecArgs("secret-value", 42, nil)

	if len(got) != 4 {
		t.Fatalf("expected mode marker + 3 args, got %d: %#v", len(got), got)
	}
	mode, ok := got[0].(pgx.QueryExecMode)
	if !ok {
		t.Fatalf("first bound arg must be a pgx.QueryExecMode, got %T", got[0])
	}
	if mode != pgx.QueryExecModeExec {
		t.Fatalf("prelude must ride QueryExecModeExec, got %v", mode)
	}
	if mode == pgx.QueryExecModeSimpleProtocol {
		t.Fatal("prelude must never ride simple protocol (secret would leak into query text)")
	}
	if got[1] != "secret-value" || got[2] != 42 || got[3] != nil {
		t.Fatalf("caller args must be preserved in order after the mode marker, got %#v", got[1:])
	}
}

// TestBoundExecArgsEmpty confirms the mode marker is present even with no
// caller args (a bare prelude statement still rides the extended protocol).
func TestBoundExecArgsEmpty(t *testing.T) {
	got := boundExecArgs()
	if len(got) != 1 {
		t.Fatalf("expected just the mode marker, got %#v", got)
	}
	if got[0] != pgx.QueryExecModeExec {
		t.Fatalf("bare prelude must still ride QueryExecModeExec, got %v", got[0])
	}
}
