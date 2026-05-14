/**
 * RFC 0038 V1 + RFC 0045 V1 — drag-drop workflow graph editor island.
 *
 * Renders a React Flow canvas over the workflow JSON loaded from a
 * `<script id="workflow-data" type="application/json">` payload that the
 * Jinja2 template renders next to the mount slot. Nodes are workflow jobs;
 * edges are workflow `edges`; cycles render with the `cycle-edge` class for
 * distinct styling. A left palette adds new jobs from the closed RFC 0034
 * block vocabulary; a right inspector edits the selected job with structured
 * widgets per RFC 0038 design synthesis F5.
 *
 * RFC 0045 v1.1: when the workflow declares a top-level `phases` array the
 * canvas renders horizontal colour bands per phase, lays nodes inside the
 * matching band, draws cross-phase dependency edges with a thick black
 * stroke, and exposes a phase metadata inspector on band-header click. Drags
 * that would move a node into a different band are refused with an inline
 * message; the inspector's `phase` field is the only way to change a job's
 * phase. v1 workflows (no `phases` array) keep the original square-grid
 * layout, thin grey edges, and job-only inspector.
 *
 * Coordinates are UI-only and are never persisted to workflow JSON.
 * Save calls the `POST` `saveUrl` (typically `/workflows/edit/<path>`)
 * with the `If-Match` sha256 from the adjacent `<script id="workflow-sha256">`
 * payload.
 */

import {
  type CSSProperties,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  ReactFlowProvider,
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
} from "reactflow";
import "reactflow/dist/style.css";

import { saveWorkflow } from "../../shared/api-client";
import { readJsonPayload } from "../../shared/mount";
import {
  ALLOWED_REVIEW_POSTURES,
  JOB_TYPES,
  REVIEWER_ACCESS_SCOPES,
  REVIEWER_CONTEXT_POLICIES,
  WRITE_SCOPE_MODES,
  type WorkflowDocument,
  type WorkflowEdge,
  type WorkflowGraphEditorProps,
  type WorkflowJob,
  type WorkflowPhase,
} from "../../shared/types";

const PALETTE_BLOCKS: Array<{ kind: string; label: string; jobType: string }> = [
  { kind: "draft", label: "Draft", jobType: "generic" },
  { kind: "review", label: "Review", jobType: "review" },
  { kind: "synthesis", label: "Synthesis", jobType: "synthesis" },
  { kind: "implementation", label: "Implementation", jobType: "build" },
  { kind: "test", label: "Test", jobType: "build" },
  { kind: "human_checkpoint", label: "Human checkpoint", jobType: "generic" },
  { kind: "support_ledger", label: "Support ledger", jobType: "generic" },
  { kind: "evidence_audit", label: "Evidence audit", jobType: "review" },
  { kind: "final_review", label: "Final review", jobType: "review" },
];

const PHASE_BAND_HEIGHT = 320;
const PHASE_HEADER_HEIGHT = 36;
const PHASE_NODE_TOP = 72;
const PHASE_COLUMN_WIDTH = 260;
const PHASE_ROW_HEIGHT = 96;
const PHASE_BAND_WIDTH = 4000;
const PHASE_PALETTE_LEN = 4;

interface InternalEdge extends WorkflowEdge {
  isCycle?: boolean;
  maxIterations?: number;
  crossPhase?: boolean;
  sourcePhase?: string;
  targetPhase?: string;
}

interface PhaseLayoutEntry {
  phase: WorkflowPhase;
  index: number;
  paletteIndex: number;
  bandTop: number;
  jobIds: string[];
}

interface PhaseLayout {
  phases: PhaseLayoutEntry[];
  byPhaseId: Map<string, PhaseLayoutEntry>;
  jobPhaseMap: Map<string, string>;
  hasExplicit: boolean;
}

function jobPhaseId(job: WorkflowJob): string {
  const raw =
    typeof job.phase === "string"
      ? job.phase
      : typeof job.phase_id === "string"
        ? job.phase_id
        : "";
  return raw;
}

function hasExplicitPhases(workflow: WorkflowDocument): boolean {
  return (workflow.phases ?? []).length > 0;
}

function phaseDisplayName(phase: WorkflowPhase): string {
  return phase.title ?? phase.name ?? phase.id;
}

function buildPhaseLayout(workflow: WorkflowDocument): PhaseLayout {
  const phases = workflow.phases ?? [];
  const jobs = workflow.jobs ?? [];
  const hasExplicit = phases.length > 0;

  const entries: PhaseLayoutEntry[] = phases.map((phase, index) => ({
    phase,
    index,
    paletteIndex: index % PHASE_PALETTE_LEN,
    bandTop: index * PHASE_BAND_HEIGHT,
    jobIds: [],
  }));

  const byPhaseId = new Map<string, PhaseLayoutEntry>();
  for (const entry of entries) {
    byPhaseId.set(entry.phase.id, entry);
  }

  const jobPhaseMap = new Map<string, string>();
  if (!hasExplicit) {
    return { phases: entries, byPhaseId, jobPhaseMap, hasExplicit };
  }

  const firstPhaseId = entries[0].phase.id;
  for (const job of jobs) {
    const declared = jobPhaseId(job);
    const resolved = declared && byPhaseId.has(declared) ? declared : firstPhaseId;
    jobPhaseMap.set(job.id, resolved);
    byPhaseId.get(resolved)!.jobIds.push(job.id);
  }

  return { phases: entries, byPhaseId, jobPhaseMap, hasExplicit };
}

