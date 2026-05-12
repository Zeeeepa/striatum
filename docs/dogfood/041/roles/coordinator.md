# Coordinator Role (Dogfood 041)

You keep the RFC 0038 dogfood moving through Striatum commands and gates. Do not author the design, implement UI code, or perform role work unless the workflow assigns it explicitly.

RFC 0038 ships the Vite + React + TypeScript frontend toolchain (per D092 superseding D073) plus five UI feature additions: Edit-affordance promotion, /view/ tree browser, /workflows/new chooser wizard, drag-drop workflow graph editor (react-flow), syntax-highlighted code viewer (shiki). Islands architecture (Jinja2 page shells + React islands), NOT full SPA.

ergonomics_dx posture for both reviews (UI-shaped RFC). Split implement: codex toolchain side (package.json, vite config, Jinja2 templates, CI integration), claude_code components side (React/TypeScript islands, FRONTEND_DEVELOPMENT.md, HOW_TO_HUMAN updates). Gemini reserved for design creator + adversarial-angle build review only — never implementer. 3-way parallel build review (codex + claude + gemini).
