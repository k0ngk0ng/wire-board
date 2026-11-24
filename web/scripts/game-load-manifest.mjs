import { assetBase, assetURL } from "./cdn-base.mjs";
import { existsSync, readFileSync, writeFileSync, readdirSync, statSync } from "node:fs";
import { createHash } from "node:crypto";
import { gzipSync } from "node:zlib";
import path from "node:path";
import { execFileSync } from "node:child_process";
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
      const workerFiles = Object.keys(bundle).filter(file => /(?:^|\/)resident-worker-[A-Za-z0-9_-]+\.js$/.test(file));
      if (workerFiles.length > 1) throw Error("Duplicate resident worker entry");
      const workerPath = workerFiles[0];
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
        for (const asset of c.viteMetadata?.importedAssets ?? []) seen.add(asset);
        for (const imp of c.imports) graph(imp, seen);



        return seen;
      }
      const records = (files) =>
        [...files].map((f) => ({
          url: assetURL(base, f),
          bytes: statSync(path.join(dir, f)).size,
        }));
      const homeFiles = graph(entry.fileName);
      if (workerPath) homeFiles.delete(workerPath);
      const home = {
        files: records(homeFiles),
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
          if (workerPath) files.add(workerPath);
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
      const owners = {};
      const candidates = new Map();
      for (const chunk of chunks) {
        const games = new Set(Object.keys(chunk.modules).flatMap(module => module.match(/\/games\/([^/]+)\//)?.[1] ?? []));
        const owner = games.size === 1 ? [...games][0] : "_common";
        owners[chunk.fileName] = owner;
        for (const css of chunk.viteMetadata?.importedCss ?? []) {
          if (!candidates.has(css)) candidates.set(css, new Set());
          candidates.get(css).add(owner);
        }
      }
      for (const [file, games] of candidates) owners[file] = games.size === 1 ? [...games][0] : "_common";
      for (const asset of Object.values(bundle).filter(file => file.type === "asset" && !owners[file.fileName])) {
        const sources = asset.originalFileNames ?? (asset.originalFileName ? [asset.originalFileName] : []);
        const games = new Set(sources.map(source => source.match(/(?:^|\/)games\/([^/]+)\//)?.[1] ?? "_common"));
        if (!games.size) for (const chunk of chunks) if (chunk.viteMetadata?.importedAssets?.has(asset.fileName)) games.add(owners[chunk.fileName] ?? "_common");
        owners[asset.fileName] = games.size === 1 ? [...games][0] : "_common";
      }
      function walk(rel = "") {
        for (const f of readdirSync(path.join(dir, rel), {
          withFileTypes: true,
        })) {
          const p = path.join(rel, f.name);
          if (f.isDirectory()) walk(p);
          else if (
            f.isFile() && !p.startsWith("assets/") &&
            (p.includes("/") || /\.(png|jpg|jpeg|webp|svg|gif|mp3|wav|ogg|flac|pvzr|json|js|wasm)$/.test(p))
          )
            media["/" + p] = statSync(path.join(dir, p)).size;
        }
      }
      walk();
      const externalPath = new URL("../../assets/external-resources.json", import.meta.url);
      const externalRaw = existsSync(externalPath) ? readFileSync(externalPath) : undefined;
      if (externalRaw) {
        const external = JSON.parse(externalRaw);
        for (const [file, record] of Object.entries(external.files)) if (!media[file]) media[file] = record.bytes;
      }
      const build = createHash("sha256")
        .update(readFileSync(path.join(dir, "index.html")))
        .digest("hex");
      writeFileSync(
        path.join(dir, "game-load-manifest.json"),
        JSON.stringify({
          build,
          sourceSHA: process.env.GITHUB_SHA || process.env.SOURCE_SHA || execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(),
          entry: assetURL(base, entry.fileName),
          worker: workerPath ? assetURL(base, workerPath) : undefined,
          workerPath,
          assetBase: base,
          games,
          media,
          home,
          owners,
          externalRegistrySHA256: externalRaw ? createHash("sha256").update(externalRaw).digest("hex") : undefined,
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
