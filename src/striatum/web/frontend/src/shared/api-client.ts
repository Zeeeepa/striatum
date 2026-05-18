/**
 * Typed HTTP wrappers around the local Striatum service endpoints
 * the islands talk to. Every call goes to a same-origin path: there is
 * no CDN, no cross-origin fetch, and no third-party telemetry.
 */

import type {
  ApiResult,
  RepoTreeResponse,
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
