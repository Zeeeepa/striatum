package blob

import (
	"errors"
	"testing"
)

func TestLoadConfigDisabledWhenEndpointMissing(t *testing.T) {
	t.Setenv("STRIATUM_BLOB_ENDPOINT", "")
	cfg, err := LoadConfig()
	if cfg != nil {
		t.Fatalf("expected nil config, got %+v", cfg)
	}
	if !errors.Is(err, ErrConfigDisabled) {
		t.Fatalf("expected ErrConfigDisabled, got %v", err)
	}
	if !IsDisabled(err) {
		t.Fatalf("IsDisabled should report true for ErrConfigDisabled")
	}
}

func TestLoadConfigRefusesPartialCredentials(t *testing.T) {
	t.Setenv("STRIATUM_BLOB_ENDPOINT", "http://localhost:9000")
	t.Setenv("STRIATUM_BLOB_ACCESS_KEY", "")
	t.Setenv("STRIATUM_BLOB_SECRET_KEY", "secret")
	_, err := LoadConfig()
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("expected *ConfigError, got %v", err)
	}
	if configErr.Field != "STRIATUM_BLOB_ACCESS_KEY" {
		t.Fatalf("expected access-key field error, got %s", configErr.Field)
	}
}

func TestLoadConfigPopulatedFromEnv(t *testing.T) {
	t.Setenv("STRIATUM_BLOB_ENDPOINT", "http://localhost:9000")
	t.Setenv("STRIATUM_BLOB_ACCESS_KEY", "ak")
	t.Setenv("STRIATUM_BLOB_SECRET_KEY", "sk")
	t.Setenv("STRIATUM_BLOB_REGION", "")
	t.Setenv("STRIATUM_BLOB_BUCKET_PREFIX", "")
	t.Setenv("STRIATUM_BLOB_PATH_STYLE", "")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Endpoint != "localhost:9000" {
		t.Fatalf("scheme not stripped: %q", cfg.Endpoint)
	}
	if cfg.UseSSL {
		t.Fatalf("UseSSL should be false for http://")
	}
	if cfg.Region != "us-east-1" {
		t.Fatalf("default region wrong: %q", cfg.Region)
	}
	if cfg.BucketPrefix != DefaultBucketPrefix {
		t.Fatalf("default prefix wrong: %q", cfg.BucketPrefix)
	}
	if !cfg.PathStyle {
		t.Fatalf("default path-style should be true")
	}
}

func TestLoadConfigHttpsEnablesSSL(t *testing.T) {
	t.Setenv("STRIATUM_BLOB_ENDPOINT", "https://s3.example.com")
	t.Setenv("STRIATUM_BLOB_ACCESS_KEY", "ak")
	t.Setenv("STRIATUM_BLOB_SECRET_KEY", "sk")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.UseSSL {
		t.Fatalf("UseSSL should be true for https://")
	}
	if cfg.Endpoint != "s3.example.com" {
		t.Fatalf("scheme not stripped: %q", cfg.Endpoint)
	}
}

func TestLoadConfigInvalidPathStyleRefused(t *testing.T) {
	t.Setenv("STRIATUM_BLOB_ENDPOINT", "http://localhost:9000")
	t.Setenv("STRIATUM_BLOB_ACCESS_KEY", "ak")
	t.Setenv("STRIATUM_BLOB_SECRET_KEY", "sk")
	t.Setenv("STRIATUM_BLOB_PATH_STYLE", "maybe")
	_, err := LoadConfig()
	var configErr *ConfigError
	if !errors.As(err, &configErr) {
		t.Fatalf("expected *ConfigError, got %v", err)
	}
	if configErr.Field != "STRIATUM_BLOB_PATH_STYLE" {
		t.Fatalf("expected path-style field, got %s", configErr.Field)
	}
}
