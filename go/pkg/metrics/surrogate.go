package metrics

// RFC 0137 Phase D — the salted per-repo surrogate (deliverable #1/#2).
//
// Multi-tenant hardening needs a per-repo dimension on the wire (the consent
// gauge and the Provenance per-repo families) WITHOUT putting a raw repository_id
// — a cross-box-linkable identifier — into a label. The surrogate is the bridge:
// bucket = HMAC-SHA256(daemon-secret, repo_id) mod K, rendered as a small
// integer. The salt is the per-daemon authority secret (RFC 0110), which is never
// exported, so a bucket is stable on-box (an operator can correlate a bucket
// across scrapes and across the consent gauge / per-repo families) but carries no
// meaning off-box and cannot be reversed to a repo_id by a tailnet scraper.
//
// K bounds the per-repo label cardinality independently of how many repositories
// the daemon serves — the series budget's structural cap for the per-repo
// dimension. Two repositories can collide onto one bucket (a birthday collision at
// ~sqrt(K)); that is an ACCEPTED property — the surrogate is deliberately lossy,
// because exposing a reversible/unique repo identifier is the larger harm. A
// collision only ever merges two repos' aggregate counts under one bucket (and, in
// the capability-scoped path, could let a repo-A token also see a colliding repo's
// bucket); it never widens cardinality or leaks an id.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"strconv"
)

// surrogateBuckets is K: the closed number of per-repo surrogate buckets. It is a
// CONSTANT so the per-repo label (`bucket`) is a bounded enum independent of the
// repository count — the per-repo dimension's structural cardinality cap. 256 is
// large enough that collisions stay rare for a local-first daemon's handful of
// registered repositories, yet small enough to keep the per-repo series count
// hard-bounded even if a misconfiguration registered many repos.
const surrogateBuckets = 256

// Surrogate maps a repository_id to its stable salted bucket. It holds the
// per-daemon secret (never exported) and the bucket count K.
type Surrogate struct {
	secret []byte
	k      uint64
}

// NewSurrogate builds a Surrogate over the per-daemon authority secret. An empty
// secret is tolerated (it yields a still-deterministic but unsalted mapping) so a
// daemon booted before the authority schema exists still renders a coherent — if
// not cryptographically unlinkable — per-repo surface rather than failing; the
// production boot path passes the real RFC 0110 authority secret.
func NewSurrogate(secret string) *Surrogate {
	return &Surrogate{secret: []byte(secret), k: surrogateBuckets}
}

// Bucket returns the surrogate bucket for a repository_id as a decimal string in
// [0, K). It is deterministic for a fixed (secret, repo_id), so the same repo maps
// to the same bucket across scrapes and across families. An empty repo_id maps to
// a reserved bucket like any other input (it never panics).
func (s *Surrogate) Bucket(repoID string) string {
	if s == nil {
		return "0"
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(repoID))
	sum := mac.Sum(nil)
	// Fold the first 8 bytes into a uint64 and reduce mod K. K is a power of two
	// today, but mod is used (not a mask) so K can change without a silent skew.
	n := binary.BigEndian.Uint64(sum[:8])
	k := s.k
	if k == 0 {
		k = surrogateBuckets
	}
	return strconv.FormatUint(n%k, 10)
}
