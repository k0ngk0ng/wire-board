import fs from "node:fs/promises";
import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { createHash } from "node:crypto";
import { gunzipSync } from "node:zlib";

const root = new URL("../../../", import.meta.url);
const cdn = process.env.PAL_ENDING_CDN === "1";
const out = new URL(cdn ? ".local/pal/cdn-ending-browser/" : ".local/pal/ending-browser/", root);
await fs.mkdir(out, { recursive: true });
const { build } = createRequire(new URL("web/package.json", root))("esbuild");
const pages = [];
for (const clip of [4, 5, 6]) {
  const metadata = JSON.parse(await fs.readFile(new URL(`.local/pal/ending-media-e2e/ending/${clip}.json`, root)));
  for (const frame of [0, 1, 14, 15, 16, Math.floor(metadata.frames / 2), metadata.frames - 1]) {
    const block = metadata.chunks.find((c) => frame >= c.first && frame < c.first + c.frames);
    const raw = gunzipSync(await fs.readFile(new URL(".local/pal/ending-media-e2e/" + block.path, root)));
    const offset = (frame - block.first) * 288 * 180 * 3;
    const rgba = new Uint8Array(320 * 200 * 4);
    for (let y = 0; y < 200; y++) for (let x = 0; x < 320; x++) {
      const source = offset + (Math.floor(y * 180 / 200) * 288 + Math.floor(x * 288 / 320)) * 3;
      const target = (y * 320 + x) * 4;
      rgba.set(raw.subarray(source, source + 3), target);
      rgba[target + 3] = 255;
    }
    pages.push({ clip, frame, rgbaSHA256: createHash("sha256").update(rgba).digest("hex") });
  }
}
assert.equal(pages.length, 21);
await fs.writeFile(new URL("reference.json", out), JSON.stringify({ pages }, null, 2) + "\n");
const source = `import React,{useRef,useEffect} from "react";
import {createRoot} from "react-dom/client";
import {PalStage} from "./games/pal/web/Stage";
import {SceneRenderer} from "./games/pal/web/scene-renderer";
import {endingPublication} from "./games/pal/web/ending-assets";
window.loading=[];
const renderer=new SceneRenderer("/pal/v1",{progress:value=>window.loading.push(value),...(${cdn}?{endingPublication:endingPublication()}:{})});
function App(){const canvas=useRef(null);useEffect(()=>{window.paintEnding=async(clip,frame)=>{const value={ending:{clip,frame},map:0,cameraX:0,cameraY:0,palette:0,night:false,draws:[]};await renderer.prepare(value);await renderer.paint(canvas.current,value);return true};window.endingReady=true},[]);return <div style={{width:"100vw",height:"100dvh",display:"grid"}}><PalStage canvasRef={canvas}/></div>}
createRoot(document.getElementById("root")).render(<App/>);`;
await build({ stdin: { contents: source, loader: "tsx", resolveDir: root.pathname }, absWorkingDir: root.pathname, nodePaths: [new URL("web/node_modules", root).pathname], bundle: true, outfile: new URL("fixture.js", out).pathname, format: "esm", jsx: "automatic", loader: { ".css": "css" }, sourcemap: false });
await fs.writeFile(new URL("index.html", out), '<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><style>html,body,#root{margin:0;width:100%;height:100%;overflow:hidden}</style></head><body><div id="root"></div><script type="module" src="/fixture.js"></script></body></html>');
