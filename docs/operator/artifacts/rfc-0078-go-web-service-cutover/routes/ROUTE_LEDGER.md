---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Go Web Service Route Ledger
author: operator [self-declared: route-decider-codex-gpt-5-002]

## Authority Statement

Daemon-owned PostgreSQL/RPC remains the live workflow authority. Repository
files are provenance only. The Go web layer must not read or write live state
directly, must not import Python service/web modules, and must not use terminal
output, tmux panes, transcripts, marker files, or `.striatum/` scratch as
state.

## Route Decisions

| Route family | Python source evidence | Decision | Go target or rationale | Required tests/guards |
|---|---|---|---|---|
| `GET /v1/health` | `src/striatum/service_routes.py`, `src/striatum/service_api_routes.py` | port | `go/pkg/webservice` health envelope | `TestHealthAndSecurityHeaders` |
| `GET /v1/runs`, `GET /v1/runs/<run_id>` | `src/striatum/service_routes.py`, `src/striatum/service_api_routes.py` | port | RPC `status` through `go/pkg/webservice` | `TestReadRouteUsesDaemonRPC` |
| `GET /v1/runs/<run_id>/why?id=...` | `src/striatum/service_api_routes.py` | port | RPC `why` | route covered by shared RPC dispatch; broader page parity deferred |
| `GET /v1/runs/<run_id>/dashboard` | `src/striatum/service_api_routes.py` | port | RPC `dashboard` | `TestReadRouteUsesDaemonRPC` |
| `GET /v1/runs/<run_id>/events` | `src/striatum/service_api_routes.py`, `src/striatum/service_sse.py` | port | RPC `run.events` streamed by `go/pkg/websse` | `TestWebServiceAdapterSSEUsesDaemonRunEvents`, `TestStreamWritesTerminalEventAndReturns` |
| `GET /v1/runs/<run_id>/artifacts` | `src/striatum/service_api_routes.py` | port | RPC `list.artifacts` | covered by shared RPC dispatch shape |
| `GET /v1/artifacts/<artifact_id>/raw` | `src/striatum/service.py`, `src/striatum/web/artifacts.py` | port | RPC `artifact.get_content`; no repo-path reads in web layer | `TestArtifactRawUsesDaemonContent` |
| `GET /workflow-templates`, `GET /workflow-templates/<id>` | `src/striatum/service_routes.py`, `src/striatum/web/workflow_generation.py` | port | RPC `workflow.templates.list/show` | covered by shared RPC dispatch shape |
| `POST /workflows/generate/preview` | `src/striatum/service_routes.py`, `src/striatum/web/workflow_generation.py` | port | RPC `workflow.generate.preview`; read-capability preview | route implemented; focused route test deferred |
| `POST /workflows/generate` | `src/striatum/service_routes.py`, `src/striatum/web/workflow_generation.py` | port | RPC `workflow.generate`; requires `AllowMutations` | mutation gate covered by `TestMutationRefusedWhenDisabled` |
| `POST /v1/invoke` | `src/striatum/service_routes.py`, `src/striatum/service_daemon.py` | port | method-based RPC dispatch with limited compatibility argv mapping | `TestMutationRefusedWhenDisabled`, `TestMutationAllowedWhenEnabled` |
| `GET /`, `/run/...` HTML pages | `src/striatum/service_routes.py`, `src/striatum/web/run_pages.py` | blocker | Minimal Go shell renders RPC data; full Jinja page parity remains a future UI slice | `TestStaticAssetServedFromGoEmbed` plus static handoff blocker |
| `/static/*` | `src/striatum/service_routes.py`, `src/striatum/web/static_assets.py` | port | `go/pkg/webassets` embedded static files | `TestStaticAssetServedFromGoEmbed` |
| `/doctor`, `/escalations*`, `/cross-repo`, `/view*`, `/workflows*`, run action POSTs | `src/striatum/service_routes.py`, `src/striatum/web/*.py` | blocker | Requires fuller Go HTML/page and action wiring over existing RPC methods | named blocker; do not delete Python web yet |
| `/dogfood*` | `src/striatum/service_routes.py`, `src/striatum/web/dogfood_historical.py`, `src/striatum/web/dogfood_routes.py` | retire | Historical dogfood browser is not current operator surface; Go service returns `410` | `TestRetiredRoutesReturnGone`, `scripts/guard_rfc0078_web_retirement.sh` |
| `/chat*` | `src/striatum/service_routes.py`, `src/striatum/web/chat_routes.py`, `src/striatum/web/chat_provider.py` | retire | Python-only local chat/provider convenience would preserve non-daemon side effects and external provider calls | `TestRetiredRoutesReturnGone`, retirement guard script |

## Notes

The retained route subset is enough to establish the Go service/security,
static, SSE, and mutation-gate seams. Full Python web deletion is not safe
until the blocked HTML/operator action routes above are ported or explicitly
retired by a follow-up decision.
