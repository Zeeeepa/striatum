package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestBarrierReadySQLJoinsOnLiveSeal asserts the minted predicate's SHAPE
// (RFC 0135): it JOINs each in-edge's staged contribution against the entity's
// LIVE seal and NEVER counts staged refs per entity. This is the unit-level
// half of the trap-killer guarantee; TestBarrierPredicateHasNoRefCount is the
// static-source half that protects future call sites.
func TestBarrierReadySQLJoinsOnLiveSeal(t *testing.T) {
	// Fan-in instance (RFC 0133): entity=job, seal=attempt.
	sql, err := BarrierReadySQL(BarrierSpec{
		EntityKind:         "job",
		InEdgesTable:       "barrier_in_edges",
		LiveSealTable:      "entity_live_seal",
		StagedContribTable: "staged_contrib",
		SealColumn:         "attempt",
	})
	if err != nil {
		t.Fatalf("BarrierReadySQL returned error: %v", err)
	}

	// The load-bearing JOIN: staged seal compared to the LIVE seal.
	if !strings.Contains(sql, "staged.attempt = live.attempt") {
		t.Fatalf("predicate must JOIN staged seal against the live seal; got:\n%s", sql)
	}
	// The live-seal relation must be an inner JOIN (every declared entity has a
	// live seal); the staged contribution is LEFT JOINed (a missing one fails
	// closed unless the edge is a terminal gap).
	if !strings.Contains(sql, "JOIN entity_live_seal") {
		t.Fatalf("predicate must JOIN the live-seal relation; got:\n%s", sql)
	}
	if !strings.Contains(sql, "LEFT JOIN staged_contrib") {
		t.Fatalf("predicate must LEFT JOIN the staged-contribution relation; got:\n%s", sql)
	}
	// The terminal-gap disjunct lets a quarantined / provably-dead in-edge fire
	// without a live-seal contribution.
	if !strings.Contains(sql, "edge.is_terminal_gap") {
		t.Fatalf("predicate must admit a terminal-acceptable gap; got:\n%s", sql)
	}
	// bool_and over the in-edges: ALL declared edges must be satisfied, never a
	// quorum-by-count over staged refs.
	if !strings.Contains(sql, "bool_and(") {
		t.Fatalf("predicate must aggregate edges with bool_and; got:\n%s", sql)
	}
	// The trap, asserted negatively: the minted predicate must NOT count staged
	// refs per entity, and must NOT order staged contributions by recency.
	lowered := strings.ToLower(sql)
	if regexp.MustCompile(`count\s*\(`).MatchString(lowered) {
		t.Fatalf("predicate must not COUNT staged refs (RFC 0135 trap); got:\n%s", sql)
	}
	if strings.Contains(lowered, "order by") {
		t.Fatalf("predicate must not read latest contribution by recency (ORDER BY); got:\n%s", sql)
	}
	// The barrier id and entity kind are bound parameters, never interpolated.
	if !strings.Contains(sql, "edge.barrier_id = $1") {
		t.Fatalf("predicate must bind the barrier id as $1; got:\n%s", sql)
	}
	if !strings.Contains(sql, "edge.entity_kind = $2") {
		t.Fatalf("predicate must bind the entity kind as $2; got:\n%s", sql)
	}
}

// TestBarrierReadySQLIsSealGeneric proves the one minted predicate serves the
// generation-keyed caller (revision coherence, RFC 0126) by parameterizing over
// the seal column — `review_generation` BECOMES the seal, with no re-keying onto
// attempt. This is the fold that justifies the whole "seal not attempt" framing.
func TestBarrierReadySQLIsSealGeneric(t *testing.T) {
	sql, err := BarrierReadySQL(BarrierSpec{
		EntityKind:         "review_obligation",
		InEdgesTable:       "review_obligations",
		LiveSealTable:      "build_review_generation",
		StagedContribTable: "review_verdicts",
		SealColumn:         "review_generation",
	})
	if err != nil {
		t.Fatalf("BarrierReadySQL returned error: %v", err)
	}
	if !strings.Contains(sql, "staged.review_generation = live.review_generation") {
		t.Fatalf("seal-generic predicate must JOIN on the named seal column; got:\n%s", sql)
	}
	if strings.Contains(sql, "attempt") {
		t.Fatalf("revision instance must not mention attempt (seal := review_generation); got:\n%s", sql)
	}
}

