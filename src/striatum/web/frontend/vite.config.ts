import { resolve } from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const rootDir = __dirname;
const srcDir = resolve(rootDir, "src");

const islandEntries: Record<string, string> = {
  "island-shared": resolve(srcDir, "shared/island-shared-entry.ts"),
  "island-tree-browser": resolve(srcDir, "islands/tree-browser/main.tsx"),
  "island-workflow-chooser": resolve(srcDir, "islands/workflow-chooser/main.tsx"),
  "island-workflow-graph-editor": resolve(srcDir, "islands/workflow-graph-editor/main.tsx"),
  "island-code-viewer": resolve(srcDir, "islands/code-viewer/main.tsx")
};

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: resolve(rootDir, "../static/build"),
    emptyOutDir: true,
    manifest: false,
    sourcemap: false,
    rollupOptions: {
      input: islandEntries,
      output: {
        entryFileNames: "[name].js",
        chunkFileNames: "island-shared-[hash].js",
        assetFileNames: "style[extname]"
      }
    }
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"]
  }
});
