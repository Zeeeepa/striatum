import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { BylineLine } from "../shared/components/BylineLine";

describe("BylineLine", () => {
  it("hides model-author strings when the row is unattested", () => {
    const html = renderToStaticMarkup(
      <BylineLine
        authorLine="author: author-codex-gpt-5.5-001"
        expectedAuthorLine="author: author-codex-gpt-5.5-001"
        attested={false}
      />,
    );

    expect(html).toContain("author: operator");
    expect(html).not.toContain("author-codex-gpt-5.5-001");
  });
});
