package db

import "fmt"

// RFC 0135 (D216) — the sealed expectation barrier primitive.
//
// Four runtime call sites each evaluate "is the expected set of contributions
// present and current?" — fan-in (RFC 0133), panel quorum (RFC 0132), revision
// coherence (RFC 0095/0126), and run.integrate (RFC 0108). Each independently
// re-discovered the same trap: counting contributions by a key that CHURNS under
// recovery (raw `attempt`, or `job_id`) silently admits a stale contribution
// from a superseded round — "the original stranding bug reborn behind a
// successful join" (RFC 0133 synthesis trap #1).
//
// The primitive keys on (stable entity id, monotonic SEAL counter): a seal is a
// non-decreasing integer per entity that advances on every event that
// invalidates a prior round's contribution (a requeue, a revision, a re-open).
// The readiness predicate JOINs each declared in-edge's staged contribution
// against the entity's LIVE seal (`staged.seal = live.seal`), so a contribution
// from a superseded seal is STRUCTURALLY absent from the count, not filtered out
// after the fact. There is no COUNT(*) of staged refs per entity and no "latest
// contribution by recency" read; either of those reintroduces the trap.
//
// This file mints that predicate ONCE, entity/seal-generic (parameterized over
// the entity table and the seal column), mirroring the
// ExpiredLeaseStillStalePredicate named-fragment precedent in
// recovery_predicates.go so a future copy cannot drift. P0 (#344) establishes
// the canonical SHAPE and the build guard (TestBarrierPredicateHasNoRefCount)
// that protects all four future callers at once; the first LIVE caller is P1
// (fan-in, entity=job, seal=attempt). No live table exists yet — P0 is a
// no-DDL slice (D215) — so the "named SQL function" here is the Go builder
// BarrierReadySQL, not a Postgres CREATE FUNCTION (which would be a migration).

// BarrierIsTerminalGapColumn is the canonical in-edge column name that marks an
// in-edge as a terminal-acceptable gap (a quarantined sibling, or a
// `structurally_unrecoverable` review seat per D214b) — an in-edge that need not
// carry a live-seal staged contribution for the barrier to fire. It is the
// `edge.is_terminal_gap` disjunct of the readiness predicate.
const BarrierIsTerminalGapColumn = "is_terminal_gap"

// BarrierSpec parameterizes the (entity, seal)-generic barrier readiness
// predicate over the concrete entity table and seal column a caller uses, so the
// one minted predicate serves all four callers without re-minting:
//
//   - Fan-in (RFC 0133):       Entity="job",               Seal="attempt"
//   - Quorum  (RFC 0132):      Entity="review_seat",       Seal="attempt"            (keyed by stable workflow_job_id)
//   - Revision (RFC 0126):     Entity="review_obligation", Seal="review_generation"
//   - Run     (RFC 0108):      Entity="run",               Seal=run integration epoch
//
// The barrier is keyed on the SEAL, never on raw `attempt`, which is exactly why
// the generation-keyed caller (revision coherence) folds in without regressing
// off `review_generation` (RFC 0126/D194): `review_generation` BECOMES the seal.
type BarrierSpec struct {
	// EntityKind is the entity discriminator value (e.g. "job", "review_seat",
	// "review_obligation", "run") the in-edge / live-seal / staged-contribution
	// rows are scoped by. It is bound as a query parameter, never interpolated.
	EntityKind string

	// InEdgesTable is the relation declaring the barrier's in-edges, one row per
	// expected contribution, carrying the BarrierIsTerminalGapColumn flag. Aliased
	// `edge` in the predicate.
	InEdgesTable string

	// LiveSealTable is the relation projecting each entity's CURRENT (live) seal,
	// joined on the stable entity id. Aliased `live`. Its seal column is read
	// as `live.<SealColumn>`. This is the load-bearing relation: it is the entity's
	// LIVE seal, NOT a per-edge count.
	LiveSealTable string

	// StagedContribTable is the relation holding staged contributions, each
	// embedding the seal it was sealed at. LEFT JOINed as `staged` so a missing
	// contribution leaves `staged.<SealColumn>` NULL and the edge fails to fire
	// (fail-closed) unless it is a terminal gap.
	StagedContribTable string

	// SealColumn is the monotonic seal column name shared by the live-seal and
	// staged-contribution relations (e.g. "attempt", "review_generation"). It is
	// validated as a bare SQL identifier before interpolation.
	SealColumn string
}

