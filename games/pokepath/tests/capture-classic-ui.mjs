import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const url = process.argv[2] ?? "http://127.0.0.1:8792/";
if (!["127.0.0.1", "localhost"].includes(new URL(url).hostname)) throw Error("A local independent fixture URL is required");
const output = join(root, ".local/pokepath/reference-visual");
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: join(root, ".local/pokepath/classic-fixture/profile"), AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_SCREENSHOT_DIR: output, AGENT_BROWSER_NO_VERSION_CHECK: "1" };
mkdirSync(output, { recursive: true });
const session = execFileSync("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", "ppvis"], { cwd: root, env: environment, encoding: "utf8" }).trim();
const browser = (...args) => {
  const result = JSON.parse(execFileSync("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, encoding: "utf8", timeout: 45000, maxBuffer: 8 << 20 }));
  if (!result.success) throw Error(JSON.stringify(result));
  return result.data;
};
const evaluate = source => browser("eval", "-b", Buffer.from(source).toString("base64")).result;
const screenshot = label => { const path = join(output, `native-${label}.png`); browser("screenshot", path); return path; };
const readyImages = () => browser("wait", "--fn", "Array.from(document.images).filter(image=>{const box=image.getBoundingClientRect();return box.width>0&&box.height>0&&box.bottom>0&&box.top<innerHeight&&box.right>0&&box.left<innerWidth}).every(image=>image.complete&&image.naturalWidth>0)&&[...document.querySelectorAll('.pokepath-profile-badges>[data-achievement]')].every(button=>getComputedStyle(button).backgroundImage.startsWith('url(\"data:image/'))");
const clickPoint = point => { browser("mouse", "move", String(point.x), String(point.y)); browser("mouse", "down", "left"); browser("mouse", "up", "left"); };
const report = { command: `node games/pokepath/tests/capture-classic-ui.mjs ${url}`, scope: "Independent native Go rules with React/Canvas fixture; authenticated platform acceptance is recorded separately", muted: true, passed: false, screenshots: {}, geometry: {} };
try {
  browser("open", url);
  browser("set", "viewport", "1238", "674");
  browser("reload");
  browser("wait", "--fn", "document.documentElement.dataset.ready==='true'");
  readyImages();
  report.geometry = evaluate("(()=>({font:[...document.fonts].some(face=>face.family==='PressStart2P'&&face.status==='loaded'),frames:document.querySelectorAll('iframe').length,cards:document.querySelectorAll('[data-party-slot]').length,menus:document.querySelectorAll('[data-classic-section]').length,scene:document.querySelector('.pokepath-classic-scene').getBoundingClientRect().toJSON(),canvas:document.querySelector('canvas').getBoundingClientRect().toJSON(),images:[...document.images].every(image=>image.src.startsWith('data:')&&image.naturalWidth>0)}))()");
  assert.equal(report.geometry.frames, 0);
  assert.equal(report.geometry.cards, 10);
  assert.equal(report.geometry.menus, 9);
  assert.equal(report.geometry.font, true);
  assert.equal(report.geometry.images, true);
  assert.deepEqual([report.geometry.canvas.x, report.geometry.canvas.y, report.geometry.canvas.width, report.geometry.canvas.height], [259, 25, 720, 624]);
  report.geometry.initialIcon = evaluate("(async()=>{const bytes=new Uint8Array(await(await fetch('/pokepath/v1/src/assets/images/icons/pokeball-close.png')).arrayBuffer()),expected='data:image/png;base64,'+btoa(Array.from(bytes,value=>String.fromCharCode(value)).join('')),paint=getComputedStyle(document.querySelector('[data-party-slot=\"0\"] button.deploy')).backgroundImage;return{closedIcon:paint.includes(expected),gradient:paint.includes('linear-gradient')}})()");
  assert.deepEqual(report.geometry.initialIcon, { closedIcon: true, gradient: true });
  report.screenshots.initial = screenshot("initial");
  browser("click", '[data-party-slot="0"] button.deploy');
  report.screenshots.placement = screenshot("placement");
  const point = evaluate("fetch('/pokepath/v1/routes/0.json').then(response=>response.json()).then(route=>{const tile=route.placements.find(tile=>tile.terrain===1),canvas=document.querySelector('canvas'),rect=canvas.getBoundingClientRect();return{x:rect.left+(tile.x+12)*rect.width/canvas.width,y:rect.top+(tile.y+12)*rect.height/canvas.height}})");
  clickPoint(point);
  browser("wait", "--fn", "document.querySelector('[data-party-slot=\"0\"] button.deploy').getAttribute('aria-label').startsWith('收回')");
  browser("mouse", "move", String(point.x + 1), String(point.y + 1));
  report.geometry.deployedIcon = evaluate("(async()=>{const bytes=new Uint8Array(await(await fetch('/pokepath/v1/src/assets/images/icons/pokeball-open.png')).arrayBuffer()),expected='data:image/png;base64,'+btoa(Array.from(bytes,value=>String.fromCharCode(value)).join('')),paint=getComputedStyle(document.querySelector('[data-party-slot=\"0\"] button.deploy')).backgroundImage;return{openIcon:paint.includes(expected),gradient:paint.includes('linear-gradient')}})()");
  assert.deepEqual(report.geometry.deployedIcon, { openIcon: true, gradient: true });
  report.screenshots.deployed = screenshot("deployed");
  browser("click", '[data-party-slot="0"] button.info');
  report.screenshots.details = screenshot("details");
  const details = evaluate("document.querySelector('.pokepath-details').getBoundingClientRect().toJSON()");
  assert.deepEqual([details.width, details.height], [608, 578]);
  report.geometry.details = details;
  browser("press", "Escape");
  for (const panel of ["shop", "box", "inventory", "routes", "dex", "profile", "settings"]) {
    browser("click", `[data-classic-section="${panel}"]`);
    readyImages();
    const box = evaluate("document.querySelector('.pokepath-central-panel').getBoundingClientRect().toJSON()");
    assert.deepEqual([box.x, box.y, box.width, box.height], [259, 25, 720, 624]);
    if (panel === "profile") {
      report.geometry.profileValues = evaluate("(()=>{const input=document.querySelector('.pokepath-profile-player input');return{owned:document.querySelector('[data-profile-stat=\"2\"] dd').textContent,shiny:document.querySelector('[data-profile-stat=\"3\"] dd').textContent,totalLevel:document.querySelector('[data-profile-stat=\"5\"] dd').textContent,name:input.value,placeholder:input.placeholder,arrows:[...document.querySelectorAll('.pokepath-profile-player > button.previous,.pokepath-profile-player > button.next')].map(button=>button.textContent)}})()");
      assert.deepEqual(report.geometry.profileValues, { owned: "1/256", shiny: "0/256", totalLevel: "1/25.600", name: "", placeholder: "Player", arrows: ["<", ">"] });
    }
    report.screenshots[panel] = screenshot(panel);
  }
  browser("click", '[data-classic-section="settings"]');
  browser("set", "viewport", "320", "674");
  browser("wait", "--fn", "document.querySelector('.pokepath-classic-menu').getBoundingClientRect().width===320");
  report.screenshots.mobile = screenshot("mobile");
  assert.equal(evaluate("document.documentElement.scrollWidth<=innerWidth"), true);
  report.passed = true;
} finally {
  writeFileSync(join(output, "native-geometry.json"), JSON.stringify(report, null, 2) + "\n");
}
process.stdout.write(JSON.stringify(report) + "\n");
