import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const owned = JSON.parse(readFileSync(join(root, ".local/pokepath/native-browser/latest.json"), "utf8"));
const label = process.argv[2] ?? "current";
assert.match(label, /^[a-z0-9][a-z0-9-]*$/);
assert.equal(owned.mutedByBrowserFlag, true);
assert.equal(new URL(owned.base).hostname, "127.0.0.1");
const output = join(root, ".local/pokepath/verification/classic-controls", label);
mkdirSync(output, { recursive: true });
const environment = { ...process.env, AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const browser = (...args) => {
  const result = JSON.parse(execFileSync("agent-browser", ["--session", owned.browsers.a.session, ...args, "--json"], { cwd: root, env: environment, encoding: "utf8", timeout: 45000, maxBuffer: 8 << 20 }));
  assert.equal(result.success, true, JSON.stringify(result));
  return result.data;
};
const evaluate = expression => browser("eval", "-b", Buffer.from(expression).toString("base64")).result;
const state = () => evaluate("fetch('/api/state').then(response=>response.json()).then(state=>({roomID:state.room?.id,autoWave:state.room?.game?.data?.autoWave,epoch:state.room?.game?.data?.epoch,sound:localStorage.getItem('wb_sound')}))");
const selector = ".pokepath-classic-controls .auto";
const report = { command: `node games/pokepath/tests/classic-controls-browser.mjs ${label}`, fixture: owned.output, scope: "Real authenticated native game controls, mouse input and computed styles; full network pixel budgets are verified separately", passed: false, samples: [], failures: [] };
const settle = () => evaluate(`(async()=>{const control=document.querySelector('${selector}');await Promise.all(control.getAnimations({subtree:true}).filter(animation=>Number.isFinite(animation.effect?.getTiming().iterations)).map(animation=>animation.finished.catch(()=>{})));await new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)));return true})()`);
const click = () => { browser("mouse", "down", "left"); browser("mouse", "up", "left"); };
const style = () => evaluate(`(()=>{const control=document.querySelector('${selector}'),css=getComputedStyle(control),scene=getComputedStyle(control.closest('.pokepath-classic-scene')),pressed=control.getAttribute('aria-pressed')==='true',color=document.createElement('canvas').getContext('2d');color.fillStyle=scene.getPropertyValue(pressed?'--pp-autoWave2':'--pp-autoWave1').trim()||(pressed?'#70ac4c':'#2c70e3');const value=color.fillStyle;return{pressed,background:css.backgroundColor,expected:value,filter:css.filter,transform:css.transform,paddingTop:css.paddingTop,transitionProperty:css.transitionProperty,transitionDuration:css.transitionDuration}})()`);
const verify = (actual, expected, name) => { try { assert.deepEqual(actual, expected, name); } catch (error) { report.failures.push(error.message); } };
const screenshot = name => {
  const path = join(output, `${name}.png`);
  browser("screenshot", path);
  const bytes = readFileSync(path);
  return { path, bytes: bytes.length, sha256: createHash("sha256").update(bytes).digest("hex") };
};
let original;
try {
  original = state();
  assert.equal(original.roomID, owned.roomID);
  assert.equal(original.sound, "off");
  report.before = original;
  const box = evaluate(`(()=>{const control=document.querySelector('${selector}');control.scrollIntoView({block:'nearest'});const box=control.getBoundingClientRect();return{x:box.x+box.width/2,y:box.y+box.height/2}})()`);
  browser("mouse", "move", String(Math.round(box.x)), String(Math.round(box.y)));
  for (const pressed of [false, true]) {
    if (style().pressed !== pressed) click();
    browser("wait", "--fn", `document.querySelector('${selector}').getAttribute('aria-pressed')==='${pressed}'`);
    browser("wait", "--fn", `fetch('/api/state').then(response=>response.json()).then(state=>state.room?.game?.data?.autoWave===${pressed})`);
    settle();
    const hovered = style();
    const hex = /^#([0-9a-f]{6})$/i.exec(hovered.expected)?.[1];
    assert.ok(hex);
    const expected = `rgb(${[0, 2, 4].map(offset => parseInt(hex.slice(offset, offset + 2), 16)).join(", ")})`;
    verify(hovered.background, expected, `${pressed}: hover preserves the dedicated theme color`);
    verify(hovered.filter, "brightness(1.15)", `${pressed}: original hover brightness`);
    verify(hovered.transitionProperty, "transform", `${pressed}: no inherited background transition`);
    verify(hovered.transitionDuration, "0.05s", `${pressed}: original transform timing`);
    const hoverScreenshot = screenshot(`${pressed ? "on" : "off"}-hover`);
    browser("mouse", "down", "left");
    settle();
    const held = style();
    verify(held.transform, "none", `${pressed}: button stays in place while pressed`);
    verify(held.filter, "brightness(1.15)", `${pressed}: held brightness`);
    verify(held.paddingTop, "3px", `${pressed}: original held content offset`);
    const heldScreenshot = screenshot(`${pressed ? "on" : "off"}-held`);
    browser("mouse", "up", "left");
    report.samples.push({ pressed, hovered, held, screenshots: [hoverScreenshot, heldScreenshot] });
  }
} finally {
  browser("mouse", "up", "left");
  if (original) {
    if (style().pressed !== original.autoWave) click();
    browser("wait", "--fn", `fetch('/api/state').then(response=>response.json()).then(state=>state.room?.game?.data?.autoWave===${original.autoWave})`);
    report.after = state();
    verify(report.after, original, "test restores initial room control state");
  }
  report.passed = report.samples.length === 2 && report.failures.length === 0;
  writeFileSync(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
}
assert.equal(report.passed, true, report.failures.join("\n"));
process.stdout.write(JSON.stringify({ passed: report.passed, samples: report.samples.length, output }) + "\n");
