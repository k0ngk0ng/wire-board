import assert from 'node:assert/strict';
import { execFile, spawn } from 'node:child_process';
import { mkdir, writeFile, readFile, rm } from 'node:fs/promises';
import { createReadStream, existsSync, statSync, readFileSync } from 'node:fs';
import { createHash, randomUUID } from 'node:crypto';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const name = process.env.HM_VIEWPORT_REPORT ?? 'current';
assert.match(name, /^[a-zA-Z0-9_-]{1,40}$/);
const base = path.join(root, '.local/hotlinemiami');
const out = path.join(base, 'verification/mobile-viewport', name);
const temporary = path.join(out, 'temporary');
for (const dir of [out, temporary, 'browser', 'go-cache', 'go-mod', 'tmp', 's'].map(dir => path.isAbsolute(dir) ? dir : path.join(base, dir))) await mkdir(dir, { recursive: true });
for (const dir of ['cache', 'config', 'data', 'profile-0', 'profile-1', 'profile-2', 'profile-3']) await mkdir(path.join(temporary, dir), { recursive: true });
const env = { ...process.env, TMPDIR: path.join(base, 'tmp'), GOTMPDIR: path.join(base, 'tmp'), GOCACHE: path.join(base, 'go-cache'), GOMODCACHE: path.join(base, 'go-mod'), GOPROXY: process.env.HM_LOCAL_GOPROXY ?? 'off', GOTOOLCHAIN: 'local', GOSUMDB: 'off', GOTELEMETRY: 'off', XDG_CACHE_HOME: path.join(temporary, 'cache'), XDG_CONFIG_HOME: path.join(temporary, 'config'), XDG_DATA_HOME: path.join(temporary, 'data'), AGENT_BROWSER_SOCKET_DIR: path.join(base, 's'), AGENT_BROWSER_NO_VERSION_CHECK: '1', WIRE_BOARD_HOTLINE_BROWSER: '1' };
const sessions = Array.from({ length: 4 }, (_, index) => 'hv-' + process.pid + '-' + index);
const opened = new Set();
const browser = async (index, ...args) => {
  const input = args[0] === 'eval' && args[1] === '--stdin' ? args.splice(2, 1)[0] : '';
  const result = await new Promise((resolve, reject) => {
    const child = execFile('agent-browser', ['--session', sessions[index], '--profile', path.join(temporary, 'profile-' + index), ...args, '--json'], { cwd: root, env, timeout: 60000, maxBuffer: 8 * 1024 * 1024 }, (error, stdout, stderr) => error ? reject(Error(stderr || stdout || error.message)) : resolve({ stdout, stderr }));
    child.stdin.end(input);
  });
  const body = JSON.parse(result.stdout); assert.ok(body.success, result.stdout); return body.data;
};
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const until = async (predicate, milliseconds = 30000) => {
  const end = Date.now() + milliseconds;
  while (Date.now() < end) { const value = await predicate(); if (value) return value; await pause(50); }
  throw Error('mobile viewport condition timed out');
};
const sourceFiles = ['games/hotlinemiami/web/Board.tsx', 'games/hotlinemiami/web/renderer.ts', 'games/hotlinemiami/web/camera-input.ts', 'games/hotlinemiami/web/TouchControls.tsx', 'games/hotlinemiami/web/style.css', 'games/hotlinemiami/web/module.tsx', 'web/src/ui/room-fullscreen.tsx', 'web/src/ui/room-fullscreen.css'];
const hashes = () => Object.fromEntries(sourceFiles.map(file => [file, createHash('sha256').update(readFileSync(path.join(root, file))).digest('hex')]));
const report = { passed: false, scope: 'actual source platform and Hotline Miami Board against the existing local HTTP, SQLite and WebSocket fixture; native Chromium inputs; frontend viewport verification, not rules matches or physical-device/performance certification', sources: hashes(), cases: [], failures: [], nativeActions: [] };
const require = createRequire(new URL('../../../web/package.json', import.meta.url));
const { createServer } = require('vite');
const react = (await import(require.resolve('@vitejs/plugin-react'))).default;
let go, server, socket, session, origin;
let serverLog = '';
let rpcSerial = 0;
const pending = new Map();
const call = (method, params = {}, selected = session) => new Promise((resolve, reject) => {
  const id = ++rpcSerial;
  const timer = setTimeout(() => { pending.delete(id); reject(Error('mobile viewport CDP timeout')); }, 30000);
  pending.set(id, { resolve, reject, timer });
  socket.send(JSON.stringify({ id, method, params, ...(selected ? { sessionId: selected } : {}) }));
});
const evaluate = async source => { const result = await call('Runtime.evaluate', { expression: source, returnByValue: true, awaitPromise: true }); assert.ok(!result.exceptionDetails, JSON.stringify(result.exceptionDetails)); return result.result.value; };
const nativeClick = async selector => {
  const point = await evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});element.scrollIntoView({block:'nearest'});const rect=element.getBoundingClientRect();return {x:rect.x+rect.width/2,y:rect.y+rect.height/2}})()`);
  await call('Input.dispatchMouseEvent', { type: 'mouseMoved', ...point });
  await call('Input.dispatchMouseEvent', { type: 'mousePressed', ...point, button: 'left', buttons: 1, clickCount: 1 });
  await call('Input.dispatchMouseEvent', { type: 'mouseReleased', ...point, button: 'left', buttons: 0, clickCount: 1 });
};
const api = async (index, url, body) => {
  const source = `(async()=>{const response=await fetch(${JSON.stringify('/api' + url)}${body ? ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + JSON.stringify(body) + ')}' : ''});return {status:response.status,body:await response.json()}})()`;
  const result = (await browser(index, 'eval', '--stdin', source)).result;
  assert.ok(result.status >= 200 && result.status < 300, JSON.stringify(result)); return result.body;
};
const command = async (index, type, extra = {}) => { const state = await api(index, '/state'); return api(index, '/rooms/' + state.room.id, { type, version: state.room.version, nonce: randomUUID(), ...extra }); };
const inspect = () => evaluate(`(()=>{const canvas=document.querySelector('[data-hotlinemiami-canvas]');const frame=canvas.__hotlineFrame;const box=element=>element.getBoundingClientRect().toJSON();const css=element=>getComputedStyle(element);const obstacles=[...document.querySelectorAll('.hm-battle-heading,.hm-score-strip,.hm-stick,.hm-touch-actions>button,.hm-bottom-line,.game-fullscreen-toolbar,.game-fullscreen-status,.chat-toggle')].filter(element=>{const r=box(element),s=css(element);return r.width>0&&r.height>0&&s.display!=='none'&&s.visibility!=='hidden'&&Number(s.opacity)!==0}).map(element=>({kind:element.className,rect:box(element)}));return {width:innerWidth,height:innerHeight,coarse:matchMedia('(pointer:coarse)').matches,canvas:box(canvas),board:box(document.querySelector('[data-hotlinemiami-board]')),camera:frame.camera,local:frame.local,initialLife:window.__hmOriginalLife,sameCanvas:window.__hmOriginalCanvas===canvas,drawCount:frame.drawCount,obstacles,controls:[...document.querySelectorAll('[data-hotlinemiami-stick]')].map(element=>({kind:element.dataset.hotlinemiamiStick,rect:box(element),display:css(element).display})),overflow:document.documentElement.scrollWidth>innerWidth,alerts:[...document.querySelectorAll('[data-hotlinemiami-board] [role=alert]')].map(element=>element.textContent)}})()`);
const check = (condition, kind, detail) => { if (!condition) report.failures.push({ kind, ...detail }); };
try {
  await rm(path.join(base, 'browser/urls.json'), { force: true }); await rm(path.join(base, 'browser/stop'), { force: true });
  for (const client of ['main', 'peer']) await writeFile(path.join(base, 'browser/network-' + client + '.json'), JSON.stringify({ rtt: 0, jitter: 0, offline: false }));
  go = spawn('go', ['test', '-count=1', '-timeout=26m', '-run', '^TestHotlineBrowserFixture$', './games/hotlinemiami/tests'], { cwd: root, env, stdio: ['ignore', 'pipe', 'pipe'] });
  go.stdout.on('data', data => { serverLog += data; }); go.stderr.on('data', data => { serverLog += data; });
  await until(async () => { if (go.exitCode !== null) throw Error('local fixture exited: ' + serverLog.slice(-3000)); return existsSync(path.join(base, 'browser/urls.json')); }, 240000);
  const urls = JSON.parse(await readFile(path.join(base, 'browser/urls.json'), 'utf8'));
  const backendVersion = await fetch(urls.main + '/api/version').then(response => response.json());
  const releaseManifest = JSON.parse(await readFile(path.join(root, 'web/dist/game-load-manifest.json'), 'utf8'));
  const developmentManifest = { ...releaseManifest, entry: '/src/main.tsx', games: { hotlinemiami: [] }, media: Object.fromEntries(Object.entries(releaseManifest.media).filter(([url]) => url.startsWith('/hotlinemiami/'))) };
  report.loadingManifest = { sourceEntry: developmentManifest.entry, backendGateBuild: developmentManifest.build, codeLoadedByVite: true, boardReadinessRequired: true, productionEquivalentSingleEffects: true, developmentVersionAssets: [] };
  const aliases = Object.fromEntries(['react', 'react-dom', 'lucide-react', 'fflate', 'three', 'react-markdown', 'remark-gfm'].map(id => [id, path.join(root, 'web/node_modules', id)]));
  server = await createServer({ configFile: false, root: path.join(root, 'web'), publicDir: false, cacheDir: path.join(temporary, 'vite-cache'), plugins: [{ name: 'production-effect-lifecycle', enforce: 'pre', transform(source, id) { if (id.split('?')[0] === path.join(root, 'web/src/main.tsx')) return source.replaceAll('<React.StrictMode>', '<>').replaceAll('</React.StrictMode>', '</>'); } }, react(), { name: 'hotline-local-resources', configureServer(vite) { vite.middlewares.use((request, response, next) => {
    if (request.url?.split('?')[0] === '/game-load-manifest.json') { response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify(developmentManifest)); return; }
    if (request.url?.split('?')[0] === '/api/version') { response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify({ ...backendVersion, assets: [], assetURLs: [] })); return; }
    if (!request.url?.startsWith('/hotlinemiami/')) return next();
    const relative = new URL(request.url, 'http://localhost').pathname.slice('/hotlinemiami/'.length);
    const candidates = [path.join(root, 'web/dist/hotlinemiami', relative), path.join(root, 'games/hotlinemiami/assets/public/hotlinemiami', relative)];
    const file = candidates.find(file => file.startsWith(path.join(root, 'web/dist/hotlinemiami') + path.sep) || file.startsWith(path.join(root, 'games/hotlinemiami/assets/public/hotlinemiami') + path.sep) ? existsSync(file) && statSync(file).isFile() : false);
    if (!file) { response.statusCode = 404; response.end(); return; }
    const size = statSync(file).size;
    response.setHeader('Content-Type', file.endsWith('.json') ? 'application/json' : file.endsWith('.js') ? 'text/javascript' : file.endsWith('.wasm') ? 'application/wasm' : file.endsWith('.png') ? 'image/png' : file.endsWith('.flac') ? 'audio/flac' : file.endsWith('.ogg') ? 'audio/ogg' : 'application/octet-stream');
    response.setHeader('Cache-Control', 'no-store'); response.setHeader('Accept-Ranges', 'bytes');
    const range = /^bytes=(\d+)-(\d*)$/.exec(request.headers.range ?? '');
    if (range) { const start = Number(range[1]), end = range[2] ? Math.min(size - 1, Number(range[2])) : size - 1; response.statusCode = 206; response.setHeader('Content-Range', `bytes ${start}-${end}/${size}`); response.setHeader('Content-Length', end - start + 1); createReadStream(file, { start, end }).pipe(response); }
    else { response.setHeader('Content-Length', size); createReadStream(file).pipe(response); }
  }); } }], resolve: { alias: aliases, dedupe: ['react', 'react-dom'] }, server: { host: '127.0.0.1', port: 0, hmr: false, fs: { allow: [root] }, proxy: { '/api': { target: urls.main, ws: true, changeOrigin: false } } } });
  await server.listen(); origin = server.resolvedUrls.local[0];
  for (let index = 0; index < 4; index++) {
    opened.add(index); await browser(index, 'open', origin);
    await api(index, '/register', { name: 'HM视区' + process.pid + '-' + index, password: 'hotline-local-test', invite: 'hotline-browser' }); await browser(index, 'reload');
  }
  const room = await api(0, '/rooms', { kind: 'hotlinemiami', capacity: 4, options: { map: 'no-talk', roundSeconds: 300, characters: ['biker', 'jacket', 'bodyguard', 'hobo'], weapons: ['m16', 'pistol', 'pistol', 'pistol'] } });
  for (let index = 1; index < 4; index++) { const latest = (await api(0, '/state')).room; await api(index, '/rooms/' + room.id, { type: 'join', version: latest.version, nonce: randomUUID() }); }
  for (let index = 0; index < 4; index++) await command(index, 'ready');
  await command(0, 'start');
  for (let index = 0; index < 4; index++) await until(async () => (await browser(index, 'eval', '--stdin', '!!document.querySelector("[data-hotlinemiami-canvas]")?.__hotlineFrame?.inputEnabled')).result, 90000);
  const started = (await api(0, '/state')).room;
  report.room = { id: started.id, epoch: started.game.data.epoch, participants: started.seats.length, controlledCharacter: started.game.data.players[0].character };
  assert.equal(report.room.controlledCharacter, 'biker');
  for (let index = 1; index < 4; index++) { await browser(index, 'close'); opened.delete(index); }
  const endpoint = await browser(0, 'get', 'cdp-url'); socket = new WebSocket(typeof endpoint === 'string' ? endpoint : endpoint.cdpUrl);
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  socket.onmessage = ({ data }) => {
    const result = JSON.parse(data), request = pending.get(result.id);
    if (request) { pending.delete(result.id); clearTimeout(request.timer); result.error ? request.reject(Error(JSON.stringify(result.error))) : request.resolve(result.result); }
    if (result.sessionId === session && result.method === 'Network.webSocketFrameSent') {
      try { const body = JSON.parse(result.params.response.payloadData); if (body.action?.type === 'input') report.nativeActions.push({ at: Math.round(result.params.timestamp * 1000), ...body.action }); } catch {}
    }
    if (result.sessionId === session && result.method === 'Network.requestWillBeSent' && result.params.request.method === 'POST') {
      try { const body = JSON.parse(result.params.request.postData); if (body.action?.type === 'input') report.nativeActions.push({ at: Math.round(result.params.timestamp * 1000), transport: 'HTTP', ...body.action }); } catch {}
    }
  };
  const target = (await call('Target.getTargets', {}, undefined)).targetInfos.find(target => target.type === 'page' && target.url.startsWith(origin)); assert.ok(target);
  session = (await call('Target.attachToTarget', { targetId: target.targetId, flatten: true }, undefined)).sessionId;
  await call('Network.enable'); await call('Page.bringToFront');
  await evaluate('window.__hmOriginalCanvas=document.querySelector("[data-hotlinemiami-canvas]");window.__hmOriginalLife=document.querySelector("[data-hotlinemiami-canvas]").__hotlineFrame.local.life;true');
  report.characterPaint = await evaluate(`(async()=>{const manifest=await fetch('/hotlinemiami/manifest.json').then(r=>r.json());const images=new Map();const result={};for(const character of manifest.characters){const sprite=manifest.sprites[character.sprite];if(!images.has(sprite.image)){const image=new Image();image.src='/hotlinemiami/'+manifest.images[sprite.image].file;await image.decode();images.set(sprite.image,image)}const frame=sprite.frames[0],canvas=document.createElement('canvas');canvas.width=frame.w;canvas.height=frame.h;const context=canvas.getContext('2d',{willReadFrequently:true});context.drawImage(images.get(sprite.image),frame.x,frame.y,frame.w,frame.h,0,0,frame.w,frame.h);const pixels=context.getImageData(0,0,frame.w,frame.h).data;let left=frame.w,right=-1,top=frame.h,bottom=-1,opaque=0;for(let y=0;y<frame.h;y++)for(let x=0;x<frame.w;x++)if(pixels[(y*frame.w+x)*4+3]>32){left=Math.min(left,x);right=Math.max(right,x);top=Math.min(top,y);bottom=Math.max(bottom,y);opaque++}result[character.id]={width:right-left+1,height:bottom-top+1,opaque,frameWidth:frame.w,frameHeight:frame.h}}return result})()`);
  const viewports = process.env.HM_VIEWPORT_FAST_RED === '1' ? [[320, 568], [890, 626], [626, 890]] : [[320, 568], [320, 740], [890, 626], [626, 890], [820, 1180], [1180, 820], [744, 1133], [1133, 744]];
  const setViewport = async (width, height, touch) => { await call('Emulation.setTouchEmulationEnabled', { enabled: touch, maxTouchPoints: touch ? 3 : 1 }); await call('Emulation.setEmitTouchEventsForMouse', { enabled: touch, configuration: 'mobile' }); await call('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: touch, screenWidth: width, screenHeight: height }); await pause(150); };
  const capture = async file => { const image = await call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await writeFile(path.join(out, file + '.png'), Buffer.from(image.data, 'base64')); };
  for (const mode of process.env.HM_VIEWPORT_FAST_RED === '1' ? ['native', 'page'] : ['normal', 'native', 'page']) {
    if (mode !== 'normal') {
      if (mode === 'page') await evaluate('(()=>{const surface=document.querySelector("[data-game-fullscreen-surface]");window.__hmFullscreenDescriptor=Object.getOwnPropertyDescriptor(surface,"requestFullscreen");Object.defineProperty(surface,"requestFullscreen",{configurable:true,value:()=>Promise.reject(Error("controlled fullscreen fallback"))});return true})()');
      await nativeClick('button[aria-label="全屏"]');
      await until(() => evaluate('document.querySelector("[data-room-fullscreen]")?.dataset.roomFullscreen'));
      const actual = await evaluate('document.querySelector("[data-room-fullscreen]").dataset.roomFullscreen'); check(actual === mode, 'fullscreen mode', { mode, actual });
      if (mode === 'page') await evaluate('(()=>{const surface=document.querySelector("[data-game-fullscreen-surface]");if(window.__hmFullscreenDescriptor)Object.defineProperty(surface,"requestFullscreen",window.__hmFullscreenDescriptor);else delete surface.requestFullscreen;delete window.__hmFullscreenDescriptor;return true})()');
    }
    for (const [width, height] of viewports) {
      await setViewport(width, height, true); if (mode === 'normal') await evaluate('document.querySelector("[data-hotlinemiami-board]").scrollIntoView({block:"center"});true');
      const before = await inspect(); const label = mode + '-' + width + 'x' + height;
      const paint = report.characterPaint.biker; const sx = before.canvas.width / before.camera.width, sy = before.canvas.height / before.camera.height;
      const body = { left: before.canvas.left + before.local.screenX - paint.width * sx / 2, top: before.canvas.top + before.local.screenY - paint.height * sy / 2, width: paint.width * sx, height: paint.height * sy };
      const overlapping = before.obstacles.filter(item => item.rect.left < body.left + body.width && item.rect.right > body.left && item.rect.top < body.top + body.height && item.rect.bottom > body.top);
      const evidence = { label, mode, viewport: [width, height], ...before, body, scaleX: sx, scaleY: sy, overlaps: overlapping };
      report.cases.push(evidence); await capture(label);
      check(Math.round(Math.min(body.width, body.height) * 1000) >= 24000, 'actual painted character is below 24 CSS px', { label, body });
      check(Math.abs(Math.round(sx / sy * 1000) - 1000) <= 2, 'world is stretched', { label, sx, sy });
      check(!before.overflow && before.sameCanvas, 'viewport overflow or remounted canvas', { label });
      check(overlapping.length === 0, 'current character covered by controls or HUD', { label, overlaps: overlapping });
      check(Math.round(body.left) >= Math.round(before.canvas.left) && Math.round(body.top) >= Math.round(before.canvas.top) && Math.round(body.left + body.width) <= Math.round(before.canvas.right) && Math.round(body.top + body.height) <= Math.round(before.canvas.bottom), 'current character outside the battle', { label, body });
      check(before.alerts.length === 0 && before.local.life === before.initialLife, 'viewport change reset life or displayed an error', { label });
      const controls = before.controls;
      if (controls.every(item => item.rect.height > 0 && item.rect.bottom <= height && item.rect.top >= 0)) {
        for (const control of controls) check(before.canvas.bottom - control.rect.bottom >= -1 && before.canvas.bottom - control.rect.bottom <= 28, 'stick is outside or too far from battle bottom', { label, control });
        const points = controls.map((control, id) => ({ id, x: control.rect.x + control.rect.width / 2, y: control.rect.y + control.rect.height / 2, radiusX: 4, radiusY: 4, force: 1 }));
        const beginning = report.nativeActions.length;
        await call('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: points });
        const moved = points.map((point, index) => ({ ...point, x: point.x + 24, y: point.y + (index ? -20 : 0) }));
        await call('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: moved });
        await pause(55); await call('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [moved[1]] }); await pause(50);
        await call('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] }); await pause(80);
        const actions = report.nativeActions.slice(beginning); evidence.actions = actions;
        check(actions.some(action => action.moveX > 0 && action.fire), 'native multi-touch move/fire missing', { label });
        check(actions.some(action => action.moveX > 0 && !action.fire), 'releasing aim also released movement', { label });
        check(actions.at(-1)?.moveX === 0 && !actions.at(-1)?.fire, 'touch release stuck', { label });
        await call('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: moved }); await pause(40);
        await call('Input.dispatchTouchEvent', { type: 'touchCancel', touchPoints: [] }); await pause(80);
        check(report.nativeActions.at(-1)?.moveX === 0 && !report.nativeActions.at(-1)?.fire, 'touch cancel stuck', { label });
        if (width === 320 && height === 568) {
          await call('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: moved }); await pause(40);
          await setViewport(626, 890, true); await pause(80);
          check(report.nativeActions.at(-1)?.moveX === 0 && !report.nativeActions.at(-1)?.fire, 'resize retained old touch input', { label });
          await call('Input.dispatchTouchEvent', { type: 'touchCancel', touchPoints: [] }).catch(error => { if (!error.message.includes('Must send a TouchStart first')) throw error; });
        }
      } else check(false, 'touch controls outside visible screen', { label, controls });
    }
    if (mode !== 'normal') { await nativeClick('button[aria-label="退出全屏"]'); await until(() => evaluate('!document.querySelector("[data-room-fullscreen]")')); }
  }
  await setViewport(1280, 900, false); await evaluate('document.querySelector("[data-hotlinemiami-board]").scrollIntoView({block:"center"});true');
  const desktop = await inspect(); report.desktop = desktop;
  const x = desktop.canvas.x + desktop.canvas.width * 3 / 4, y = desktop.canvas.y + desktop.canvas.height / 4;
  await call('Input.dispatchMouseEvent', { type: 'mouseMoved', x, y }); await pause(100);
  const aimed = await inspect();
  const wx = aimed.camera.x + aimed.camera.width * 3 / 4, wy = aimed.camera.y + aimed.camera.height / 4;
  const expected = (Math.round(Math.atan2(wy * 256 - aimed.local.y, wx * 256 - aimed.local.x) * 1024 / (Math.PI * 2)) + 1024) % 1024;
  check(Math.abs(aimed.local.aim - expected) <= 1, 'mouse-to-world aim diverged after resize', { actual: aimed.local.aim, expected });
  check(desktop.controls.every(control => control.rect.width === 0 || control.rect.height === 0), 'PC displays touch controls', {});
  report.sameCanvas = aimed.sameCanvas; report.passed = report.failures.length === 0;
} catch (error) {
  report.error = error.stack;
  report.pages = [];
  for (const index of opened) { try { report.pages.push({ index, body: await browser(index, 'eval', '--stdin', 'document.body.innerText.slice(0,3000)'), errors: await browser(index, 'errors') }); } catch {} }
}
finally {
  if (socket?.readyState === WebSocket.OPEN) { try { await call('Input.dispatchTouchEvent', { type: 'touchCancel', touchPoints: [] }); } catch {} socket.close(); }
  for (const index of opened) { try { await browser(index, 'close'); } catch {} }
  await server?.close();
  if (go) { await writeFile(path.join(base, 'browser/stop'), 'stop\n'); await Promise.race([new Promise(resolve => go.once('close', resolve)), pause(15000)]); if (go.exitCode === null) go.kill('SIGTERM'); }
  const after = hashes(); report.sourceChangedDuringRun = sourceFiles.filter(file => report.sources[file] !== after[file]);
  if (report.sourceChangedDuringRun.length) report.passed = false;
  await writeFile(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n');
  await writeFile(path.join(out, 'server.log'), serverLog);
  await writeFile(path.join(out, 'summary.json'), JSON.stringify({ passed: report.passed, cases: report.cases.length, failures: report.failures.length, error: report.error, report: path.join(out, 'result.json') }, null, 2) + '\n');
  await rm(temporary, { recursive: true, force: true });
  process.stdout.write(JSON.stringify({ passed: report.passed, cases: report.cases.length, failures: report.failures.length, error: report.error, output: out }) + '\n');
}
process.exitCode = report.passed ? 0 : 1;
