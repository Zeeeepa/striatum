/**
 * RFC 0038 V1 — `/workflows/new` chooser wizard island.
 *
 * Fetches the workflow template catalog from `templatesUrl` (default
 * `/workflow-templates`) on mount, then walks the operator through six
 * steps: shape → lane set → lane modifiers → required details →
 * preview (`POST` to `previewUrl`) → confirm + save (`POST` to
 * `generateUrl` with `confirm_write: true`).
 *
 * Each step's Next button is disabled until required selections are made.
 * Editing any field on steps 1–4 after visiting step 5 invalidates the
 * preview and forces a fresh preview request when step 5 is re-entered.
 * The save step always runs through a `<dialog>`-driven operator
 * confirmation.
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
  WorkflowLaneSet,
  WorkflowShape,
  WorkflowTemplateCatalog,
} from "../../shared/types";

interface FormState {
  shape: string;
  laneSet: string;
  modifiers: string[];
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
  catalog?: WorkflowTemplateCatalog;
  error?: string;
}

const STEP_LABELS = [
  "Shape",
  "Lane set",
  "Modifiers",
  "Details",
  "Preview",
  "Save",
];

function isModifierEnabled(
  mod: { id: string; incompatible_with?: string[] },
  selected: string[],
): boolean {
  if (!mod.incompatible_with) return true;
  return mod.incompatible_with.every((id) => !selected.includes(id));
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
    modifiers: form.modifiers,
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
    modifiers: [],
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
      setCatalogState({ status: "ready", catalog: res.data });
    });
  }, [props.templatesUrl]);

  const catalog = catalogState.catalog;

  const selectedShape: WorkflowShape | null = useMemo(
    () =>
      catalog ? catalog.shapes.find((s) => s.id === form.shape) ?? null : null,
    [catalog, form.shape],
  );

  const filteredLaneSets: WorkflowLaneSet[] = useMemo(() => {
    if (!catalog) return [];
    if (!selectedShape) return catalog.lane_sets;
    if (
      !selectedShape.default_lane_sets ||
      selectedShape.default_lane_sets.length === 0
    ) {
      return catalog.lane_sets;
    }
    return catalog.lane_sets.filter((ls) =>
      selectedShape.default_lane_sets.includes(ls.id),
    );
  }, [catalog, selectedShape]);

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
        return Boolean(form.laneSet);
      case 2:
        return true;
      case 3:
        return (
          Boolean(form.workflowId.trim()) &&
          Boolean(form.scaffoldRoot.trim()) &&
          Boolean(form.artifactRoot.trim())
        );
      case 4:
        return preview.status === "ready";
      default:
        return false;
    }
  }, [step, form, preview.status]);

  useEffect(() => {
    if (step !== 4) return;
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
  if (catalogState.status === "error" || !catalog) {
    return (
      <div className="island-root chooser">
        <div className="island-error" role="alert">
          Failed to load workflow templates:{" "}
          {catalogState.error ?? "unknown error"}
        </div>
      </div>
    );
  }

  const renderShapeStep = () => (
    <div role="radiogroup" aria-label="Workflow shape" className="chooser-cards">
      {catalog.shapes.map((shape) => (
        <button
          type="button"
          role="radio"
          aria-checked={form.shape === shape.id}
          key={shape.id}
          className="chooser-card"
          onClick={() => updateForm({ shape: shape.id, laneSet: "" })}
        >
          <h3>{shape.label}</h3>
          <div className="summary">{shape.summary}</div>
          <div className="recommended-for">Use it for: {shape.recommended_for}</div>
        </button>
      ))}
    </div>
  );

  const renderLaneSetStep = () => (
    <div role="radiogroup" aria-label="Lane set" className="chooser-cards">
      {filteredLaneSets.map((ls) => (
        <button
          type="button"
          role="radio"
          aria-checked={form.laneSet === ls.id}
          key={ls.id}
          className="chooser-card"
          onClick={() => updateForm({ laneSet: ls.id })}
        >
          <h3>{ls.label}</h3>
          <div className="summary">{ls.summary}</div>
          <div className="recommended-for">Use it for: {ls.recommended_for}</div>
        </button>
      ))}
      {filteredLaneSets.length === 0 && (
        <div className="island-loading">
          No lane sets are recommended for this shape.
        </div>
      )}
    </div>
  );

  const renderModifierStep = () => {
    const mods = catalog.modifiers ?? [];
    if (mods.length === 0) {
      return (
        <div className="island-loading">
          No optional modifiers exist for this catalog version.
        </div>
      );
    }
    return (
      <ul className="chooser-cards" role="group" aria-label="Lane modifiers">
        {mods.map((mod) => {
          const enabled = isModifierEnabled(mod, form.modifiers);
          const checked = form.modifiers.includes(mod.id);
          return (
            <li key={mod.id} style={{ listStyle: "none" }}>
              <button
                type="button"
                aria-checked={checked}
                aria-disabled={!enabled}
                role="checkbox"
                className="chooser-card"
                onClick={() => {
                  if (!enabled) return;
                  updateForm({
                    modifiers: checked
                      ? form.modifiers.filter((m) => m !== mod.id)
                      : [...form.modifiers, mod.id],
                  });
                }}
              >
                <h3>{mod.label}</h3>
                <div className="summary">{mod.summary}</div>
                {!enabled && (
                  <div className="recommended-for">
                    Incompatible with current modifier selection.
                  </div>
                )}
              </button>
            </li>
          );
        })}
      </ul>
    );
  };

  const renderDetailsStep = () => (
    <div className="chooser-fields">
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
      {step === 0 && renderShapeStep()}
      {step === 1 && renderLaneSetStep()}
      {step === 2 && renderModifierStep()}
      {step === 3 && renderDetailsStep()}
      {step === 4 && renderPreviewStep()}
      {step === 5 && (
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
        {step < 5 && (
          <button
            type="button"
            className="primary-button"
            disabled={!canAdvance}
            onClick={() => setStep((s) => Math.min(STEP_LABELS.length - 1, s + 1))}
          >
            Next
          </button>
        )}
        {step === 5 && (
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

export const __testing = { buildSpec, isModifierEnabled };
