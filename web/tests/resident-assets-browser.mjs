import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const modulePath = path.resolve(process.argv[2] ?? "");
assert.ok(process.argv[2]);
const engine = process.env.BROWSER_ENGINE ?? "webkit";
assert.ok(["webkit", "chromium"].includes(engine));
const out = path.join(root, ".local/yuyuhakusho/resident-assets");
const temporary = path.join(out, engine + "-temporary");
await mkdir(temporary, { recursive: true });
process.env.TMPDIR = temporary;
const require = createRequire(new URL("../package.json", import.meta.url));
const built = await require("vite").build({ configFile: false, root, cacheDir: path.join(temporary,"vite-cache"), publicDir: false, logLevel:"error", build:{write:false,minify:true,target:"es2022",lib:{entry:path.join(root,"web/src/game-api/resident-worker.ts"),formats:["es"],fileName:"resident-worker"}} });
const bundle = Array.isArray(built) ? built[0] : built;
const worker = Buffer.from(bundle.output.find(file=>file.type==="chunk").code);
await writeFile(path.join(temporary, "client.ts"), `import {beginResidentResources,declareResidentResources,sealResidentResources,releaseResidentResources} from ${JSON.stringify(path.join(root,"web/src/game-api/resident-assets.ts"))};window.beginProbe=payload=>beginResidentResources(window.probeWorker,payload.lease,payload.build,payload.resources,payload.origins,new AbortController().signal);window.declareProbe=(lease,resources)=>declareResidentResources(lease,resources,new AbortController().signal);window.sealProbe=lease=>sealResidentResources(lease,new AbortController().signal);window.releaseProbe=releaseResidentResources;window.clientReady=true;`);
await require("esbuild").build({entryPoints:[path.join(temporary,"client.ts")],outfile:path.join(temporary,"client.js"),bundle:true,format:"esm",legalComments:"none"});
const client = await readFile(path.join(temporary,"client.js"));
const svg = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="32" height="24"><path fill="#36785c" d="M0 0h32v24H0z"/></svg>');
const frames = 24000;
const wav = Buffer.alloc(44 + frames * 2);
wav.write("RIFF", 0); wav.writeUInt32LE(wav.length - 8, 4); wav.write("WAVEfmt ", 8); wav.writeUInt32LE(16, 16); wav.writeUInt16LE(1, 20); wav.writeUInt16LE(1, 22); wav.writeUInt32LE(48000, 24); wav.writeUInt32LE(96000, 28); wav.writeUInt16LE(2, 32); wav.writeUInt16LE(16, 34); wav.write("data", 36); wav.writeUInt32LE(frames * 2, 40);
for (let index = 0; index < frames; index++) wav.writeInt16LE(Math.round(Math.sin(index * Math.PI * 880 / 48000) * 12000), 44 + index * 2);
const hits = [];
const media = createServer((request, response) => {
  const name = new URL(request.url, "http://127.0.0.1").pathname;
  if (name !== "/resident-worker.js") hits.push({ name, range: request.headers.range ?? "" });
  response.setHeader("Access-Control-Allow-Origin", "*");
  response.setHeader("Access-Control-Expose-Headers", "Content-Length,Content-Range,Accept-Ranges");
  response.setHeader("Cache-Control", "no-store");
  const body = name === "/picture.svg" ? svg : ["/tone.wav", "/head-tone.wav"].includes(name) ? wav : name === "/resident-worker.js" ? worker : undefined;
  response.writeHead(body ? 200 : 404, { "Content-Type": name.endsWith(".wav") ? "audio/wav" : name.endsWith(".js") ? "text/javascript" : "image/svg+xml", "Content-Length": body?.length ?? 0, "Accept-Ranges": "bytes" });
  response.flushHeaders();
  if (name === "/picture.svg") setTimeout(() => response.end(body), 100);
  else response.end(body);
});
await new Promise(resolve => media.listen(0, "127.0.0.1", resolve));
const cdn = `http://127.0.0.1:${media.address().port}`;
let dataRequests = 0;
const app = createServer((request, response) => {
  const pathname = new URL(request.url, "http://127.0.0.1").pathname;
  if (pathname === "/asset-cache-worker.js") { response.writeHead(200,{"Content-Type":"text/javascript","Service-Worker-Allowed":"/","Cache-Control":"no-store"});response.end("importScripts("+JSON.stringify(cdn+"/resident-worker.js")+");");return; }
  if (pathname === "/resident-worker.js") {
    response.writeHead(200, { "Content-Type": "text/javascript", "Service-Worker-Allowed": "/", "Cache-Control": "no-store" }); response.end(worker); return;
  }
  if (pathname === "/local-picture.svg") { response.writeHead(200,{"Content-Type":"image/svg+xml","Content-Length":svg.length});response.end(svg);return; }
  if (pathname === "/client.js") { response.writeHead(200,{"Content-Type":"text/javascript"});response.end(client);return; }
  if (pathname === "/api/state") { dataRequests++; response.writeHead(200, { "Content-Type": "application/json" }); response.end(JSON.stringify({ frame: dataRequests })); return; }
  if (pathname === "/bga/picture.svg") { response.writeHead(302, { Location: cdn + "/picture.svg" }); response.end(); return; }
  response.writeHead(200, { "Content-Type": "text/html" }); response.end('<meta name="viewport" content="width=device-width,initial-scale=1"><button id="play" style="width:140px;height:90px">播放</button><img id="picture"><audio id="tone"></audio><script>window.probeWorker=' + JSON.stringify(cdn + '/resident-worker.js') + ';</script><script type="module" src="/client.js"></script>');
});
await new Promise(resolve => app.listen(0, "127.0.0.1", resolve));
const base = `http://127.0.0.1:${app.address().port}`;
const engines = await import(pathToFileURL(modulePath));
const browser = await engines[engine].launch({ headless: false, ...(process.argv[3] ? { executablePath: path.resolve(process.argv[3]) } : {}) });
const report = { passed: false, engine, checks: [], logs: [], hits };
try {
  const page = await browser.newPage({ viewport: { width: 844, height: 390 }, isMobile: true, hasTouch: true });
  page.setDefaultTimeout(15000);
  page.on("pageerror", error => report.logs.push(String(error)));
  page.on("console", message => { if (message.type() === "error") report.logs.push(message.text()); });
  await page.goto(base);
  await page.waitForFunction(()=>window.clientReady);
  await page.evaluate(async cdn=>{
    await beginProbe({lease:'first',build:'same-build',resources:[],origins:[location.origin,cdn]});
  },cdn);
  await page.evaluate(async () => {
    await Promise.race([navigator.serviceWorker.ready,new Promise((resolve,reject)=>setTimeout(()=>reject(Error("worker activation timeout")),5000))]);
    if (!navigator.serviceWorker.controller) await new Promise(resolve => navigator.serviceWorker.addEventListener('controllerchange', resolve, { once: true }));
    window.resident = async payload => {
      const channel = new MessageChannel();
      const result = new Promise((resolve, reject) => { const timer=setTimeout(()=>reject(Error("worker command timeout: "+payload.type)),5000);channel.port1.onmessage = ({ data }) => {clearTimeout(timer);data.ok ? resolve(data) : reject(Error(data.error))}; });
      navigator.serviceWorker.controller.postMessage(payload, [channel.port2]);
      return result;
    };
  });
  const resources = [{ url: base + "/bga/picture.svg", bytes: svg.length }, { url: cdn + "/tone.wav", bytes: wav.length }];
  await page.evaluate(async ({ resources, cdn }) => {
    await declareProbe('first',resources);
    await Promise.all([...(resources.map(async file => { const response = await fetch(file.url); if (!response.ok || (await response.arrayBuffer()).byteLength !== file.bytes) throw Error("preload failed"); })),new Promise(resolve=>setTimeout(resolve,20)).then(()=>fetch(resources[0].url)).then(response=>response.arrayBuffer())]);
    await sealProbe('first');
  }, { resources, cdn });
  const residency = await page.evaluate(()=>resident({type:'seal',lease:'first'}));
  assert.equal(residency.loaded,svg.length+wav.length);
  assert.equal(residency.total,svg.length+wav.length);
  const workerResult = await page.evaluate(async cdn => {
    const source = 'onmessage=async({data})=>{try{const response=await fetch(data);postMessage({status:response.status,bytes:(await response.arrayBuffer()).byteLength})}catch(error){postMessage({error:String(error)})}}';
    const url = URL.createObjectURL(new Blob([source],{type:'text/javascript'}));
    const worker = new Worker(url);
    const read = target => new Promise(resolve=>{worker.onmessage=event=>resolve(event.data);worker.postMessage(target)});
    try { return {known:await read(cdn+'/tone.wav'),missing:await read(cdn+'/worker-missing.svg')} }
    finally { worker.terminate();URL.revokeObjectURL(url) }
  },cdn);
  assert.deepEqual(workerResult.known,{status:200,bytes:wav.length});
  assert.equal(workerResult.missing.status,504);
  report.checks.push({name:'dedicated game workers use local bytes and cannot redownload',passed:true,...workerResult});
  const warmHits = hits.length;
  assert.equal(warmHits, 2);
  const initial = await page.evaluate(async ({ cdn }) => {
    const image = document.getElementById('picture'); image.src = '/bga/picture.svg?retry=1'; await image.decode();
    const range = await fetch(cdn + '/tone.wav', { headers: { Range: 'bytes=4-15' }, cache: 'reload' });
    return { image: [image.naturalWidth, image.naturalHeight], status: range.status, contentRange: range.headers.get('Content-Range'), bytes: [...new Uint8Array(await range.arrayBuffer())] };
  }, { cdn });
  assert.deepEqual(initial.image, [32, 24]);
  assert.equal(initial.status, 206);
  assert.equal(initial.contentRange, `bytes 4-15/${wav.length}`);
  assert.deepEqual(initial.bytes, [...wav.subarray(4, 16)]);
  await page.evaluate(cdn => {
    const audio = document.getElementById('tone'); audio.src = cdn + '/tone.wav';
    window.nativeAudio = { played: false };
    document.getElementById('play').onclick = () => { void audio.play().then(() => nativeAudio.played = true); };
  }, cdn);
  await page.touchscreen.tap(70, 45);
  await page.waitForFunction(() => nativeAudio.played && document.getElementById('tone').currentTime > 0.1);
  assert.equal(hits.length, warmHits, "native image/audio playback went back to the CDN");
  report.checks.push({ name: "native images, audio and byte ranges", passed: true, ...initial });
  const unknown = await page.evaluate(async cdn => (await fetch(cdn + '/missing.svg?retry=2')).status, cdn);
  assert.equal(unknown, 504);
  assert.equal(hits.length, warmHits, "missing media triggered a new CDN request");
  await page.evaluate(async resources => {
    await resident({ type: 'begin', lease: 'second', build: 'same-build', origins: [location.origin, new URL(resources[1].url).origin], resources });
    await resident({ type: 'release', lease: 'first' });
    for (const resource of resources) await (await fetch(resource.url, { cache: 'reload' })).arrayBuffer();
    await resident({ type: 'seal', lease: 'second' });
  }, resources);
  assert.equal(hits.length, warmHits, "rematch fetched successfully loaded assets again");
  if (engine === "chromium") {
    const session = await page.context().newCDPSession(page);
    await session.send("ServiceWorker.enable");
    await session.send("ServiceWorker.stopAllWorkers");
    assert.equal(await page.evaluate(async cdn => (await fetch(cdn + '/missing-after-restart.svg')).status, cdn), 504);
    assert.equal(hits.length, warmHits, "worker restart forgot the sealed match");
    report.checks.push({ name: "worker termination and restart", passed: true });
  }
  await page.evaluate(async resources => {
    const cache = await caches.open('wire-board-resident-assets-v1');
    await cache.delete(new URL('/picture.svg', new URL(resources[1].url).origin).href);
  }, resources);
  const lost = await page.evaluate(async () => (await fetch('/bga/picture.svg?broken=3')).status);
  assert.equal(lost, 504);
  assert.equal(hits.length, warmHits, "lost successful bytes were downloaded again");
  const data = await page.evaluate(async () => (await (await fetch('/api/state')).json()).frame);
  assert.ok(data > 0);
  report.checks.push({ name: "warm rematch, late release, missing and lost bytes, live data", passed: true });
  await page.screenshot({ path: path.join(out, engine + '.png') });
  await page.evaluate(async cdn => {
    await beginProbe({lease:'missing-required',build:'same-build',origins:[location.origin,cdn],resources:[]});
    await declareProbe('missing-required',[{url:cdn+'/required-but-not-loaded.svg',bytes:100}]);
  },cdn);
  await assert.rejects(page.evaluate(()=>sealProbe('missing-required')),/资源尚未完整加载/);
  assert.equal(hits.length,warmHits);
  report.checks.push({name:"declared mandatory resources block premature readiness",passed:true});
  await page.evaluate(async({cdn,bytes})=>{
    await beginProbe({lease:'initial-fallback',build:'same-build',origins:[location.origin,cdn],resources:[{url:cdn+'/initial-missing.svg',bytes}]});
    await fetch(cdn+'/initial-missing.svg');
    await (await fetch('/local-picture.svg')).arrayBuffer();
    await resident({type:'alias',lease:'initial-fallback',url:cdn+'/initial-missing.svg',source:location.origin+'/local-picture.svg'});
    await sealProbe('initial-fallback');
    const before=await (await fetch(cdn+'/initial-missing.svg')).arrayBuffer();
    if(before.byteLength!==bytes)throw Error('fallback alias was not retained');
  },{cdn,bytes:svg.length});
  assert.equal(hits.filter(hit=>hit.name==='/initial-missing.svg').length,1);
  report.checks.push({name:"an initial loading alternative is retained without later retries",passed:true});
  const headHits = hits.length;
  await page.evaluate(async ({cdn,bytes})=>{
    await beginProbe({lease:'head-first',build:'same-build',origins:[location.origin,cdn],resources:[{url:cdn+'/head-tone.wav',bytes}]});
    const head=await fetch(cdn+'/head-tone.wav',{method:'HEAD'});
    if(head.status!==200||(await head.arrayBuffer()).byteLength!==0)throw Error('invalid HEAD response');
    await sealProbe('head-first');
    const response=await fetch(cdn+'/head-tone.wav',{headers:{Range:'bytes=0-7'}});
    if(response.status!==206||(await response.arrayBuffer()).byteLength!==8)throw Error('HEAD did not retain full bytes');
  },{cdn,bytes:wav.length});
  assert.equal(hits.length-headHits,1);
  report.checks.push({name:'an initial HEAD saves one complete body for later native Range reads',passed:true});
  report.passed = true;
} catch (error) {
  report.error = String(error);
  throw error;
} finally {
  await writeFile(path.join(out, engine + '-result.json'), JSON.stringify(report, null, 2) + '\n');
  await browser.close();
  await new Promise(resolve => app.close(resolve));
  await new Promise(resolve => media.close(resolve));
  await rm(temporary, { recursive: true, force: true });
}
