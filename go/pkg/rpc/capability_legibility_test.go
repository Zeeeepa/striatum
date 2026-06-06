package rpc

import (
	"errors"
	"strings"
	"testing"
)

// #182 / RFC 0111: a capability_missing refusal must be legible — it must name
// the missing capability, the method, and the remediation (the new
// `daemon token-create` verb), not just "daemon RPC authorization failed".
func TestRequireAllowedNamesMissingCapabilityAndRemediation(t *testing.T) {
	apply := CapabilityApply
	denied := AuthContext{Decision: "denied", DenialReason: "capability_missing"}

	err := RequireAllowed(denied, "run.integrate", &apply)
	if err == nil {
		t.Fatal("expected an error for a denied auth context")
	}
	var rpcErr *Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error is not *rpc.Error: %T", err)
	}
	if rpcErr.Code != "capability_missing" {
		t.Fatalf("code = %q, want capability_missing", rpcErr.Code)
	}
	for _, want := range []string{"run.integrate", "apply"} {
		if !strings.Contains(rpcErr.Message, want) {
			t.Fatalf("message %q does not name %q", rpcErr.Message, want)
		}
	}
	if rpcErr.Details["method"] != "run.integrate" {
		t.Fatalf("details.method = %#v", rpcErr.Details["method"])
	}
	if rpcErr.Details["required_capability"] != "apply" {
		t.Fatalf("details.required_capability = %#v", rpcErr.Details["required_capability"])
	}
	remediation, _ := rpcErr.Details["remediation"].(string)
	if !strings.Contains(remediation, "daemon token-create") || !strings.Contains(remediation, "apply") {
		t.Fatalf("details.remediation does not point at the token-create verb: %q", remediation)
	}
}

// An allowed context yields no error.
func TestRequireAllowedAllowsAllowed(t *testing.T) {
	apply := CapabilityApply
	if err := RequireAllowed(AuthContext{Decision: "allowed"}, "run.integrate", &apply); err != nil {
		t.Fatalf("allowed context returned error: %v", err)
	}
}

// Without route metadata the message degrades gracefully but still carries the
// denial code (e.g. direct handler unit tests pass empty method/capability).
func TestRequireAllowedDegradesWithoutMetadata(t *testing.T) {
	err := RequireAllowed(AuthContext{Decision: "denied", DenialReason: "token_invalid"}, "", nil)
	var rpcErr *Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("error is not *rpc.Error: %T", err)
	}
	if rpcErr.Code != "token_invalid" {
		t.Fatalf("code = %q", rpcErr.Code)
	}
}
