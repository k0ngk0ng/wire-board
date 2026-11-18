import assert from "node:assert/strict";
import { createServer } from "node:http";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { join } from "node:path";
import { createHash, randomUUID } from "node:crypto";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const execute = promisify(execFile);
const ts = require("typescript");
const React = require("react");
const { renderToStaticMarkup } = require("react-dom/server");
const output = join(root, ".local/pokepath/verification/profile-render");
const identity = randomUUID().slice(0, 8);
const temporary = join(output, `fixture-${identity}`);
const profile = join(output, `profile-${identity}`);
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter", AGENT_BROWSER_NO_VERSION_CHECK: "1" };
const assetRoot = join(root, "games/pokepath/assets/public/pokepath/v1");
const catalog = JSON.parse(await readFile(join(assetRoot, "catalog.json"), "utf8"));
const dictionary = JSON.parse(await readFile(join(assetRoot, "ui-text.json"), "utf8"));
const stats = { timePlayed: 66, stars: 1234, maxStars: 4000, pokemonOwned: 1, shinyAmount: 0, highestPokemonLevel: 1, totalPokemonLevel: 1, totalGold: 1234567, itemAmount: 0, wavesCompleted: 1234, highestHit: 1234.5678, defeatedEnemies: 987654, defeatedSpecies: 347, appliedStuns: 1234, appliedSlows: 5678, appliedBurns: 9012, appliedPoisons: 3456, appliedCurses: 7890, resets: 1234, stolenGold: 1234567, maxGoldPerWave: { value: 1234567, routeTag: "1-1" }, maxGoldPerTime: { value: 0, routeTag: "" }, pokemonTotal: 257, itemTotal: 178, enemyTotal: 347 };
const report = { command: "node games/pokepath/tests/profile-render-oracle.mjs", scope: "Fixed render-only research DOM versus independent React profile; no upstream Main or rule runtime; full unmasked 720x624 panel pixels", muted: true, passed: false, sources: [], input: stats, cases: [] };
await mkdir(temporary, { recursive: true });
const sources = ["game/scenes/ProfileScene.js", "utils/Element.js", "utils/Input.js", "utils/Utility.js"];
const extracted = [];
for (const path of sources) {
  const source = await readFile(join(root, ".local/pokepath/research/source/src/js", path), "utf8");
  report.sources.push({ path, sha256: createHash("sha256").update(source).digest("hex"), version: "1.7.3" });
  const parsed = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const name = path.split("/").at(-1).replace(".js", "");
  const declaration = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === name);
  assert.ok(declaration);
  const methods = name === "ProfileScene" ? ["render", "update", "updatePortraitOptions"] : name === "Utility" ? ["numberDot", "minutsToTime"] : undefined;
  const members = declaration.members.filter(node => !methods || methods.includes(node.name?.getText(parsed))).map(node => node.getText(parsed));
  extracted.push(`class ${name}{${members.join("\n")}}`);
}
const referenceModule = ts.transpileModule(extracted.join("\n"), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
const referenceStylePaths = [".local/pokepath/research/styles.css", ".local/pokepath/research/scenes.css", ".local/pokepath/rejected-embedding/assets/src/css/profile.css"];
const referenceStyles = await Promise.all(referenceStylePaths.map(async path => {
  const bytes = await readFile(join(root, path));
  report.sources.push({ path, sha256: createHash("sha256").update(bytes).digest("hex"), version: "1.7.3" });
  return bytes.toString();
}));
const nativeStyles = await Promise.all(["style.css", "classic.css"].map(path => readFile(join(root, "games/pokepath/web", path), "utf8")));
report.nativeSources = await Promise.all(["ProfilePanel.tsx", "I18n.tsx", "assets.ts", "number-format.ts", "style.css", "classic.css"].map(async name => {
  const path = `games/pokepath/web/${name}`;
  const bytes = await readFile(join(root, path));
  return { path, bytes: bytes.length, sha256: createHash("sha256").update(bytes).digest("hex") };
}));
const fonts = ["NGC", "PressStart2P", "Lexend", "Unown"];
await require("esbuild").build({ stdin: { contents: 'export {ProfilePanel} from "./games/pokepath/web/ProfilePanel"; export {I18nProvider} from "./games/pokepath/web/I18n";', resolveDir: root }, outfile: join(temporary, "native.mjs"), bundle: true, platform: "node", format: "esm", jsx: "automatic", nodePaths: [join(root, "web/node_modules")], plugins: [{ name: "same-react", setup(plugin) { plugin.onResolve({ filter: /^react(?:-dom)?(?:\/jsx-runtime)?$/ }, args => ({ path: require.resolve(args.path), external: true })); } }] });
await require("esbuild").build({ stdin: { contents: 'import {AssetStore} from "./games/pokepath/web/assets";const store=new AssetStore(new URL("/pokepath/v1/",location.href),()=>{});await store.initialize();await store.font(new URL(location.href).searchParams.get("font"));', resolveDir: root }, outfile: join(temporary, "font.mjs"), bundle: true, platform: "browser", format: "esm" });
const { ProfilePanel, I18nProvider } = await import(pathToFileURL(join(temporary, "native.mjs")).href);
const store = { images: new Map(catalog.achievements.map(item => [item.image, true]).concat([["src/assets/images/portraits/0.png", true]])), path: path => path, url: path => `/pokepath/v1/${path}` };
const page = (kind, language, theme, font) => {
  const variables = Object.entries(catalog.themes[theme]);
  const style = Object.fromEntries(variables.map(([key, value]) => [`--pp-${key}`, value]));
  style["--pp-font"] = font;
  const view = { stars: stats.stars, progression: { portrait: 0, profileName: "Player", profileStats: stats, achievements: {}, settings: { language, theme, font } } };
  const content = kind === "native" ? renderToStaticMarkup(React.createElement("section", { className: "pokepath-board", style }, React.createElement("div", { className: "pokepath-classic-scene" }, React.createElement(I18nProvider, { language, dictionary, catalog }, React.createElement(ProfilePanel, { view, assets: { catalog }, store, disabled: false, onAct() {}, onClose() {} }))))) : '<div class="section-scene-container" style="width:720px;height:624px"></div>';
  const fixture = `const text=${JSON.stringify(dictionary.text)},allPokemon=Array(${catalog.collection.length}),itemCollectionTotal=178,achievementReworkData=${JSON.stringify(catalog.achievements)};const playSound=()=>{};${referenceModule}\nconst scene=Object.create(ProfileScene.prototype);scene.container=document.querySelector('.section-scene-container');scene.main={lang:${language},utility:new Utility(),checkShinyCount(){},tooltip:{bindTo(){}},player:{name:'Player',portrait:0,stars:${stats.stars},shinyAmount:0,itemAmount:0,enemyDefeatedAmount:347,achievementProgress:{stolenGold:${stats.stolenGold}},achievementsRework:Array.from({length:39},()=>({status:false})),stats:{...${JSON.stringify(stats)},maxGoldPerWave:[1234567,'1-1'],maxGoldPerTime:[0,null]}}};scene.render();scene.update();`;
  return `<!doctype html><html><head><base href="/pokepath/v1/"><meta charset="utf-8"><style>${kind === "native" ? nativeStyles.join("\n") : referenceStyles.join("\n")}html,body{margin:0;width:100%;height:100%}body{font-family:'${font}',sans-serif;${variables.map(([key, value]) => `--${key}:${value}`).join(";")}}.pokepath-board{height:674px;min-height:0}</style></head><body data-font="${font}" data-theme="${theme}">${content}<script type="module">${kind === "reference" ? fixture : "await import('/font.mjs');"}await document.fonts.ready;await Promise.all([...document.images].map(image=>image.decode()));const images=[...document.querySelectorAll('*')].map(element=>getComputedStyle(element).backgroundImage).filter(value=>value.startsWith('url(')).map(value=>value.slice(5,-2));await Promise.all(images.map(src=>new Promise((resolve,reject)=>{const image=new Image();image.onload=resolve;image.onerror=reject;image.src=src})));document.documentElement.dataset.ready='true';</script></body></html>`;
};
let session;
let server;
const browser = async (...args) => {
  const response = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 32 << 20 });
  const value = JSON.parse(response.stdout);
  assert.equal(value.success, true, JSON.stringify(value));
  return value.data;
};
try {
  server = createServer(async (request, response) => {
    try {
      const url = new URL(request.url, "http://127.0.0.1");
      if (url.pathname === "/font.mjs") {
        response.writeHead(200, { "Content-Type": "text/javascript" });
        response.end(await readFile(join(temporary, "font.mjs")));
      } else if (url.pathname.startsWith("/pokepath/v1/")) {
        const path = url.pathname.slice("/pokepath/v1/".length);
        assert.ok(!path.includes(".."));
        const bytes = await readFile(join(assetRoot, path));
        response.writeHead(200, { "Content-Type": path.endsWith(".ttf") ? "font/ttf" : "image/png" });
        response.end(bytes);
      } else if (["/reference", "/native"].includes(url.pathname)) {
        const content = page(url.pathname.slice(1), Number(url.searchParams.get("language")), url.searchParams.get("theme"), url.searchParams.get("font"));
        response.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
        response.end(content);
      } else { response.writeHead(404); response.end(); }
    } catch (error) { response.writeHead(500); response.end(String(error)); }
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", `ppprofile-${identity}`], { cwd: root, env: environment })).stdout.trim();
  const cases = Array.from({ length: 10 }, (_, language) => ({ language, theme: "original", font: language === 8 ? "NGC" : "PressStart2P" }));
  for (const theme of Object.keys(catalog.themes)) for (const font of fonts) if (theme !== "original" || font !== "NGC") cases.push({ language: 8, theme, font });
  cases.push({ language: 2, theme: "classic", font: "Lexend" });
  for (const parameters of cases) {
    const label = `${parameters.language}-${parameters.theme}-${parameters.font}`;
    const measurement = { ...parameters, reference: {}, native: {} };
    for (const kind of ["reference", "native"]) {
      await browser("open", `http://127.0.0.1:${server.address().port}/${kind}?${new URLSearchParams(parameters)}`);
      await browser("set", "viewport", "1238", "674");
      await browser("wait", "--fn", "document.documentElement.dataset.ready==='true'");
      await browser("mouse", "move", "1220", "660");
      const screenshot = join(output, `${label}-${kind}.png`);
      await browser("screenshot", screenshot);
      const selection = kind === "native" ? [".pokepath-profile-player input", ".pokepath-profile-badges", ".pokepath-profile-statistics", "[data-profile-stat]", "[data-profile-stat] dd"] : [".profile-scene-name", ".profile-scene-achievements-container", ".profile-stats-container", ".profile-stat", ".profile-stat-value"];
      measurement[kind] = (await browser("eval", `(()=>{const selectors=${JSON.stringify(selection)},rect=element=>{const r=element.getBoundingClientRect();return[r.x,r.y,r.width,r.height]};return{input:rect(document.querySelector(selectors[0])),badges:rect(document.querySelector(selectors[1])),statistics:rect(document.querySelector(selectors[2])),rows:[...document.querySelectorAll(selectors[3])].map(rect),values:[...document.querySelectorAll(selectors[4])].map(element=>element.textContent),labels:[...document.querySelectorAll(selectors[3])].map(element=>element.firstElementChild.textContent)}})()`)).result;
      measurement[kind].screenshot = screenshot;
      measurement[kind].sha256 = createHash("sha256").update(await readFile(screenshot)).digest("hex");
    }
    const sources = await Promise.all(["reference", "native"].map(async kind => "data:image/png;base64," + (await readFile(measurement[kind].screenshot)).toString("base64")));
    const pixels = (await browser("eval", "-b", Buffer.from(`(async()=>{const images=await Promise.all(${JSON.stringify(sources)}.map(src=>new Promise(resolve=>{const image=new Image();image.onload=()=>resolve(image);image.src=src})));const buffers=images.map(image=>{const canvas=document.createElement('canvas');canvas.width=720;canvas.height=624;const context=canvas.getContext('2d');context.drawImage(image,259,25,720,624,0,0,720,624);return context.getImageData(0,0,720,624).data});let differences=0,error=0;for(let index=0;index<buffers[0].length;index++)if(buffers[0][index]!==buffers[1][index]){differences++;error+=Math.abs(buffers[0][index]-buffers[1][index])}const regions={};for(const[name,[left,top,right,bottom]]of Object.entries({input:[53,89,213,121],portrait:[81,15,178,95],badges:[261,17,688,125],stats:[20,140,700,609]})){let changed=0;for(let y=top;y<bottom;y++)for(let x=left;x<right;x++)for(let channel=0;channel<4;channel++){const index=(y*720+x)*4+channel;if(buffers[0][index]!==buffers[1][index])changed++}regions[name]=changed}return{differences,regions,meanAbsoluteChannelError:error/(720*624*4)}})()`).toString("base64"))).result;
    Object.assign(measurement, pixels);
    report.cases.push(measurement);
  }
  for (const value of report.cases) {
    assert.deepEqual(value.native.values, value.reference.values, `language ${value.language}: profile values`);
    assert.deepEqual(value.native.labels, value.reference.labels, `language ${value.language}: profile labels`);
    for (const key of ["input", "badges", "statistics", "rows"]) assert.deepEqual(value.native[key], value.reference[key], `${value.theme}/${value.font}: ${key}`);
    assert.equal(value.differences, 0, `${value.theme}/${value.font}: ${value.differences} RGBA channels differ`);
  }
  report.passed = true;
} catch (error) {
  report.error = error.stack ?? String(error);
  process.exitCode = 1;
} finally {
  if (session) { try { await browser("close"); } catch {} }
  if (server) await new Promise(resolve => server.close(resolve));
  await rm(profile, { recursive: true, force: true });
  await rm(temporary, { recursive: true, force: true });
  await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
}
process.stdout.write(JSON.stringify({ passed: report.passed, cases: report.cases.length, error: report.error, output }) + "\n");
