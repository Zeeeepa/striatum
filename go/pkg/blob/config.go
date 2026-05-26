package blob

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Config carries the daemon-global S3 configuration. Per-repo bucket
// names are not part of this struct; they live on
// striatumd.repositories.blob_bucket and are looked up at request time.
type Config struct {
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	PathStyle    bool
	BucketPrefix string
	UseSSL       bool
	ServerName   string // SNI override; usually empty.
}

// ConfigError indicates an invalid or incomplete daemon-global blob
// configuration. The daemon refuses to enter blob-publishing flows
// when this fires.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("blob config %s: %s", e.Field, e.Message)
}

// ErrConfigDisabled is returned by LoadConfig when STRIATUM_BLOB_ENDPOINT
// is unset. Callers can use this to distinguish "operator hasn't
// configured blob storage" from "configuration is malformed."
var ErrConfigDisabled = errors.New("blob storage not configured (STRIATUM_BLOB_ENDPOINT unset)")

// LoadConfig reads daemon-global blob configuration from environment
// variables. Returns (nil, ErrConfigDisabled) when blob storage is not
// configured; returns (nil, *ConfigError) for malformed values.
//
// The daemon resolves operator intent in this order:
//
//   - STRIATUM_BLOB_ENDPOINT unset → ErrConfigDisabled. RFC 0072 is opt-in
//     until adopters wire it; production daemons run without blob support
//     until configured. Publish paths refuse to route blob-class artifacts
//     in that case.
//   - STRIATUM_BLOB_ENDPOINT set, credentials missing → *ConfigError.
//     Half-configured blob storage is worse than disabled storage.
func LoadConfig() (*Config, error) {
	endpoint := strings.TrimSpace(os.Getenv("STRIATUM_BLOB_ENDPOINT"))
	if endpoint == "" {
		return nil, ErrConfigDisabled
	}
	access := strings.TrimSpace(os.Getenv("STRIATUM_BLOB_ACCESS_KEY"))
	secret := strings.TrimSpace(os.Getenv("STRIATUM_BLOB_SECRET_KEY"))
	if access == "" {
		return nil, &ConfigError{Field: "STRIATUM_BLOB_ACCESS_KEY", Message: "required when STRIATUM_BLOB_ENDPOINT is set"}
	}
	if secret == "" {
		return nil, &ConfigError{Field: "STRIATUM_BLOB_SECRET_KEY", Message: "required when STRIATUM_BLOB_ENDPOINT is set"}
	}
	useSSL := !strings.HasPrefix(endpoint, "http://")
	if strings.HasPrefix(endpoint, "https://") {
		useSSL = true
	}
	region := strings.TrimSpace(os.Getenv("STRIATUM_BLOB_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	prefix := strings.TrimSpace(os.Getenv("STRIATUM_BLOB_BUCKET_PREFIX"))
	if prefix == "" {
		prefix = DefaultBucketPrefix
	}
	pathStyle := true
	if value := strings.TrimSpace(os.Getenv("STRIATUM_BLOB_PATH_STYLE")); value != "" {
		switch strings.ToLower(value) {
		case "1", "true", "yes", "on":
			pathStyle = true
		case "0", "false", "no", "off":
			pathStyle = false
		default:
			return nil, &ConfigError{Field: "STRIATUM_BLOB_PATH_STYLE", Message: fmt.Sprintf("invalid boolean %q", value)}
		}
	}
	return &Config{
		Endpoint:     stripScheme(endpoint),
		Region:       region,
		AccessKey:    access,
		SecretKey:    secret,
		PathStyle:    pathStyle,
		BucketPrefix: prefix,
		UseSSL:       useSSL,
	}, nil
}

// IsDisabled reports whether err means "operator hasn't configured
// blob storage." Helper so callers can quietly degrade to "no-blob"
// behavior without leaking the sentinel everywhere.
func IsDisabled(err error) bool {
	return errors.Is(err, ErrConfigDisabled)
}

func stripScheme(endpoint string) string {
	switch {
	case strings.HasPrefix(endpoint, "https://"):
		return strings.TrimPrefix(endpoint, "https://")
	case strings.HasPrefix(endpoint, "http://"):
		return strings.TrimPrefix(endpoint, "http://")
	default:
		return endpoint
	}
}
