package db

import "testing"

func TestWriteBoundaryAtLeast(t *testing.T) {
	order := []WriteBoundaryPhase{PhaseNone, PhaseAuditOnly, PhaseAuditArtifacts, PhaseFull}
	for i, p := range order {
		for j, q := range order {
			if got, want := p.AtLeast(q), i >= j; got != want {
				t.Fatalf("%s.AtLeast(%s) = %v, want %v", p, q, got, want)
			}
		}
	}
	if WriteBoundaryPhase("bogus").Valid() {
		t.Fatal("bogus phase reported valid")
	}
}

// TestResolveWriteBoundary covers the flag/derivation rules: an explicit valid
// flag wins; an empty flag derives audit_only from a v3 audit format and none
// otherwise; an invalid flag errors (fail closed, not silently none).
func TestResolveWriteBoundary(t *testing.T) {
	for _, tc := range []struct {
		flag   string
		audit  string
		want   WriteBoundaryPhase
		wantOK bool
	}{
		{"", "v2", PhaseNone, true},
		{"", "v3", PhaseAuditOnly, true},
		{"audit_artifacts", "v2", PhaseAuditArtifacts, true},
		{"full", "v3", PhaseFull, true},
		{"none", "v3", PhaseNone, true},
		{"  audit_only  ", "v2", PhaseAuditOnly, true},
		{"bogus", "v3", PhaseNone, false},
	} {
		got, err := ResolveWriteBoundary(tc.flag, tc.audit)
		if tc.wantOK && err != nil {
			t.Fatalf("ResolveWriteBoundary(%q,%q) errored: %v", tc.flag, tc.audit, err)
		}
		if !tc.wantOK && err == nil {
			t.Fatalf("ResolveWriteBoundary(%q,%q) should have errored", tc.flag, tc.audit)
		}
		if tc.wantOK && got != tc.want {
			t.Fatalf("ResolveWriteBoundary(%q,%q) = %s, want %s", tc.flag, tc.audit, got, tc.want)
		}
	}
}

// TestRequiredAuthorityCapabilitiesByPhase asserts the capability-parity
// requirement tracks the active phase (RFC 0110 §8.2): P1 requires
// artifact_sd_append, P2 also event_sd_append, and pre-P1 requires nothing.
func TestRequiredAuthorityCapabilitiesByPhase(t *testing.T) {
	t.Cleanup(func() { SetActiveWriteBoundary(PhaseNone) })
	cases := map[WriteBoundaryPhase][]string{
		PhaseNone:           nil,
		PhaseAuditOnly:      nil,
		PhaseAuditArtifacts: {"artifact_sd_append"},
		PhaseFull:           {"artifact_sd_append", "event_sd_append"},
	}
	for phase, want := range cases {
		SetActiveWriteBoundary(phase)
		got := RequiredAuthorityCapabilities()
		if len(got) != len(want) {
			t.Fatalf("phase %s: required = %v, want %v", phase, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("phase %s: required = %v, want %v", phase, got, want)
			}
		}
	}
}

func TestSupportedAuthorityCapabilitiesCoverReassertedStamps(t *testing.T) {
	supported := toSet(SupportedAuthorityCapabilities())
	for capability := range capabilityProtectedTable {
		if !supported[capability] {
			t.Fatalf("SupportedAuthorityCapabilities missing write stamp %q", capability)
		}
	}
	for capability := range readScopeReasserts {
		if !supported[capability] {
			t.Fatalf("SupportedAuthorityCapabilities missing read stamp %q", capability)
		}
	}
}
