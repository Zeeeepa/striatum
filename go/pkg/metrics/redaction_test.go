package metrics

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// goldenPath is the committed byte-for-byte exposition the redaction test pins.
const goldenPath = "testdata/metrics_golden.txt"

// sentinelBuiltAt / sentinelNow fix the snapshot age so the rendered body is
// deterministic (age == 12.000000) and the golden stays byte-stable.
var (
	sentinelBuiltAt = time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	sentinelNow     = sentinelBuiltAt.Add(12 * time.Second)
)

// sentinelLiterals are distinctive values planted in the snapshot input. None
// may appear anywhere in the rendered /metrics body.
var sentinelLiterals = []string{
	"/home/halbritt/git/secret-target-repo",
	"feature/leak-sentinel-branch",
	"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	"--dangerous-prompt=LEAK_SENTINEL_ARGV",
	"author: leaker-model-007",
	"/var/lib/striatum/exotic-leak-path",
	"LEAK_SENTINEL",
	"leaker-model",
}

// forbiddenShapes catch a leaked *value* under an already-allowed label name —
// the case the golden hash alone cannot see (it only catches changed label
// names). Each shape mirrors a class the exporter must never emit: a 40-hex git
// sha, a filesystem path, a branch/path slash form, an argv `--flag=value`
// fragment, and an `author:` byline.
var forbiddenShapes = []*regexp.Regexp{
	regexp.MustCompile(`[0-9a-f]{40}`),                   // git object sha
	regexp.MustCompile(`(?:/[A-Za-z0-9._-]+){2,}`),       // filesystem path
	regexp.MustCompile(`[A-Za-z0-9_]+/[A-Za-z0-9._/-]+`), // branch / path slash shape
	regexp.MustCompile(`--[A-Za-z][A-Za-z0-9-]*=`),       // argv flag=value
	regexp.MustCompile(`\bauthor:`),                      // role byline
}

// sentinelRuns plants every sensitive provenance class onto run observations
// whose only wire-eligible field is State. Aggregate counts: running=2,
// blocked=1, completed=4, needs_operator=1, exotic->other=1; stranded=3.
func sentinelRuns() []RunObservation {
	return []RunObservation{
		{State: "running"},
		{State: "running", RepoPath: "/home/halbritt/git/secret-target-repo", Branch: "feature/leak-sentinel-branch"},
		{State: "blocked", HeadSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", PromptFragment: "--dangerous-prompt=LEAK_SENTINEL_ARGV"},
		{State: "completed"},
		{State: "completed"},
		{State: "completed"},
		{State: "completed", AuthorByline: "author: leaker-model-007"},
		{State: "needs_operator"},
		{State: "exotic_unmapped_state", RepoPath: "/var/lib/striatum/exotic-leak-path"},
	}
}

func renderSentinelSnapshot(t *testing.T) []byte {
	t.Helper()
	snap := BuildSnapshot(sentinelBuiltAt, sentinelRuns(), 3)
	var buf bytes.Buffer
	if err := snap.WriteText(&buf, sentinelNow); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	return buf.Bytes()
}

// TestMetricsRedactionGoldenAndForbiddenContent is the exfiltration contract and
// the load-bearing backstop: it pins the exposition byte-for-byte against a
// committed golden AND independently asserts the body carries no sensitive
// value, even though the snapshot input was seeded with every sensitive class.
// Both must pass. It runs under `go test ./...` / `make check` — not as a
// manual-only check.
func TestMetricsRedactionGoldenAndForbiddenContent(t *testing.T) {
	body := renderSentinelSnapshot(t)

	// Regenerate the golden with STRIATUM_UPDATE_GOLDEN=1 when the intended
	// label *names* change; the diff is then a deliberate, reviewed edit.
	if os.Getenv("STRIATUM_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, body, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated golden %s", goldenPath)
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", goldenPath, err)
	}
	if !bytes.Equal(body, want) {
		t.Fatalf("rendered /metrics body does not match golden %s.\n--- got ---\n%s\n--- want ---\n%s",
			goldenPath, body, want)
	}

	text := string(body)
	for _, lit := range sentinelLiterals {
		if strings.Contains(text, lit) {
			t.Errorf("forbidden sentinel %q leaked into /metrics body", lit)
		}
	}
	for _, re := range forbiddenShapes {
		if loc := re.FindString(text); loc != "" {
			t.Errorf("forbidden-content shape %q matched %q in /metrics body", re.String(), loc)
		}
	}
}
