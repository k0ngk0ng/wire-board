import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { randomUUID, createHash } from 'node:crypto';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const run = process.env.CS16_MOBILE_CONTROLS_REPORT ?? 'current';
assert.match(run, /^[a-z0-9-]+$/);
const out = path.join(root, '.local/cs16/verification/mobile-controls', run);
const temporary = path.join(root, '.local/cs16/tmp/mobile-controls', randomUUID());
const environment = { ...process.env, TMPDIR: path.join(temporary, 'tmp'), XDG_CACHE_HOME: path.join(temporary, 'cache'), AGENT_BROWSER_PROFILE: path.join(temporary, 'profile'), AGENT_BROWSER_SOCKET_DIR: path.join(root, '.local/cs16/s'), AGENT_BROWSER_NO_VERSION_CHECK: '1', AGENT_BROWSER_ARGS: '--mute-audio,--disable-breakpad,--disable-crash-reporter' };
for (const directory of [out, temporary, environment.TMPDIR, environment.XDG_CACHE_HOME, environment.AGENT_BROWSER_PROFILE, environment.AGENT_BROWSER_SOCKET_DIR]) await mkdir(directory, { recursive: true });
const require = createRequire(new URL('../../../web/package.json', import.meta.url));
const execute = promisify(execFile);
const report = { passed: false, platform: process.platform, scope: 'Production CS Board, public GameController and GameFullscreen with trusted Chromium keyboard/touch; GPU renderer and asset loading replaced; no original runtime, match-rule, performance or physical-device claim', checks: [], events: [], sources: {} };
const files = ['games/cs16/web/Board.tsx', 'games/cs16/web/style.css', 'games/cs16/web/TouchControls.tsx', 'games/cs16/web/touch-input.ts', 'games/cs16/web/touch-ownership.ts', 'games/cs16/web/mobile-controls.css', 'games/cs16/tests/mobile-controls-fixture.tsx', 'games/cs16/tests/mobile-controls-browser.mjs', 'web/src/ui/chat-position.ts','web/src/controllers/game-controller.ts', 'web/src/ui/room-fullscreen.tsx'];
for (const file of files) { try { report.sources[file] = createHash('sha256').update(await readFile(path.join(root, file))).digest('hex'); } catch {} }
let session, socket, server, attached, serial = 0;
const pending = new Map();
const browser = async (...args) => {
  const result = await execute('agent-browser', ['--session', session, ...args, '--json'], { cwd: root, env: environment, timeout: 35000, maxBuffer: 4 << 20 });
  const value = JSON.parse(result.stdout);
  assert.equal(value.success, true, result.stdout);
  return value.data;
};
const evaluate = async code => (await browser('eval', '-b', Buffer.from(code).toString('base64'))).result;
const wait = condition => browser('wait', '--fn', condition);
const call = (method, params = {}, sessionId = attached) => new Promise((resolve, reject) => {
  const id = ++serial;
  const timer = setTimeout(() => { pending.delete(id); reject(Error(method + ' timeout')); }, 10000);
  pending.set(id, { resolve, reject, timer });
  socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
});
const key = (type, code = 'Tab', extra = {}) => call('Input.dispatchKeyEvent', { type, code, key: code === 'Tab' ? 'Tab' : code.startsWith('Key') ? code.slice(3).toLowerCase() : 'Escape', windowsVirtualKeyCode: code === 'Tab' ? 9 : code.startsWith('Key') ? code.charCodeAt(3) : 27, ...extra });
const check = (name, value) => { report.checks.push({ name, passed: !!value }); };