// TestBarrierReadySQLRejectsInjectableSpec proves the only injection fence at
// the predicate boundary holds: a malformed spec (the table/seal names are
// interpolated, not bound) is refused rather than producing an injectable
// fragment, and an empty EntityKind is rejected.
func TestBarrierReadySQLRejectsInjectableSpec(t *testing.T) {
	base := BarrierSpec{
		EntityKind:         "job",
		InEdgesTable:       "barrier_in_edges",
		LiveSealTable:      "entity_live_seal",
		StagedContribTable: "staged_contrib",
		SealColumn:         "attempt",
	}
	if _, err := BarrierReadySQL(base); err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}

	cases := []struct {
		name  string
		spec  BarrierSpec
		field string
	}{
		{"empty entity kind", withEntityKind(base, ""), "EntityKind"},
		{"injectable seal column", withSeal(base, "attempt; DROP TABLE jobs"), "SealColumn"},
		{"quoted table", withInEdges(base, `"barrier_in_edges"`), "InEdgesTable"},
		{"whitespace seal", withSeal(base, "attempt "), "SealColumn"},
		{"leading digit", withSeal(base, "1seal"), "SealColumn"},
		{"empty schema segment", withLiveSeal(base, "schema..table"), "LiveSealTable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BarrierReadySQL(tc.spec); err == nil {
				t.Fatalf("expected %s rejection, got nil error", tc.field)
			}
		})
	}

	// A schema-qualified identifier is still accepted (P1 may put tables in a
	// dedicated schema).
	if _, err := BarrierReadySQL(withInEdges(base, "striatum.barrier_in_edges")); err != nil {
		t.Fatalf("schema-qualified identifier rejected: %v", err)
	}
}

func withEntityKind(s BarrierSpec, v string) BarrierSpec { s.EntityKind = v; return s }
func withSeal(s BarrierSpec, v string) BarrierSpec       { s.SealColumn = v; return s }
func withInEdges(s BarrierSpec, v string) BarrierSpec    { s.InEdgesTable = v; return s }
func withLiveSeal(s BarrierSpec, v string) BarrierSpec   { s.LiveSealTable = v; return s }

// barrierGuardedPackages lists the package source directories (relative to this
// db package) that a barrier readiness shape may legitimately live in — the
// minted predicate here (db), and the four future callers' homes (mutations for
// fan-in/quorum/revision evaluation, reads for the doctor/status projections).
// The guard scans all of them so all four callers are protected at once
// (RFC 0135 — the static guard protects four call sites).
var barrierGuardedPackages = []string{
	".", // pkg/db — where the predicate is minted
	"../mutations",
	"../reads",
}

// barrierShapeMarkers are case-insensitive substrings that mark a string literal
// as a barrier readiness shape. A literal containing one of these AND a
// forbidden ref-COUNT / recency-coherence shape fails the build. Keeping the
// match narrow (it must look like a barrier query, not any COUNT(*)) avoids
// flagging the many legitimate COUNT(*) aggregates elsewhere in the runtime.
var barrierShapeMarkers = []string{
	"barrier_in_edges",
	"staged_contrib",
	"barrier_ready",
	"sealed barrier",
	"sealed expectation barrier",
}

// forbiddenBarrierCountPattern matches a bare COUNT(...) of staged refs — the
// "count staged refs per entity" trap (RFC 0133 alternatives-considered: the
// countdown latch). forbiddenBarrierRecencyPattern matches an ORDER BY ...
// LIMIT 1 / DISTINCT ON recency read used AS the coherence source — the "latest
// contribution by recency" trap. Either inside a barrier-shaped literal fails
// the build.
var (
	forbiddenBarrierCountPattern   = regexp.MustCompile(`(?i)count\s*\(`)
	forbiddenBarrierRecencyPattern = regexp.MustCompile(`(?i)order\s+by[\s\S]*limit\s+1|distinct\s+on`)
)