function jobNodeLabel(job: WorkflowJob): string {
  const lines = [`${job.id}`, `${job.type ?? "generic"}`];
  if (job.require_attested_lane === true) {
    lines.push("require_attested_lane=true");
  }
  return lines.join("\n");
}

// GH #13: fields the inspector only exposes when `job.type === "review"`.
// Changing a job away from `review` hides the inspector controls but used to
// leave the underlying values on the job object, so node labels, the textual
// summary, and the saved JSON would still carry stale review-only settings.
// `purgeStaleFieldsForType` drops these when the type transitions from
// `review` to anything else. The list intentionally tracks the review-only
// fieldset rendered by `Inspector`; broader cross-type normalization is out
// of scope per docs/issues/13/SCOPE.md GH13-6.
const REVIEW_ONLY_JOB_FIELDS = [
  "require_attested_lane",
  "review_posture",
  "reviewer_access_scope",
  "reviewer_context_policy",
] as const;

function purgeStaleFieldsForType(
  job: WorkflowJob,
  prevType: string | undefined,
  nextType: string | undefined,
): WorkflowJob {
  if (prevType === "review" && nextType !== "review") {
    const cleaned: WorkflowJob = { ...job };
    for (const field of REVIEW_ONLY_JOB_FIELDS) {
      delete (cleaned as Record<string, unknown>)[field];
    }
    return cleaned;
  }
  return job;
}

function jobsToNodes(workflow: WorkflowDocument): Node[] {
  const jobs = workflow.jobs ?? [];
  if (!hasExplicitPhases(workflow)) {
    const cols = Math.max(1, Math.ceil(Math.sqrt(jobs.length)));
    return jobs.map((job, index) => ({
      id: job.id,
      type: "default",
      position: {
        x: (index % cols) * 220,
        y: Math.floor(index / cols) * 140,
      },
      data: { label: jobNodeLabel(job) },
    }));
  }

  const layout = buildPhaseLayout(workflow);
  const nodes: Node[] = [];
  for (const phaseEntry of layout.phases) {
    const phaseJobs = phaseEntry.jobIds
      .map((id) => jobs.find((j) => j.id === id))
      .filter((j): j is WorkflowJob => Boolean(j));

    const groupOrder: string[] = [];
    const groupRows = new Map<string, WorkflowJob[]>();
    for (const job of phaseJobs) {
      const groupKey = job.parallel_group ?? `__solo__:${job.id}`;
      if (!groupRows.has(groupKey)) {
        groupOrder.push(groupKey);
        groupRows.set(groupKey, []);
      }
      groupRows.get(groupKey)!.push(job);
    }

    groupOrder.forEach((groupKey, groupIndex) => {
      const rows = groupRows.get(groupKey)!;
      rows.forEach((job, rowIndex) => {
        const x = groupIndex * PHASE_COLUMN_WIDTH + rowIndex * 24;
        const y =
          phaseEntry.bandTop + PHASE_NODE_TOP + rowIndex * PHASE_ROW_HEIGHT;
        nodes.push({
          id: job.id,
          type: "default",
          position: { x, y },
          data: { label: jobNodeLabel(job) },
        });
      });
    });
  }
  return nodes;
}

function workflowToEdges(workflow: WorkflowDocument): Edge[] {
  const layout = buildPhaseLayout(workflow);
  const out: Edge[] = [];
  for (const e of workflow.edges ?? []) {
    const sourcePhase = layout.jobPhaseMap.get(e.from) ?? "";
    const targetPhase = layout.jobPhaseMap.get(e.to) ?? "";
    const crossPhase =
      layout.hasExplicit &&
      sourcePhase !== "" &&
      targetPhase !== "" &&
      sourcePhase !== targetPhase;
    const baseEdge: Edge = {
      id: `e-${e.from}->${e.to}-${e.on ?? "completed"}`,
      source: e.from,
      target: e.to,
      label: e.on ?? "completed",
      data: { on: e.on ?? "completed" },
    };
    if (crossPhase) {
      out.push({
        ...baseEdge,
        className: "cross-phase-edge",
        style: { stroke: "#000", strokeWidth: 3 },
        data: {
          on: e.on ?? "completed",
          crossPhase: true,
          sourcePhase,
          targetPhase,
        },
      });
    } else {
      out.push(baseEdge);
    }
  }
  for (const c of workflow.cycles ?? []) {
    const sourcePhase = layout.jobPhaseMap.get(c.from) ?? "";
    const targetPhase = layout.jobPhaseMap.get(c.to) ?? "";
    const crossPhase =
      layout.hasExplicit &&
      sourcePhase !== "" &&
      targetPhase !== "" &&
      sourcePhase !== targetPhase;
    const className = crossPhase ? "cycle-edge cross-phase-edge" : "cycle-edge";
    out.push({
      id: `c-${c.from}->${c.to}`,
      source: c.from,
      target: c.to,
      label: `↻ ${c.on_verdict ?? "needs_revision"} ×${c.max_iterations ?? 1}`,
      className,
      style: crossPhase ? { stroke: "#000", strokeWidth: 3 } : undefined,
      data: {
        isCycle: true,
        on_verdict: c.on_verdict ?? "needs_revision",
        max_iterations: c.max_iterations ?? 1,
        ...(crossPhase ? { crossPhase: true, sourcePhase, targetPhase } : {}),
      },
    });
  }
  return out;
}

