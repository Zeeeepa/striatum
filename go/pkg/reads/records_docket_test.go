package reads

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/halbritt/striatum/go/pkg/pgtest"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

func TestRecordsDocketValidatesParams(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]any
		code   string
	}{
		{
			name:   "missing repository_id",
			params: map[string]any{},
			code:   "repo_not_registered",
		},
		{
			name:   "missing run_id",
			params: map[string]any{"repository_id": "repo_1"},
			code:   "schema_invalid",
		},
		{
			name:   "bad format",
			params: map[string]any{"repository_id": "repo_1", "run_id": "run_1", "format": "html"},
			code:   "schema_invalid",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := HandleRecordsDocket(context.Background(), nil, rpc.Envelope{Params: c.params})
			if err == nil {
				t.Fatalf("expected error code %q, got nil", c.code)
			}
			rpcErr, ok := err.(*rpc.Error)
			if !ok {
				t.Fatalf("expected *rpc.Error, got %T: %v", err, err)
			}
			if rpcErr.Code != c.code {
				t.Fatalf("expected code %q, got %q", c.code, rpcErr.Code)
			}
		})
	}
}

func TestRecordsDocketRendersArtifactAndGeneratedRecordRows(t *testing.T) {
	ctx := context.Background()
	runner := pgtest.Pool(t).Runner
	repoID := "repo_records_docket_171"
	runID := "run_records_docket_171"
	now := time.Date(2026, 6, 28, 4, 0, 0, 0, time.UTC)
	crSeedRepo(t, ctx, runner, repoID, now)
	crSeedRun(t, ctx, runner, repoID, runID, "striatum/rfc-0171", now, false, nil)

	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.artifacts (
		  repository_id, artifact_id, run_id, job_id, logical_name, artifact_kind,
		  repo_path, content_sha256, size_bytes, publish_mode, created_at, attempt
		) VALUES ($1,$2,$3,$4,'brief','operator_brief','docs/BRIEF.md',$5,42,'create',$6,1)`,
		repoID, "art_brief_171", runID, "job_"+runID, strings.Repeat("a", 64), now); err != nil {
		t.Fatalf("seed artifact: %v", err)
	}
	if err := runner.Exec(ctx, `
		INSERT INTO striatumd.generated_records (
		  repository_id, record_id, source_path, source_commit, record_class,
		  run_id, job_id, artifact_id, content_sha256, blob_key, blob_sha256,
		  content_type, size_bytes, retention_class, created_at
		) VALUES ($1,$2,'docs/operator/artifacts/run/REPORT.md','commit_171','operator_report',
		          $3,$4,NULL,$5,'generated/run/REPORT.md',$5,'text/markdown',99,'generated_exhaust',$6)`,
		repoID, "rec_report_171", runID, "job_"+runID, strings.Repeat("b", 64), now.Add(time.Second)); err != nil {
		t.Fatalf("seed generated record: %v", err)
	}

	result, err := HandleRecordsDocket(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID,
		"run_id":        runID,
	}})
	if err != nil {
		t.Fatalf("HandleRecordsDocket: %v", err)
	}
	if result["format"] != "markdown" || result["entry_count"] != 2 {
		t.Fatalf("result metadata = %#v", result)
	}
	body, _ := result["body"].(string)
	for _, want := range []string{
		"# Striatum Record Docket",
		"`artifact:art_brief_171`",
		"`record:rec_report_171`",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("docket body missing %q:\n%s", want, body)
		}
	}

	jsonResult, err := HandleRecordsDocket(ctx, runner, rpc.Envelope{Params: map[string]any{
		"repository_id": repoID,
		"run_id":        runID,
		"format":        "json",
	}})
	if err != nil {
		t.Fatalf("HandleRecordsDocket json: %v", err)
	}
	if jsonResult["content_type"] != "application/json" {
		t.Fatalf("json content_type = %#v", jsonResult["content_type"])
	}
	if body, _ := jsonResult["body"].(string); !strings.Contains(body, `"schema_version": "striatum.records.docket.v1"`) {
		t.Fatalf("json body = %s", body)
	}
}
