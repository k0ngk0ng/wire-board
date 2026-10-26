import { createRequire } from "node:module";
import { mkdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
const require = createRequire(
  new URL("../../web/package.json", import.meta.url),
);
const { build } = require("esbuild");
const out = resolve(process.argv[2]);
mkdirSync(out, { recursive: true });
await build({
  entryPoints: ["scripts/fullscreen/chat-harness.tsx"],
  bundle: true,
  minify: false,
  outfile: resolve(out, "harness.js"),
  jsx: "automatic",
  nodePaths: [resolve("web/node_modules")],
  define: { "process.env.NODE_ENV": '"development"' },
  loader: { ".svg": "dataurl", ".png": "dataurl", ".webp": "dataurl" },
  // The isolated chat fixture has no game catalog. Keep the actual PlayerName
  // and its context, while avoiding the application's Vite-only catalog loader.
  plugins: [
    {
      name: "chat-catalog",
      setup(builder) {
        builder.onResolve({ filter: /^\.\/game-api\/registry$/ }, () => ({
          path: "catalog",
          namespace: "chat-catalog",
        }));
        builder.onLoad({ filter: /.*/, namespace: "chat-catalog" }, () => ({
          contents: "export const gameName = (kind) => kind;",
        }));
      },
    },
  ],
});
writeFileSync(
  resolve(out, "index.html"),
  `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="harness.css"></head><body><div id="root"></div><script src="harness.js"></script></body></html>`,
);
