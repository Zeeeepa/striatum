/**
 * Shared prop and API types for every Striatum web UI island.
 *
 * Codex's Jinja2 templates emit `data-props='{{ props | tojson }}'` payloads
 * (for small props) and adjacent `<script type="application/json">` /
 * `<script type="text/plain">` tags (for workflow JSON, file bytes, etc.).
 *
 * **This file is the single source of truth for the prop contract between
 * the Python templates and the React islands.** Any prop-shape change must
 * update this file first; the matching template change follows in the
 * toolchain implementer's scope.
 *
 * Template shapes mirrored here (see `src/striatum/web/templates/`):
 *
 * | Template                | data-props                                                           |
 * | ----------------------- | -------------------------------------------------------------------- |
 * | `view_tree.html`        | `{ rootPath, treeUrl }`                                              |
 * | `workflow_new.html`     | `{ allowMutations, templatesUrl, previewUrl, generateUrl }`          |
 * | `view_file.html`        | `{ path, language }` (+ adjacent `<pre><code>` carries the bytes)    |
 * | `workflow_edit.html`    | `{ path, saveUrl, fallback }` (+ `<script id="workflow-data">` and  |
 * |                         | `<script id="workflow-sha256">` carry the workflow JSON and sha256)  |
 */

// ---------------------------------------------------------------------------
// Common API envelope shapes
// ---------------------------------------------------------------------------

export interface ApiOk<T> {
  ok: true;
  data: T;
}

export interface ApiErr {
  ok: false;
  error: {
    code: string;
    message: string;
    errors?: Array<{ field_path?: string; message: string }>;
  };
}

export type ApiResult<T> = ApiOk<T> | ApiErr;

// ---------------------------------------------------------------------------
// Tree browser island
// ---------------------------------------------------------------------------

export interface RepoTreeEntry {
  name: string;
  path: string;
  kind: "dir" | "file";
  size: number | null;
  mtime_utc: string | null;
}

export interface RepoTreeResponse {
  path: string;
  entries: RepoTreeEntry[];
  truncated: boolean;
}

export interface TreeBrowserProps {
  /** Initial directory to load. Empty string = repo root. */
  rootPath: string;
  /** API URL for the tree endpoint; defaults to `/v1/repo/tree`. */
  treeUrl?: string;
  /** Base URL for single-file view links; defaults to `/view/`. */
  viewBase?: string;
  /** Optional display label for the root crumb; defaults to `/`. */
  rootLabel?: string;
}

// ---------------------------------------------------------------------------
// Workflow chooser wizard island
// ---------------------------------------------------------------------------

/**
 * One row of the workflow template catalog, mirroring
 * `striatum.workflow_generator.catalog.list_templates()` output.
 *
 * The server returns shapes and lane sets in a single flat array; consumers
 * partition by `kind`. V1.5 dropped the `{shapes, lane_sets, modifiers}`
 * wrapper that never existed on the server (RFC 0038 V1.5 F2).
 */
export interface WorkflowTemplate {
  template_id: string;
  kind: "shape" | "lane_set";
  display_name: string;
  summary: string;
  recommended_for: string | string[];
  default_lane_sets?: string[];
  required_options?: string[];
  graph_preview?: unknown;
}

export interface WorkflowTemplateListResponse {
  templates: WorkflowTemplate[];
}

export interface GeneratedWorkflowFile {
  path: string;
  bytes: number;
  status?: "created" | "updated" | "skipped";
}

export interface GeneratedWorkflow {
  workflow: Record<string, unknown>;
  files: GeneratedWorkflowFile[];
  graph?: { nodes: number; edges: number; cycles?: number };
  warnings: string[];
}

export interface WorkflowChooserProps {
  /** Whether the local service was started with `--allow-mutations`. */
  allowMutations: boolean;
  /** GET endpoint returning the workflow template catalog. */
  templatesUrl?: string;
  /** POST endpoint that previews generation (no on-disk writes). */
  previewUrl?: string;
  /** POST endpoint that writes the generated workflow files. */
  generateUrl?: string;
  /** Optional default scaffold/artifact roots shown in the form. */
  defaultScaffoldRoot?: string;
  defaultArtifactRoot?: string;
}

// ---------------------------------------------------------------------------
// Workflow graph editor island
// ---------------------------------------------------------------------------

