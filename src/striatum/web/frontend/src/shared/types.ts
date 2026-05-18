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
 * | `view_file.html`        | `{ path, language }` (+ adjacent `<pre><code>` carries the bytes)    |
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
