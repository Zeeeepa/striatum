package mutations

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestLaneCredentialDomainRejectsModeledCredentialCacheInsideRepo(t *testing.T) {
	repo := t.TempDir()
	err := enforceLaneCredentialDomain(context.Background(), supervisionStartConfig{
		RepoRoot:        repo,
		OriginalCommand: []string{"claude"},
		LaunchEnv: map[string]string{
			"CLAUDE_SECURESTORAGE_CONFIG_DIR": filepath.Join(repo, ".claude-secure"),
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "lane_credential_cache_inside_repo" {
		t.Fatalf("err = %v, want lane_credential_cache_inside_repo", err)
	}
}

func TestLaneCredentialDomainRejectsRelativeModeledCredentialCacheInsideRepo(t *testing.T) {
	repo := t.TempDir()
	err := enforceLaneCredentialDomain(context.Background(), supervisionStartConfig{
		RepoRoot:        repo,
		OriginalCommand: []string{"claude"},
		LaunchEnv: map[string]string{
			"CLAUDE_SECURESTORAGE_CONFIG_DIR": filepath.Join("docs", ".lane-auth", "secure"),
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "lane_credential_cache_inside_repo" {
		t.Fatalf("err = %v, want lane_credential_cache_inside_repo", err)
	}
}

func TestLaneCredentialDomainRejectsUnmodeledProviderSelectorInsideRepo(t *testing.T) {
	repo := t.TempDir()
	err := enforceLaneCredentialDomain(context.Background(), supervisionStartConfig{
		RepoRoot:        repo,
		OriginalCommand: []string{"claude"},
		LaunchEnv: map[string]string{
			"ANTHROPIC_CONFIG_DIR": filepath.Join(repo, ".anthropic"),
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "lane_uncovered_credential_selector_inside_repo" {
		t.Fatalf("err = %v, want lane_uncovered_credential_selector_inside_repo", err)
	}
}

func TestLaneCredentialDomainRejectsRelativeUnmodeledProviderSelectorInsideRepo(t *testing.T) {
	repo := t.TempDir()
	err := enforceLaneCredentialDomain(context.Background(), supervisionStartConfig{
		RepoRoot:        repo,
		OriginalCommand: []string{"claude"},
		LaunchEnv: map[string]string{
			"ANTHROPIC_CONFIG_DIR": filepath.Join("docs", ".lane-auth", "anthropic"),
		},
	})
	var rpcErr *rpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != "lane_uncovered_credential_selector_inside_repo" {
		t.Fatalf("err = %v, want lane_uncovered_credential_selector_inside_repo", err)
	}
}

func TestLaneCredentialDomainAllowsOrdinaryLaneEnvInsideRepo(t *testing.T) {
	repo := t.TempDir()
	err := enforceLaneCredentialDomain(context.Background(), supervisionStartConfig{
		RepoRoot:        repo,
		OriginalCommand: []string{"claude"},
		LaunchEnv: map[string]string{
			"AGY_HOME":           filepath.Join(repo, ".agy"),
			"FIXTURE_CONFIG_DIR": filepath.Join(repo, "fixtures"),
			"CLAUDE_CONFIG_DIR":  filepath.Join(t.TempDir(), "claude"),
		},
	})
	if err != nil {
		t.Fatalf("ordinary lane env rejected: %v", err)
	}
}
