package rpc

import (
	"sync"
	"time"
)

// dedupeDefaults bound the pre-auth request-id and handshake dedupe stores so a
// pre-auth caller (markRequest runs in handle() BEFORE Authorizer.Authorize)
// cannot grow daemon memory without bound. The cap is a hard ceiling on live
// entries; the TTL evicts entries older than the duplicate-detection window the
// daemon actually needs (a request_id replay is only meaningful while a client
// could still be retrying). Both are generous relative to real traffic yet small
// enough that an unauthenticated flood is O(cap), not O(requests).
const (
	defaultDedupeMaxEntries = 50000
	defaultDedupeTTL        = 10 * time.Minute
)

// boundedSeen is a size-capped, TTL-evicting set of opaque keys. It replaces the
// previously unbounded map[string]struct{} dedupe stores (server.seenRequests /
// server.handshakeSeen). Add reports whether the key was already present
// (post-eviction), which is exactly the duplicate signal the caller needs.
//
// Eviction strategy: lazy TTL sweep on every Add plus a hard size cap. When the
// cap is hit after a sweep, the oldest entries (by insertion time) are dropped
// until the store is back under the cap. This keeps live memory bounded by
// maxEntries regardless of how many distinct keys a (possibly pre-auth) caller
// presents.
type boundedSeen struct {
	mu         sync.Mutex
	maxEntries int
	ttl        time.Duration
	now        func() time.Time
	entries    map[string]time.Time
}

func newBoundedSeen(maxEntries int, ttl time.Duration) *boundedSeen {
	if maxEntries <= 0 {
		maxEntries = defaultDedupeMaxEntries
	}
	if ttl <= 0 {
		ttl = defaultDedupeTTL
	}
	return &boundedSeen{
		maxEntries: maxEntries,
		ttl:        ttl,
		now:        time.Now,
		entries:    make(map[string]time.Time),
	}
}

// Add records key and returns true if it was already present within the TTL
// window. An empty key is never stored and never reported as a duplicate (the
// daemon assigns no dedupe meaning to a blank request_id), matching the prior
// markRequest behaviour.
func (b *boundedSeen) Add(key string) bool {
	if key == "" {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	b.sweepLocked(now)
	if _, ok := b.entries[key]; ok {
		return true
	}
	b.entries[key] = now
	b.enforceCapLocked()
	return false
}

// Contains reports whether key is present within the TTL window without
// recording it. Used by handshake lookups (hasHandshake).
func (b *boundedSeen) Contains(key string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sweepLocked(b.now())
	_, ok := b.entries[key]
	return ok
}

// Len returns the live entry count after a TTL sweep. Test/diagnostic only.
func (b *boundedSeen) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sweepLocked(b.now())
	return len(b.entries)
}

func (b *boundedSeen) sweepLocked(now time.Time) {
	cutoff := now.Add(-b.ttl)
	for key, at := range b.entries {
		if at.Before(cutoff) {
			delete(b.entries, key)
		}
	}
}

// enforceCapLocked drops the oldest entries until the store is at or below the
// hard cap. It runs after a TTL sweep, so it only fires under sustained pressure
// (more than maxEntries distinct keys inside the TTL window) — the exact pre-auth
// flood the cap exists to bound.
func (b *boundedSeen) enforceCapLocked() {
	for len(b.entries) > b.maxEntries {
		var oldestKey string
		var oldestAt time.Time
		first := true
		for key, at := range b.entries {
			if first || at.Before(oldestAt) {
				oldestKey = key
				oldestAt = at
				first = false
			}
		}
		if first {
			return
		}
		delete(b.entries, oldestKey)
	}
}
