/**
 * RFC 0038 V1.5 — `/workflows/new` chooser wizard island.
 *
 * V1.5 consumes the flat `{ templates: WorkflowTemplate[] }` response that
 * the server's `/workflow-templates` route has always emitted (see
 * `service._handle_workflow_templates` → `catalog.list_templates`). The
 * V1 component expected a `{ shapes, lane_sets, modifiers }` envelope that
 * never existed; that mismatch left the chooser stuck on the loading state
 * in production.
 *
 * The wizard walks the operator through four steps:
 *   1. Template — pick a `kind: "shape"` row, which derives the workflow
 *      `shape` and the candidate `default_lane_sets`.
 *   2. Details — workflow_id, name, scaffold_root, artifact_root,
 *      branch_suggestion, lane_set.
 *   3. Preview — POST `previewUrl` and render `GeneratedWorkflow.files`.
 *   4. Save — confirm + POST `generateUrl` with `confirm_write: true`.
 *
 * Editing any field after visiting the preview step invalidates the
 * preview; the save step always runs through a `<dialog>` confirmation.
 */

import { useEffect, useMemo, useRef, useState } from "react";
import {
  fetchWorkflowTemplates,
  generateWorkflowPreview,
  generateWorkflowWrite,
  type GenerationSpec,
} from "../../shared/api-client";
import type {
  GeneratedWorkflow,
  WorkflowChooserProps,
  WorkflowTemplate,
} from "../../shared/types";

interface FormState {
  shape: string;
  laneSet: string;
  workflowId: string;
  name: string;
  scaffoldRoot: string;
  artifactRoot: string;
  branchSuggestion: string;
  laneCommands: Record<string, string>;
}

interface PreviewState {
  status: "idle" | "loading" | "ready" | "error";
  generatedAt?: number;
  data?: GeneratedWorkflow;
  error?: string;
}

interface CatalogState {
  status: "idle" | "loading" | "ready" | "error";
  templates?: WorkflowTemplate[];
  error?: string;
}

const STEP_LABELS = ["Template", "Details", "Preview", "Save"];

function recommendedForText(value: string | string[] | undefined): string {
  if (!value) return "";
  return Array.isArray(value) ? value.join("; ") : value;
}

function buildSpec(form: FormState): GenerationSpec {
  const laneCommands: Record<string, string[]> = {};
  for (const [laneId, raw] of Object.entries(form.laneCommands)) {
    const parts = raw
      .split(/\s+/)
      .map((p) => p.trim())
      .filter(Boolean);
    if (parts.length > 0) laneCommands[laneId] = parts;
  }
  return {
    shape: form.shape,
    lane_set: form.laneSet,
    modifiers: [],
    workflow_id: form.workflowId,
    name: form.name || form.workflowId,
    scaffold_root: form.scaffoldRoot,
    artifact_root: form.artifactRoot,
    branch_suggestion: form.branchSuggestion || undefined,
    lane_commands: Object.keys(laneCommands).length ? laneCommands : undefined,
  };
}

