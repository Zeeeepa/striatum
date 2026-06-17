package rpc

import (
	"fmt"
	"testing"
	"time"
)

// TestBoundedSeenCapHoldsUnderManyDistinctKeys is the #358 item-2 security
// regression: markRequest runs pre-auth, so an unauthenticated caller presenting
// an unbounded stream of distinct request_ids must NOT grow daemon memory without
// bound. With a hard cap of N, live entries stay <= N no matter how many distinct
// keys arrive.
func TestBoundedSeenCapHoldsUnderManyDistinctKeys(t *testing.T) {
	const cap = 100
	b := newBoundedSeen(cap, time.Hour)
	for i := 0; i < 100*cap; i++ {
		if dup := b.Add(fmt.Sprintf("req-%d", i)); dup {
			t.Fatalf("distinct key %d reported as duplicate", i)
		}
		if got := b.Len(); got > cap {
			t.Fatalf("after %d distinct keys, live entries=%d exceeds cap=%d", i+1, got, cap)
		}
	}
	if got := b.Len(); got != cap {
		t.Fatalf("after a sustained distinct-key flood, live entries=%d, want exactly cap=%d", got, cap)
	}
}

// TestBoundedSeenDetectsDuplicateWithinWindow confirms the dedupe semantics
// markRequest depends on: a repeated request_id inside the TTL window is reported
// as a duplicate, while an empty key is never stored nor flagged.
func TestBoundedSeenDetectsDuplicateWithinWindow(t *testing.T) {
	b := newBoundedSeen(1000, time.Hour)
	if b.Add("rid-1") {
		t.Fatal("first occurrence of rid-1 reported as duplicate")
	}
	if !b.Add("rid-1") {
		t.Fatal("second occurrence of rid-1 NOT reported as duplicate")
	}
	if b.Add("") {
		t.Fatal("empty key reported as duplicate")
	}
	if b.Add("") {
		t.Fatal("empty key stored and then reported as duplicate")
	}
}

// TestBoundedSeenEvictsExpiredEntries confirms TTL eviction frees entries past
// the window, so even within the cap the store does not retain stale keys
// indefinitely (and an expired request_id is dedupe-forgotten).
func TestBoundedSeenEvictsExpiredEntries(t *testing.T) {
	now := time.Unix(0, 0)
	b := newBoundedSeen(1000, time.Minute)
	b.now = func() time.Time { return now }

	if b.Add("rid-a") {
		t.Fatal("first add reported as duplicate")
	}
	if got := b.Len(); got != 1 {
		t.Fatalf("live entries=%d, want 1", got)
	}

	now = now.Add(2 * time.Minute) // past the TTL
	if got := b.Len(); got != 0 {
		t.Fatalf("after TTL, live entries=%d, want 0 (expired entry not swept)", got)
	}
	// The same key after expiry must be treated as fresh, not a duplicate.
	if b.Add("rid-a") {
		t.Fatal("re-add of an expired key reported as duplicate; TTL window not honoured")
	}
}

// TestServerMarkRequestBoundedPreAuth drives markRequest directly (the exact
// pre-auth call site at server.handle) with a flood of distinct request_ids and
// asserts the backing store stays bounded.
func TestServerMarkRequestBoundedPreAuth(t *testing.T) {
	s := NewServer()
	// Shrink the cap so the test is fast but still proves the invariant.
	s.seenRequests = newBoundedSeen(256, time.Hour)
	for i := 0; i < 10000; i++ {
		s.markRequest(fmt.Sprintf("pre-auth-flood-%d", i))
	}
	if got := s.seenRequests.Len(); got > 256 {
		t.Fatalf("pre-auth markRequest flood grew dedupe store to %d entries, exceeds cap 256", got)
	}
}
