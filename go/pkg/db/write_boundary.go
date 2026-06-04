package db

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// WriteBoundaryPhase is the RFC 0110 §7 phased write-closure phase. It is an
// operator-committed declaration (the --pg-write-boundary flag) kept in lockstep
// with the applied owner bundles: the daemon routes a protected surface's writes
// through that surface's owner-owned SECURITY DEFINER append function once the
// phase has reached the surface, and the active phase is the doctor
// pg_write_boundary posture string — so the operator-facing claim and the
// DB-enforced reality cannot drift (§2, C-PHASED-WRITE-CLOSURE).
type WriteBoundaryPhase string

const (
	// PhaseNone: no surface is SD-gated (pre-RFC-0110 / pre-cutover routing).
	PhaseNone WriteBoundaryPhase = "none"
	// PhaseAuditOnly: audit_log is SD-gated (P0).
	PhaseAuditOnly WriteBoundaryPhase = "audit_only"
	// PhaseAuditArtifacts: audit_log + artifacts are SD-gated (P1).
	PhaseAuditArtifacts WriteBoundaryPhase = "audit_artifacts"
	// PhaseFull: audit_log + artifacts + events are SD-gated (P2); only this
	// phase may claim "the daemon's durable write paths are DB-enforced".
	PhaseFull WriteBoundaryPhase = "full"
)

var writeBoundaryRank = map[WriteBoundaryPhase]int{
	PhaseNone:           0,
	PhaseAuditOnly:      1,
	PhaseAuditArtifacts: 2,
	PhaseFull:           3,
}

// AtLeast reports whether p is at or beyond other in the phase ordering.
func (p WriteBoundaryPhase) AtLeast(other WriteBoundaryPhase) bool {
	return writeBoundaryRank[p] >= writeBoundaryRank[other]
}

// Valid reports whether p is one of the four defined phases.
func (p WriteBoundaryPhase) Valid() bool {
	_, ok := writeBoundaryRank[p]
	return ok
}

var activeWriteBoundary atomic.Pointer[WriteBoundaryPhase]

// SetActiveWriteBoundary installs the process write-boundary phase (once, at
// bootstrap). An invalid phase is normalized to PhaseNone.
func SetActiveWriteBoundary(phase WriteBoundaryPhase) {
	if !phase.Valid() {
		phase = PhaseNone
	}
	activeWriteBoundary.Store(&phase)
}

// ActiveWriteBoundary returns the process write-boundary phase (PhaseNone until
// bootstrap sets it).
func ActiveWriteBoundary() WriteBoundaryPhase {
	if p := activeWriteBoundary.Load(); p != nil {
		return *p
	}
	return PhaseNone
}

// ResolveWriteBoundary computes the effective write-boundary phase from the
// operator flag and the active audit hash format. An explicit, valid flag wins;
// otherwise the phase is derived so a v3 audit cutover (audit_log SD-gated)
// reports at least audit_only and a pre-cutover daemon reports none. This keeps
// the doctor posture honest without forcing every existing v3 deployment to set
// the new flag on upgrade.
func ResolveWriteBoundary(flagValue, auditFormat string) (WriteBoundaryPhase, error) {
	if v := WriteBoundaryPhase(strings.TrimSpace(flagValue)); v != "" {
		if !v.Valid() {
			return PhaseNone, fmt.Errorf("invalid --pg-write-boundary %q (want none|audit_only|audit_artifacts|full)", flagValue)
		}
		return v, nil
	}
	if auditFormat == AuditHashFormatV3 {
		return PhaseAuditOnly, nil
	}
	return PhaseNone, nil
}
