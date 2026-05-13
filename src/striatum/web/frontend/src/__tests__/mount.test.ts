import { afterEach, describe, expect, it } from "vitest";
import { readJsonPayload, readTextPayload } from "../shared/mount";

afterEach(() => {
  document.body.innerHTML = "";
});

describe("mount.readJsonPayload", () => {
  it("parses the script tag contents", () => {
    document.body.innerHTML =
      '<script id="payload" type="application/json">{"a": 1}</script>';
    expect(readJsonPayload<{ a: number }>("payload", { a: 0 })).toEqual({ a: 1 });
  });

  it("returns the fallback when the element is missing", () => {
    expect(readJsonPayload("missing", { ok: true })).toEqual({ ok: true });
  });

  it("returns the fallback when the contents are unparseable", () => {
    document.body.innerHTML =
      '<script id="bad" type="application/json">not-json</script>';
    expect(readJsonPayload("bad", { fallback: true })).toEqual({ fallback: true });
  });
});

describe("mount.readTextPayload", () => {
  it("returns the raw text content", () => {
    document.body.innerHTML =
      '<script id="raw" type="text/plain">hello\nworld</script>';
    expect(readTextPayload("raw")).toBe("hello\nworld");
  });

  it("returns an empty string when the element is missing", () => {
    expect(readTextPayload("absent")).toBe("");
  });
});