function syncWorkflowJobs(
  workflow: WorkflowDocument,
  jobs: WorkflowJob[],
): WorkflowDocument {
  return { ...workflow, jobs };
}

function syncWorkflowEdges(
  workflow: WorkflowDocument,
  edges: Edge[],
): WorkflowDocument {
  const repoEdges: WorkflowEdge[] = [];
  const cycles: WorkflowDocument["cycles"] = [];
  for (const e of edges) {
    const data = (e.data ?? {}) as InternalEdge;
    if (data.isCycle) {
      cycles!.push({
        from: e.source,
        to: e.target,
        on_verdict:
          (data as { on_verdict?: string }).on_verdict ?? "needs_revision",
        max_iterations:
          (data as { max_iterations?: number }).max_iterations ?? 1,
      });
    } else {
      const verdict = (data.on as WorkflowEdge["on"]) ?? "completed";
      // RFC 0045: derived cross-phase metadata is presentation-only and must
      // not be written back to workflow.json. Strip it here.
      repoEdges.push({ from: e.source, to: e.target, on: verdict });
    }
  }
  return { ...workflow, edges: repoEdges, cycles };
}

function newJobFromBlock(
  block: { kind: string; jobType: string },
  existing: WorkflowJob[],
): WorkflowJob {
  const baseId = block.kind;
  let n = 1;
  while (existing.some((j) => j.id === `${baseId}_${n}`)) n += 1;
  return {
    id: `${baseId}_${n}`,
    type: block.jobType,
    title: block.kind,
    role_id: "",
    lane_id: "",
    objective: "",
    task_prompt: { path: "" },
    write_scope:
      block.jobType === "review"
        ? {
            mode: "review_only_artifact_scope",
            allowed_paths: [],
            forbidden_paths: [".striatum/"],
          }
        : {
            mode: "repo_write",
            allowed_paths: [],
            forbidden_paths: [".striatum/"],
          },
    expected_artifacts: [],
  };
}

function safeArr<T>(v: T[] | undefined): T[] {
  return Array.isArray(v) ? v : [];
}

/**
 * Compute the phase id that a Y coordinate falls into. Returns `null` when
 * Y lies outside any band (above the first / below the last). Pure helper
 * exported for unit coverage.
 */
function phaseIdForY(layout: PhaseLayout, y: number): string | null {
  if (!layout.hasExplicit) return null;
  for (const entry of layout.phases) {
    const top = entry.bandTop;
    const bottom = top + PHASE_BAND_HEIGHT;
    if (y >= top && y < bottom) return entry.phase.id;
  }
  return null;
}

type GraphSelection =
  | { kind: "job"; id: string }
  | { kind: "phase"; id: string }
  | null;

interface PhaseBandsProps {
  layout: PhaseLayout;
  selectedPhaseId: string | null;
  onSelectPhase: (id: string) => void;
}

function PhaseBands({ layout, selectedPhaseId, onSelectPhase }: PhaseBandsProps) {
  if (!layout.hasExplicit) return null;
  // GH #6: reactflow 11.11.4 does not export `ViewportPortal` (added in v12).
  // The phase-band overlay needs the viewport transform to pan/zoom with the
  // graph, which requires either v12 or a manual `useViewport()` rebuild.
  // V1 ships without the overlay so the editor renders cleanly; the
  // overlay returns as RFC 0045 V1.5 polish (tracked in CHANGELOG +
  // RFC 0046/0047/0048 V1.7 backlog reflections).
  void layout;
  void selectedPhaseId;
  void onSelectPhase;
  void phaseDisplayName;
  void PHASE_BAND_WIDTH;
  void PHASE_BAND_HEIGHT;
  void PHASE_HEADER_HEIGHT;
  return null;
}

interface PhaseInspectorProps {
  phase: WorkflowPhase;
  jobs: WorkflowJob[];
  synthesisJob: WorkflowJob | undefined;
  selectedJobId?: string;
  onSelectJob: (jobId: string) => void;
  onChangePhase: (phaseId: string, patch: Partial<WorkflowPhase>) => void;
}

