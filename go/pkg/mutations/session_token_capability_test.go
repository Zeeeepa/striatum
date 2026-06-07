package mutations

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

// TestSessionBoundTokenCoversLaneLoopRPCs guards the regression where the
// session-bound lane token (mintSessionBoundToken) granted only {claim, write},
// so the agent-loop daemon receiver's supervise.status readiness gate
// (loop.go daemonReceiverReady, a CapabilityRead method) was denied with
// "daemon RPC authorization failed" — and a supervised lane looped forever,
// never claiming its first packet. The token's grants MUST cover the
// RequiredCapability of every RPC the lane loop drives autonomously; this couples
// the grant set to the live daemon registry so it cannot silently drift again.
func TestSessionBoundTokenCoversLaneLoopRPCs(t *testing.T) {
	granted := map[rpc.Capability]bool{}
	for _, c := range sessionBoundCapabilities {
		granted[c] = true
	}
	required := map[string]*rpc.Capability{}
	for _, m := range rpc.SortedMethods() {
		required[m.Method] = m.RequiredCapability
	}
	// The RPCs a supervised lane drives without an operator: the receiver's
	// readiness gate (supervise.status), the work-lifecycle entrypoint
	// (work.await_packet), and artifact publication (artifact.publish). A
	// reviewer lane additionally records its verdict (review.submit /
	// review.verdict, CapabilityReview) — #202: without that grant every
	// session-minted token failed review.submit with capability_missing, so a
	// review job's only legal end-state was unreachable and each review
	// escalated to a human checkpoint.
	for _, method := range []string{"supervise.status", "work.await_packet", "artifact.publish", "review.submit", "review.verdict"} {
		capability, ok := required[method]
		if !ok {
			t.Fatalf("method %q is not in the daemon registry (rpc.SortedMethods)", method)
		}
		if capability == nil {
			continue // method requires no capability
		}
		if !granted[*capability] {
			t.Fatalf("session-bound lane token lacks capability %q required by %q — a supervised lane would wedge before claiming; sessionBoundCapabilities=%v",
				*capability, method, sessionBoundCapabilities)
		}
	}
}
