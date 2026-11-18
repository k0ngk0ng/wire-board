import { mkdir, symlink, writeFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { preparePokepathNative } from "../build/browser-runtime.mjs";
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const { build } = require("esbuild");
const directory = new URL("../../../.local/pokepath/classic-fixture/", import.meta.url);
await mkdir(directory, { recursive: true });
await build({ entryPoints: [new URL("ClassicFixture.tsx", import.meta.url).pathname], outfile: new URL("fixture.js", directory).pathname, bundle: true, platform: "browser", format: "esm", jsx: "automatic", nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))] });
await build({ entryPoints: [new URL("BoardFixture.tsx", import.meta.url).pathname], outfile: new URL("board.js", directory).pathname, bundle: true, platform: "browser", format: "esm", jsx: "automatic", define: { "import.meta.env.DEV": "true", "import.meta.env.BASE_URL": '"/"' }, nodePaths: [fileURLToPath(new URL("../../../web/node_modules", import.meta.url))] });
await rm(new URL("pokepath", directory), { force: true, recursive: true });
await mkdir(new URL("pokepath/v1/", directory), { recursive: true });
for (const name of ["catalog.json", "resources.json", "ui-text.json", "routes", "src"]) await symlink(new URL(`../assets/public/pokepath/v1/${name}`, import.meta.url).pathname, new URL(`pokepath/v1/${name}`, directory));
await symlink(preparePokepathNative() + "/pokepath/v1/native", new URL("pokepath/v1/native", directory));
await writeFile(new URL("index.html", directory), '<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="img-src &#39;self&#39; data:"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="fixture.css"><style>html,body,#root{margin:0;width:100%;height:100%;background:#111}#root>.pokepath-board{height:100%;min-height:0}</style></head><body><div id="root"></div><script type="module" src="fixture.js"></script></body></html>');
process.stdout.write(fileURLToPath(directory) + "\n");

await writeFile(new URL("board.html", directory), '<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><link rel="stylesheet" href="board.css"><style>html,body,#root{margin:0;width:100%;height:100%;background:#111}#root>.pokepath-board{height:100%;min-height:0}</style></head><body><div id="root"></div><script type="module" src="board.js"></script></body></html>');
