import { assetBase, assetURL } from "./cdn-base.mjs";
import { readFileSync, writeFileSync, readdirSync, statSync } from "node:fs";
import { createHash } from "node:crypto";
import { gzipSync } from "node:zlib";
import path from "node:path";
export function gameLoadManifest() {
  let base = "/";
  return {
    configResolved(config) {
      base = assetBase(config.base);
    },
    name: "game-load-manifest",
    enforce: "post",
    writeBundle(options, bundle) {
      const dir = options.dir;
      const chunks = Object.values(bundle).filter((x) => x.type === "chunk");
      const entry =
        chunks.find(
          (x) => x.isEntry && x.facadeModuleId?.endsWith("/index.html"),
        ) ?? chunks.find((x) => x.isEntry);
      function graph(file, seen = new Set()) {
        if (seen.has(file)) return seen;
        seen.add(file);
        const c = bundle[file];
        if (c?.type !== "chunk") return seen;
        for (const css of c.viteMetadata?.importedCss ?? []) seen.add(css);
        for (const imp of c.imports) graph(imp, seen);
        // Lazy imports can be mutually exclusive maps or optional renderers.
        // Their owning Board loads the selected branch and reports ready only
        // after initialization. Preloading every branch defeats that contract.
        return seen;
      }
      const records = (files) =>
        [...files].map((f) => ({
          url: assetURL(base, f),
          bytes: statSync(path.join(dir, f)).size,
        }));
      const homeFiles = graph(entry.fileName);
      const home = {
        bytes: [...homeFiles].reduce(
          (n, f) => n + statSync(path.join(dir, f)).size,
          0,
        ),
        modules: [...homeFiles].flatMap((f) =>
          Object.keys(bundle[f]?.modules ?? {}).map((p) =>
            p.replace(/^.*\/wire-board\//, ""),
          ),
        ),
      };
      const games = {};
      for (const c of chunks) {
        const id = Object.keys(c.modules)
          .map((p) => p.match(/\/games\/([^/]+)\/web\/module\.tsx$/)?.[1])
          .find(Boolean);
        if (id) {
          const files = graph(c.fileName);
          const modulePath = Object.keys(c.modules).find((p) =>
            p.endsWith(`/games/${id}/web/module.tsx`),
          );
          const catalog = readFileSync(
            modulePath.replace("module.tsx", "catalog.tsx"),
            "utf8",
          );
          if (/legacy:\s*true/.test(catalog)) {
            const legacy = chunks.find((chunk) =>
              Object.keys(chunk.modules).some((p) =>
                p.endsWith("/legacy/room-ui.tsx"),
              ),
            );
            if (legacy) graph(legacy.fileName, files);
          }
          games[id] = records(files);
        }
      }
      const media = {};
      function walk(rel = "") {
        for (const f of readdirSync(path.join(dir, rel), {
          withFileTypes: true,
        })) {
          const p = path.join(rel, f.name);
          if (f.isDirectory()) walk(p);
          else if (
            !p.startsWith("assets/") &&
            /\.(png|jpg|jpeg|webp|svg|gif|mp3|wav|ogg|wasm)$/.test(p)
          )
            media["/" + p] = statSync(path.join(dir, p)).size;
        }
      }
      walk();
      const build = createHash("sha256")
        .update(readFileSync(path.join(dir, "index.html")))
        .digest("hex");
      writeFileSync(
        path.join(dir, "game-load-manifest.json"),
        JSON.stringify({
          build,
          entry: assetURL(base, entry.fileName),
          assetBase: base,
          games,
          media,
          home,
        }),
      );
      for (const f of Object.keys(bundle).filter((f) => /\.(js|css)$/.test(f)))
        writeFileSync(
          path.join(dir, f + ".gz"),
          gzipSync(readFileSync(path.join(dir, f))),
        );
    },
  };
}
