import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchRepoTree } from "../shared/api-client";

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

