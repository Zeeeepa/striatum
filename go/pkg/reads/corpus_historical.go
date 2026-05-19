package reads

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// historicalDogfoodPrefix is the canonical bucket-side prefix the
// migration script uploads under (mirrors blob.HistoricalDogfoodKey).
const historicalDogfoodPrefix = "dogfood-historical/"

// HandleCorpusListHistoricalDogfoods enumerates dogfood IDs for which
// at least one file was migrated to the per-repo bucket.
//
// Response shape:
//
//	{
//	  "dogfood_ids": ["001", "001-v2", "002", ...]
//	}
//
// Requires the daemon to be configured for blob storage; refuses with
// blob_disabled otherwise.
func HandleCorpusListHistoricalDogfoods(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	if packageBlobClient == nil {
		return nil, rpc.NewError("blob_disabled", "daemon is not configured for blob storage", nil)
	}
	bucket, err := lookupRepoBlobBucketRead(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	if bucket == "" {
		return nil, rpc.NewError("blob_disabled", fmt.Sprintf("repository %s has no blob_bucket configured", repositoryID), nil)
	}
	prefixes, err := packageBlobClient.ListCommonPrefixes(ctx, bucket, historicalDogfoodPrefix, "/")
	if err != nil {
		return nil, rpc.NewError("blob_list_failed", err.Error(), map[string]any{"bucket": bucket})
	}
	ids := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		// "dogfood-historical/<id>/" -> "<id>"
		trimmed := strings.TrimPrefix(p, historicalDogfoodPrefix)
		trimmed = strings.TrimSuffix(trimmed, "/")
		if trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return map[string]any{"dogfood_ids": ids}, nil
}

// HandleCorpusListHistoricalDogfoodFiles enumerates the files under
// one dogfood ID's prefix. Returns relative paths (without the
// `dogfood-historical/<id>/` prefix) so the web UI can render
// path-shaped links directly.
//
// Required params: dogfood_id.
//
// Response shape:
//
//	{
//	  "dogfood_id": "042",
//	  "files": [
//	    {"rel_path": "BUILD_HANDOFF.md", "size_bytes": 12345, "last_modified": "..."},
//	    ...
//	  ]
//	}
func HandleCorpusListHistoricalDogfoodFiles(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	dogfoodID := stringParam(envelope, "dogfood_id")
	if dogfoodID == "" {
		return nil, rpc.NewError("schema_invalid", "corpus.list_historical_dogfood_files requires dogfood_id", nil)
	}
	if strings.Contains(dogfoodID, "/") || dogfoodID == "." || dogfoodID == ".." {
		return nil, rpc.NewError("schema_invalid", "dogfood_id must not contain path separators", nil)
	}
	if packageBlobClient == nil {
		return nil, rpc.NewError("blob_disabled", "daemon is not configured for blob storage", nil)
	}
	bucket, err := lookupRepoBlobBucketRead(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	if bucket == "" {
		return nil, rpc.NewError("blob_disabled", fmt.Sprintf("repository %s has no blob_bucket configured", repositoryID), nil)
	}
	prefix := historicalDogfoodPrefix + dogfoodID + "/"
	entries, err := packageBlobClient.ListByPrefix(ctx, bucket, prefix)
	if err != nil {
		return nil, rpc.NewError("blob_list_failed", err.Error(), map[string]any{"bucket": bucket, "prefix": prefix})
	}
	files := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		rel := strings.TrimPrefix(e.Key, prefix)
		if rel == "" {
			continue
		}
		row := map[string]any{
			"rel_path":   rel,
			"size_bytes": e.Size,
		}
		if e.LastModified != "" {
			row["last_modified"] = e.LastModified
		}
		files = append(files, row)
	}
	return map[string]any{
		"dogfood_id": dogfoodID,
		"files":      files,
	}, nil
}

// HandleCorpusFetchHistoricalDogfoodFile fetches one historical-migration
// blob and returns it base64-encoded. Mirrors HandleArtifactGetContent
// for content addressed by (dogfood_id, rel_path) rather than artifact
// id, since the migrated bodies have no striatumd.artifacts row.
//
// Required params: dogfood_id, rel_path.
//
// Response shape:
//
//	{
//	  "dogfood_id":   "042",
//	  "rel_path":     "BUILD_HANDOFF.md",
//	  "content_type": "text/markdown; charset=utf-8",
//	  "body_base64":  "...",
//	  "sha256":       "...",
//	  "source":       "blob"
//	}
//
// Sha256 verification: the daemon reads the X-Striatum-Sha256 user
// metadata via HeadObject, then GETs the body, then re-hashes and
// compares. A mismatch refuses with blob_read_failed.
func HandleCorpusFetchHistoricalDogfoodFile(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	dogfoodID := stringParam(envelope, "dogfood_id")
	relPath := stringParam(envelope, "rel_path")
	if dogfoodID == "" || relPath == "" {
		return nil, rpc.NewError("schema_invalid", "corpus.fetch_historical_dogfood_file requires dogfood_id and rel_path", nil)
	}
	if strings.Contains(dogfoodID, "/") || dogfoodID == "." || dogfoodID == ".." {
		return nil, rpc.NewError("schema_invalid", "dogfood_id must not contain path separators", nil)
	}
	if strings.HasPrefix(relPath, "/") || strings.Contains(relPath, "..") {
		return nil, rpc.NewError("schema_invalid", "rel_path must be sanitized", nil)
	}
	if packageBlobClient == nil {
		return nil, rpc.NewError("blob_disabled", "daemon is not configured for blob storage", nil)
	}
	bucket, err := lookupRepoBlobBucketRead(ctx, runner, repositoryID)
	if err != nil {
		return nil, err
	}
	if bucket == "" {
		return nil, rpc.NewError("blob_disabled", fmt.Sprintf("repository %s has no blob_bucket configured", repositoryID), nil)
	}
	key := historicalDogfoodPrefix + dogfoodID + "/" + strings.TrimPrefix(relPath, "/")

	// Probe sha256 metadata so GetBytes can verify integrity.
	remoteSha, exists, err := packageBlobClient.RemoteSha256(ctx, bucket, key)
	if err != nil {
		return nil, rpc.NewError("blob_read_failed", err.Error(), map[string]any{"bucket": bucket, "key": key})
	}
	if !exists {
		return nil, rpc.NewError("not_found", fmt.Sprintf("historical dogfood file not found: %s/%s", dogfoodID, relPath), nil)
	}
	if remoteSha == "" {
		return nil, rpc.NewError(
			"blob_read_failed",
			fmt.Sprintf("historical blob %s lacks the X-Striatum-Sha256 integrity anchor", key),
			map[string]any{"bucket": bucket, "key": key},
		)
	}
	body, err := packageBlobClient.GetBytes(ctx, bucket, key, remoteSha)
	if err != nil {
		return nil, rpc.NewError("blob_read_failed", err.Error(), map[string]any{"bucket": bucket, "key": key})
	}
	contentType := contentTypeForRelPath(relPath)
	return map[string]any{
		"dogfood_id":   dogfoodID,
		"rel_path":     relPath,
		"content_type": contentType,
		"body_base64":  base64.StdEncoding.EncodeToString(body),
		"sha256":       remoteSha,
		"size_bytes":   len(body),
		"source":       "blob",
	}, nil
}

// contentTypeForRelPath maps a historical file's relative path to a
// conservative content type. Mirrors the producer-side mapping in
// go/pkg/mutations.artifactContentType so the web UI can decide
// between render-as-markdown and raw-download without re-deriving.
func contentTypeForRelPath(relPath string) string {
	lower := strings.ToLower(relPath)
	switch {
	case strings.HasSuffix(lower, ".md"):
		return "text/markdown; charset=utf-8"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".txt"):
		return "text/plain; charset=utf-8"
	case strings.HasSuffix(lower, ".yml"), strings.HasSuffix(lower, ".yaml"):
		return "application/x-yaml"
	case strings.HasSuffix(lower, ".sh"):
		return "text/x-shellscript; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