// TestBarrierPredicateHasNoRefCount is the RFC 0135 static build guard. It
// parses every non-test .go source file in the guarded packages and fails the
// build if any string literal that looks like a barrier readiness query
// contains a bare ref-COUNT(*) shape or a recency-latest coherence read. This
// protects all four future barrier callers from independently re-discovering the
// stale-attempt trap (RFC 0133 synthesis trap #1), generalized to every seal.
//
// It is modeled on the existing static guards
// (git_invocation_guard_test.go, terminal_marker_guard_test.go): parse with
// go/parser, walk the AST, inspect BasicLit string nodes.
func TestBarrierPredicateHasNoRefCount(t *testing.T) {
	for _, pkgDir := range barrierGuardedPackages {
		files, err := filepath.Glob(filepath.Join(pkgDir, "*.go"))
		if err != nil {
			t.Fatalf("glob %s: %v", pkgDir, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			fset := token.NewFileSet()
			parsed, err := parser.ParseFile(fset, file, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", file, err)
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				lit, ok := node.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				if !looksLikeBarrierShape(value) {
					return true
				}
				if forbiddenBarrierCountPattern.MatchString(value) {
					pos := fset.Position(lit.Pos())
					t.Fatalf("%s: barrier readiness shape counts staged refs (COUNT(...)); RFC 0135 forbids a per-entity ref count — JOIN each in-edge's staged contribution on the LIVE seal (staged.seal = live.seal) instead:\n%s", pos, value)
				}
				if forbiddenBarrierRecencyPattern.MatchString(value) {
					pos := fset.Position(lit.Pos())
					t.Fatalf("%s: barrier readiness shape reads the latest contribution by recency (ORDER BY ... LIMIT 1 / DISTINCT ON); RFC 0135 forbids recency as the coherence source — JOIN on the LIVE seal instead:\n%s", pos, value)
				}
				return true
			})
		}
	}
}

func looksLikeBarrierShape(value string) bool {
	lowered := strings.ToLower(value)
	for _, marker := range barrierShapeMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// TestBarrierGuardFiresOnRefCountShape is the both-ways check the task requires:
// it exercises the guard's matchers against a synthetic forbidden shape so the
// guard provably FAILS the build on a COUNT(*)-of-staged-refs barrier (and on a
// recency-latest read), not just passes on the clean current tree. If this ever
// stops detecting the trap, the static guard above is toothless.
func TestBarrierGuardFiresOnRefCountShape(t *testing.T) {
	forbiddenCount := `SELECT COUNT(*) >= edge.declared
  FROM barrier_in_edges edge
  JOIN staged_contrib staged USING (entity_kind, entity_id)
 WHERE edge.barrier_id = $1`
	if !looksLikeBarrierShape(forbiddenCount) {
		t.Fatalf("guard failed to recognize a barrier-shaped literal: %s", forbiddenCount)
	}
	if !forbiddenBarrierCountPattern.MatchString(forbiddenCount) {
		t.Fatalf("guard failed to flag a COUNT(*)-of-staged-refs barrier shape: %s", forbiddenCount)
	}

	forbiddenRecency := `SELECT staged.verdict
  FROM staged_contrib staged
 WHERE staged.barrier_id = $1
 ORDER BY staged.created_at DESC
 LIMIT 1`
	if !looksLikeBarrierShape(forbiddenRecency) {
		t.Fatalf("guard failed to recognize a recency barrier-shaped literal: %s", forbiddenRecency)
	}
	if !forbiddenBarrierRecencyPattern.MatchString(forbiddenRecency) {
		t.Fatalf("guard failed to flag a recency-latest coherence read: %s", forbiddenRecency)
	}

	// The clean minted predicate must NOT trip either matcher (it is barrier-
	// shaped but trap-free).
	clean, err := BarrierReadySQL(BarrierSpec{
		EntityKind:         "job",
		InEdgesTable:       "barrier_in_edges",
		LiveSealTable:      "entity_live_seal",
		StagedContribTable: "staged_contrib",
		SealColumn:         "attempt",
	})
	if err != nil {
		t.Fatalf("BarrierReadySQL error: %v", err)
	}
	if !looksLikeBarrierShape(clean) {
		t.Fatalf("minted predicate is not recognized as a barrier shape: %s", clean)
	}
	if forbiddenBarrierCountPattern.MatchString(clean) {
		t.Fatalf("minted predicate wrongly flagged as a COUNT shape: %s", clean)
	}
	if forbiddenBarrierRecencyPattern.MatchString(clean) {
		t.Fatalf("minted predicate wrongly flagged as a recency shape: %s", clean)
	}
}
