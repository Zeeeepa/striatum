package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
)

func TestReadmeFrontDoorStatusStaysCurrent(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	readmeRaw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	readme := string(readmeRaw)
	versionRaw, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	version := "v" + strings.TrimPrefix(strings.TrimSpace(string(versionRaw)), "v")
	runtimeSchema := fmt.Sprintf("runtime schema %d", db.LatestDaemonDBVersion)
	ownerBundle := fmt.Sprintf("owner bundle %04d", db.LatestOwnerBundleVersion)

	for _, want := range []string{version, runtimeSchema, ownerBundle} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README is missing current front-door status %q", want)
		}
	}
	for _, stale := range []string{"schema 22", "v2.33.0 - latest release", "v2.33.0 — latest release"} {
		if strings.Contains(readme, stale) {
			t.Fatalf("README still contains stale status %q", stale)
		}
	}

	activeWork := readmeProjectStatusRow(t, readme, "Active work")
	for _, want := range []string{
		"striatum operator bootstrap --markdown",
		"docs/operator/BRIEF.md",
		"docs/operator/rfc-roadmap.md",
		"open GitHub issues",
	} {
		if !strings.Contains(activeWork, want) {
			t.Fatalf("README Active work row missing %q: %s", want, activeWork)
		}
	}
	if regexp.MustCompile(`#\d+`).MatchString(activeWork) {
		t.Fatalf("README Active work row must not pin volatile issue ids: %s", activeWork)
	}
}

func readmeProjectStatusRow(t *testing.T, readme string, label string) string {
	t.Helper()
	for _, line := range strings.Split(readme, "\n") {
		if strings.HasPrefix(line, "| "+label+" |") {
			return line
		}
	}
	t.Fatalf("README Project Status row %q not found", label)
	return ""
}
