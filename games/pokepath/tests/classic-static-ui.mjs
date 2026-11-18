import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { createHash } from "node:crypto";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const url = process.argv[2] ?? "http://127.0.0.1:8792/";
assert.ok(["127.0.0.1", "localhost"].includes(new URL(url).hostname));
const output = join(root, ".local/pokepath/verification/classic-static");
const profile = join(output, "profile");
mkdirSync(output, { recursive: true });
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const session = execFileSync("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "ppstatic"], { cwd: root, env: environment, encoding: "utf8" }).trim();
const browser = (...args) => {
  const value = JSON.parse(execFileSync("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, encoding: "utf8", timeout: 45000, maxBuffer: 8 << 20 }));
  assert.equal(value.success, true, JSON.stringify(value));
  return value.data;
};
const reference = join(root, ".local/pokepath/reference-visual/stable-initial-chinese.png");
const actual = join(output, "actual.png");
const report = { command: `node games/pokepath/tests/classic-static-ui.mjs ${url}`, passed: false, samples: [] };
try {
  browser("open", url);
  browser("set", "viewport", "1238", "674");
  browser("reload");
  browser("wait", "--fn", "document.documentElement.dataset.ready==='true'&&document.fonts.status==='loaded'");
  report.font = browser("eval", "getComputedStyle(document.querySelector('.pokepath-classic-scene')).fontFamily").result;
  assert.match(report.font, /, sans-serif$/);
  report.styles = browser("eval", "(()=>{const style=selector=>getComputedStyle(document.querySelector(selector));return{diamondBoxSizing:getComputedStyle(document.querySelector('.pokepath-classic-card'),'::before').boxSizing,previewHeight:document.querySelector('.pokepath-enemy-preview').getBoundingClientRect().height,titleFilter:style('.pokepath-enemy-info > h3').filter,labelShadow:style('.pokepath-enemy-info dt').textShadow,valueShadow:style('.pokepath-enemy-info dd').textShadow,formationFilter:style('.pokepath-formations-load button').filter,menuTexture:style('.pokepath-classic-menu').backgroundImage.startsWith('url(\"data:image/png')}})()").result;
  assert.equal(report.styles.diamondBoxSizing, "content-box");
  assert.equal(report.styles.previewHeight, 47);
  assert.match(report.styles.titleFilter, /drop-shadow/);
  assert.notEqual(report.styles.labelShadow, "none");
  assert.equal(report.styles.valueShadow, report.styles.labelShadow);
  assert.equal(report.styles.formationFilter, "brightness(0.7)");
  assert.equal(report.styles.menuTexture, true);
  report.lock = browser("eval", "(()=>{const icon=document.querySelector('.pokepath-card-lock'),label=document.querySelector('.pokepath-card-requirement'),style=getComputedStyle(icon),text=getComputedStyle(label);return{fontSize:style.fontSize,lineHeight:style.lineHeight,opacity:style.opacity,parent:label.contains(icon),bottom:text.bottom,labelLineHeight:text.lineHeight}})()").result;
  assert.deepEqual(report.lock, { fontSize: "30px", lineHeight: "48px", opacity: "0.5", parent: true, bottom: "8px", labelLineHeight: "14px" });
  browser("mouse", "move", "615", "20");
  browser("screenshot", actual);
  const sources = [reference, actual].map(path => "data:image/png;base64," + readFileSync(path).toString("base64"));
  const points = [[240, 220], [10, 400], [10, 600], [240, 600], [1000, 400], [1200, 400], [1000, 520], [1200, 520]];
  const script = `(async()=>{const sources=${JSON.stringify(sources)},points=${JSON.stringify(points)};return Promise.all(sources.map(src=>new Promise((resolve,reject)=>{const image=new Image();image.onerror=reject;image.onload=()=>{const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;const context=canvas.getContext('2d');context.drawImage(image,0,0);resolve(points.map(([x,y])=>[...context.getImageData(x,y,1,1).data]))};image.src=src})))})()`;
  const evaluated = JSON.parse(execFileSync("agent-browser", ["--session", session, "eval", "--stdin", "--json"], { input: script, cwd: root, env: environment, encoding: "utf8", timeout: 45000, maxBuffer: 8 << 20 }));
  assert.equal(evaluated.success, true, JSON.stringify(evaluated));
  const [expected, pixels] = evaluated.data.result;
  report.sources = [reference, actual].map(path => ({ path, sha256: createHash("sha256").update(readFileSync(path)).digest("hex") }));
  report.samples = points.map((point, index) => ({ point, expected: expected[index], actual: pixels[index] }));
  for (const sample of report.samples) assert.deepEqual(sample.actual, sample.expected, `static pixel ${sample.point}`);
  report.passed = true;
} finally {
  try { browser("close"); } finally { rmSync(profile, { recursive: true, force: true }); }
  writeFileSync(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
}
process.stdout.write(JSON.stringify(report) + "\n");
