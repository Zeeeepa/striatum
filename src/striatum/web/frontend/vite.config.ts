import { existsSync } from "node:fs";
import { resolve } from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vite";

const rootDir = __dirname;
const srcDir = resolve(rootDir, "src");

const islandEntries: Record<string, string> = {
  "island-shared": resolve(srcDir, "main.ts"),
  "island-tree-browser": resolve(srcDir, "islands/tree-browser/main.tsx"),
  "island-workflow-chooser": resolve(srcDir, "islands/workflow-chooser/main.tsx"),
  "island-workflow-graph-editor": resolve(srcDir, "islands/workflow-graph-editor/main.tsx"),
  "island-code-viewer": resolve(srcDir, "islands/code-viewer/main.tsx")
};

function placeholderIslandPlugin(): Plugin {
  const virtualPrefix = "\0striatum-placeholder:";
  const placeholders = new Map(
    Object.entries(islandEntries).map(([name, path]) => [path, `${virtualPrefix}${name}`])
  );

  return {
    name: "striatum-placeholder-islands",
    enforce: "pre",
    resolveId(source) {
      const absolute = resolve(rootDir, source);
      const direct = placeholders.get(source) ?? placeholders.get(absolute);
      if (direct && !existsSync(source) && !existsSync(absolute)) {
        return direct;
      }
      return null;
    },
    load(id) {
      if (!id.startsWith(virtualPrefix)) {
        return null;
      }
      const name = id.slice(virtualPrefix.length);
      return [
        "export {};",
        `console.info("Striatum frontend island placeholder loaded: ${name}");`
      ].join("\n");
    }
  };
}

export default defineConfig({
  plugins: [placeholderIslandPlugin(), react()],
  build: {
    outDir: resolve(rootDir, "../static/build"),
    emptyOutDir: true,
    manifest: true,
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
