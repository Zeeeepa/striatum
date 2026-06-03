package workflowtemplates

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const CatalogSchemaVersion = "striatum.workflow_templates.v1"

const catalogRelPath = "go/pkg/workflowtemplates/catalog.json"

//go:embed catalog.json
var catalogFS embed.FS

var harnessProfileToolFamilies = map[string]struct{}{
	"generic":     {},
	"codex":       {},
	"claude_code": {},
	"agy":         {},
}

var multiPhaseShapeEntry = map[string]any{
	"template_id":       "multi_phase",
	"kind":              "shape",
	"display_name":      "Multi-phase workflow",
	"summary":           "Run phase-scoped parallel tracks behind explicit synthesis gates.",
	"recommended_for":   []any{"large work split into ordered design, build, review, or release phases"},
	"default_lane_sets": []any{"author_reviewer", "multi_review", "single_agent"},
	"required_options":  []any{"workflow_id", "artifact_root", "phases"},
	"graph_preview": map[string]any{
		"nodes": []any{
			map[string]any{"id": "phase_1_track", "label": "Phase 1 track"},
			map[string]any{"id": "phase_1__synthesis", "label": "Phase 1 synthesis"},
			map[string]any{"id": "phase_2_track", "label": "Phase 2 track"},
			map[string]any{"id": "phase_2__synthesis", "label": "Phase 2 synthesis"},
		},
		"edges": []any{
			map[string]any{"from": "phase_1_track", "to": "phase_1__synthesis"},
			map[string]any{"from": "phase_1__synthesis", "to": "phase_2_track"},
			map[string]any{"from": "phase_2_track", "to": "phase_2__synthesis"},
		},
		"cycles": []any{},
	},
}

// RFC 0106 support tiers. A shape is `supported` only if it has a green RFC 0105
// unattended-reliability fixture (adapterconformance.ReliabilityFixtureShapes);
// every other shape is `experimental` — it exists and may be valuable, but is
// not yet proven to run unattended. The classification below is the single
// source of truth the catalog stamps onto each shape entry, the workflow.lint
// experimental-shape warning consults (SupportTierForShape), and the RFC 0106
// graduation guard test reconciles against the fixture registry so the tier
// cannot lie.
const (
	SupportTierSupported    = "supported"
	SupportTierExperimental = "experimental"
)

// supportedShapes are the shape template_ids with a green RFC 0105 fixture.
// Keep in sync with adapterconformance.ReliabilityFixtureShapes (the graduation
// guard fails if they drift). Per RFC 0106, no shape graduates here without a
// passing fixture, and no NEW shapes are authored until the catalog graduates.
var supportedShapes = map[string]struct{}{
	"minimal":                {},
	"review":                 {},
	"code_change":            {},
	"multi_review_synthesis": {},
}

// SupportTierForShape returns the RFC 0106 support tier for a shape template_id.
// Unknown / ungated shapes are `experimental` (the honest default: unproven
// until a fixture says otherwise).
func SupportTierForShape(templateID string) string {
	if _, ok := supportedShapes[templateID]; ok {
		return SupportTierSupported
	}
	return SupportTierExperimental
}

type Catalog struct {
	Shapes                  []map[string]any
	LaneSets                []map[string]any
	RolePacks               []map[string]any
	AdversaryPacks          []map[string]any
	HarnessProfileFragments []map[string]any
}

// stampSupportTiers sets `support_tier` on every shape entry from the RFC 0106
// classification, so the API (Get/List), the rendered catalog doc, and any
// consumer see the tier without it being duplicated across catalog.json.
func (c *Catalog) stampSupportTiers() {
	for _, entry := range c.Shapes {
		entry["support_tier"] = SupportTierForShape(stringValue(entry, "template_id"))
	}
}

type Error struct {
	Message   string
	FieldPath string
	Hint      string
}

func (e *Error) Error() string {
	return e.Message
}