function PhaseInspector({
  phase,
  jobs,
  synthesisJob,
  selectedJobId,
  onSelectJob,
  onChangePhase,
}: PhaseInspectorProps) {
  return (
    <div className="graph-editor-inspector">
      <h3>Phase — {phase.id}</h3>
      <p className="phase-inspector-meta">
        {jobs.length} job{jobs.length === 1 ? "" : "s"}
        {synthesisJob ? ` · synthesis: ${synthesisJob.id}` : ""}
      </p>

      <div className="inspector-field">
        <label htmlFor={`phase-title-${phase.id}`}>title</label>
        <input
          id={`phase-title-${phase.id}`}
          value={phase.title ?? ""}
          onChange={(e) => onChangePhase(phase.id, { title: e.target.value })}
        />
      </div>

      <div className="inspector-field">
        <label htmlFor={`phase-description-${phase.id}`}>description</label>
        <textarea
          id={`phase-description-${phase.id}`}
          rows={3}
          value={phase.description ?? ""}
          onChange={(e) =>
            onChangePhase(phase.id, { description: e.target.value })
          }
        />
      </div>

      <div className="inspector-field">
        <label>synthesis_job_id</label>
        <p className="phase-inspector-meta">
          {phase.synthesis_job_id ?? "(none)"}
        </p>
      </div>

      <div className="inspector-field">
        <label>jobs in phase</label>
        <ul className="phase-inspector-jobs" aria-label={`jobs in ${phase.id}`}>
          {jobs.map((job) => (
            <li key={job.id}>
              <button
                type="button"
                aria-current={selectedJobId === job.id}
                onClick={() => onSelectJob(job.id)}
              >
                <strong>{job.id}</strong>
                {" — "}
                {job.type ?? "generic"}
                {job.lane_id ? ` · ${job.lane_id}` : ""}
                {job.parallel_group ? ` · ${job.parallel_group}` : ""}
              </button>
            </li>
          ))}
          {jobs.length === 0 && (
            <li>
              <span className="phase-inspector-meta">No jobs in this phase.</span>
            </li>
          )}
        </ul>
      </div>
    </div>
  );
}

interface InspectorProps {
  job: WorkflowJob | undefined;
  workflow: WorkflowDocument;
  onChange: (jobId: string, patch: Partial<WorkflowJob>) => void;
  onDelete: (jobId: string) => void;
}