export interface WorkflowJob {
  id: string;
  type?: string;
  title?: string;
  objective?: string;
  role_id?: string;
  lane_id?: string;
  task_prompt?: { path?: string };
  write_scope?: {
    mode?: string;
    allowed_paths?: string[];
    forbidden_paths?: string[];
    repo_write?: boolean;
  };
  expected_artifacts?: Array<{
    kind?: string;
    logical_name?: string;
    path?: string;
    author_line?: string;
    required?: boolean;
  }>;
  review_posture?: string;
  reviewer_access_scope?: string;
  reviewer_context_policy?: string;
  fresh_session_required?: boolean;
  required_review_postures?: string[];
  parallel_group?: string;
  /** RFC 0045 v1.1 — canonical phase pointer (must match a `phases[].id`). */
  phase?: string;
  /** RFC 0045 v1.1 read-compat — early fixtures emitted `phase_id`; the
   *  editor normalises both via `jobPhaseId(job)` but writes only `phase`. */
  phase_id?: string;
  [key: string]: unknown;
}

/**
 * RFC 0045 v1.1 phase descriptor. Phases are an ordered, opt-in top-level
 * field on the workflow document; v1 workflows carry no `phases` array and
 * the editor renders them with the legacy single-phase layout.
 */
export interface WorkflowPhase {
  id: string;
  title?: string;
  name?: string;
  description?: string;
  /** Job id whose accepting verdict gates the next phase. */
  synthesis_job_id?: string;
  /** Optional explicit band colour; falls back to a deterministic palette. */
  color?: string;
}

export interface WorkflowEdge {
  from: string;
  to: string;
  on?: "completed" | "failed" | "blocked";
}

export interface WorkflowCycle {
  from: string;
  to: string;
  on_verdict?: string;
  max_iterations?: number;
}

export interface WorkflowDocument {
  schema_version?: string;
  workflow_id?: string;
  workflow_version?: string;
  name?: string;
  roles?: Record<string, { definition_path?: string }>;
  lanes?: Record<
    string,
    {
      adapter?: string;
      display_model?: string;
      command?: string[];
      capabilities?: string[];
    }
  >;
  jobs?: WorkflowJob[];
  edges?: WorkflowEdge[];
  cycles?: WorkflowCycle[];
  /** RFC 0045 v1.1 — ordered phase list. Absent or empty means v1
   *  single-phase rendering. */
  phases?: WorkflowPhase[];
  [key: string]: unknown;
}

export interface WorkflowGraphEditorProps {
  /** Repo-relative workflow.json path. */
  path: string;
  /** POST endpoint (kept absolute since the template renders the prefix). */
  saveUrl: string;
  /** True when the legacy form-driven editor below should be left visible as a fallback. */
  fallback?: boolean;
  /** Cancel-link destination; defaults to `/workflows/<path>`. */
  cancelUrl?: string;
  /** DOM id of the adjacent workflow JSON payload tag. Default: `workflow-data`. */
  workflowDataElementId?: string;
  /** DOM id of the adjacent sha256 payload tag. Default: `workflow-sha256`. */
  workflowSha256ElementId?: string;
}

// ---------------------------------------------------------------------------
// Code viewer island
// ---------------------------------------------------------------------------

export type CodeViewerLanguage =
  | "json"
  | "python"
  | "typescript"
  | "javascript"
  | "bash"
  | "yaml"
  | "toml"
  | "markdown"
  | "sql"
  | "plaintext";

export interface CodeViewerProps {
  /** Repo-relative path (used for the file label and Raw link). */
  path: string;
  /** Server-detected language hint; may be a short form like `py` or `yml`. */
  language: CodeViewerLanguage | string;
  /** Raw-file URL; defaults to `/view/raw/<path>`. */
  rawUrl?: string;
  /** DOM id of the adjacent server-rendered `<pre><code>` block. Default: derived from data-props parent. */
  sourceElementSelector?: string;
}

// ---------------------------------------------------------------------------
// RFC 0050 V1 shared operator UI component contracts
// ---------------------------------------------------------------------------

export const RUN_STATES = [
  "needs_branch_confirmation",
  "ready",
  "running",
  "blocked",
  "completed",
  "failed",
  "canceled",
] as const;

export type RunState = (typeof RUN_STATES)[number];

export const RUN_RENDER_STATES = [
  ...RUN_STATES,
  "paused",
  "compromised",
] as const;

export type RunRenderState = (typeof RUN_RENDER_STATES)[number];

export interface RunStatePillProps {
  state: RunState;
  pausedAt?: string | null;
  classModifier?: string;
}

export const JOB_STATES = [
  "blocked",
  "queued",
  "ready",
  "claimed",
  "running",
  "completed",
  "failed",
  "canceled",
  "skipped",
  "stale_lease",
  "waiting_human",
] as const;

