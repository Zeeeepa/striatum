# Design review (devils_advocate)

Sweep:
1. CSP — does Jinja2 introduce any inline-script needs?
2. SVG layout — does the layered algorithm handle cycles
   (revision loops) correctly?
3. Hash-route redirect — works for all 5 routes including deep
   ones with multiple slashes?
4. Dark mode — palette covers state colors that need to be
   readable in both modes (e.g., `failed` red on dark bg).
5. Zero regression — JSON API and SSE feed unchanged?
6. Test plan completeness.
7. Jinja2 as runtime dep — is the wheel-size hit acceptable?

Verdict ∈ {accept, accept_with_findings, needs_revision, reject}.
