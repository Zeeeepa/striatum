/**
 * Typed HTTP wrappers around the local Striatum service endpoints
 * the islands talk to. Every call goes to a same-origin path: there is
 * no CDN, no cross-origin fetch, and no third-party telemetry.
 */

import type {
  ApiResult,
  GeneratedWorkflow,
  RepoTreeResponse,
  WorkflowDocument,
  WorkflowTemplateCatalog,
} from "./types";

async function handle<T>(resp: Response): Promise<ApiResult<T>> {
  let body: unknown = null;
  try {
    body = await resp.json();
  } catch {
    return {
      ok: false,
      error: {
        code: "non_json_response",
        message: `HTTP ${resp.status} ${resp.statusText}`,
      },
    };
  }
  if (resp.ok && body && typeof body === "object" && (body as { ok?: unknown }).ok === true) {
    return body as ApiResult<T>;
  }
  if (body && typeof body === "object" && (body as { error?: unknown }).error) {
    return body as ApiResult<T>;
  }
  return {
    ok: false,
    error: {
      code: "unexpected_shape",
      message: `HTTP ${resp.status}`,
    },
  };
}

export async function fetchRepoTree(
  path: string,
  baseUrl: string = "/v1/repo/tree",
): Promise<ApiResult<RepoTreeResponse>> {
  const sep = baseUrl.includes("?") ? "&" : "?";
  const url = `${baseUrl}${sep}path=` + encodeURIComponent(path);
  const resp = await fetch(url, {
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });
  return handle<RepoTreeResponse>(resp);
}

export async function fetchWorkflowTemplates(
  url: string = "/workflow-templates",
): Promise<ApiResult<WorkflowTemplateCatalog>> {
  const resp = await fetch(url, {
    credentials: "same-origin",
    headers: { Accept: "application/json" },
  });
  return handle<WorkflowTemplateCatalog>(resp);
}

export interface GenerationSpec {
  shape: string;
  lane_set: string;
  modifiers?: string[];
  workflow_id: string;
  name?: string;
  scaffold_root: string;
  artifact_root: string;
  branch_suggestion?: string;
  lane_commands?: Record<string, string[]>;
  options?: Record<string, unknown>;
}

export async function generateWorkflowPreview(
  spec: GenerationSpec,
  url: string = "/workflows/generate/preview",
): Promise<ApiResult<GeneratedWorkflow>> {
  const resp = await fetch(url, {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ spec }),
  });
  return handle<GeneratedWorkflow>(resp);
}

export async function generateWorkflowWrite(
  spec: GenerationSpec,
  url: string = "/workflows/generate",
): Promise<ApiResult<GeneratedWorkflow>> {
  const resp = await fetch(url, {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ spec, confirm_write: true }),
  });
  return handle<GeneratedWorkflow>(resp);
}

export interface SaveWorkflowResult {
  status: 200 | 412 | number;
  body: unknown;
}

export async function saveWorkflow(
  relPath: string,
  body: WorkflowDocument,
  diskSha256: string,
  url?: string,
): Promise<SaveWorkflowResult> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  if (diskSha256) headers["If-Match"] = `"${diskSha256}"`;
  const target = url ?? "/workflows/edit/" + relPath;
  const resp = await fetch(target, {
    method: "POST",
    credentials: "same-origin",
    headers,
    body: JSON.stringify(body),
  });
  let parsed: unknown = null;
  try {
    parsed = await resp.json();
  } catch {
    parsed = null;
  }
  return { status: resp.status, body: parsed };
}
