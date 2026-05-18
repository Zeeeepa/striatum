import { resolve } from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const rootDir = __dirname;
const srcDir = resolve(rootDir, "src");

const islandEntries: Record<string, string> = {
  "island-shared": resolve(srcDir, "shared/island-shared-entry.ts"),
  "island-tree-browser": resolve(srcDir, "islands/tree-browser/main.tsx"),
  "island-recovery-panel": resolve(srcDir, "islands/recovery-panel/main.tsx"),
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
        chunkFileNames: "[name]-[hash].js",
        assetFileNames: "style[extname]",
        manualChunks(id) {
          const normalized = id.split("\\").join("/");
          if (
            normalized.includes("/node_modules/shiki/") ||
            normalized.includes("/node_modules/@shikijs/")
          ) {
            return "island-shiki";
          }
        }
      }
    }
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx"]
  }
});