// BarrierReadySQL is the generalized JOIN barrier predicate, minted ONCE and
// parameterized by spec (RFC 0135 — "The generalized JOIN barrier predicate and
// the trap-killer property"). It returns a single-row, single-column boolean
// query (`bool_and(...)`) that is TRUE iff every declared in-edge either is a
// terminal-acceptable gap OR has a staged contribution whose embedded seal
// EQUALS the entity's live seal. The barrier's id is bound as `$1` and the
// entity-kind discriminator as `$2`.
//
// The query shape is the contract:
//
//	SELECT bool_and(
//	         edge.is_terminal_gap            -- terminal-acceptable gap (quarantine / provably-dead seat)
//	      OR staged.<seal> = live.<seal>     -- the load-bearing JOIN on the LIVE seal
//	       )
//	  FROM <in_edges>      edge
//	  JOIN <live_seal>      live   USING (entity_kind, entity_id)
//	  LEFT JOIN <staged>   staged USING (entity_kind, entity_id)
//	 WHERE edge.barrier_id = $1
//	   AND edge.entity_kind = $2;
//
// It NEVER emits COUNT(*) of staged refs per entity and NEVER reads "the latest
// contribution by recency" — a superseded-seal contribution is structurally
// invisible because the JOIN demands `staged.seal = live.seal`. The static guard
// TestBarrierPredicateHasNoRefCount fails the build on any barrier shape that
// reintroduces a bare ref-COUNT(*) or a recency-latest coherence source.
//
// SealColumn and the table names are caller-controlled identifiers, not user
// input; SealColumn is nonetheless validated as a bare identifier so a malformed
// spec fails loudly rather than producing an injectable fragment.
func BarrierReadySQL(spec BarrierSpec) (string, error) {
	if err := validateBarrierSpec(spec); err != nil {
		return "", err
	}
	return fmt.Sprintf(`SELECT bool_and(
         edge.%[1]s
      OR staged.%[2]s = live.%[2]s
       )
  FROM %[3]s      edge
  JOIN %[4]s      live   USING (entity_kind, entity_id)
  LEFT JOIN %[5]s   staged USING (entity_kind, entity_id)
 WHERE edge.barrier_id = $1
   AND edge.entity_kind = $2`,
		BarrierIsTerminalGapColumn, // %[1]s — the terminal-gap disjunct
		spec.SealColumn,            // %[2]s — the load-bearing JOIN on the LIVE seal
		spec.InEdgesTable,          // %[3]s
		spec.LiveSealTable,         // %[4]s
		spec.StagedContribTable,    // %[5]s
	), nil
}

// validateBarrierSpec rejects an empty or malformed spec so a caller cannot mint
// an injectable fragment. SealColumn and the table names must be bare SQL
// identifiers (lowercase letters, digits, and underscore, not starting with a
// digit) — they are interpolated, never bound, so this is the only injection
// fence at the predicate boundary.
func validateBarrierSpec(spec BarrierSpec) error {
	if spec.EntityKind == "" {
		return fmt.Errorf("barrier spec: EntityKind is required")
	}
	for name, value := range map[string]string{
		"InEdgesTable":       spec.InEdgesTable,
		"LiveSealTable":      spec.LiveSealTable,
		"StagedContribTable": spec.StagedContribTable,
		"SealColumn":         spec.SealColumn,
	} {
		if !isBareSQLIdentifier(value) {
			return fmt.Errorf("barrier spec: %s %q is not a bare SQL identifier", name, value)
		}
	}
	return nil
}

// isBareSQLIdentifier reports whether s is a non-empty, optionally
// schema-qualified bare SQL identifier (each segment: an ASCII letter or
// underscore start, then letters/digits/underscores). It deliberately rejects
// quoting, whitespace, and punctuation so the interpolated table/column names in
// BarrierReadySQL cannot carry an injection.
func isBareSQLIdentifier(s string) bool {
	if s == "" {
		return false
	}
	segmentStart := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if segmentStart {
				return false // empty segment (".x", "x.", "x..y")
			}
			segmentStart = true
			continue
		}
		isLetterOrUnderscore := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		if segmentStart {
			if !isLetterOrUnderscore {
				return false
			}
			segmentStart = false
			continue
		}
		if !isLetterOrUnderscore && !isDigit {
			return false
		}
	}
	return !segmentStart // a trailing '.' leaves segmentStart true → reject
}
