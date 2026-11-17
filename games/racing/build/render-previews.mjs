import assert from "node:assert/strict";
import { createServer } from "node:http";
import { readFile, mkdir, rm, writeFile } from "node:fs/promises";
import { execFile, execFileSync } from "node:child_process";
import { promisify } from "node:util";
import { createHash, randomUUID } from "node:crypto";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const input = path.resolve(root, process.argv[2] ?? ".local/racing-content/miniature");
const output = path.resolve(root, process.argv[3] ?? ".local/racing-content/preview-renders");
assert(input.startsWith(root) && output.startsWith(root));
await mkdir(output);
const temporary = path.join(output, "temporary");
const sockets = path.join(root, ".s", `rp${randomUUID().slice(0, 5)}`);
for (const folder of [temporary, sockets, ...["cache", "data", "config"].map(name => path.join(temporary, name))]) await mkdir(folder, { recursive: true });
const config = path.join(temporary, "agent-browser.json");
await writeFile(config, "{}\n");
const env = { ...process.env, TMPDIR: temporary, AGENT_BROWSER_SOCKET_DIR: sockets,
  AGENT_BROWSER_CONFIG: config, AGENT_BROWSER_PROFILE: path.join(temporary, "profile"),
  XDG_CACHE_HOME: path.join(temporary, "cache"), XDG_DATA_HOME: path.join(temporary, "data"), XDG_CONFIG_HOME: path.join(temporary, "config"),
  AGENT_BROWSER_ARGS: "--disable-breakpad,--disable-crash-reporter,--enable-unsafe-swiftshader,--use-gl=angle,--use-angle=swiftshader" };
const session = execFileSync("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "racing-previews"], { cwd: root, env, encoding: "utf8" }).trim();
const run = promisify(execFile);
async function browser(args, source) {
  const pending = run("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env, encoding: "utf8", timeout: 45000, maxBuffer: 16 * 1024 * 1024 });
  pending.child.stdin.end(source ?? "");
  const result = JSON.parse((await pending).stdout);
  assert(result.success, JSON.stringify(result));
  return result.data;
}
const html = `<!doctype html><html><head><meta charset="utf-8"><script type="importmap">{"imports":{"three":"/three/build/three.module.js"}}</script></head><body style="margin:0;background:#edf3ee"><canvas id="preview"></canvas><script type="module">
import * as T from 'three';
import {GLTFLoader} from '/three/examples/jsm/loaders/GLTFLoader.js';
const canvas=document.querySelector('canvas'),renderer=new T.WebGLRenderer({canvas,antialias:true,preserveDrawingBuffer:true});
renderer.setPixelRatio(1);renderer.setSize(384,216,false);renderer.outputColorSpace=T.SRGBColorSpace;
renderer.toneMapping=T.ACESFilmicToneMapping;renderer.toneMappingExposure=1.2;
const loader=new GLTFLoader();
window.renderPreview=async id=>{
 const scene=new T.Scene();scene.background=new T.Color('#edf3ee');
 scene.add(new T.HemisphereLight(0xffffff,0x697863,2.4));const light=new T.DirectionalLight(0xffffff,2);light.position.set(3,7,5);scene.add(light);
 const asset=await loader.loadAsync('/models/'+id+'.glb'),model=asset.scene;scene.add(model);
 const box=new T.Box3().setFromObject(model),centre=box.getCenter(new T.Vector3()),size=box.getSize(new T.Vector3());model.position.sub(centre);
 const span=Math.max(size.x,size.z,size.y*1.5),view=span*.54,aspect=384/216;
 const camera=new T.OrthographicCamera(-view*aspect,view*aspect,view,-view,.001,span*30);
 camera.position.set(span*1.2,span*1.5,span*1.35);camera.lookAt(0,0,0);camera.updateProjectionMatrix();
 await new Promise(requestAnimationFrame);renderer.render(scene,camera);renderer.getContext().finish();
 const result=canvas.toDataURL('image/png');scene.traverse(node=>{if(node.geometry)node.geometry.dispose();if(node.material){for(const material of Array.isArray(node.material)?node.material:[node.material]){material.map?.dispose();material.dispose();}}});
 return result;
};window.previewReady=true;
</script></body></html>`;
const ids = ["france", "coastal", "uk", "park", "germany"];
const server = createServer(async (request, response) => {
  try {
    const name = new URL(request.url, "http://localhost").pathname;
    if (name === "/preview.html") { response.setHeader("Content-Type", "text/html; charset=utf-8"); response.end(html); return; }
    let file;
    if (name.startsWith("/three/") && name.endsWith(".js")) {
      file = path.resolve(root, "web/node_modules/three", name.slice(7));
      assert(file.startsWith(path.join(root, "web/node_modules/three") + path.sep));
      response.setHeader("Content-Type", "text/javascript");
    } else {
      const id = /^\/models\/([a-z]+)\.glb$/.exec(name)?.[1];
      assert(ids.includes(id));
      file = path.join(input, `${id}.glb`);
      response.setHeader("Content-Type", "model/gltf-binary");
    }
    response.end(await readFile(file));
  } catch { response.statusCode = 404; response.end(); }
});
await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
await writeFile(path.join(output, "session.json"), JSON.stringify({ session, sockets, temporary }));
const report = [];
try {
  await browser(["open", `http://127.0.0.1:${server.address().port}/preview.html`]);
  await browser(["wait", "--fn", "window.previewReady === true"]);
  for (const id of ids) {
    const result = (await browser(["eval", "--stdin"], `(async () => await window.renderPreview(${JSON.stringify(id)}))()`)).result;
    assert(result.startsWith("data:image/png;base64,"));
    const bytes = Buffer.from(result.split(",")[1], "base64");
    await writeFile(path.join(output, `${id}.png`), bytes);
    report.push({ id, width: 384, height: 216, sha256: createHash("sha256").update(bytes).digest("hex"), sourceSHA256: createHash("sha256").update(await readFile(path.join(input, `${id}.glb`))).digest("hex") });
  }
  await writeFile(path.join(output, "report.json"), `${JSON.stringify({ passed: true, source: "original NFS4 named menu track models", renderer: "Three.js 0.180.0; Chromium SwiftShader; fixed orthographic camera", viewFactor: .54, images: report }, null, 2)}\n`);
  console.log(JSON.stringify({ passed: true, previews: report.length, output }));
} finally {
  await browser(["close"]);
  await new Promise(resolve => server.close(resolve));
  await rm(temporary, { recursive: true, force: true });
  await rm(sockets, { recursive: true, force: true });
}
