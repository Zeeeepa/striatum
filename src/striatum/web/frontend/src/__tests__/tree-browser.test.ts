import { describe, expect, it } from "vitest";
import { __testing } from "../islands/tree-browser";

const { normalizePath, parentOf, fuzzyMatch, joinUrl } = __testing;

describe("tree-browser.normalizePath", () => {
  it("strips leading and trailing slashes", () => {
    expect(normalizePath("/docs/")).toBe("docs");
    expect(normalizePath("docs/dogfood/")).toBe("docs/dogfood");
    expect(normalizePath("docs")).toBe("docs");
  });

  it("collapses empty / dot to root", () => {
    expect(normalizePath("")).toBe("");
    expect(normalizePath(".")).toBe("");
    expect(normalizePath("/")).toBe("");
  });
});

describe("tree-browser.parentOf", () => {
  it("returns the parent path", () => {
    expect(parentOf("docs/dogfood/041")).toBe("docs/dogfood");
    expect(parentOf("docs")).toBe("");
    expect(parentOf("")).toBe("");
  });
});

describe("tree-browser.fuzzyMatch", () => {
  it("matches subsequence anywhere in the path", () => {
    expect(
      fuzzyMatch(
        { name: "decision_log.md", path: "docs/decision_log.md", kind: "file", size: null, mtime_utc: null },
        "dlmd",
      ),
    ).toBe(true);
  });

  it("is case-insensitive", () => {
    expect(
      fuzzyMatch(
        { name: "README.md", path: "README.md", kind: "file", size: null, mtime_utc: null },
        "rdme",
      ),
    ).toBe(true);
  });

  it("returns true on empty needle", () => {
    expect(
      fuzzyMatch(
        { name: "x", path: "x", kind: "file", size: null, mtime_utc: null },
        "",
      ),
    ).toBe(true);
  });

  it("rejects non-subsequence", () => {
    expect(
      fuzzyMatch(
        { name: "abc", path: "abc", kind: "file", size: null, mtime_utc: null },
        "z",
      ),
    ).toBe(false);
  });
});

describe("tree-browser.joinUrl", () => {
  it("inserts a single slash between base and path", () => {
    expect(joinUrl("/view/", "docs/SPEC.md")).toBe("/view/docs/SPEC.md");
    expect(joinUrl("/view", "docs/SPEC.md")).toBe("/view/docs/SPEC.md");
  });

  it("strips a leading slash from path", () => {
    expect(joinUrl("/view/", "/docs/SPEC.md")).toBe("/view/docs/SPEC.md");
  });
});
