import { describe, expect, it } from "vitest";
import { __testing } from "../islands/code-viewer";

const {
  detectLanguage,
  escapeHtml,
  injectLineNumbers,
  plainTextHtml,
  SHIKI_LANG_MAP,
  SUPPORTED_LANGS,
  MAX_HIGHLIGHT_BYTES,
  COLLAPSE_LINE_THRESHOLD,
} = __testing;

describe("code-viewer.detectLanguage", () => {
  it("maps explicit language hints through SHIKI_LANG_MAP", () => {
    expect(detectLanguage("py", "anything.txt")).toBe("python");
    expect(detectLanguage("yml", "config.txt")).toBe("yaml");
    expect(detectLanguage("ts", "x.ts")).toBe("typescript");
  });

  it("falls back to filename extension", () => {
    expect(detectLanguage(undefined, "Makefile.toml")).toBe("toml");
    expect(detectLanguage(undefined, "script.sh")).toBe("bash");
  });

  it("returns plaintext when nothing matches", () => {
    expect(detectLanguage(undefined, "no-extension")).toBe("plaintext");
    expect(detectLanguage("klingon", "x.kln")).toBe("plaintext");
  });
});

describe("code-viewer.escapeHtml", () => {
  it("escapes the standard set", () => {
    expect(escapeHtml("<script>&\"'")).toBe(
      "&lt;script&gt;&amp;&quot;&#39;",
    );
  });

  it("does not execute injected markup when assembled into HTML", () => {
    const hostile = "<script>alert(1)</script>";
    const wrapped = `<pre>${escapeHtml(hostile)}</pre>`;
    expect(wrapped).not.toContain("<script>");
    expect(wrapped).toContain("&lt;script&gt;");
  });
});

describe("code-viewer.plainTextHtml", () => {
  it("wraps each line in a numbered row", () => {
    const html = plainTextHtml("alpha\nbeta");
    expect(html).toContain('class="line-number"');
    expect(html).toContain("alpha");
    expect(html).toContain("beta");
  });

  it("escapes hostile content", () => {
    const html = plainTextHtml("<script>alert(1)</script>");
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });
});

describe("code-viewer.injectLineNumbers", () => {
  it("decorates Shiki <span class=line> rows with explicit line numbers", () => {
    const shikiHtml =
      '<pre><code><span class="line">first</span><span class="line">second</span></code></pre>';
    const out = injectLineNumbers(shikiHtml);
    expect(out).toContain("line-number");
    expect((out.match(/line-number/g) ?? []).length).toBe(2);
  });

  it("returns the input unchanged when no <code> wrapper is found", () => {
    expect(injectLineNumbers("<div>not-shiki</div>")).toBe("<div>not-shiki</div>");
  });
});

describe("code-viewer constants", () => {
  it("limits Shiki to the eight named grammars (plus plaintext fallback)", () => {
    for (const lang of [
      "json",
      "python",
      "typescript",
      "javascript",
      "bash",
      "yaml",
      "toml",
      "markdown",
      "sql",
    ]) {
      expect(SUPPORTED_LANGS).toContain(lang);
    }
  });

  it("exposes a 5 MB skip-Shiki guard and the 500-line collapse threshold", () => {
    expect(MAX_HIGHLIGHT_BYTES).toBe(5 * 1024 * 1024);
    expect(COLLAPSE_LINE_THRESHOLD).toBe(500);
  });

  it("maps every Shiki extension to a supported grammar", () => {
    for (const lang of Object.values(SHIKI_LANG_MAP)) {
      expect(SUPPORTED_LANGS).toContain(lang);
    }
  });
});