function Inspector({ job, workflow, onChange, onDelete }: InspectorProps) {
  if (!job) {
    return (
      <div className="graph-editor-inspector">
        <h3>Inspector</h3>
        <p className="muted">Select a node to edit its fields.</p>
      </div>
    );
  }
  const roleOptions = [""].concat(Object.keys(workflow.roles ?? {}));
  const laneOptions = [""].concat(Object.keys(workflow.lanes ?? {}));
  const phases = workflow.phases ?? [];
  const allowedPaths = safeArr(job.write_scope?.allowed_paths);
  const forbiddenPaths = safeArr(job.write_scope?.forbidden_paths);
  const requiredPostures = safeArr(job.required_review_postures);
  const expectedArtifacts = safeArr(job.expected_artifacts);

  const updateWriteScope = (
    patch: Partial<NonNullable<WorkflowJob["write_scope"]>>,
  ) => {
    const next = { ...(job.write_scope ?? {}), ...patch };
    onChange(job.id, { write_scope: next });
  };

  return (
    <div className="graph-editor-inspector">
      <h3>Inspector — {job.id}</h3>

      <div className="inspector-field">
        <label htmlFor={`inspector-id-${job.id}`}>id</label>
        <input
          id={`inspector-id-${job.id}`}
          value={job.id}
          onChange={(e) => onChange(job.id, { id: e.target.value })}
        />
      </div>

      <div className="inspector-field">
        <label htmlFor={`inspector-type-${job.id}`}>type</label>
        <select
          id={`inspector-type-${job.id}`}
          value={job.type ?? "generic"}
          onChange={(e) => onChange(job.id, { type: e.target.value })}
        >
          {JOB_TYPES.map((t) => (
            <option key={t} value={t}>
              {t}
            </option>
          ))}
        </select>
      </div>

      <div className="inspector-field">
        <label htmlFor={`inspector-title-${job.id}`}>title</label>
        <input
          id={`inspector-title-${job.id}`}
          value={job.title ?? ""}
          onChange={(e) => onChange(job.id, { title: e.target.value })}
        />
      </div>

      <div className="inspector-field">
        <label htmlFor={`inspector-objective-${job.id}`}>objective</label>
        <textarea
          id={`inspector-objective-${job.id}`}
          rows={3}
          value={job.objective ?? ""}
          onChange={(e) => onChange(job.id, { objective: e.target.value })}
        />
      </div>

      <div className="inspector-field">
        <label htmlFor={`inspector-role-${job.id}`}>role_id</label>
        <select
          id={`inspector-role-${job.id}`}
          value={job.role_id ?? ""}
          onChange={(e) => onChange(job.id, { role_id: e.target.value })}
        >
          {roleOptions.map((r) => (
            <option key={r} value={r}>
              {r || "(none)"}
            </option>
          ))}
        </select>
      </div>

      <div className="inspector-field">
        <label htmlFor={`inspector-lane-${job.id}`}>lane_id</label>
        <select
          id={`inspector-lane-${job.id}`}
          value={job.lane_id ?? ""}
          onChange={(e) => onChange(job.id, { lane_id: e.target.value })}
        >
          {laneOptions.map((l) => (
            <option key={l} value={l}>
              {l || "(none)"}
            </option>
          ))}
        </select>
      </div>

      {phases.length > 0 && (
        <div className="inspector-field">
          <label htmlFor={`inspector-phase-${job.id}`}>phase</label>
          <select
            id={`inspector-phase-${job.id}`}
            value={jobPhaseId(job)}
            onChange={(e) =>
              onChange(job.id, { phase: e.target.value || undefined })
            }
          >
            <option value="">(unset)</option>
            {phases.map((p) => (
              <option key={p.id} value={p.id}>
                {phaseDisplayName(p)}
              </option>
            ))}
          </select>
        </div>
      )}

      <div className="inspector-field">
        <label>write_scope.mode</label>
        <fieldset
          style={{ display: "flex", gap: 8, border: 0, padding: 0 }}
          aria-label="write_scope mode"
        >
          {WRITE_SCOPE_MODES.map((mode) => (
            <label
              key={mode}
              style={{ display: "flex", gap: 4, alignItems: "center" }}
            >
              <input
                type="radio"
                name={`write-scope-mode-${job.id}`}
                value={mode}
                checked={(job.write_scope?.mode ?? "repo_write") === mode}
                onChange={() => updateWriteScope({ mode })}
              />
              {mode}
            </label>
          ))}
        </fieldset>
      </div>

      <div className="inspector-field">
        <label>write_scope.allowed_paths</label>
        <RepeatingPathField
          values={allowedPaths}
          onChange={(values) => updateWriteScope({ allowed_paths: values })}
          ariaLabel="allowed paths"
        />
      </div>

      <div className="inspector-field">
        <label>write_scope.forbidden_paths</label>
        <RepeatingPathField
          values={forbiddenPaths}
          onChange={(values) => updateWriteScope({ forbidden_paths: values })}
          ariaLabel="forbidden paths"
        />
      </div>

      {job.type === "review" && (
        <>
          <div className="inspector-field">
            <label style={{ display: "flex", gap: 6, alignItems: "center" }}>
              <input
                type="checkbox"
                checked={job.require_attested_lane === true}
                onChange={(e) =>
                  onChange(job.id, {
                    require_attested_lane: e.target.checked || undefined,
                  })
                }
              />
              require_attested_lane
            </label>
          </div>
          <div className="inspector-field">
            <label>review_posture</label>
            <fieldset
              style={{
                display: "grid",
                gridTemplateColumns: "1fr 1fr",
                gap: 4,
                border: 0,
                padding: 0,
              }}
              aria-label="review posture"
            >
              {ALLOWED_REVIEW_POSTURES.map((posture) => (
                <label
                  key={posture}
                  style={{ display: "flex", gap: 4, alignItems: "center" }}
                >
                  <input
                    type="radio"
                    name={`review-posture-${job.id}`}
                    value={posture}
                    checked={job.review_posture === posture}
                    onChange={() =>
                      onChange(job.id, { review_posture: posture })
                    }
                  />
                  {posture}
                </label>
              ))}
            </fieldset>
          </div>
          <div className="inspector-field">
            <label htmlFor={`inspector-access-${job.id}`}>
              reviewer_access_scope
            </label>
            <select
              id={`inspector-access-${job.id}`}
              value={job.reviewer_access_scope ?? ""}
              onChange={(e) =>
                onChange(job.id, {
                  reviewer_access_scope: e.target.value || undefined,
                })
              }
            >
              <option value="">(unset)</option>
              {REVIEWER_ACCESS_SCOPES.map((scope) => (
                <option key={scope} value={scope}>
                  {scope}
                </option>
              ))}
            </select>
          </div>
          <div className="inspector-field">
            <label htmlFor={`inspector-context-${job.id}`}>
              reviewer_context_policy
            </label>
            <select
              id={`inspector-context-${job.id}`}
              value={job.reviewer_context_policy ?? ""}
              onChange={(e) =>
                onChange(job.id, {
                  reviewer_context_policy: e.target.value || undefined,
                })
              }
            >
              <option value="">(unset)</option>
              {REVIEWER_CONTEXT_POLICIES.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>
        </>
      )}

      {job.type === "build" && (
        <div className="inspector-field">
          <label>required_review_postures</label>
          <ChipMultiSelect
            options={[...ALLOWED_REVIEW_POSTURES]}
            values={requiredPostures}
            onChange={(values) =>
              onChange(job.id, {
                required_review_postures: values.length ? values : undefined,
              })
            }
            ariaLabel="required review postures"
          />
        </div>
      )}

      <div className="inspector-field">
        <label htmlFor={`inspector-parallel-${job.id}`}>parallel_group</label>
        <input
          id={`inspector-parallel-${job.id}`}
          value={job.parallel_group ?? ""}
          onChange={(e) =>
            onChange(job.id, { parallel_group: e.target.value || undefined })
          }
        />
      </div>

      <div className="inspector-field">
        <label>expected_artifacts ({expectedArtifacts.length})</label>
        <ExpectedArtifactsEditor
          values={expectedArtifacts}
          onChange={(values) =>
            onChange(job.id, { expected_artifacts: values })
          }
        />
      </div>

      <div className="inspector-field">
        <button
          type="button"
          className="secondary-button"
          onClick={() => onDelete(job.id)}
        >
          Delete job
        </button>
      </div>
    </div>
  );
}

interface RepeatingPathFieldProps {
  values: string[];
  onChange: (values: string[]) => void;
  ariaLabel: string;
}

function RepeatingPathField({
  values,
  onChange,
  ariaLabel,
}: RepeatingPathFieldProps) {
  return (
    <div aria-label={ariaLabel}>
      {values.map((value, index) => (
        <div className="repeating-row" key={index}>
          <input
            value={value}
            onChange={(e) => {
              const next = values.slice();
              next[index] = e.target.value;
              onChange(next);
            }}
          />
          <button
            type="button"
            className="secondary-button compact-button"
            aria-label={`Remove ${ariaLabel} entry ${index + 1}`}
            onClick={() => {
              const next = values.slice();
              next.splice(index, 1);
              onChange(next);
            }}
          >
            ×
          </button>
        </div>
      ))}
      <button
        type="button"
        className="secondary-button compact-button"
        onClick={() => onChange([...values, ""])}
      >
        + Add path
      </button>
    </div>
  );
}

interface ChipMultiSelectProps {
  options: string[];
  values: string[];
  onChange: (values: string[]) => void;
  ariaLabel: string;
}

function ChipMultiSelect({
  options,
  values,
  onChange,
  ariaLabel,
}: ChipMultiSelectProps) {
  const remaining = options.filter((o) => !values.includes(o));
  return (
    <div aria-label={ariaLabel}>
      <div className="chip-set">
        {values.map((v) => (
          <span className="chip" key={v}>
            {v}
            <button
              type="button"
              aria-label={`Remove ${v}`}
              onClick={() => onChange(values.filter((x) => x !== v))}
            >
              ×
            </button>
          </span>
        ))}
      </div>
      {remaining.length > 0 && (
        <select
          value=""
          onChange={(e) => {
            if (!e.target.value) return;
            onChange([...values, e.target.value]);
          }}
        >
          <option value="">+ Add…</option>
          {remaining.map((o) => (
            <option key={o} value={o}>
              {o}
            </option>
          ))}
        </select>
      )}
    </div>
  );
}

interface ExpectedArtifactsEditorProps {
  values: NonNullable<WorkflowJob["expected_artifacts"]>;
  onChange: (values: NonNullable<WorkflowJob["expected_artifacts"]>) => void;
}

function ExpectedArtifactsEditor({
  values,
  onChange,
}: ExpectedArtifactsEditorProps) {
  return (
    <div>
      {values.map((entry, index) => (
        <div
          key={index}
          style={{
            border: "1px solid var(--border)",
            borderRadius: "var(--radius)",
            padding: "var(--space-2)",
            marginBottom: "var(--space-1)",
            background: "var(--bg-base)",
          }}
        >
          <div className="repeating-row">
            <input
              placeholder="kind"
              value={entry.kind ?? ""}
              onChange={(e) => {
                const next = values.slice();
                next[index] = { ...entry, kind: e.target.value };
                onChange(next);
              }}
            />
            <input
              placeholder="logical_name"
              value={entry.logical_name ?? ""}
              onChange={(e) => {
                const next = values.slice();
                next[index] = { ...entry, logical_name: e.target.value };
                onChange(next);
              }}
            />
            <button
              type="button"
              className="secondary-button compact-button"
              aria-label={`Remove expected artifact ${index + 1}`}
              onClick={() => {
                const next = values.slice();
                next.splice(index, 1);
                onChange(next);
              }}
            >
              ×
            </button>
          </div>
          <input
            placeholder="path"
            style={{ width: "100%", marginTop: 4 }}
            value={entry.path ?? ""}
            onChange={(e) => {
              const next = values.slice();
              next[index] = { ...entry, path: e.target.value };
              onChange(next);
            }}
          />
          <input
            placeholder="author_line"
            style={{ width: "100%", marginTop: 4 }}
            value={entry.author_line ?? ""}
            onChange={(e) => {
              const next = values.slice();
              next[index] = { ...entry, author_line: e.target.value };
              onChange(next);
            }}
          />
          <label style={{ display: "block", marginTop: 4 }}>
            <input
              type="checkbox"
              checked={entry.required ?? false}
              onChange={(e) => {
                const next = values.slice();
                next[index] = { ...entry, required: e.target.checked };
                onChange(next);
              }}
            />
            required
          </label>
        </div>
      ))}
      <button
        type="button"
        className="secondary-button compact-button"
        onClick={() => onChange([...values, { kind: "other", required: true }])}
      >
        + Add artifact
      </button>
    </div>
  );
}

function WorkflowGraphEditorImpl(props: WorkflowGraphEditorProps) {
  const workflowDataElementId = props.workflowDataElementId ?? "workflow-data";
  const workflowSha256ElementId =
    props.workflowSha256ElementId ?? "workflow-sha256";
  const cancelUrl = props.cancelUrl ?? "/workflows/" + props.path;

  const initialWorkflow = useMemo<WorkflowDocument>(
    () => readJsonPayload<WorkflowDocument>(workflowDataElementId, { jobs: [] }),
    [workflowDataElementId],
  );
  const diskSha256 = useMemo<string>(
    () => readJsonPayload<string>(workflowSha256ElementId, ""),
    [workflowSha256ElementId],
  );

  const [workflow, setWorkflow] = useState<WorkflowDocument>(initialWorkflow);
  const [nodes, setNodes] = useState<Node[]>(() => jobsToNodes(initialWorkflow));
  const [edges, setEdges] = useState<Edge[]>(() =>
    workflowToEdges(initialWorkflow),
  );
  const [selection, setSelection] = useState<GraphSelection>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [stale, setStale] = useState(false);
  const [dragError, setDragError] = useState<string | null>(null);

  const layout = useMemo(() => buildPhaseLayout(workflow), [workflow]);

  useEffect(() => {
    setWorkflow((prev) => syncWorkflowEdges(prev, edges));
  }, [edges]);

  const onNodesChange = useCallback(
    (changes: NodeChange[]) => {
      if (!layout.hasExplicit) {
        setNodes((ns) => applyNodeChanges(changes, ns));
        return;
      }
      let refusedJobId: string | null = null;
      setNodes((ns) => {
        const priorById = new Map(
          ns.map((n) => [n.id, { x: n.position.x, y: n.position.y }]),
        );
        const filtered = changes.map((change) => {
          if (change.type === "position" && change.position) {
            const declared = layout.jobPhaseMap.get(change.id);
            const destPhase = phaseIdForY(layout, change.position.y);
            if (destPhase && declared && destPhase !== declared) {
              refusedJobId = change.id;
              const prior = priorById.get(change.id) ?? change.position;
              return {
                ...change,
                position: { x: prior.x, y: prior.y },
              };
            }
          }
          return change;
        });
        return applyNodeChanges(filtered, ns);
      });
      if (refusedJobId) {
        const declared = layout.jobPhaseMap.get(refusedJobId) ?? "?";
        setDragError(
          `Drag refused: ${refusedJobId} belongs to phase ${declared}. Use the inspector phase field to move it, then add the required phase_synthesis dependency.`,
        );
      } else if (
        changes.some((c) => c.type === "position" && c.dragging === false)
      ) {
        setDragError(null);
      }
    },
    [layout],
  );
  const onEdgesChange = useCallback(
    (changes: EdgeChange[]) => setEdges((es) => applyEdgeChanges(changes, es)),
    [],
  );
  const onConnect = useCallback((conn: Connection) => {
    setEdges((es) =>
      addEdge(
        {
          ...conn,
          id: `e-${conn.source}->${conn.target}-${Date.now()}`,
          label: "completed",
          data: { on: "completed" },
        },
        es,
      ),
    );
  }, []);

  const handleAddBlock = (block: {
    kind: string;
    jobType: string;
    label: string;
  }) => {
    setWorkflow((prev) => {
      const jobs = safeArr(prev.jobs);
      const job = newJobFromBlock(block, jobs);
      const nextJobs = [...jobs, job];
      const next = syncWorkflowJobs(prev, nextJobs);
      setNodes(jobsToNodes(next));
      setEdges(workflowToEdges(next));
      return next;
    });
  };

  const handleJobChange = (jobId: string, patch: Partial<WorkflowJob>) => {
    setWorkflow((prev) => {
      const typeTouched = Object.prototype.hasOwnProperty.call(patch, "type");
      const jobs = safeArr(prev.jobs).map((j) => {
        if (j.id !== jobId) return j;
        const merged = { ...j, ...patch };
        // GH #13: when the type changes away from `review`, drop the
        // review-only fields so the saved JSON, node label, and textual
        // summary stop carrying ghost state for the new type.
        return typeTouched
          ? purgeStaleFieldsForType(merged, j.type, patch.type)
          : merged;
      });
      const renamed = patch.id && patch.id !== jobId;
      const next = syncWorkflowJobs(prev, jobs);
      const phaseTouched = Object.prototype.hasOwnProperty.call(patch, "phase");
      if (renamed) {
        // A rename means existing node/edge ids need to follow. Re-derive both
        // from the workflow so phase-aware layout and cross-phase edge
        // tagging stay consistent.
        setNodes(jobsToNodes(next));
        setEdges(workflowToEdges(next));
        setSelection({ kind: "job", id: patch.id as string });
      } else if (phaseTouched) {
        setNodes(jobsToNodes(next));
        setEdges(workflowToEdges(next));
      } else {
        setNodes((ns) =>
          ns.map((n) => {
            const updatedJob = jobs.find((j) => j.id === jobId);
            return n.id === jobId && updatedJob
              ? {
                  ...n,
                  data: {
                    ...n.data,
                    label: jobNodeLabel(updatedJob),
                  },
                }
              : n;
          }),
        );
      }
      return next;
    });
  };

  const handleJobDelete = (jobId: string) => {
    setWorkflow((prev) => {
      const jobs = safeArr(prev.jobs).filter((j) => j.id !== jobId);
      const next = syncWorkflowJobs(prev, jobs);
      setNodes((ns) => ns.filter((n) => n.id !== jobId));
      setEdges((es) => es.filter((e) => e.source !== jobId && e.target !== jobId));
      setSelection((curr) =>
        curr && curr.kind === "job" && curr.id === jobId ? null : curr,
      );
      return next;
    });
  };

  const handlePhaseChange = (
    phaseId: string,
    patch: Partial<WorkflowPhase>,
  ) => {
    setWorkflow((prev) => {
      const phases = (prev.phases ?? []).map((p) =>
        p.id === phaseId ? { ...p, ...patch } : p,
      );
      return { ...prev, phases };
    });
  };

  const handleSave = async () => {
    setError(null);
    setSaving(true);
    setStale(false);
    const body = syncWorkflowEdges(workflow, edges);
    const res = await saveWorkflow(props.path, body, diskSha256, props.saveUrl);
    setSaving(false);
    if (res.status === 200) {
      window.location.href = "/workflows/" + props.path;
      return;
    }
    if (res.status === 412) {
      setStale(true);
      return;
    }
    const errBody = (res.body as { error?: { message?: string } } | null)?.error;
    setError(errBody?.message ?? `Save failed (${res.status})`);
  };

  const selectedJob =
    selection?.kind === "job"
      ? workflow.jobs?.find((j) => j.id === selection.id)
      : undefined;
  const selectedPhaseEntry =
    selection?.kind === "phase" ? layout.byPhaseId.get(selection.id) : undefined;

  const textualSummary = useMemo(() => {
    const lines: string[] = [];
    for (const p of safeArr(workflow.phases)) {
      lines.push(`Phase ${p.id} (${phaseDisplayName(p)})`);
    }
    for (const j of safeArr(workflow.jobs)) {
      const phase = jobPhaseId(j);
      const requireAttested =
        j.require_attested_lane === true ? " require_attested_lane=true" : "";
      lines.push(
        `Job ${j.id} (${j.type ?? "generic"})${
          phase ? ` phase=${phase}` : ""
        }${requireAttested}`,
      );
    }
    for (const e of safeArr(workflow.edges)) {
      lines.push(`Edge ${e.from} -> ${e.to} on ${e.on ?? "completed"}`);
    }
    for (const c of safeArr(workflow.cycles)) {
      lines.push(
        `Cycle ${c.from} -> ${c.to} on ${c.on_verdict ?? "needs_revision"} max ${
          c.max_iterations ?? 1
        }`,
      );
    }
    return lines.join("\n");
  }, [workflow]);

  const flowStyle: CSSProperties = { width: "100%", height: "100%" };

  const phaseJobs = selectedPhaseEntry
    ? safeArr(workflow.jobs).filter(
        (j) => layout.jobPhaseMap.get(j.id) === selectedPhaseEntry.phase.id,
      )
    : [];
  const synthesisJob = selectedPhaseEntry?.phase.synthesis_job_id
    ? safeArr(workflow.jobs).find(
        (j) => j.id === selectedPhaseEntry.phase.synthesis_job_id,
      )
    : undefined;

  return (
    <div className="island-root">
      <div className="graph-editor">
        <div className="graph-editor-palette" aria-label="Block palette">
          <h3>Add block</h3>
          {PALETTE_BLOCKS.map((block) => (
            <button
              key={block.kind}
              type="button"
              onClick={() => handleAddBlock(block)}
            >
              + {block.label}
            </button>
          ))}
        </div>
        <div className="graph-editor-canvas">
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onNodeClick={(_, node) =>
              setSelection({ kind: "job", id: node.id })
            }
            onPaneClick={() => setSelection(null)}
            fitView
            style={flowStyle}
          >
            <PhaseBands
              layout={layout}
              selectedPhaseId={
                selection?.kind === "phase" ? selection.id : null
              }
              onSelectPhase={(id) => setSelection({ kind: "phase", id })}
            />
            <Background />
            <MiniMap pannable zoomable />
            <Controls />
          </ReactFlow>
          {dragError && (
            <div
              className="graph-editor-phase-drag-error"
              role="alert"
              aria-live="polite"
            >
              {dragError}
            </div>
          )}
          <div
            className="graph-editor-textual"
            role="region"
            aria-label="Workflow graph textual summary"
          >
            {textualSummary}
          </div>
        </div>
        {selectedPhaseEntry ? (
          <PhaseInspector
            phase={selectedPhaseEntry.phase}
            jobs={phaseJobs}
            synthesisJob={synthesisJob}
            selectedJobId={
              selection?.kind === "job" ? selection.id : undefined
            }
            onSelectJob={(jobId) => setSelection({ kind: "job", id: jobId })}
            onChangePhase={handlePhaseChange}
          />
        ) : (
          <Inspector
            job={selectedJob}
            workflow={workflow}
            onChange={handleJobChange}
            onDelete={handleJobDelete}
          />
        )}
      </div>
      {error && (
        <div className="island-error" role="alert">
          {error}
        </div>
      )}
      {stale && (
        <div className="island-error" role="alert">
          File changed on disk. Reload to fetch the new contents.
        </div>
      )}
      <div className="graph-editor-actions">
        <button
          type="button"
          className="primary-button"
          onClick={() => void handleSave()}
          disabled={saving}
        >
          {saving ? "Saving…" : "Save workflow"}
        </button>
        <a className="secondary-button" href={cancelUrl}>
          Cancel
        </a>
      </div>
    </div>
  );
}

export default function WorkflowGraphEditor(props: WorkflowGraphEditorProps) {
  return (
    <ReactFlowProvider>
      <WorkflowGraphEditorImpl {...props} />
    </ReactFlowProvider>
  );
}

export const __testing = {
  jobsToNodes,
  jobNodeLabel,
  workflowToEdges,
  syncWorkflowEdges,
  syncWorkflowJobs,
  newJobFromBlock,
  purgeStaleFieldsForType,
  REVIEW_ONLY_JOB_FIELDS,
  PALETTE_BLOCKS,
  buildPhaseLayout,
  hasExplicitPhases,
  jobPhaseId,
  phaseIdForY,
  PHASE_BAND_HEIGHT,
  PHASE_NODE_TOP,
  PHASE_COLUMN_WIDTH,
  PHASE_ROW_HEIGHT,
};