try {
  await require('esbuild').build({ entryPoints: [path.join(root, 'games/cs16/tests/mobile-controls-fixture.tsx')], outfile: path.join(temporary, 'fixture.js'), bundle: true, jsx: 'automatic', nodePaths: [path.join(root, 'web/node_modules')], define: { 'process.env.NODE_ENV': '"production"' }, legalComments: 'none', plugins: [{ name: 'isolated-scoreboard-rendering', setup(build) { build.onLoad({ filter: /games\/cs16\/web\/(?:assets|renderer)\.ts$/ }, args => ({ contents: args.path.endsWith('/assets.ts') ? `export async function loadBattleAssets(){return {audit:{},sounds:new Map(),weapons:[],manifest:{maps:{cs_bloodstrike:{rules:{sha256:'a'.repeat(64)}}},hud:await fetch('/fixture-hud.json').then(response=>response.json())},motion:player=>player,canSmooth:()=>false,dynamics(){}}}` : 'export class BattleRenderer{visualStats={};prepare(){return Promise.resolve()}update(){}draw(view,player){return player}feedback(){}event(){}dispose(){}}', loader: 'ts' })); } }] });
  const controls = JSON.parse(await readFile(path.join(root, 'games/cs16/controls.json'), 'utf8'));
  const profile = { revision: 1, turboHz: 10, keyboard: Object.fromEntries(controls.actions.map(action => [action.id, action.keys?.[0] ?? ''])) };
  const model = { id: 'game:cs16', name: '反恐精英', buttons: controls.actions.map(action => ({ id: action.id, label: action.label })), defaults: profile };
  const routes = new Map([
    ['/', [Buffer.from('<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/fixture.css"><style>body{margin:0;background:#87978b}.table-chat{position:absolute;left:8px;bottom:8px;z-index:20;background:#fff;padding:4px}.table-chat input{width:90px}.chat-toggle{position:absolute;right:8px;bottom:8px;z-index:30;width:44px;height:44px}</style></head><body><div id="root"></div><script src="/fixture.js"></script></body></html>'), 'text/html']],
    ['/fixture.js', [await readFile(path.join(temporary, 'fixture.js')), 'text/javascript']],
    ['/fixture.css', [await readFile(path.join(temporary, 'fixture.css')), 'text/css']],
    ['/idle', [Buffer.from('<!doctype html><title>Other tab</title>'), 'text/html']],
    ['/api/controllers', [Buffer.from(JSON.stringify({ models: [model], profiles: { 'game:cs16': profile } })), 'application/json']],
  ]);
  const manifest=JSON.parse(await readFile(path.join(root,'games/cs16/assets/public/cs16/manifest.json'),'utf8'));
  const atlas=await readFile(path.join(root,'games/cs16/assets/public/cs16',manifest.hud.atlas.file));
  assert.equal(createHash('sha256').update(atlas).digest('hex'),manifest.hud.atlas.sha256);
  report.hudAtlas=manifest.hud.atlas;
  routes.set('/fixture-hud.json',[Buffer.from(JSON.stringify(manifest.hud)),'application/json']);
  routes.set('/cs16/'+manifest.hud.atlas.file,[atlas,'image/png']);
  server = createServer((request, response) => { const entry = routes.get(new URL(request.url, 'http://localhost').pathname); response.writeHead(entry ? 200 : 404, { 'Content-Type': entry?.[1] ?? 'text/plain' }); response.end(entry?.[0] ?? ''); });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  const origin = 'http://127.0.0.1:' + server.address().port;
  session = (await execute('agent-browser', ['session', 'id', '--scope', 'worktree', '--prefix', 'cmob'], { cwd: root, env: environment })).stdout.trim();
  await browser('open', origin);
  await browser('set', 'viewport', '1100', '800');
  await wait('document.querySelector("[data-cs16-board]")?.dataset.ready==="true"&&!!document.querySelector("[data-controller-input=cs16]")');
  const endpoint = await browser('get', 'cdp-url');
  socket = new WebSocket(typeof endpoint === 'string' ? endpoint : endpoint.cdpUrl);
  socket.onmessage = ({ data }) => { const message = JSON.parse(data), item = pending.get(message.id); if (!item) return; clearTimeout(item.timer); pending.delete(message.id); message.error ? item.reject(Error(JSON.stringify(message.error))) : item.resolve(message.result); };
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  const targets = await call('Target.getTargets', {}, undefined);
  const target = targets.targetInfos.find(target => target.type === 'page' && target.url.startsWith(origin));
  attached = (await call('Target.attachToTarget', { targetId: target.targetId, flatten: true }, undefined)).sessionId;
  const frame = () => evaluate('new Promise(resolve=>requestAnimationFrame(()=>resolve(document.querySelector("[data-cs16-canvas]").__cs16Frame)))');
  const point = async (selector, id=0) => evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});if(!element)return null;const box=element.getBoundingClientRect(),x=Math.round(box.left+box.width/2),y=Math.round(box.top+box.height/2),hit=document.elementFromPoint(x,y);return {x,y,id:${id},radiusX:3,radiusY:3,force:1,hit:hit===element||element.contains(hit)}})()`);
  const touch = (type, points) => call('Input.dispatchTouchEvent',{type,touchPoints:points.map(({hit,...point})=>point)});
  const tap = async selector => {const target=await point(selector);if(!target)return false;assert.ok(target.hit,selector+' covered');await call('Input.synthesizeTapGesture',{x:target.x,y:target.y,duration:50,tapCount:1,gestureSourceType:'touch'});await frame();return true};
  const coarse = async (width,height) => {await browser('set','viewport',String(width),String(height));await call('Emulation.setDeviceMetricsOverride',{width,height,deviceScaleFactor:1,mobile:true});await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5});await call('Emulation.setEmitTouchEventsForMouse',{enabled:true,configuration:'mobile'});await wait('matchMedia("(pointer:coarse)").matches');await frame()};
  await browser('focus','[data-cs16-board]');
  const pcBefore=await evaluate('window.cs16MobileAudit.inputs.length');
  await call('Input.dispatchKeyEvent',{type:'keyDown',code:'KeyJ',key:'j',windowsVirtualKeyCode:74});await frame();
  check('PC keyboard fire remains available',await evaluate(`window.cs16MobileAudit.inputs.slice(${pcBefore}).some(input=>input.fire)`));
  await call('Input.dispatchKeyEvent',{type:'keyUp',code:'KeyJ',key:'j',windowsVirtualKeyCode:74});await frame();
  const canvasPoint=await point('[data-cs16-canvas]');
  await call('Input.dispatchMouseEvent',{type:'mousePressed',x:canvasPoint.x,y:canvasPoint.y,button:'left',buttons:1,clickCount:1});
  await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:canvasPoint.x,y:canvasPoint.y,button:'left',buttons:0,clickCount:1});
  await wait('document.pointerLockElement===document.querySelector("[data-cs16-canvas]")');
  const mouseBefore=await evaluate('window.cs16MobileAudit.inputs.length');
  await call('Input.dispatchMouseEvent',{type:'mousePressed',x:canvasPoint.x,y:canvasPoint.y,button:'left',buttons:1,clickCount:1});await frame();
  check('PC locked mouse fire remains available',await evaluate(`window.cs16MobileAudit.inputs.slice(${mouseBefore}).some(input=>input.fire)`));
  await call('Input.dispatchMouseEvent',{type:'mouseReleased',x:canvasPoint.x,y:canvasPoint.y,button:'left',buttons:0,clickCount:1});
  await evaluate('document.exitPointerLock();true');
  await coarse(320,760);
  await evaluate('new Promise((resolve,reject)=>{const image=new Image();image.onload=resolve;image.onerror=reject;image.src="/cs16/hud/atlas.png"})');
  check('independent buy button in the legal buy area',await evaluate('!!document.querySelector(".cs16-touch-utility [data-cs16-open-buy]")'));
  await evaluate('window.cs16MobileAudit.buyState(false,true);true');await frame();
  check('buy button hidden outside the area',await evaluate('!document.querySelector(".cs16-touch-utility [data-cs16-open-buy]")'));
  await evaluate('window.cs16MobileAudit.buyState(true,false);true');await frame();
  check('buy button hidden outside the allowed time',await evaluate('!document.querySelector(".cs16-touch-utility [data-cs16-open-buy]")'));
  await evaluate('window.cs16MobileAudit.buyState(true,true);true');await frame();
  check('flashlight is independent from More',await evaluate('!!document.querySelector(".cs16-touch-utility [data-cs16-action=flashlight]")'));
  check('screen aiming is the only default fire method',await evaluate('document.querySelector("[data-cs16-stick=look]")?.dataset.cs16FireArea==="true"&&!document.querySelector("[data-cs16-action=fire]")&&!document.querySelector(".cs16-touch-more-panel input[type=checkbox]")'));
  const layouts=[[320,760],[844,390],[466,678],[678,466],[626,890],[890,626],[744,1133],[1133,744],[820,1180],[1180,820],[1024,1366],[1366,1024]];
  report.layouts=[];
  for(const [width,height] of layouts){
    await coarse(width,height);
    for(const fullscreen of [false,true]){
      if(fullscreen){await evaluate('Element.prototype.requestFullscreen=()=>Promise.reject(Error("page fullscreen audit"));true');await tap('#fullscreen');await wait('!!document.querySelector("[data-room-fullscreen]")')}
      const geometry=await evaluate(`(()=>{const root=document.querySelector('[data-cs16-board]'),ammo=root.querySelector('.cs16-ammo'),box=ammo.getBoundingClientRect(),digits=[...ammo.querySelectorAll('[data-cs16-sprite^=number_]')].map(element=>{const rect=element.getBoundingClientRect();const hit=document.elementFromPoint(rect.left+rect.width/2,rect.top+rect.height/2);return {left:Math.round(rect.left*1000),right:Math.round(rect.right*1000),top:Math.round(rect.top*1000),bottom:Math.round(rect.bottom*1000),height:Math.round(rect.height*1000),mask:getComputedStyle(element).maskImage,blocked:!!hit?.closest('button,input,.cs16-touch-more-panel,.cs16-buy-menu,.table-chat')}});const buttons=[...root.querySelectorAll('.cs16-touch button')].filter(element=>element.getClientRects().length&&getComputedStyle(element).visibility!=='hidden').map(element=>{const rect=element.getBoundingClientRect();return {action:element.dataset.cs16Action||element.dataset.cs16OpenBuy||element.dataset.cs16ControlsMenu,left:Math.round(rect.left),right:Math.round(rect.right),top:Math.round(rect.top),bottom:Math.round(rect.bottom),width:Math.round(rect.width),height:Math.round(rect.height),owned:(()=>{const hit=document.elementFromPoint(rect.left+rect.width/2,rect.top+rect.height/2);return hit===element||element.contains(hit)})()}});return {box:{left:Math.round(box.left),right:Math.round(box.right),top:Math.round(box.top),bottom:Math.round(box.bottom)},digits,buttons,width:innerWidth,height:innerHeight,ammoText:ammo.getAttribute('aria-label')}})()`);
      report.layouts.push({width,height,fullscreen,...geometry});
      check(`${width}x${height} ${fullscreen} original ammo digits stay readable and unoccluded`,geometry.digits.length===5&&geometry.digits.every(digit=>digit.height>=25000&&!digit.blocked&&digit.mask!=='none'&&digit.left>=0&&digit.right<=width*1000&&digit.top>=0&&digit.bottom<=height*1000));
      check(`${width}x${height} ${fullscreen} touch controls stay within viewport`,geometry.buttons.every(button=>button.left>=0&&button.right<=width&&button.top>=0&&button.bottom<=height&&button.width>=44&&button.height>=44&&button.owned));
      if(width===320||width===890||width===1180)await browser('screenshot',path.join(out,`${width}-${height}-${fullscreen}.png`));
      if(fullscreen){await tap('[aria-label=退出全屏]');await wait('!document.querySelector("[data-room-fullscreen]")')}
    }
  }
  await coarse(890,626);
  const move=await point('[data-cs16-stick=move]',0),aim=await point('[data-cs16-stick=look]',1);
  assert.ok(move.hit&&aim.hit,'thumb origins covered');
  await touch('touchStart',[move]);await touch('touchMove',[{...move,y:move.y-30}]);
  await touch('touchStart',[{...move,y:move.y-30},aim]);
  const held=await frame();check('two thumbs move and fire at once',held.touch.moveY>0&&held.touch.actions.includes('fire'));
  await touch('touchMove',[{...move,y:move.y-30},{...aim,x:aim.x-32,y:aim.y-10}]);
  const dragged=await frame();check('screen fire keeps shooting while aiming',dragged.touch.actions.includes('fire')&&dragged.yaw!==held.yaw);
  await touch('touchEnd',[{...move,y:move.y-30}]);
  check('releasing movement preserves firing',(await frame()).touch.actions.includes('fire'));
  await touch('touchEnd',[]);check('releasing aim stops firing',!(await frame()).touch.actions.includes('fire'));
  await touch('touchStart',[move,aim]);await touch('touchCancel',[]);check('cancel releases both thumbs',(await frame()).touch.actions.length===0);
  await browser('focus','[data-cs16-board]');await touch('touchStart',[aim]);await call('Input.dispatchKeyEvent',{type:'keyDown',code:'KeyW',key:'w',windowsVirtualKeyCode:87});check('keyboard takeover releases old screen fire',!(await frame()).touch.actions.includes('fire'));await call('Input.dispatchKeyEvent',{type:'keyUp',code:'KeyW',key:'w',windowsVirtualKeyCode:87});await touch('touchEnd',[]);
  await touch('touchStart',[aim]);await coarse(626,890);await touch('touchEnd',[]);check('rotation drops old shooting',(await frame()).touch.actions.length===0);
  await coarse(320,760);
  const rawTap=async selector=>{const target=await point(selector);if(!target)return false;assert.ok(target.hit,selector+' covered');await touch('touchStart',[target]);await frame();await touch('touchEnd',[target]);await frame();return true};
  for(const selector of ['.cs16-touch-utility [data-cs16-action=flashlight]','.cs16-touch-utility [data-cs16-action=reload]','[data-cs16-select]','.chat-toggle']){
    const exists=await point(selector);if(!exists){check(selector+' exists without firing',false);continue}
    const before=await evaluate('window.cs16MobileAudit.inputs.length');await rawTap(selector);if(selector.includes('flashlight'))check('independent flashlight submits an operation',await evaluate(`window.cs16MobileAudit.inputs.slice(${before}).some(input=>input.flashlight)`));check(selector+' does not fire',!(await frame()).touch.actions.includes('fire'));
  }
  await evaluate('window.cs16MobileAudit.chat(false);true');await frame();
  await rawTap('[data-cs16-controls-menu]');await wait('!!document.querySelector(".cs16-touch-more-panel")');
  check('More has no alternate fire option or duplicate purchase/flashlight',await evaluate('!document.querySelector(".cs16-touch-more-panel input[type=checkbox],.cs16-touch-more-panel [data-cs16-open-buy],.cs16-touch-more-panel [data-cs16-action=flashlight]")'));
  const utility=await point('.cs16-touch-more-panel [data-cs16-action=drop]',2),zone=await point('[data-cs16-stick=look]',2);
  await touch('touchStart',[utility]);await touch('touchMove',[{...zone,id:2}]);check('dragging utility into aim does not become fire',!(await frame()).touch.actions.includes('fire'));await touch('touchEnd',[]);
  await rawTap('[data-cs16-controls-menu]');
  const bought=await rawTap('.cs16-touch-utility [data-cs16-open-buy]');
  if(bought){await wait('!!document.querySelector(".cs16-buy-menu")');const menu=await point('.cs16-buy-menu');await touch('touchStart',[menu]);check('buy menu never fires',!(await frame()).touch.actions.includes('fire'));await touch('touchEnd',[]);await rawTap('.cs16-buy-menu [data-controller-cancel]')}
  await browser('focus','[data-cs16-board]');
  const currentAim=await point('[data-cs16-stick=look]',1);await touch('touchStart',[currentAim]);await call('Emulation.setTouchEmulationEnabled',{enabled:false});await call('Emulation.setEmitTouchEventsForMouse',{enabled:false});await frame();check('switching from touch mode releases fire',(await frame()).touch.actions.length===0);await call('Emulation.setTouchEmulationEnabled',{enabled:true,maxTouchPoints:5});await touch('touchEnd',[currentAim]).catch(error=>{if(!/Must send a TouchStart|No active touch/.test(error.message))throw error});await touch('touchCancel',[]).catch(error=>{if(!/Must send a TouchStart|No active touch/.test(error.message))throw error});
  report.events=await evaluate('window.cs16MobileAudit.events');report.inputActions=await evaluate('window.cs16MobileAudit.inputs');
  check('trusted touch input submits shooting operations',report.events.some(event=>event.type==='touchstart'&&event.trusted)&&report.inputActions.some(input=>input.fire));
  report.passed=report.checks.every(check=>check.passed);
} catch (error) {
  report.error = error.stack;
  try { report.events = await evaluate('window.cs16MobileAudit.events'); report.browserErrors = await browser('errors'); await browser('screenshot', path.join(out, 'failure.png')); } catch {}
} finally {
  if (socket) socket.close();
  for (const item of pending.values()) clearTimeout(item.timer);
  if (session) { try { await browser('close'); } catch {} }
  if (server) await new Promise(resolve => server.close(resolve));
  await rm(temporary, { recursive: true, force: true });
  report.sourceChangedDuringRun = [];
  for (const [file, digest] of Object.entries(report.sources)) if (createHash('sha256').update(await readFile(path.join(root, file))).digest('hex') !== digest) report.sourceChangedDuringRun.push(file);
  if (report.sourceChangedDuringRun.length) report.passed = false;
  await writeFile(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n');
}
console.log(JSON.stringify({ passed: report.passed, checks: report.checks, error: report.error, report: path.join(out, 'result.json') }));
process.exitCode = report.passed ? 0 : 1;
