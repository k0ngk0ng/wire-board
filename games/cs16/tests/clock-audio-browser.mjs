import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const name = process.env.CS16_CLOCK_REPORT ?? 'current';
assert.match(name, /^[a-z0-9-]+$/);
const out = path.join(root, '.local/cs16/verification/clock-audio', name), temporary = path.join(out, 'temporary');
const environment = { ...process.env, TMPDIR: path.join(temporary, 'tmp'), XDG_CACHE_HOME: path.join(temporary, 'cache'), XDG_CONFIG_HOME: path.join(temporary, 'config'), XDG_DATA_HOME: path.join(temporary, 'data'), AGENT_BROWSER_PROFILE: path.join(temporary, 'profile'), AGENT_BROWSER_SOCKET_DIR: path.join(root, '.local/cs16/s'), AGENT_BROWSER_NO_VERSION_CHECK: '1', AGENT_BROWSER_ARGS: '--mute-audio,--disable-breakpad,--disable-crash-reporter' };
for (const directory of [out, temporary, environment.TMPDIR, environment.XDG_CACHE_HOME, environment.XDG_CONFIG_HOME, environment.XDG_DATA_HOME, environment.AGENT_BROWSER_PROFILE, environment.AGENT_BROWSER_SOCKET_DIR]) await mkdir(directory, { recursive: true });
const require = createRequire(new URL('../../../web/package.json', import.meta.url)), execute = promisify(execFile);
const files = ['games/cs16/web/Board.tsx', 'games/cs16/web/presentation.ts', 'games/cs16/web/classic-hud.ts', 'games/cs16/web/feedback.ts', 'games/cs16/web/clock.css', 'games/cs16/tests/clock-audio-fixture.tsx', 'games/cs16/tests/clock-audio-browser.mjs', 'web/src/audio.ts'];
const hashes = async () => Object.fromEntries(await Promise.all(files.map(async file => { try { return [file, createHash('sha256').update(await readFile(path.join(root, file))).digest('hex')]; } catch { return [file, null]; } })));
const report = { passed: false, scope: 'Production CS Board/GameFullscreen/clock logic and real platform native media; controlled projections with renderer/motion loading replaced; Chromium viewports, not original-game runtime, complete network matches, GPU/performance or physical-device certification', sources: await hashes(), cases: [], audioResources: [], requests: [] };
let server, session, socket, attached, serial = 0;
const pending = new Map();
const browser = async (...args) => { const result = await execute('agent-browser', ['--session', session, ...args, '--json'], { cwd: root, env: environment, timeout: 35000, maxBuffer: 4 << 20 }); const body = JSON.parse(result.stdout); assert.ok(body.success, result.stdout); return body.data; };
const call = (method, params = {}, selected = attached) => new Promise((resolve, reject) => { const id = ++serial; const timer = setTimeout(() => { pending.delete(id); reject(Error(method + ' timeout')); }, 10000); pending.set(id, { resolve, reject, timer }); socket.send(JSON.stringify({ id, method, params, ...(selected ? { sessionId: selected } : {}) })); });
const evaluate = async code => { const value = await call('Runtime.evaluate', { expression: code, returnByValue: true, awaitPromise: true }); assert.equal(value.exceptionDetails, undefined, JSON.stringify(value.exceptionDetails)); return value.result.value; };
const pause = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const until = async predicate => { for (let attempt = 0; attempt < 150; attempt++) { const value = await predicate(); if (value) return value; await pause(20); } throw Error('clock/audio condition timed out'); };
const effectCount = () => evaluate('window.cs16ClockAudit.calls.filter(call=>call.method==="effect"&&call.args[0]==="cs16:c4-beep").length');
try {
  const manifest = JSON.parse(await readFile(path.join(root, 'games/cs16/assets/public/cs16/manifest.json'), 'utf8'));
  const sources = [];
  for (let stage = 1; stage <= 5; stage++) {
    const key = `weapons/c4_beep${stage}.wav`, resource = manifest.sounds[key];
    const bytes = await readFile(path.join(root, 'games/cs16/assets/public/cs16', resource.file));
    assert.equal(bytes.length, resource.bytes); assert.equal(createHash('sha256').update(bytes).digest('hex'), resource.sha256);
    sources.push([key, '/cs16/' + resource.file]); report.audioResources.push({ key, ...resource });
  }
  const hud = manifest.hud;
  await require('esbuild').build({ entryPoints: [path.join(root, 'games/cs16/tests/clock-audio-fixture.tsx')], outfile: path.join(temporary, 'fixture.js'), bundle: true, jsx: 'automatic', nodePaths: [path.join(root, 'web/node_modules')], define: { 'process.env.NODE_ENV': '"production"' }, legalComments: 'none', plugins: [{ name: 'clock-fixture-no-gpu-or-motion', setup(build) { build.onLoad({ filter: /games\/cs16\/web\/(?:assets|renderer)\.ts$/ }, args => ({ contents: args.path.endsWith('/assets.ts') ? `export async function loadBattleAssets(){return {audit:{},sounds:new Map(${JSON.stringify(sources)}),weapons:[],manifest:{hud:${JSON.stringify(hud)},maps:{de_dust2:{rules:{sha256:'a'.repeat(64)}}}},motion:player=>player,canSmooth:()=>false,dynamics(){}}}` : 'export class BattleRenderer{visualStats={};prepare(){return Promise.resolve()}update(){}draw(view,player){return player}feedback(){}event(){}dispose(){}}', loader: 'ts' })); } }] });
  const controls = JSON.parse(await readFile(path.join(root, 'games/cs16/controls.json'), 'utf8'));
  const profile = { revision: 1, turboHz: 10, keyboard: Object.fromEntries(controls.actions.map(action => [action.id, action.keys?.[0] ?? ''])) }, model = { id: 'game:cs16', name: '反恐精英', buttons: controls.actions.map(action => ({ id: action.id, label: action.label })), defaults: profile };
  const routes = new Map([
    ['/', [Buffer.from('<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><style>body{margin:0;background:#52665f}</style></head><body><div id="root"></div><script src="/fixture.js"></script></body></html>'), 'text/html']],
    ['/fixture.js', [await readFile(path.join(temporary, 'fixture.js')), 'text/javascript']], ['/fixture.css', [await readFile(path.join(temporary, 'fixture.css')), 'text/css']],
    ['/api/controllers', [Buffer.from(JSON.stringify({ models: [model], profiles: { 'game:cs16': profile } })), 'application/json']], ['/idle', [Buffer.from('<!doctype html><title>Background test</title>'), 'text/html']],
  ]);
  for (const resource of [hud.atlas, ...report.audioResources]) routes.set('/cs16/' + resource.file, [await readFile(path.join(root, 'games/cs16/assets/public/cs16', resource.file)), resource.mime]);
  server = createServer((request, response) => { const pathname = new URL(request.url, 'http://localhost').pathname, route = routes.get(pathname); if (pathname.startsWith('/cs16/')) report.requests.push({ path: pathname, method: request.method, range: request.headers.range }); if (!route) { response.writeHead(404); response.end(); return; } const [bytes, mime] = route; const range = /^bytes=(\d+)-(\d*)$/.exec(request.headers.range ?? ''); if (range) { const start = Number(range[1]), end = range[2] ? Math.min(bytes.length - 1, Number(range[2])) : bytes.length - 1; response.writeHead(206, { 'Content-Type': mime, 'Accept-Ranges': 'bytes', 'Content-Range': `bytes ${start}-${end}/${bytes.length}`, 'Content-Length': end - start + 1 }); response.end(bytes.subarray(start, end + 1)); } else { response.writeHead(200, { 'Content-Type': mime, 'Content-Length': bytes.length, 'Accept-Ranges': 'bytes' }); response.end(request.method === 'HEAD' ? undefined : bytes); } });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = 'http://127.0.0.1:' + server.address().port;
  session = (await execute('agent-browser', ['session', 'id', '--scope', 'worktree', '--prefix', 'ct' + process.pid], { cwd: root, env: environment })).stdout.trim();
  await browser('open', origin); await browser('wait', '--fn', 'document.querySelector("[data-cs16-board]")?.dataset.ready==="true"'); await browser('click', '#unlock');
  const endpoint = await browser('get', 'cdp-url'); socket = new WebSocket(typeof endpoint === 'string' ? endpoint : endpoint.cdpUrl);
  socket.onmessage = ({ data }) => { const body = JSON.parse(data), item = pending.get(body.id); if (!item) return; clearTimeout(item.timer); pending.delete(body.id); body.error ? item.reject(Error(JSON.stringify(body.error))) : item.resolve(body.result); };
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  const target = (await call('Target.getTargets', {}, undefined)).targetInfos.find(target => target.type === 'page' && target.url.startsWith(origin)); attached = (await call('Target.attachToTarget', { targetId: target.targetId, flatten: true }, undefined)).sessionId;
  const measure = () => evaluate('(()=>{const clock=document.querySelector(".cs16-match-hud"),board=document.querySelector("[data-cs16-board]"),box=element=>element.getBoundingClientRect().toJSON();return {coarse:matchMedia("(pointer:coarse)").matches,clock:clock?box(clock):undefined,board:box(board),text:clock?.textContent,obstacles:[...document.querySelectorAll(".cs16-vitals,.cs16-money,.cs16-ammo,.game-fullscreen-toolbar")].filter(element=>element.getClientRects().length&&getComputedStyle(element).display!=="none").map(element=>({name:element.className,rect:box(element)})),overflow:document.documentElement.scrollWidth>innerWidth}})()');
  const screenshot = async name => { const image = await call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await writeFile(path.join(out, name + '.png'), Buffer.from(image.data, 'base64')); };
  await call('Emulation.setDeviceMetricsOverride', { width: 1280, height: 900, deviceScaleFactor: 1, mobile: false });
  report.desktop = await measure(); assert.ok(report.desktop.clock); await evaluate('window.cs16ClockAudit.planted(true);true'); await pause(50); assert.equal((await measure()).clock, undefined, 'PC exposes a C4 countdown'); await screenshot('desktop-planted-no-countdown');
  await evaluate('window.cs16ClockAudit.planted(false);true'); await pause(30);
  for (const mode of ['normal', 'native', 'page']) {
    if (mode === 'page') await evaluate('Element.prototype.requestFullscreen=()=>Promise.reject(Error("controlled page fullscreen"));true');
    if (mode !== 'normal') { await browser('click', '#fullscreen'); await until(() => evaluate('document.querySelector("[data-room-fullscreen]")?.dataset.roomFullscreen')); }
    for (const [width, height] of [[320, 568], [890, 626], [626, 890], [820, 1180], [1180, 820]]) {
      await call('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 5 }); await call('Emulation.setDeviceMetricsOverride', { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: true });
      if (mode === 'normal') await evaluate('document.querySelector("[data-cs16-board]").scrollIntoView({block:"center"});true'); await pause(50);
      const layout = await measure(); report.cases.push({ mode, width, height, ...layout });
      assert.ok(layout.coarse && layout.clock && !layout.overflow); assert.ok(Math.round((layout.clock.top - layout.board.top) * 1000) >= 0 && Math.round((layout.clock.top - layout.board.top) * 1000) <= 16000, 'touch round clock is not at the top safe edge');
      assert.ok(layout.obstacles.every(({rect}) => Math.round(layout.clock.left*1000)>=Math.round(rect.right*1000)||Math.round(layout.clock.right*1000)<=Math.round(rect.left*1000)||Math.round(layout.clock.top*1000)>=Math.round(rect.bottom*1000)||Math.round(layout.clock.bottom*1000)<=Math.round(rect.top*1000)), 'touch round clock overlaps another HUD or fullscreen toolbar');
      assert.ok(layout.clock.left >= layout.board.left && layout.clock.right <= layout.board.right); await screenshot(mode + '-' + width + 'x' + height);
      await evaluate('window.cs16ClockAudit.planted(true);true'); await pause(30); assert.equal((await measure()).clock, undefined, 'touch exposes a C4 countdown'); await evaluate('window.cs16ClockAudit.planted(false);true'); await pause(30);
    }
    if (mode !== 'normal') { await browser('click', '[aria-label="退出全屏"]'); await until(() => evaluate('!document.querySelector("[data-room-fullscreen]")')); }
  }
  await evaluate('window.cs16ClockAudit.progress("plant");true'); await until(() => evaluate('!!document.querySelector("[role=progressbar]")')); report.plantProgress = await evaluate('document.querySelector("[role=progressbar]").getAttribute("aria-valuenow")');
  await evaluate('window.cs16ClockAudit.progress("defuse");true'); await until(() => evaluate('!!document.querySelector("[role=progressbar]")')); report.defuseProgress = await evaluate('document.querySelector("[role=progressbar]").getAttribute("aria-valuenow")'); assert.equal((await measure()).clock, undefined);
  await evaluate('window.cs16ClockAudit.progress("");window.cs16ClockAudit.planted(true);true'); await pause(50);
  await evaluate('window.cs16ClockAudit.cue(9);true'); await until(async () => await effectCount() === 1); await until(() => evaluate('window.cs16ClockAudit.media.some(item=>item.type==="playing"&&item.lane==="cs16:c4-beep")'));
  await evaluate('window.cs16ClockAudit.cue(9);true'); await pause(80); assert.equal(await effectCount(), 1);
  for (const [code, stage] of [[18, 2], [27, 3], [36, 4], [45, 5]]) { await evaluate(`window.cs16ClockAudit.cue(${code});true`); await until(() => evaluate(`window.cs16ClockAudit.media.some(item=>item.type==="playing"&&item.source.includes("c4_beep${stage}.flac"))`)); }
  assert.equal(await effectCount(), 5);
  await evaluate('window.cs16ClockAudit.muted(true);true'); await pause(30); await evaluate('window.cs16ClockAudit.cue(53);true'); await pause(50); assert.equal(await effectCount(), 5);
  await evaluate('window.cs16ClockAudit.muted(false);true'); await pause(30); await evaluate('window.cs16ClockAudit.cue(61);true'); await pause(80); assert.equal(await effectCount(), 5, 'unmute catches up a previous cue'); await evaluate('window.cs16ClockAudit.cue(69);true'); await until(async () => await effectCount() === 6);
  await evaluate('window.cs16ClockAudit.cue(101,true);true'); await pause(30); await evaluate('window.cs16ClockAudit.cue(109);true'); await pause(60); assert.equal(await effectCount(), 6); await evaluate('window.cs16ClockAudit.cue(117);true'); await until(async () => await effectCount() === 7);
  const background = await call('Target.createTarget', { url: origin + '/idle' }, undefined); await call('Target.activateTarget', { targetId: background.targetId }, undefined); await until(() => evaluate('document.hidden')); await evaluate('window.cs16ClockAudit.cue(125);true'); await pause(50); assert.equal(await effectCount(), 7);
  await call('Target.activateTarget', { targetId: target.targetId }, undefined); await until(() => evaluate('!document.hidden')); await evaluate('window.cs16ClockAudit.cue(133);true'); await pause(80); assert.equal(await effectCount(), 7, 'foreground return catches up a previous cue'); await evaluate('window.cs16ClockAudit.cue(141);true'); await until(async () => await effectCount() === 8); await call('Target.closeTarget', { targetId: background.targetId }, undefined);
  await evaluate('window.cs16ClockAudit.planted(false);true'); await pause(50); assert.equal(await evaluate('[...document.querySelectorAll("audio[data-platform-effect-lane]")].some(element=>element.dataset.platformEffectLane==="cs16:c4-beep"&&!element.paused)'), false);
  report.calls = await evaluate('window.cs16ClockAudit.calls'); report.media = await evaluate('window.cs16ClockAudit.media'); assert.ok(report.calls.filter(call=>call.method==='effect').every(call=>call.args[0]==='cs16:c4-beep')); assert.equal(report.calls.filter(call=>call.method==='spatial').length, 0); report.passed = true;
} catch (error) { report.error = error.stack ?? String(error); if(attached)try{report.calls=await evaluate('window.cs16ClockAudit.calls');report.media=await evaluate('window.cs16ClockAudit.media');}catch{} }
finally { socket?.close(); for (const item of pending.values()) clearTimeout(item.timer); if (session) await browser('close').catch(() => {}); if (server) await new Promise(resolve => server.close(resolve)); const after = await hashes(); report.changedSources = files.filter(file=>report.sources[file]!==after[file]); if (report.changedSources.length) report.passed=false; await writeFile(path.join(out,'result.json'),JSON.stringify(report,null,2)+'\n'); await rm(temporary,{recursive:true,force:true}); process.stdout.write(JSON.stringify({passed:report.passed,error:report.error,output:out})+'\n'); }
process.exitCode=report.passed?0:1;
