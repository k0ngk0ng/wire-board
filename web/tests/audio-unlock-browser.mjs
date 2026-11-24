import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import { createHash } from "node:crypto";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const modulePath = path.resolve(process.argv[2] ?? "");
assert.ok(
  process.argv[2] && !path.relative(root, modulePath).startsWith(".."),
  "Provide the installed WebKit automation module inside this workspace",
);
const out = path.join(root, ".local/yuyuhakusho/controller-audio-browser");
const temporary = path.join(out, "temporary");
await mkdir(temporary, { recursive: true });
process.env.TMPDIR = temporary;
const require = createRequire(new URL("../package.json", import.meta.url));
const ts = require("typescript");
const reference = process.env.REFERENCE_SHA;
if (reference) assert.match(reference, /^[a-f0-9]{40}$/);
const engine = process.env.BROWSER_ENGINE ?? "webkit";
assert.ok(["webkit", "chromium"].includes(engine));
const report = {
  passed: false,
  browser: engine,
  nativeTouch: true,
  referenceSHA: reference,
  workingSourceDigests: {},
  executedSourceDigests: {},
};
for (const name of ["audio.ts", "audio-silence.ts"])
  report.workingSourceDigests[name] = createHash("sha256")
    .update(await readFile(path.join(root, "web/src", name)))
    .digest("hex");
await writeFile(
  path.join(temporary, "index.html"),
  `<meta name="viewport" content="width=device-width,initial-scale=1"><button id="c" style="width:140px;height:100px;touch-action:none">C</button><script type="module" src="/entry.mjs"></script>`,
);
await writeFile(
  path.join(temporary, "entry.mjs"),
  `import {GameAudio} from ${JSON.stringify("/@fs" + path.join(root, "web/src/audio.ts"))};const audio=new GameAudio;audio.mount();const media=document.querySelector('audio[data-platform-audio]');const play=media.play.bind(media);const state=window.audioUnlockProbe={presses:0,plays:0,errors:[],results:[],bytes:[]};media.addEventListener('error',()=>state.errors.push(media.error?.code));media.play=()=>{state.plays++;state.bytes.push(atob(media.src.split(',')[1]).length);return play()};const button=document.getElementById('c');button.addEventListener('pointerdown',()=>{state.presses++;void audio.unlock().then(value=>state.results.push(value))});button.addEventListener('touchstart',event=>event.preventDefault(),{passive:false});window.disposeAudioUnlockProbe=()=>audio.dispose();window.audioUnlockReady=true;`,
);
const server = await require("vite").createServer({
  configFile: false,
  root: temporary,
  cacheDir: path.join(temporary, "vite-cache"),
  publicDir: false,
  server: { host: "127.0.0.1", port: 0, hmr: false, fs: { allow: [root] } },
});
await server.listen();
const engines = await import(pathToFileURL(modulePath).href);
const browser = await engines[engine].launch({
  headless: true,
  ...(engine === "chromium" ? { executablePath: process.argv[3] } : {}),
});
const page = await browser.newPage({
  viewport: { width: 500, height: 350 },
  isMobile: true,
  hasTouch: true,
});
try {
  if (reference)
    for (const name of ["audio.ts", "audio-silence.ts"]) {
      const source = execFileSync(
        "git",
        ["show", reference + ":web/src/" + name],
        { cwd: root, encoding: "utf8" },
      );
      report.executedSourceDigests[name] = createHash("sha256")
        .update(source)
        .digest("hex");
      const body = ts.transpileModule(source, {
        compilerOptions: {
          module: ts.ModuleKind.ESNext,
          target: ts.ScriptTarget.ES2022,
          removeComments: true,
        },
      }).outputText;
      await page.route("**/src/" + name + "*", (route) =>
        route.fulfill({ contentType: "application/javascript", body }),
      );
    }
  if (!reference)
    report.executedSourceDigests = { ...report.workingSourceDigests };
  await page.goto(server.resolvedUrls.local[0]);
  await page.waitForFunction(() => window.audioUnlockReady);
  await page.touchscreen.tap(70, 50);
  await page.waitForTimeout(250);
  report.first = await page.evaluate(() => structuredClone(audioUnlockProbe));
  for (let count = 0; count < 40; count++) {
    await page.touchscreen.tap(70, 50);
    await page.waitForTimeout(15);
  }
  await page.waitForTimeout(250);
  report.after = await page.evaluate(() => structuredClone(audioUnlockProbe));
  assert.equal(report.after.presses, 41);
  assert.equal(
    report.after.plays,
    1,
    "Rapid controls restarted native unlock playback",
  );
  assert.deepEqual(report.after.errors, [], "WebKit rejected the silent PCM");
  assert.deepEqual(
    report.after.results,
    Array(41).fill(true),
    "Native unlock remained pending or failed",
  );
  assert.deepEqual(report.after.bytes, [844]);
  report.passed = true;
} catch (error) {
  report.error = String(error);
  process.exitCode = 1;
} finally {
  await page
    .screenshot({
      path: path.join(
        out,
        engine + (reference ? "-reference.png" : "-current.png"),
      ),
    })
    .catch(() => {});
  await browser.close();
  await server.close();
  await writeFile(
    path.join(out, engine + (reference ? "-reference.json" : "-current.json")),
    JSON.stringify(report, null, 2) + "\n",
  );
  await rm(temporary, { recursive: true, force: true });
  console.log(JSON.stringify(report));
}
