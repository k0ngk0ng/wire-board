import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { referenceDOMClass, verifyDOMOracle } from "./dom-oracle.mjs";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const sourcePath = "game/scenes/MenuScene.js";
const source = await readFile(new URL(`../../../.local/pokepath/research/source/src/js/${sourcePath}`, import.meta.url), "utf8");
const parsed = ts.createSourceFile("MenuScene.js", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
const constants = ts.transpileModule(parsed.statements.filter(node => ts.isVariableStatement(node)).map(node => node.getText(parsed)).join("\n"), { compilerOptions: { removeComments: true, target: ts.ScriptTarget.ES2022 } }).outputText;
const sources = await Promise.all([
  referenceDOMClass("utils/Element.js", "Element", ["constructor", "create"]),
  referenceDOMClass("utils/Input.js", "Input", ["constructor", "render", "renderTypeText"]),
  referenceDOMClass(sourcePath, "MenuScene", ["render", "renderOfficial", "renderSettings", "renderAudio", "renderShortcuts", "renderData", "renderCredits", "createSection", "createArrowRow", "createSliderRow", "createOpacitySliderRow", "createOptionRow", "createLabelValueRow", "bindArrowEvents", "makeButton", "makeButtonWithHover", "syncExportDataButtonState", "update"])
]);
const settings = { audio: { master: 12, music: 4, ui: 4, effects: 1 }, language: 8, theme: "original", font: "PressStart2P", showDamage: true, autoStop: false, autoStopBoss: false, autoReset: 0, displayHealth: 0, mapEffectsMode: 1, indicatorField: false, indicatorShape: false, fastTarget: false, durationCC: false, particles: true, disableTagColors: false, mute: [false, true, false, true, false], effectOpacity: { projectiles: 85, areas: 90, beams: 80, waves: 75, auras: 50, bombardment: 70, special: 65 } };
const input = { view: { progression: { settings } } };
const reference = `const{dictionary,input,parameters}=window.oracleData;const text=dictionary.text,playSound=()=>{},setVolume=()=>{},isSaveExportDisabled=()=>true;${constants}\n${sources.map(source=>source.code).join("\n")}const settings=input.view.progression.settings;const config={...settings,font:OPTION.font.indexOf(parameters.font),mapEffects:settings.mapEffectsMode,showParticles:settings.particles};const scene=Object.create(MenuScene.prototype);Object.assign(scene,{container:document.querySelector('.section-scene-container'),main:{lang:Number(parameters.language),uiTheme:['original','classic','monochrome','halloween'].indexOf(parameters.theme),mute:settings.mute},getConfig:()=>config});window.settingsReference=scene;scene.render();scene.update();`;
const scroll = (reference, native) => async ({ kind, evaluate }) => {
  const selectors = kind === "reference" ? [".menu-scene-container", reference] : [".pokepath-settings-list", native];
  await evaluate(`(()=>{const [pane,target]=${JSON.stringify(selectors)}.map(selector=>document.querySelector(selector));pane.scrollTop=Math.max(0,target.offsetTop-12);return pane.scrollTop})()`);
};
const probe = selectors => `(()=>{const s=${JSON.stringify(selectors)},items=selector=>[...document.querySelectorAll(selector)],text=element=>element?.innerText?.trim()??'';const groups=items(s.section).map((element,index)=>({title:text(element.querySelector(s.title)),rowCount:element.querySelectorAll(s.row).length,sliders:[...element.querySelectorAll('input[type=range]')].map(input=>[input.value,input.min,input.max,input.step,text(input.parentElement.querySelector(s.output))]),links:[...element.querySelectorAll('a')].map(link=>[text(link),link.href]),options:index===1||index===2?[...element.querySelectorAll(s.option)].map(row=>[text(row.firstElementChild),[...row.querySelectorAll(s.choice)].map(button=>[text(button),button.matches(s.selected)])]):[]}));return{groups}})()`;
const verifyNative = async ({ browser, evaluate, ready }) => {
  const bring = async selector => await evaluate(`document.querySelector(${JSON.stringify(selector)}).scrollIntoView({block:'center'})`);
  const center = (selector, horizontal = 0.5) => evaluate(`(()=>{const control=document.querySelector(${JSON.stringify(selector)}),r=control.getBoundingClientRect(),x=Math.round(r.x+r.width*${horizontal}),y=Math.round(r.y+r.height/2);return{x,y,hit:control.contains(document.elementFromPoint(x,y))}})()`);
  const click = async selector => {
    await bring(selector);
    await ready();
    const point = await center(selector);
    assert.ok(point.hit, JSON.stringify({ selector, point }));
    await browser("mouse", "move", String(point.x), String(point.y));
    await browser("mouse", "down", "left");
    await browser("mouse", "up", "left");
  };
  const change = async (selector, key) => {
    await bring(selector);
    await evaluate(`document.querySelector(${JSON.stringify(selector)}).focus()`);
    await browser("press", key);
  };
  const audio = "#pokepath-volume-music";
  const opacity = "#pokepath-effect-auras";
  const area = '[data-setting-name="mute.3"] button:last-child';
  const beam = '[data-setting-name="mute.4"] button:last-child';
  await change(audio, "End");
  await change(audio, "Home");
  await change(opacity, "ArrowRight");
  await click('[data-setting-name="mapEffectsMode"] button:last-child');
  await click(beam);
  const actions = [
    { type: "settings", name: "audio.music", value: 20 },
    { type: "settings", name: "audio.music", value: 0 },
    { type: "settings", name: "effectOpacity.auras", value: 55 },
    { type: "settings", name: "mapEffectsMode", value: 2 },
    { type: "settings", name: "mute.4", enabled: true }
  ];
  assert.deepEqual(await evaluate("window.settingReceipts"), actions);
  assert.deepEqual(await evaluate(`([${JSON.stringify(audio)},${JSON.stringify(opacity)}].map(selector=>document.querySelector(selector).parentElement.querySelector('output').textContent))`), ["0", "55%"]);
  assert.deepEqual(await evaluate(`([${JSON.stringify(area)},${JSON.stringify(beam)}].map(selector=>document.querySelector(selector).getAttribute('aria-pressed')))`), ["true", "true"]);
  await evaluate("window.setSettingsReadOnly(true)");
  await ready();
  assert.equal(await evaluate("[...document.querySelectorAll('.pokepath-settings-list input, .pokepath-settings-list button')].filter(control=>control.closest('[data-setting-name]')).every(control=>control.disabled)"), true);
  await change(audio, "End");
  assert.deepEqual(await evaluate("window.settingReceipts"), actions);
  await evaluate("window.setSettingsReadOnly(false)");
  await browser("set", "viewport", "320", "674");
  await ready();
  const root = fileURLToPath(new URL("../../../", import.meta.url));
  const endpoint = (await browser("get", "cdp-url")).cdpUrl;
  const origin = await evaluate("location.href");
  const tap = async (selector, horizontal) => {
    const point = await center(selector, horizontal);
    assert.ok(point.hit && point.x > 0 && point.x < 320 && point.y >= 44 && point.y < 674, JSON.stringify({ selector, point }));
    execFileSync(process.execPath, ["scripts/browser_touch.mjs", endpoint, origin, "tap", String(point.x), String(point.y), String(point.x), String(point.y)], { cwd: root, env: { ...process.env, TMPDIR: `${root}.local/tmp` }, timeout: 15000 });
  };
  const language = '[data-setting-next="language"]';
  await bring(language);
  await tap(language);
  const languageValue = await evaluate("(Number(new URL(location.href).searchParams.get('language'))+1)%10");
  actions.push({ type: "settings", name: "language", value: languageValue });
  assert.deepEqual(await evaluate("window.settingReceipts"), actions);
  await bring(audio);
  await tap(audio, 0.98);
  actions.push({ type: "settings", name: "audio.music", value: 20 });
  assert.deepEqual(await evaluate("window.settingReceipts"), actions);
  const geometry = await evaluate("(()=>{const list=document.querySelector('.pokepath-settings-list');return [list.clientWidth,list.scrollWidth]})()");
  assert.equal(geometry[0], geometry[1]);
  await tap(".pokepath-classic-settings .pokepath-central-close");
  assert.equal(await evaluate("Boolean(document.querySelector('.pokepath-classic-settings'))"), false);
  return { actions, readOnly: true, mobile: { width: 320, geometry, closed: true } };
};
await verifyDOMOracle({ name: "settings-render", command: "node games/pokepath/tests/settings-render-oracle.mjs", sources: sources.map(source => source.identity), styles: [".local/pokepath/rejected-embedding/assets/src/css/menu.css"], reference, native: 'import "./games/pokepath/tests/SettingsFixture.tsx";', input, cases: [{ language: "8", theme: "original", font: "PressStart2P" }, { language: "2", theme: "monochrome", font: "Lexend" }], steps: [{ name: "first-screen" }, { name: "audio", action: scroll(".menu-scene-section:nth-child(3)", '[data-settings-section="audio"]') }, { name: "shortcut", action: scroll(".menu-scene-section:nth-child(4)", '[data-settings-section="shortcuts"]') }, { name: "credits", action: scroll(".menu-scene-section:nth-child(6)", '[data-settings-section="credits"]') }], measure: { reference: probe({ section: ".menu-scene-section", title: ".menu-scene-section-title", row: ".menu-scene-row", output: ".menu-scene-value", option: ".menu-scene-option-row", choice: ".menu-scene-option-button", selected: ".selected" }), native: probe({ section: ".pokepath-settings-card", title: "h2", row: ".pokepath-settings-row", output: "output", option: ".pokepath-settings-row:has(.pokepath-setting-buttons)", choice: "button", selected: '[aria-pressed="true"]' }) }, verifyNative });