func Load() (*Catalog, error) {
	if value := os.Getenv("STRIATUM_WORKFLOW_TEMPLATE_CATALOG"); strings.TrimSpace(value) != "" {
		return LoadFile(value)
	}
	body, err := catalogFS.ReadFile("catalog.json")
	if err == nil {
		return LoadBytes(body)
	}
	path, err := FindCatalogPath()
	if err != nil {
		return nil, err
	}
	return LoadFile(path)
}

func LoadFile(path string) (*Catalog, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, &Error{Message: "workflow template catalog could not be loaded"}
	}
	return LoadBytes(body)
}

func LoadBytes(body []byte) (*Catalog, error) {
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, &Error{Message: "workflow template catalog could not be loaded"}
	}
	if root["schema_version"] != CatalogSchemaVersion {
		return nil, &Error{Message: "workflow template catalog has unsupported schema_version"}
	}

	shapes, err := entryList(root, "shapes")
	if err != nil {
		return nil, err
	}
	if err := validateEntries(shapes, "shapes", "shape"); err != nil {
		return nil, err
	}
	laneSets, err := entryList(root, "lane_sets")
	if err != nil {
		return nil, err
	}
	if err := validateEntries(laneSets, "lane_sets", "lane_set"); err != nil {
		return nil, err
	}
	rolePacks, err := optionalEntryList(root, "role_packs")
	if err != nil {
		return nil, err
	}
	if err := validateEntries(rolePacks, "role_packs", "role_pack"); err != nil {
		return nil, err
	}
	adversaryPacks, err := optionalEntryList(root, "adversary_packs")
	if err != nil {
		return nil, err
	}
	if err := validateEntries(adversaryPacks, "adversary_packs", "adversary_pack"); err != nil {
		return nil, err
	}
	fragments, err := optionalEntryList(root, "harness_profile_fragments")
	if err != nil {
		return nil, err
	}
	if err := validateHarnessFragments(fragments); err != nil {
		return nil, err
	}

	catalog := &Catalog{
		Shapes:                  shapes,
		LaneSets:                laneSets,
		RolePacks:               rolePacks,
		AdversaryPacks:          adversaryPacks,
		HarnessProfileFragments: fragments,
	}
	catalog.ensureMultiPhaseShape()
	catalog.stampSupportTiers()
	return catalog, nil
}

func FindCatalogPath() (string, error) {
	if value := os.Getenv("STRIATUM_WORKFLOW_TEMPLATE_CATALOG"); strings.TrimSpace(value) != "" {
		return value, nil
	}

	starts := []string{}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		starts = append(starts, filepath.Dir(file))
	}

	seen := map[string]struct{}{}
	for _, start := range starts {
		path, ok := findCatalogAbove(start, seen)
		if ok {
			return path, nil
		}
	}
	return "", &Error{Message: "workflow template catalog could not be loaded"}
}

func (c *Catalog) List(kind string) ([]map[string]any, error) {
	if kind != "" && kind != "shape" && kind != "lane_set" && kind != "role_pack" && kind != "adversary_pack" {
		return nil, &Error{
			Message:   "template kind must be shape, lane_set, role_pack, or adversary_pack",
			FieldPath: "kind",
			Hint:      "Use `workflow templates list --kind shape`, `--kind lane_set`, `--kind role_pack`, or `--kind adversary_pack`.",
		}
	}
	entries := []map[string]any{}
	if kind == "" || kind == "adversary_pack" {
		entries = append(entries, cloneEntries(c.AdversaryPacks)...)
	}
	if kind == "" || kind == "shape" {
		entries = append(entries, cloneEntries(c.Shapes)...)
	}
	if kind == "" || kind == "lane_set" {
		entries = append(entries, cloneEntries(c.LaneSets)...)
	}
	if kind == "" || kind == "role_pack" {
		entries = append(entries, cloneEntries(c.RolePacks)...)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		leftKind := stringValue(entries[i], "kind")
		rightKind := stringValue(entries[j], "kind")
		if leftKind != rightKind {
			return leftKind < rightKind
		}
		return stringValue(entries[i], "template_id") < stringValue(entries[j], "template_id")
	})
	return entries, nil
}

