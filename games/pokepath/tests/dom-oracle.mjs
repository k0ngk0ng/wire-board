import assert from "node:assert/strict";
import { createServer } from "node:http";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { createHash, randomUUID } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { join } from "node:path";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const execute = promisify(execFile);
export const domSHA = bytes => createHash("sha256").update(bytes).digest("hex");

export async function referenceDOMClass(path, name, methods) {
  const ts = require("typescript");
  const source = await readFile(join(root, ".local/pokepath/research/source/src/js", path), "utf8");
  const parsed = ts.createSourceFile(path, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  const declaration = parsed.statements.find(node => ts.isClassDeclaration(node) && node.name?.text === name);
  assert.ok(declaration, name);
  const members = declaration.members.filter(node => methods ? methods.includes(ts.isConstructorDeclaration(node) ? "constructor" : node.name?.getText(parsed)) : !ts.isConstructorDeclaration(node)).map(node => node.getText(parsed));
  const code = ts.transpileModule(`class ${name}{${members.join("\n")}}`, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022, removeComments: true } }).outputText;
  return { code, identity: { path, sha256: domSHA(source), version: "1.7.3" } };
}

export async function verifyDOMOracle({ name, command, scope, sources, styles, reference, native, input, cases, steps, measure, verifyNative, fontSelectors, layout, referenceControl = false, platformStyles = false }) {
  const output = join(root, ".local/pokepath/verification", referenceControl ? `${name}-reference-control` : name);
  const identity = randomUUID().slice(0, 8);
  const temporary = join(output, `fixture-${identity}`);
  const profile = join(output, `profile-${identity}`);
  const assetRoot = join(root, "games/pokepath/assets/public/pokepath/v1");
  const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter" };
  const catalog = JSON.parse(await readFile(join(assetRoot, "catalog.json"), "utf8"));
  const dictionary = JSON.parse(await readFile(join(assetRoot, "ui-text.json"), "utf8"));
  const report = { command, passed: false, scope: scope ?? "Fixed research DOM and original UI input handlers versus independent React components; no upstream game runtime; complete unmasked 720x624 panel", muted: true, sources: [...sources], input, cases: [] };
  const screens = new Map();
  report.referenceControl = referenceControl;
  if (referenceControl) report.scope = "Positive control: two fresh executions of the same original research DOM, styles and trusted UI inputs; no product fidelity claim";
  const referenceStyles = await Promise.all([".local/pokepath/research/styles.css", ".local/pokepath/research/scenes.css", ...styles].map(async path => {
    const bytes = await readFile(join(root, path));
    report.sources.push({ path, sha256: domSHA(bytes) });
    return bytes.toString().replaceAll('url("../assets/', 'url("/pokepath/v1/src/assets/');
  }));
  const nativeStyles = await Promise.all(["style.css", "classic.css"].map(path => readFile(join(root, "games/pokepath/web", path), "utf8")));
  if (platformStyles) nativeStyles.unshift(await readFile(join(root, "web/src/style.css"), "utf8"));
  await mkdir(temporary, { recursive: true });
  await require("esbuild").build({ stdin: { contents: native, resolveDir: root, sourcefile: "oracle-native.tsx", loader: "tsx" }, outfile: join(temporary, "native.mjs"), bundle: true, platform: "browser", format: "esm", jsx: "automatic", nodePaths: [join(root, "web/node_modules")] });
  const page = (kind, parameters) => {
    if (referenceControl) kind = "reference";
    const theme = catalog.themes[parameters.theme];
    const variables = Object.entries(theme).flatMap(([key, value]) => [`--${key}:${value}`, `--pp-${key}:${value}`]).join(";");
    const data = { catalog, dictionary, input, parameters };
    const setup = kind === "reference" ? reference : "await import('/native.mjs');";
    return `<!doctype html><html><head><base href="/pokepath/v1/"><meta charset="utf-8"><style>${kind === "reference" ? referenceStyles.join("\n") : nativeStyles.join("\n")}html,body{margin:0;width:100%;height:100%}html{${variables};--pp-font:${parameters.font}}body{font-family:'${parameters.font}',sans-serif;background:var(--sceneBackground)}.pokepath-board{height:674px;min-height:0}</style></head><body data-font="${parameters.font}" data-theme="${parameters.theme}">${kind === "reference" ? '<div class="section-scene-container" style="width:720px;height:624px"></div>' : '<div id="root"></div>'}<script type="module">window.oracleData=${JSON.stringify(data)};${setup}await document.fonts.ready;document.documentElement.dataset.ready='true';</script></body></html>`;
  };
  let server;
  let session;
  const browser = async (...args) => {
    let response;
    try { response = await execute("agent-browser", ["--session", session, ...args, "--json"], { cwd: root, env: environment, timeout: 45000, maxBuffer: 32 << 20 }); }
    catch (error) { throw Error(error.stdout || error.stderr || error.message); }
    const value = JSON.parse(response.stdout);
    assert.equal(value.success, true, JSON.stringify(value));
    return value.data;
  };
  const evaluate = async code => (await browser("eval", "-b", Buffer.from(code).toString("base64"))).result;
  const ready = async () => {
    await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))");
    await browser("wait", "--fn", "document.documentElement.dataset.ready==='true'&&document.fonts.status==='loaded'&&[...document.images].every(image=>image.complete&&image.naturalWidth>0)");
    await evaluate("(async()=>{const urls=[...document.querySelectorAll('*')].flatMap(element=>[...getComputedStyle(element).backgroundImage.matchAll(/url\\(\"([^\"]+)\"\\)/g)].map(match=>match[1]));await Promise.all(urls.map(src=>new Promise((resolve,reject)=>{const image=new Image();image.onload=resolve;image.onerror=reject;image.src=src})));return true})()");
    await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))");
  };
  try {
    server = createServer(async (request, response) => {
      try {
        const url = new URL(request.url, "http://127.0.0.1");
        if (["/reference", "/native"].includes(url.pathname)) {
          const body = page(url.pathname.slice(1), Object.fromEntries(url.searchParams));
          response.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
          response.end(body);
        } else if (url.pathname === "/native.mjs") {
          const bytes = await readFile(join(temporary, "native.mjs"));
          response.writeHead(200, { "Content-Type": "text/javascript" });
          response.end(bytes);
        } else if (screens.has(url.pathname)) {
          const bytes = await readFile(screens.get(url.pathname));
          response.writeHead(200, { "Content-Type": "image/png", "Cache-Control": "no-store" });
          response.end(bytes);
        } else if (url.pathname.startsWith("/pokepath/v1/")) {
          const path = url.pathname.slice(13);
          assert.ok(!path.includes(".."));
          const bytes = await readFile(join(assetRoot, path));
          response.writeHead(200, { "Content-Type": path.endsWith(".ttf") ? "font/ttf" : path.endsWith(".json") ? "application/json" : "image/png" });
          response.end(bytes);
        } else { response.writeHead(404); response.end(); }
      } catch (error) { response.writeHead(500); response.end(String(error)); }
    });
    await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
    session = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", `do-${identity}`], { cwd: root, env: environment })).stdout.trim();
    await browser("open", "about:blank");
    await browser("set", "viewport", "1238", "674");
    for (const parameters of cases) {
      const matrix = steps.filter(step => !step.when || step.when(parameters));
      const measured = [];
      for (const kind of ["reference", "native"]) {
        const effectiveKind = referenceControl ? "reference" : kind;
        await browser("open", `http://127.0.0.1:${server.address().port}/${kind}?${new URLSearchParams(parameters)}`);
        await ready();
        for (let index = 0; index < matrix.length; index++) {
          const step = matrix[index];
          if (step.action) await step.action({ kind: effectiveKind, browser, evaluate, parameters });
          await ready();
          await browser("mouse", "move", "1220", "660");
          await evaluate("(async()=>{await new Promise(requestAnimationFrame);await Promise.all(document.getAnimations().filter(animation=>animation.constructor.name==='CSSTransition').map(animation=>animation.finished.catch(()=>{})));await new Promise(requestAnimationFrame);return true})()");
          const animations = await evaluate(`(()=>{const animations=document.getAnimations().filter(animation=>animation.effect.getTiming().iterations===Infinity);for(const animation of animations){animation.pause();animation.currentTime=${step.animationMS ?? 0}}return animations.map(animation=>({duration:animation.effect.getTiming().duration,iterations:'Infinity',frames:animation.effect.getKeyframes().map(frame=>({offset:frame.offset,backgroundPositionX:frame.backgroundPositionX,backgroundPositionY:frame.backgroundPositionY})),time:animation.currentTime}))})()`);
          await evaluate("new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))");
          const label = `${Object.values(parameters).join("-")}-${index}-${step.name}-${kind}`;
          const path = join(output, `${label}.png`);
          await browser("screenshot", path);
          screens.set(`/screens/${label}.png`, path);
          const value = { parameters, step: step.name, screenshot: path, sha256: domSHA(await readFile(path)), animations, value: await evaluate(measure[effectiveKind]) };
          if (layout && index === 0) value.layout = await evaluate(layout[effectiveKind]);
          if (step.inspectFonts && fontSelectors) {
            const endpoint = (await browser("get", "cdp-url")).cdpUrl;
            const pageURL = await evaluate("location.href");
            value.fonts = JSON.parse((await execute(process.execPath, [join(root, "games/pokepath/tests/dom-fonts.mjs"), endpoint, pageURL, fontSelectors[effectiveKind]], { cwd: root, env: environment, timeout: 15000 })).stdout);
          }
          measured[index] ??= {};
          measured[index][kind] = value;
        }
      }
      for (const measurement of measured) {
        const sources = ["reference", "native"].map(kind => [...screens].find(([, path]) => path === measurement[kind].screenshot)[0]);
        const pixels = await evaluate(`(async()=>{const images=await Promise.all(${JSON.stringify(sources)}.map(src=>new Promise(resolve=>{const image=new Image();image.onload=()=>resolve(image);image.src=src})));const buffers=images.map(image=>{const canvas=document.createElement('canvas');canvas.width=720;canvas.height=624;const context=canvas.getContext('2d');context.drawImage(image,259,25,720,624,0,0,720,624);return context.getImageData(0,0,720,624).data});let differences=0,error=0,maxError=0;const rows=Array(624).fill(0);for(let index=0;index<buffers[0].length;index++)if(buffers[0][index]!==buffers[1][index]){differences++;rows[Math.floor(index/4/720)]++;const delta=Math.abs(buffers[0][index]-buffers[1][index]);error+=delta;maxError=Math.max(maxError,delta)}const points={head:[],grid:[],filter:[]};for(let pixel=0;pixel<720*624;pixel++){const y=Math.floor(pixel/720),group=y<49?'head':y<386?'grid':y>=530&&y<576?'filter':undefined,index=pixel*4;if(group&&points[group].length<8&&[0,1,2].some(channel=>buffers[0][index+channel]!==buffers[1][index+channel]))points[group].push([pixel%720+259,y+25,[...buffers[0].slice(index,index+4)],[...buffers[1].slice(index,index+4)]])}return{differences,maxAbsoluteChannelError:maxError,meanAbsoluteChannelError:error/(720*624*4),rows,points}})()`);
        report.cases.push({ ...measurement, ...pixels });
      }
    }
    if (verifyNative && !referenceControl) report.nativeOperations = await verifyNative({ browser, evaluate, ready });
    for (const value of report.cases) {
      assert.deepEqual(value.native.value, value.reference.value, `${JSON.stringify(value.native.parameters)} ${value.native.step}: DOM and input receipts`);
      assert.deepEqual(value.native.animations, value.reference.animations, `${JSON.stringify(value.native.parameters)} ${value.native.step}: fixed animation phase and keyframes`);
    }
    report.behaviorPassed = true;
    report.exactPixelCases = report.cases.filter(value => value.differences === 0).length;
    for (const value of report.cases) {
      assert.equal(value.differences, 0, `${JSON.stringify(value.native.parameters)} ${value.native.step}: ${value.differences} RGBA channels differ`);
    }
    report.passed = true;
  } catch (error) {
    report.error = error.stack ?? String(error);
    if (session) { try { report.browserErrors = await browser("errors"); } catch {} }
    process.exitCode = 1;
  }
  finally {
    if (session) { try { await browser("close"); } catch {} }
    if (server) await new Promise(resolve => server.close(resolve));
    await rm(profile, { recursive: true, force: true });
    await rm(temporary, { recursive: true, force: true });
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
  process.stdout.write(JSON.stringify({ passed: report.passed, cases: report.cases.length, output, error: report.error }) + "\n");
  return report;
}
