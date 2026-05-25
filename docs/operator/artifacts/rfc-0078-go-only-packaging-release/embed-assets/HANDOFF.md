---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["go/pkg/db/sql/", "go/pkg/workflowtemplates/catalog.go", "go/pkg/workflowtemplates/catalog_test.go"]
---

# Go Embed Runtime Assets Handoff
author: operator [self-declared: asset-packager-codex-gpt-5-001]

## Current State

Go already owns two runtime asset classes needed by this packaging gate:

- daemon SQL migrations are embedded by `go/pkg/db/migrations.go`;
- the workflow template catalog is embedded by `go/pkg/workflowtemplates/catalog.go`.

The archive smoke checks `striatumd --describe` and requires a nonzero
embedded migration count. Existing Go tests cover migration loading and
workflow template catalog loading.

## Not Landed

Local web static/templates and skill/plugin templates still live under Python
package-data paths. This workflow did not duplicate them into Go embed
packages because the web/service and skills/plugin parity gates have not
decided the retained route/template set.

## Blocker For Deletion

Before deleting Python package data, the relevant follow-up gate must either:

- port retained web/static/template and skill/plugin template assets into Go
  embed packages with tests; or
- explicitly retire those product routes/templates and update docs.