export default function WorkflowChooser(props: WorkflowChooserProps) {
  const [catalogState, setCatalogState] = useState<CatalogState>({ status: "idle" });
  const [step, setStep] = useState<number>(0);
  const [form, setForm] = useState<FormState>({
    shape: "",
    laneSet: "",
    workflowId: "",
    name: "",
    scaffoldRoot: props.defaultScaffoldRoot ?? "docs/dogfood/<id>/",
    artifactRoot: props.defaultArtifactRoot ?? "docs/dogfood/<id>/",
    branchSuggestion: "",
    laneCommands: {},
  });
  const [preview, setPreview] = useState<PreviewState>({ status: "idle" });
  const [writeStatus, setWriteStatus] = useState<{
    status: "idle" | "loading" | "ok" | "error";
    error?: string;
  }>({ status: "idle" });
  const dialogRef = useRef<HTMLDialogElement | null>(null);

  useEffect(() => {
    setCatalogState({ status: "loading" });
    void fetchWorkflowTemplates(props.templatesUrl).then((res) => {
      if (!res.ok) {
        setCatalogState({ status: "error", error: res.error.message });
        return;
      }
      setCatalogState({ status: "ready", templates: res.data.templates });
    });
  }, [props.templatesUrl]);

  const templates = catalogState.templates ?? [];
  const shapes = useMemo(
    () => templates.filter((t) => t.kind === "shape"),
    [templates],
  );
  const laneSets = useMemo(
    () => templates.filter((t) => t.kind === "lane_set"),
    [templates],
  );

  const selectedShape: WorkflowTemplate | null = useMemo(
    () => shapes.find((s) => s.template_id === form.shape) ?? null,
    [shapes, form.shape],
  );

  const filteredLaneSets: WorkflowTemplate[] = useMemo(() => {
    if (!selectedShape) return laneSets;
    const ids = selectedShape.default_lane_sets ?? [];
    if (ids.length === 0) return laneSets;
    return laneSets.filter((ls) => ids.includes(ls.template_id));
  }, [laneSets, selectedShape]);

  const updateForm = (patch: Partial<FormState>) => {
    setForm((prev) => ({ ...prev, ...patch }));
    if (preview.status === "ready" || preview.status === "error") {
      setPreview({ status: "idle" });
    }
  };

  const canAdvance = useMemo(() => {
    switch (step) {
      case 0:
        return Boolean(form.shape);
      case 1:
        return (
          Boolean(form.laneSet) &&
          Boolean(form.workflowId.trim()) &&
          Boolean(form.scaffoldRoot.trim()) &&
          Boolean(form.artifactRoot.trim())
        );
      case 2:
        return preview.status === "ready";
      default:
        return false;
    }
  }, [step, form, preview.status]);

  useEffect(() => {
    if (step !== 2) return;
    if (preview.status === "ready" || preview.status === "loading") return;
    setPreview({ status: "loading" });
    void generateWorkflowPreview(buildSpec(form), props.previewUrl).then((res) => {
      if (!res.ok) {
        setPreview({ status: "error", error: res.error.message });
        return;
      }
      setPreview({
        status: "ready",
        generatedAt: Date.now(),
        data: res.data,
      });
    });
  }, [step, preview.status, form, props.previewUrl]);

  const openConfirm = () => {
    if (!props.allowMutations) return;
    if (preview.status !== "ready") return;
    dialogRef.current?.showModal();
  };

  const cancelConfirm = () => {
    dialogRef.current?.close();
  };

  const performWrite = async () => {
    setWriteStatus({ status: "loading" });
    const res = await generateWorkflowWrite(buildSpec(form), props.generateUrl);
    if (!res.ok) {
      setWriteStatus({ status: "error", error: res.error.message });
      return;
    }
    setWriteStatus({ status: "ok" });
    dialogRef.current?.close();
    const firstFile = res.data.files?.[0]?.path;
    if (firstFile) {
      window.location.href = "/workflows/" + firstFile;
    }
  };

  if (catalogState.status === "loading" || catalogState.status === "idle") {
    return (
      <div className="island-root chooser">
        <div className="island-loading" role="status">
          Loading workflow templates…
        </div>
      </div>
    );
  }
  if (catalogState.status === "error") {
    return (
      <div className="island-root chooser">
        <div className="island-error" role="alert">
          Failed to load workflow templates:{" "}
          {catalogState.error ?? "unknown error"}
        </div>
      </div>
    );
  }
  if (shapes.length === 0) {
    return (
      <div className="island-root chooser">
        <div className="island-error" role="alert">
          The workflow template catalog returned no shapes.
        </div>
      </div>
    );
  }

  const renderTemplateStep = () => (
    <div role="radiogroup" aria-label="Workflow template" className="chooser-cards">
      {shapes.map((shape) => (
        <button
          type="button"
          role="radio"
          aria-checked={form.shape === shape.template_id}
          key={shape.template_id}
          className="chooser-card"
          onClick={() => {
            const firstLaneSet =
              (shape.default_lane_sets ?? []).find((id) =>
                laneSets.some((ls) => ls.template_id === id),
              ) ?? "";
            updateForm({ shape: shape.template_id, laneSet: firstLaneSet });
          }}
        >
          <h3>{shape.display_name}</h3>
          <div className="summary">{shape.summary}</div>
          <div className="recommended-for">
            Use it for: {recommendedForText(shape.recommended_for)}
          </div>
        </button>
      ))}
    </div>
  );

  const renderDetailsStep = () => (
    <div className="chooser-fields">
      <label htmlFor="chooser-lane-set">lane_set</label>
      <select
        id="chooser-lane-set"
        value={form.laneSet}
        onChange={(e) => updateForm({ laneSet: e.target.value })}
      >
        <option value="" disabled>
          — select a lane set —
        </option>
        {filteredLaneSets.map((ls) => (
          <option key={ls.template_id} value={ls.template_id}>
            {ls.display_name}
          </option>
        ))}
      </select>
      {filteredLaneSets.length === 0 && (
        <div className="island-loading">
          No lane sets are recommended for this template.
        </div>
      )}
      <label htmlFor="chooser-workflow-id">workflow_id</label>
      <input
        id="chooser-workflow-id"
        value={form.workflowId}
        required
        onChange={(e) => updateForm({ workflowId: e.target.value })}
      />
      <label htmlFor="chooser-workflow-name">name (optional)</label>
      <input
        id="chooser-workflow-name"
        value={form.name}
        onChange={(e) => updateForm({ name: e.target.value })}
      />
      <label htmlFor="chooser-scaffold-root">scaffold_root</label>
      <input
        id="chooser-scaffold-root"
        value={form.scaffoldRoot}
        required
        onChange={(e) => updateForm({ scaffoldRoot: e.target.value })}
      />
      <label htmlFor="chooser-artifact-root">artifact_root</label>
      <input
        id="chooser-artifact-root"
        value={form.artifactRoot}
        required
        onChange={(e) => updateForm({ artifactRoot: e.target.value })}
      />
      <label htmlFor="chooser-branch">branch_suggestion (optional)</label>
      <input
        id="chooser-branch"
        value={form.branchSuggestion}
        onChange={(e) => updateForm({ branchSuggestion: e.target.value })}
      />
    </div>
  );

  const renderPreviewStep = () => {
    if (preview.status === "loading" || preview.status === "idle") {
      return <div className="island-loading">Building preview…</div>;
    }
    if (preview.status === "error") {
      return (
        <div className="island-error" role="alert">
          {preview.error}
        </div>
      );
    }
    const data = preview.data!;
    const tsLabel = preview.generatedAt
      ? new Date(preview.generatedAt).toLocaleTimeString()
      : "";
    return (
      <div className="chooser-preview">
        <div className="preview-meta">
          Preview generated at {tsLabel}. Preview writes nothing on disk.
        </div>
        {data.warnings.length > 0 && (
          <div className="island-error" role="status">
            <strong>Warnings:</strong>
            <ul>
              {data.warnings.map((w, i) => (
                <li key={i}>{w}</li>
              ))}
            </ul>
          </div>
        )}
        <h3>Files</h3>
        <ul>
          {data.files.map((f) => (
            <li key={f.path}>
              <code>{f.path}</code> ({f.bytes} bytes)
            </li>
          ))}
        </ul>
        <h3>workflow.json</h3>
        <pre>{JSON.stringify(data.workflow, null, 2)}</pre>
      </div>
    );
  };

  return (
    <div className="island-root chooser">
      <ol className="chooser-step-list">
        {STEP_LABELS.map((label, i) => (
          <li
            key={label}
            aria-current={i === step ? "step" : undefined}
            className={i < step ? "done" : ""}
          >
            {i + 1}. {label}
          </li>
        ))}
      </ol>
      {step === 0 && renderTemplateStep()}
      {step === 1 && renderDetailsStep()}
      {step === 2 && renderPreviewStep()}
      {step === 3 && (
        <div className="chooser-preview">
          <p>
            Confirm to call <code>POST /workflows/generate</code> with{" "}
            <code>confirm_write: true</code>.
          </p>
          {writeStatus.status === "error" && (
            <div className="island-error" role="alert">
              {writeStatus.error}
            </div>
          )}
          {writeStatus.status === "ok" && (
            <div className="island-success" role="status">
              Workflow written.
            </div>
          )}
          {!props.allowMutations && (
            <div className="island-error" role="status">
              The local service is read-only (start it with{" "}
              <code>--allow-mutations</code> to enable writes).
            </div>
          )}
        </div>
      )}
      <div className="chooser-actions">
        <button
          type="button"
          className="secondary-button"
          disabled={step === 0}
          onClick={() => setStep((s) => Math.max(0, s - 1))}
        >
          Back
        </button>
        {step < 3 && (
          <button
            type="button"
            className="primary-button"
            disabled={!canAdvance}
            onClick={() => setStep((s) => Math.min(STEP_LABELS.length - 1, s + 1))}
          >
            Next
          </button>
        )}
        {step === 3 && (
          <button
            type="button"
            className="primary-button"
            disabled={!props.allowMutations || writeStatus.status === "loading"}
            onClick={openConfirm}
          >
            Confirm & Save
          </button>
        )}
      </div>
      <dialog
        ref={dialogRef}
        className="chooser-confirm-dialog"
        aria-labelledby="chooser-confirm-title"
      >
        <h3 id="chooser-confirm-title">Write generated workflow files</h3>
        <p>
          This will write the previewed files to disk. The local service must
          be running with <code>--allow-mutations</code>.
        </p>
        <div className="chooser-actions">
          <button type="button" className="secondary-button" onClick={cancelConfirm}>
            Cancel
          </button>
          <button
            type="button"
            className="primary-button"
            onClick={() => void performWrite()}
            disabled={writeStatus.status === "loading"}
          >
            Write files
          </button>
        </div>
      </dialog>
    </div>
  );
}

export const __testing = { buildSpec, recommendedForText };
