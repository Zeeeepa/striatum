/**
 * RFC 0038 V1 — drag-drop workflow graph editor island.
 *
 * Renders a React Flow canvas over the workflow JSON loaded from a
 * `<script id="workflow-data" type="application/json">` payload that the
 * Jinja2 template renders next to the mount slot. Nodes are workflow jobs;
 * edges are workflow `edges`; cycles render with the `cycle-edge` class for
 * distinct styling. A left palette adds new jobs from the closed RFC 0034
 * block vocabulary; a right inspector edits the selected job with structured
 * widgets per RFC 0038 design synthesis F5.
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

interface InternalEdge extends WorkflowEdge {
  isCycle?: boolean;
  maxIterations?: number;
}

function jobsToNodes(workflow: WorkflowDocument): Node[] {
  const jobs = workflow.jobs ?? [];
  const cols = Math.max(1, Math.ceil(Math.sqrt(jobs.length)));
  return jobs.map((job, index) => ({
    id: job.id,
    type: "default",
    position: {
      x: (index % cols) * 220,
      y: Math.floor(index / cols) * 140,
    },
    data: { label: `${job.id}\n${job.type ?? "generic"}` },
  }));
}

function workflowToEdges(workflow: WorkflowDocument): Edge[] {
  const out: Edge[] = [];
  for (const e of workflow.edges ?? []) {
    out.push({
      id: `e-${e.from}->${e.to}-${e.on ?? "completed"}`,
      source: e.from,
      target: e.to,
      label: e.on ?? "completed",
      data: { on: e.on ?? "completed" },
    });
  }
  for (const c of workflow.cycles ?? []) {
    out.push({
      id: `c-${c.from}->${c.to}`,
      source: c.from,
      target: c.to,
      label: `↻ ${c.on_verdict ?? "needs_revision"} ×${c.max_iterations ?? 1}`,
      className: "cycle-edge",
      data: {
        isCycle: true,
        on_verdict: c.on_verdict ?? "needs_revision",
        max_iterations: c.max_iterations ?? 1,
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
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [stale, setStale] = useState(false);

  useEffect(() => {
    setWorkflow((prev) => syncWorkflowEdges(prev, edges));
  }, [edges]);

  const onNodesChange = useCallback(
    (changes: NodeChange[]) => setNodes((ns) => applyNodeChanges(changes, ns)),
    [],
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
      return next;
    });
  };

  const handleJobChange = (jobId: string, patch: Partial<WorkflowJob>) => {
    setWorkflow((prev) => {
      const jobs = safeArr(prev.jobs).map((j) =>
        j.id === jobId ? { ...j, ...patch } : j,
      );
      const renamed = patch.id && patch.id !== jobId;
      const next = syncWorkflowJobs(prev, jobs);
      if (renamed) {
        setNodes((ns) =>
          ns.map((n) =>
            n.id === jobId
              ? {
                  ...n,
                  id: patch.id as string,
                  data: {
                    ...n.data,
                    label: `${patch.id}\n${
                      jobs.find((j) => j.id === patch.id)?.type ?? "generic"
                    }`,
                  },
                }
              : n,
          ),
        );
        setEdges((es) =>
          es.map((e) => ({
            ...e,
            source: e.source === jobId ? (patch.id as string) : e.source,
            target: e.target === jobId ? (patch.id as string) : e.target,
          })),
        );
        setSelectedJobId(patch.id as string);
      } else {
        setNodes((ns) =>
          ns.map((n) =>
            n.id === jobId
              ? {
                  ...n,
                  data: {
                    ...n.data,
                    label: `${jobId}\n${
                      jobs.find((j) => j.id === jobId)?.type ?? "generic"
                    }`,
                  },
                }
              : n,
          ),
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
      setSelectedJobId((curr) => (curr === jobId ? null : curr));
      return next;
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

  const selectedJob = workflow.jobs?.find((j) => j.id === selectedJobId);

  const textualSummary = useMemo(() => {
    const lines: string[] = [];
    for (const j of safeArr(workflow.jobs)) {
      lines.push(`Job ${j.id} (${j.type ?? "generic"})`);
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
            onNodeClick={(_, node) => setSelectedJobId(node.id)}
            onPaneClick={() => setSelectedJobId(null)}
            fitView
            style={flowStyle}
          >
            <Background />
            <MiniMap pannable zoomable />
            <Controls />
          </ReactFlow>
          <div
            className="graph-editor-textual"
            role="region"
            aria-label="Workflow graph textual summary"
          >
            {textualSummary}
          </div>
        </div>
        <Inspector
          job={selectedJob}
          workflow={workflow}
          onChange={handleJobChange}
          onDelete={handleJobDelete}
        />
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
  workflowToEdges,
  syncWorkflowEdges,
  syncWorkflowJobs,
  newJobFromBlock,
  PALETTE_BLOCKS,
};
