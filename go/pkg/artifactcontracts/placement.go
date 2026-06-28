package artifactcontracts

import "strings"

const (
	PlacementBlobExhaust        = "blob_exhaust"
	PlacementGitPublication     = "git_publication"
	PlacementGitPointerManifest = "git_pointer_manifest"

	BlobRequiredPosture = "blob_required"
)

var allowedPlacements = map[string]bool{
	PlacementBlobExhaust:        true,
	PlacementGitPublication:     true,
	PlacementGitPointerManifest: true,
}

var legacyBlobExhaustKinds = map[string]bool{
	"finding":                      true,
	"findings_ledger":              true,
	"synthesis":                    true,
	"support_ledger":               true,
	"action_item_ledger":           true,
	"collaboration_ledger":         true,
	"harness_improvement_proposal": true,
	"progress_note":                true,
}

func AllowedPlacementSet() map[string]bool {
	out := map[string]bool{}
	for key, value := range allowedPlacements {
		out[key] = value
	}
	return out
}

func AllowedPlacementList() []string {
	return []string{PlacementBlobExhaust, PlacementGitPublication, PlacementGitPointerManifest}
}

func IsAllowedPlacement(placement string) bool {
	return allowedPlacements[strings.TrimSpace(placement)]
}

func LegacyBlobExhaustKindSet() map[string]bool {
	out := map[string]bool{}
	for key, value := range legacyBlobExhaustKinds {
		out[key] = value
	}
	return out
}

func DefaultPlacementForKind(kind string) string {
	if legacyBlobExhaustKinds[strings.TrimSpace(kind)] {
		return PlacementBlobExhaust
	}
	return PlacementGitPublication
}

func ResolvePlacement(kind string, placement any) string {
	text, _ := placement.(string)
	text = strings.TrimSpace(text)
	if allowedPlacements[text] {
		return text
	}
	return DefaultPlacementForKind(kind)
}

func PlacementUsesBlob(placement string) bool {
	return strings.TrimSpace(placement) == PlacementBlobExhaust
}

func PlacementUsesGitAnchor(placement string) bool {
	switch strings.TrimSpace(placement) {
	case PlacementGitPublication, PlacementGitPointerManifest:
		return true
	default:
		return false
	}
}

func BlobRequiredPostureDeclared(value any) bool {
	item, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if boolish(item["blob_required"]) {
		return true
	}
	for _, key := range []string{"artifact_placement_posture", "artifact_blob_posture", "blob_posture"} {
		if blobRequiredToken(item[key]) {
			return true
		}
	}
	return BlobRequiredPostureDeclared(item["options"])
}

func boolish(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func blobRequiredToken(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(text)) {
	case BlobRequiredPosture, "required":
		return true
	default:
		return false
	}
}
