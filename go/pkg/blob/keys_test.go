package blob

import (
	"strings"
	"testing"
)

func TestValidateBucketName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"ok-minimal", "abc", false},
		{"ok-with-hyphens", "striatum-repo-abc123", false},
		{"ok-max-length", strings.Repeat("a", 63), false},
		{"too-short", "ab", true},
		{"too-long", strings.Repeat("a", 64), true},
		{"leading-hyphen", "-striatum", true},
		{"trailing-hyphen", "striatum-", true},
		{"double-hyphen", "stria--tum", true},
		{"uppercase", "Striatum", true},
		{"underscore", "stria_tum", true},
		{"dot", "stria.tum", true},
		{"empty", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateBucketName(c.input)
			if c.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", c.input)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected nil error for %q, got %v", c.input, err)
			}
		})
	}
}

func TestDefaultBucketName(t *testing.T) {
	cases := []struct {
		name         string
		prefix       string
		repositoryID string
		want         string
	}{
		{"empty-prefix-uses-default", "", "repo_abc123", "striatum-repo-abc123"},
		{"explicit-prefix", "team-", "repo_abc123", "team-repo-abc123"},
		{"already-lowercase-clean", "striatum-", "myrepo", "striatum-myrepo"},
		{"uppercase-input", "striatum-", "MyRepo", "striatum-myrepo"},
		{"dots-become-hyphens", "striatum-", "my.repo.name", "striatum-my-repo-name"},
		{"collapses-double-hyphens", "striatum-", "my..repo", "striatum-my-repo"},
		{"strips-trailing-hyphens", "striatum-", "myrepo___", "striatum-myrepo"},
		{"clips-to-63-chars", "striatum-", strings.Repeat("a", 100), "striatum-" + strings.Repeat("a", 54)},
		{"empty-id-becomes-repo", "striatum-", "", "striatum-repo"},
		{"only-invalid-becomes-repo", "striatum-", "___...", "striatum-repo"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DefaultBucketName(c.prefix, c.repositoryID)
			if got != c.want {
				t.Fatalf("DefaultBucketName(%q, %q) = %q, want %q", c.prefix, c.repositoryID, got, c.want)
			}
			if err := ValidateBucketName(got); err != nil {
				t.Fatalf("DefaultBucketName produced invalid name %q: %v", got, err)
			}
		})
	}
}

func TestArtifactKey(t *testing.T) {
	got := ArtifactKey("run_abc", "job_xyz", "finding-1.md")
	want := "runs/run_abc/jobs/job_xyz/artifacts/finding-1.md"
	if got != want {
		t.Fatalf("ArtifactKey = %q, want %q", got, want)
	}
}

func TestArtifactKeyPanicsOnEmpty(t *testing.T) {
	cases := []struct {
		name        string
		run, job, n string
	}{
		{"empty-run", "", "j", "n"},
		{"empty-job", "r", "", "n"},
		{"empty-name", "r", "j", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for empty input")
				}
			}()
			_ = ArtifactKey(c.run, c.job, c.n)
		})
	}
}

func TestHistoricalDogfoodKey(t *testing.T) {
	cases := []struct {
		name      string
		dogfoodID string
		relPath   string
		want      string
	}{
		{"plain", "054b", "decisions/V1_ACCEPTANCE.md", "dogfood-historical/054b/decisions/V1_ACCEPTANCE.md"},
		{"strip-leading-slash", "001-v2", "/findings/finding-1.md", "dogfood-historical/001-v2/findings/finding-1.md"},
		{"strip-dot-slash", "016", "./prompts/draft.md", "dogfood-historical/016/prompts/draft.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := HistoricalDogfoodKey(c.dogfoodID, c.relPath)
			if got != c.want {
				t.Fatalf("HistoricalDogfoodKey(%q, %q) = %q, want %q", c.dogfoodID, c.relPath, got, c.want)
			}
		})
	}
}