func (c *Catalog) Get(templateID string) (map[string]any, error) {
	entries, err := c.List("")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if stringValue(entry, "template_id") == templateID {
			return entry, nil
		}
	}
	return nil, &Error{
		Message:   fmt.Sprintf("unknown workflow template: %s", templateID),
		FieldPath: "template_id",
		Hint:      "Run `striatum workflow templates list` to see available templates.",
	}
}

func entryList(root map[string]any, key string) ([]map[string]any, error) {
	raw, ok := root[key].([]any)
	if !ok {
		return nil, &Error{Message: fmt.Sprintf("workflow template catalog %s must be a list", key)}
	}
	entries := make([]map[string]any, 0, len(raw))
	for index, value := range raw {
		entry, ok := value.(map[string]any)
		if !ok {
			return nil, &Error{Message: fmt.Sprintf("workflow template catalog %s[%d] must be an object", key, index)}
		}
		entries = append(entries, cloneMap(entry))
	}
	return entries, nil
}

func optionalEntryList(root map[string]any, key string) ([]map[string]any, error) {
	if _, ok := root[key]; !ok {
		return []map[string]any{}, nil
	}
	return entryList(root, key)
}

func validateEntries(entries []map[string]any, key string, expectedKind string) error {
	for index, entry := range entries {
		templateID := stringValue(entry, "template_id")
		if templateID == "" {
			return &Error{Message: fmt.Sprintf("workflow template catalog %s[%d] missing template_id", key, index)}
		}
		if stringValue(entry, "kind") != expectedKind {
			return &Error{Message: fmt.Sprintf("workflow template catalog %s[%d] has wrong kind", key, index)}
		}
	}
	return nil
}

func validateHarnessFragments(fragments []map[string]any) error {
	seen := map[string]struct{}{}
	for index, fragment := range fragments {
		profileID := stringValue(fragment, "profile_id")
		if profileID == "" {
			return &Error{Message: fmt.Sprintf("harness_profile_fragments[%d] missing profile_id", index)}
		}
		if _, ok := seen[profileID]; ok {
			return &Error{Message: fmt.Sprintf("harness_profile_fragments[%d] duplicate profile_id %q", index, profileID)}
		}
		seen[profileID] = struct{}{}
		if stringValue(fragment, "kind") != "harness_profile_fragment" {
			return &Error{Message: fmt.Sprintf("harness_profile_fragments[%d] has wrong kind", index)}
		}
		family := stringValue(fragment, "tool_family")
		if _, ok := harnessProfileToolFamilies[family]; !ok {
			return &Error{Message: fmt.Sprintf("harness_profile_fragments[%d] has unknown tool_family %q", index, family)}
		}
		if strings.TrimSpace(stringValue(fragment, "native_delegation_instruction")) == "" {
			return &Error{Message: fmt.Sprintf("harness_profile_fragments[%d] missing non-empty native_delegation_instruction", index)}
		}
	}
	return nil
}

func (c *Catalog) ensureMultiPhaseShape() {
	for _, entry := range c.Shapes {
		if stringValue(entry, "template_id") == "multi_phase" {
			return
		}
	}
	c.Shapes = append(c.Shapes, cloneMap(multiPhaseShapeEntry))
}

func findCatalogAbove(start string, seen map[string]struct{}) (string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if _, ok := seen[dir]; ok {
			return "", false
		}
		seen[dir] = struct{}{}
		candidate := filepath.Join(dir, catalogRelPath)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func cloneEntries(entries []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		out = append(out, cloneMap(entry))
	}
	return out
}

func cloneMap(entry map[string]any) map[string]any {
	body, err := json.Marshal(entry)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func stringValue(entry map[string]any, key string) string {
	value, _ := entry[key].(string)
	return value
}