export type JobState = (typeof JOB_STATES)[number];

export interface JobStatePillProps {
  state: JobState;
  classModifier?: string;
}

export const VERDICT_KINDS = [
  "accept",
  "accept_with_findings",
  "needs_revision",
  "reject",
] as const;

export type VerdictKind = (typeof VERDICT_KINDS)[number];

export const VERDICT_PROVENANCES = [
  "natural",
  "operator-override",
  "cycle-revised",
] as const;

export type VerdictProvenance = (typeof VERDICT_PROVENANCES)[number];

export interface VerdictChipProps {
  verdict: VerdictKind;
  provenance: VerdictProvenance;
  rationale?: string;
  cycleIndex?: number;
  cycleLimit?: number;
}

export const ATTESTATION_REASONS = [
  "no_attached_supervisor",
  "pid_gone",
  "pid_identity_mismatch",
  "lane_command_mismatch",
  "session_missing",
  "session_mismatch",
  "run_mismatch",
] as const;

export type AttestationReason = (typeof ATTESTATION_REASONS)[number];

export interface LaneAttestationChipProps {
  attested: boolean;
  reason?: AttestationReason;
  supervisorId?: string | null;
  operatorLabel?: string | null;
}

export const POSTURE_KINDS = [
  "neutral",
  "devils_advocate",
  "security",
  "threat_model",
  "latency_performance",
  "ergonomics_dx",
  "accessibility",
  "compliance_license",
  "supply_chain",
] as const;

export type PostureKind = (typeof POSTURE_KINDS)[number];

export type PostureValue = PostureKind | `custom:${string}`;

export interface PostureChipProps {
  posture: PostureValue | string;
}

export interface BylineLineProps {
  authorLine: string | null;
  expectedAuthorLine?: string | null;
  attested?: boolean;
  operatorLabel?: string | null;
  displayAuthor?: never;
  display_author?: never;
}

export const LANE_EVIDENCE_STATES = ["not_yet_correlated", "override"] as const;

export type LaneEvidenceState = (typeof LANE_EVIDENCE_STATES)[number];

export interface LaneEvidenceChipProps {
  state?: LaneEvidenceState;
  rationale?: string | null;
}

export const EXPECTED_ARTIFACT_KINDS = [
  "prompt",
  "finding",
  "synthesis",
  "decision",
  "marker",
  "handoff",
  "support_ledger",
  "action_item_ledger",
  "findings_ledger",
  "harness_improvement_proposal",
  "patch_summary",
  "test_report",
  "other",
] as const;

export type ExpectedArtifactKind = (typeof EXPECTED_ARTIFACT_KINDS)[number];

export interface ExpectedArtifact {
  logical_name: string;
  kind: ExpectedArtifactKind;
  path: string;
  required?: boolean;
  expected_author_line?: string | null;
  author_line?: string | null;
}

export interface PublishedArtifact {
  logical_name?: string | null;
  kind?: ExpectedArtifactKind | string | null;
  path: string;
  sha256?: string | null;
  author_line?: string | null;
  published_at?: string | null;
  lane_attestation_chip?: { attested?: boolean } | null;
}

export interface SessionSummary {
  session_id: string;
  lane_id?: string | null;
  role_id?: string | null;
}

export interface LeaseSummary {
  lease_id: string;
  job_id?: string | null;
  session_id?: string | null;
}

export interface ExpectedArtifactsTableProps {
  expectedArtifacts: ExpectedArtifact[];
  actualArtifacts: PublishedArtifact[];
  session: SessionSummary | null;
  lease: LeaseSummary | null;
}

// ---------------------------------------------------------------------------
// Closed vocabularies (mirrored on the Python side via workflow validator)
// ---------------------------------------------------------------------------

export const ALLOWED_REVIEW_POSTURES = [
  "neutral",
  "devils_advocate",
  "security",
  "threat_model",
  "latency_performance",
  "ergonomics_dx",
  "accessibility",
  "compliance_license",
  "supply_chain",
] as const;

export const JOB_TYPES = ["generic", "review", "synthesis", "build"] as const;

export const EDGE_VERDICTS = ["completed", "failed", "blocked"] as const;

export const REVIEWER_ACCESS_SCOPES = [
  "document_only",
  "artifact_augmented",
  "repo_level",
] as const;

export const REVIEWER_CONTEXT_POLICIES = ["fresh", "cross_round"] as const;

export const WRITE_SCOPE_MODES = [
  "no_repo_write",
  "review_only_artifact_scope",
  "repo_write",
] as const;
