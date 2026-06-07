package db

// RecoveryHandledLeaseReleaseReasons enumerates the lease release_reason values
// that mark an expired lease as ALREADY handled by recovery (a transfer or a
// requeue moved its job to a fresh lease/attempt). Such a lease is historical
// provenance, not an actionable stale condition — the job has since moved on —
// so every recovery/stale-lease scan that joins on `state = 'expired'` must
// exclude it, or it re-reports the same already-released lease (#179) and, on
// the auto-publish path, re-credits the job from the prior attempt's on-disk
// artifact (#203).
//
// expireLeases stamps a genuinely stale lease state='expired',
// release_reason='expired', so released_at alone cannot discriminate; the
// transfer/requeue release reasons are what mark a lease as already handled.
var RecoveryHandledLeaseReleaseReasons = []string{
	"recovery_transfer",
	"operator_transfer",
	"recovery_requeue",
}

// ExpiredLeaseStillStalePredicate is the SQL boolean fragment that, given a
// lease alias `l` already constrained to `l.state = 'expired'`, is true only
// when the lease still represents an UNRESOLVED stale condition (it was NOT
// released by a recovery transfer/requeue). It is NULL-safe: a lease with a
// NULL release_reason (legacy/ordinary expiry) is retained.
//
// Three call sites share this fragment (HandleRecoveryAuto's auto-publish join,
// HandleRecoveryStaleLeases, and the dashboard stale-lease projection) so a
// fourth copy cannot drift out of sync (#179, #203). The alias is fixed to `l`
// because every call site already names its lease join `l`.
const ExpiredLeaseStillStalePredicate = `(l.release_reason IS NULL
	 OR l.release_reason NOT IN ('recovery_transfer', 'operator_transfer', 'recovery_requeue'))`
