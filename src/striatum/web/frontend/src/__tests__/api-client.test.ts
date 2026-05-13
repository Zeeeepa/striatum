import { afterEach, describe, expect, it, vi } from "vitest";
import {
  fetchRepoTree,
  fetchWorkflowTemplates,
  generateWorkflowPreview,
  generateWorkflowWrite,
  saveWorkflow,
} from "../shared/api-client";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.restoreAllMocks();
});

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "Content-Type": "application/json" },
  });
}

describe("api-client.fetchRepoTree", () => {
  it("encodes the path and unwraps an ok envelope", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({ ok: true, data: { path: "docs", entries: [], truncated: false } }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const res = await fetchRepoTree("docs/dogfood/041");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "/v1/repo/tree?path=docs%2Fdogfood%2F041",
    );
    expect(res.ok).toBe(true);
    if (res.ok) {
      expect(res.data.path).toBe("docs");
    }
  });

  it("honors an override tree endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({ ok: true, data: { path: "", entries: [], truncated: false } }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    await fetchRepoTree("docs", "/api/repo/tree");
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/api/repo/tree?path=docs");
  });

  it("propagates a server error envelope", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(
          { ok: false, error: { code: "path_outside_repo", message: "outside" } },
          { status: 400 },
        ),
      );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const res = await fetchRepoTree("../etc/passwd");
    expect(res.ok).toBe(false);
    if (!res.ok) {
      expect(res.error.code).toBe("path_outside_repo");
    }
  });
});

describe("api-client.fetchWorkflowTemplates", () => {
  it("calls the catalog endpoint", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({ ok: true, data: { shapes: [], lane_sets: [] } }),
    );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const res = await fetchWorkflowTemplates();
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/workflow-templates");
    expect(res.ok).toBe(true);
  });
});

describe("api-client.generateWorkflowPreview", () => {
  it("posts the spec without confirm_write", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ ok: true, data: { workflow: {}, files: [], warnings: [] } }),
      );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    await generateWorkflowPreview({
      shape: "review",
      lane_set: "single_agent",
      workflow_id: "demo",
      scaffold_root: "docs/demo",
      artifact_root: "docs/demo",
    });
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(init?.method).toBe("POST");
    const body = JSON.parse(init?.body as string);
    expect(body).toHaveProperty("spec.workflow_id", "demo");
    expect(body).not.toHaveProperty("confirm_write");
  });
});

describe("api-client.generateWorkflowWrite", () => {
  it("always sets confirm_write to true", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ ok: true, data: { workflow: {}, files: [], warnings: [] } }),
      );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    await generateWorkflowWrite({
      shape: "review",
      lane_set: "single_agent",
      workflow_id: "demo",
      scaffold_root: "docs/demo",
      artifact_root: "docs/demo",
    });
    const body = JSON.parse(
      (fetchMock.mock.calls[0]?.[1] as RequestInit).body as string,
    );
    expect(body.confirm_write).toBe(true);
  });
});

describe("api-client.saveWorkflow", () => {
  it("attaches an If-Match header when a sha256 is present", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ok: true, data: {} }));
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    await saveWorkflow("examples/wf.json", { workflow_id: "demo" }, "abc123");
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const headers = (init?.headers ?? {}) as Record<string, string>;
    expect(headers["If-Match"]).toBe('"abc123"');
  });

  it("skips If-Match when there is no sha", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ ok: true, data: {} }));
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    await saveWorkflow("examples/wf.json", { workflow_id: "demo" }, "");
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const headers = (init?.headers ?? {}) as Record<string, string>;
    expect(headers["If-Match"]).toBeUndefined();
  });

  it("returns the raw status for stale-precondition responses", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(
          { ok: false, error: { code: "stale", message: "stale" } },
          { status: 412 },
        ),
      );
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    const res = await saveWorkflow("examples/wf.json", { workflow_id: "demo" }, "x");
    expect(res.status).toBe(412);
  });
});
