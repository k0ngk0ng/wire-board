import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const modulePath = path.resolve(process.argv[2] ?? "");
assert.ok(process.argv[2] && !path.relative(root, modulePath).startsWith(".."));
const engine = process.env.BROWSER_ENGINE ?? "webkit";
assert.ok(["webkit", "chromium"].includes(engine));
const reference = process.env.TRANSIENT_REFERENCE_SHA;
assert.match(reference ?? "", /^[a-f0-9]{40}$/);
const out = path.join(root, ".local/yuyuhakusho/effect-mixer");
const temporary = path.join(out, engine + "-temporary");
await mkdir(temporary, { recursive: true });
process.env.TMPDIR = temporary;
const require = createRequire(new URL("../package.json", import.meta.url));
const engines = await import(pathToFileURL(modulePath));
const browser = await engines[engine].launch({ headless: false, ...(process.argv[3] ? { executablePath: path.resolve(process.argv[3]) } : {}) });
const adapters = ["games/yuyuhakusho/web/audio.ts", "games/basketball/web/audio.ts"];
const report = { passed: false, engine, reference, physicalPhone: false, cases: [], sourceSHA256: {} };
for (const file of ["web/src/audio.ts", "web/src/audio-spatial.ts", ...adapters]) report.sourceSHA256[file] = createHash("sha256").update(await readFile(path.join(root, file))).digest("hex");
let server;
try {
  const files = new Map();
  server = createServer((request, response) => {
    const name = new URL(request.url, "http://127.0.0.1").pathname.slice(1);
    const body = files.get(name);
    response.writeHead(body ? 200 : 404, { "Content-Type": name.endsWith(".mjs") ? "text/javascript" : "text/html" });
    response.end(body);
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const url = `http://127.0.0.1:${server.address().port}/`;
  for (const mode of ["reference", "current"]) {
    const entry = path.join(temporary, mode + ".ts");
    await writeFile(entry, `import {GameAudio} from ${JSON.stringify(path.join(root, "web/src/audio.ts"))};import {MatchAudio} from ${JSON.stringify(path.join(root, adapters[0]))};import {BasketballAudio} from ${JSON.stringify(path.join(root, adapters[1]))};const audio=new GameAudio;audio.mount();const original=audio.channel('probe');const port={...original};for(const name of ['effect','transient','playWithFallback'])port[name]=(...args)=>{const row={method:name,phase:probe.phase};probe.requests.push(row);return original[name](...args).then(ok=>{row.ok=ok;return ok})};const yuyu=new MatchAudio(port,'','probe',0);yuyu.sync({phase:'playing',options:{arena:0}},undefined,true,true,false);const basketball=new BasketballAudio(port);document.getElementById('c').addEventListener('pointerdown',()=>{void audio.unlock();const n=++probe.presses;yuyu.consume({frame:String(n),paused:false},[0x77,0x7b]);void basketball.consume([19,21],n).then(result=>probe.results.push({...result,phase:probe.phase}))});window.finishProbe=()=>{yuyu.dispose();basketball.dispose();audio.dispose()};window.muteProbe=()=>audio.setEnabled(false);window.unmuteProbe=()=>audio.setEnabled(true);window.probeReady=true;`);
    const bundle = path.join(temporary, mode + ".mjs");
    await require("esbuild").build({ entryPoints: [entry], outfile: bundle, bundle: true, format: "esm", legalComments: "none", define: { "import.meta.env": JSON.stringify({ BASE_URL: "/" }) }, plugins: mode === "reference" ? [{ name: "frozen-adapters", setup(build) { build.onLoad({ filter: /games\/(?:yuyuhakusho|basketball)\/web\/audio\.ts$/ }, args => ({ contents: execFileSync("git", ["show", reference + ":" + path.relative(root, args.path).split(path.sep).join("/")], { cwd: root, encoding: "utf8" }), loader: "ts", resolveDir: path.dirname(args.path) })); } }] : [] });
    files.set(mode + ".mjs", await readFile(bundle));
    files.set(mode + ".html", Buffer.from(`<meta name="viewport" content="width=device-width,initial-scale=1"><button id="c" style="width:200px;height:180px;touch-action:none">C</button><script type="module" src="${mode}.mjs"></script>`));
    const page = await browser.newPage({ viewport: { width: 1174, height: 744 }, isMobile: true, hasTouch: true });
    await page.addInitScript(() => {
      const state = window.probe = { phase: "warmup", presses: 0, plays: [], loads: [], decodes: [], started: [], requests: [], results: [], rms: {}, errors: [] };
      const play = HTMLMediaElement.prototype.play;
      HTMLMediaElement.prototype.play = function () {
        const row = { phase: state.phase, stream: !!this.srcObject, source: this.src.split("/").at(-1) };
        state.plays.push(row);
        return play.call(this).then(value => { row.ok = true; return value; }, error => { row.error = String(error); throw error; });
      };
      const load = HTMLMediaElement.prototype.load;
      HTMLMediaElement.prototype.load = function () { state.loads.push(state.phase); return load.call(this); };
      const decode = AudioContext.prototype.decodeAudioData;
      AudioContext.prototype.decodeAudioData = function (...args) { state.decodes.push(state.phase); return decode.apply(this, args); };
      const start = AudioBufferSourceNode.prototype.start;
      AudioBufferSourceNode.prototype.start = function (...args) { state.started.push(state.phase); return start.apply(this, args); };
      const connect = AudioNode.prototype.connect;
      const analysers = new WeakMap();
      AudioNode.prototype.connect = function (destination, ...args) {
        if (!(destination instanceof MediaStreamAudioDestinationNode)) return connect.call(this, destination, ...args);
        let analyser = analysers.get(destination);
        if (!analyser) {
          analyser = this.context.createAnalyser();
          analysers.set(destination, analyser);
          connect.call(analyser, destination);
          const data = new Float32Array(analyser.fftSize);
          setInterval(() => {
            analyser.getFloatTimeDomainData(data);
            let sum = 0;
            for (const value of data) sum += value * value;
            state.rms[state.phase] = Math.max(state.rms[state.phase] ?? 0, Math.sqrt(sum / data.length));
          }, 20);
        }
        return connect.call(this, analyser, ...args);
      };
    });
    page.on("pageerror", error => report.cases.push({ mode, pageError: String(error) }));
    await page.goto(url + mode + ".html");
    await page.waitForFunction(() => window.probeReady);
    await page.touchscreen.tap(100, 90);
    await page.waitForTimeout(1500);
    await page.evaluate(() => { probe.phase = "rapid-c"; });
    for (let index = 0; index < 80; index++) { await page.touchscreen.tap(100, 90); await page.waitForTimeout(40); }
    await page.waitForTimeout(500);
    const streams = await page.evaluate(() => [...document.querySelectorAll("audio")].filter(media => media.srcObject).map(media => ({ time: media.currentTime, paused: media.paused, readyState: media.readyState, error: media.error?.code, voices: media.dataset.spatialVoices })));
    await page.evaluate(() => { muteProbe(); probe.phase = "muted"; });
    for (let index = 0; index < 10; index++) await page.touchscreen.tap(100, 90);
    await page.waitForTimeout(150);
    await page.evaluate(() => { unmuteProbe(); probe.phase = "unmuted"; });
    await page.touchscreen.tap(100, 90);
    await page.waitForTimeout(500);
    const state = await page.evaluate(() => structuredClone(probe));
    const value = { mode, ...state, streams };
    report.cases.push(value);
    await page.screenshot({ path: path.join(out, engine + "-" + mode + ".png") });
    await page.evaluate(() => finishProbe());
    await page.close();
    if (mode === "current") {
      assert.equal(state.started.filter(phase => phase === "rapid-c").length, 320);
      assert.equal(state.decodes.filter(phase => phase === "rapid-c").length, 0);
      assert.equal(state.plays.filter(row => row.phase === "rapid-c").length, 0);
      assert.ok(state.loads.filter(phase => phase === "rapid-c").length <= 2);
      assert.ok(state.rms["rapid-c"] > 0.001);
      assert.equal(state.started.filter(phase => phase === "muted").length, 0);
      assert.ok(state.started.includes("unmuted"));
      assert.equal(streams.length, 1);
      assert.ok(streams[0].time > 0 && !streams[0].paused && !streams[0].error);
      assert.ok(state.results.filter(row => row.phase === "rapid-c").every(row => !row.failed.length));
    }
  }
  report.passed = true;
} catch (error) {
  report.error = String(error);
  throw error;
} finally {
  await writeFile(path.join(out, engine + "-result.json"), JSON.stringify(report, null, 2) + "\n");
  await browser.close();
  await new Promise(resolve => server ? server.close(resolve) : resolve());
  await rm(temporary, { recursive: true, force: true });
}
