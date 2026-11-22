import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdir, readFile, writeFile, rm } from 'node:fs/promises';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import { createHash } from 'node:crypto';

test('shared fullscreen uses overlay controls and maximizes five proportional chess boards', { timeout: 300000 }, async () => {
  const root = fileURLToPath(new URL('../../', import.meta.url));
  const red = process.env.FULLSCREEN_LAYOUT_PHASE === 'red';
  const name = process.env.BOARD_FULLSCREEN_REPORT ?? (red ? 'red' : 'current');
  assert.match(name, /^[a-z0-9]+(?:-[a-z0-9]+)*$/);
  const out = path.join(root, '.local/room-tools-followup/verification/fullscreen', name);
  const temporary = path.join(out, 'temporary');
  const environment = { ...process.env, TMPDIR: path.join(root, '.local/chft', process.pid.toString(36)), XDG_CACHE_HOME: path.join(temporary, 'cache'), XDG_CONFIG_HOME: path.join(temporary, 'config'), XDG_DATA_HOME: path.join(temporary, 'data'), AGENT_BROWSER_PROFILE: path.join(temporary, 'profile'), AGENT_BROWSER_SOCKET_DIR: path.join(root, '.local/room-tools-followup/s'), AGENT_BROWSER_NO_VERSION_CHECK: '1' };
  for (const directory of [out, temporary, environment.TMPDIR, environment.XDG_CACHE_HOME, environment.XDG_CONFIG_HOME, environment.XDG_DATA_HOME, environment.AGENT_BROWSER_PROFILE, environment.AGENT_BROWSER_SOCKET_DIR]) await mkdir(directory, { recursive: true });
  const execute = promisify(execFile);
  const session = (await execute('agent-browser', ['session', 'id', '--scope', 'worktree', '--prefix', 'chfs'], { cwd: root, env: environment })).stdout.trim();
  const browser = async (...args) => {
    let result;
    try {
      result = await execute('agent-browser', ['--session', session, '--profile', environment.AGENT_BROWSER_PROFILE, ...args, '--json'], { cwd: root, env: environment, timeout: 30000, maxBuffer: 4 << 20 });
    } catch (error) {
      const failure = { command: args, message: error.message, stdout: error.stdout, stderr: error.stderr };
      failure.doctor = await execute('agent-browser', ['--session', session, 'doctor', '--json'], { cwd: root, env: environment, timeout: 10000, maxBuffer: 1 << 20 }).then(value => ({ stdout: value.stdout, stderr: value.stderr }), value => ({ stdout: value.stdout, stderr: value.stderr, message: value.message }));
      await writeFile(path.join(out, 'browser-' + args[0] + '-failure.json'), JSON.stringify(failure, null, 2) + '\n');
      throw Error(JSON.stringify(failure), { cause: error });
    }
    const value = JSON.parse(result.stdout);
    assert.ok(value.success, result.stdout);
    return value.data;
  };
  const require = createRequire(new URL('../package.json', import.meta.url));
  const { createServer } = require('vite');
  const react = require('@vitejs/plugin-react').default;
  await writeFile(path.join(temporary, 'index.html'), `<meta name="viewport" content="width=device-width,initial-scale=1"><style>*{box-sizing:border-box}body{margin:0}</style><div id="root"></div><script>window.fullscreenBoardAssets=${JSON.stringify('/@fs' + root.replace(/\/$/, ''))}</script><script type="module" src="/@fs${path.join(root, 'web/tests/BoardFullscreenFixture.tsx')}"></script>`);
  const server = await createServer({ configFile: false, root: temporary, cacheDir: path.join(temporary, 'vite-cache'), publicDir: false, plugins: [react()], resolve: { alias: { react: path.join(root, 'web/node_modules/react'), 'react-dom': path.join(root, 'web/node_modules/react-dom'), 'lucide-react': path.join(root, 'web/node_modules/lucide-react') }, dedupe: ['react', 'react-dom'] }, server: { host: '127.0.0.1', port: 0, fs: { allow: [root] } } });
  await server.listen();
  const report = { passed: false, phase: red ? 'red' : 'acceptance', scope: 'Actual Board/Chat/GameFullscreen DOM with anonymized static views, trusted Chromium mouse/touch, native and explicit page fallback; no complete rule match or physical device claim', checks: [], layouts: [], sources: {} };
  const sourceFiles = ['web/src/ui/room-fullscreen.tsx', 'web/src/ui/room-fullscreen.css', 'web/src/ui/fullscreen-overlay.ts', 'web/src/ui/map-controls.ts', 'web/tests/BoardFullscreenFixture.tsx', 'web/tests/board-fullscreen-browser.test.mjs', 'web/tests/fixtures/board-fullscreen.json', ...['gomoku', 'weiqi', 'chess', 'xiangqi', 'junqi'].map(kind => 'games/' + kind + '/web/style.css'), 'games/gomoku/web/Board.tsx'];
  const sourceHashes = async () => Object.fromEntries(await Promise.all(sourceFiles.map(async file => [file, createHash('sha256').update(await readFile(path.join(root, file))).digest('hex')])));
  report.sources = await sourceHashes();
  const check = (name, passed, detail) => { report.checks.push({ name, passed: !!passed, detail }); };
  let socket;
  try {
    if (process.platform === 'linux') assert.ok(Buffer.byteLength(path.join(environment.TMPDIR, 'com.google.Chrome.123456/SingletonSocket')) < 108, 'Chromium singleton socket exceeds the Linux pathname limit');
    await browser('open', server.resolvedUrls.local[0]);
    await browser('wait', '--fn', 'window.fullscreenBoardProbe?.ready===true');
    const endpoint = await browser('get', 'cdp-url');
    socket = new WebSocket(typeof endpoint === 'string' ? endpoint : endpoint.cdpUrl);
    await new Promise((accept, reject) => { socket.onopen = accept; socket.onerror = reject; });
    let ordinal = 0;
    const pending = new Map();
    socket.onmessage = ({ data }) => { const value = JSON.parse(data); const waiting = pending.get(value.id); if (!waiting) return; pending.delete(value.id); clearTimeout(waiting.timer); value.error ? waiting.reject(Error(value.error.message)) : waiting.accept(value.result); };
    let attached;
    const call = (method, params = {}, selected = attached) => new Promise((accept, reject) => { const id = ++ordinal; const timer = setTimeout(() => { pending.delete(id); reject(Error('CDP timeout ' + method)); }, 15000); pending.set(id, { accept, reject, timer }); socket.send(JSON.stringify({ id, method, params, sessionId: selected === null ? undefined : selected })); });
    const targets = await call('Target.getTargets', {}, null);
    const target = targets.targetInfos.find(value => value.type === 'page' && value.url.startsWith(server.resolvedUrls.local[0]));
    ({ sessionId: attached } = await call('Target.attachToTarget', { targetId: target.targetId, flatten: true }, null));
    const evaluate = async expression => { const value = await call('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }); assert.ok(!value.exceptionDetails, JSON.stringify(value.exceptionDetails)); return value.result.value; };
    const frames = () => evaluate('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve))))');
    const wait = async expression => { for (let index = 0; index < 100; index++) { try { if (await evaluate(expression)) return; } catch {} await new Promise(resolve => setTimeout(resolve, 30)); } throw Error('UI condition timed out: ' + expression); };
    const viewport = async (width, height, touch) => { await call('Emulation.setDeviceMetricsOverride', { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: touch }); await call('Emulation.setTouchEmulationEnabled', { enabled: touch, maxTouchPoints: touch ? 3 : 1 }); await call('Page.bringToFront'); await frames(); };
    const activate = async (selector, touch) => {
      await evaluate(`document.querySelector(${JSON.stringify(selector)}).scrollIntoView({block:'nearest',inline:'nearest'});true`);
      const point = await evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});const rect=element.getBoundingClientRect();return {x:rect.x+rect.width/2,y:rect.y+rect.height/2}})()`);
      if (touch) { await call('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ ...point, id: 1 }] }); await call('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [{ ...point, id: 1 }] }); }
      else { await call('Input.dispatchMouseEvent', { type: 'mousePressed', ...point, button: 'left', buttons: 1, clickCount: 1 }); await call('Input.dispatchMouseEvent', { type: 'mouseReleased', ...point, button: 'left', buttons: 0, clickCount: 1 }); }
      await frames();
    };
    const capture = async name => { const image = await call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }); await writeFile(path.join(out, name + '.png'), Buffer.from(image.data, 'base64')); };
    const selection = () => evaluate('JSON.stringify([...document.querySelectorAll("[data-controller-group=board] [aria-pressed=true],[data-controller-group=grid] [aria-pressed=true]")].map(element=>({...element.dataset})))');
    const operate = async (current, touch, id) => {
      const before = await evaluate('window.fullscreenBoardProbe.actions.length');
      if (current.kind === 'chess' || current.kind === 'xiangqi') {
        const occupied = await evaluate(`([...document.querySelectorAll('[data-${current.kind}-square][aria-pressed=true]')].map(element=>element.dataset.${current.kind}Square))`);
        const width = current.kind === 'chess' ? 8 : 9;
        const label = value => String.fromCharCode(97 + value % width) + (current.kind === 'chess' ? Math.floor(value / width) + 1 : 9 - Math.floor(value / width));
        const move = current.data.legal.find(move => !move.promotion && !occupied.includes(label(move.from))) ?? current.data.legal[0];
        await activate(`[data-${current.kind}-square="${label(move.from)}"]`, touch);
        await activate(`[data-${current.kind}-square="${label(move.to)}"]`, touch);
        await wait(`!!document.querySelector('[data-${current.kind}-confirm]:not(:disabled)')`);
        await activate(`[data-${current.kind}-confirm]`, touch);
      } else {
        let cell;
        if (current.kind === 'junqi') {
          const piece = current.data.pieces.find(piece => piece.seat === current.you);
          cell = `[data-junqi-node="${piece.pos}"]`;
        } else {
          const occupied = await evaluate(`([...document.querySelectorAll('[data-${current.kind}-cell][aria-pressed=true]')].map(element=>Number(element.dataset.${current.kind}Cell)))`);
          const legal = current.kind === 'gomoku' ? current.data.board.flatMap((value, index) => value ? [] : [index]) : current.data.legal;
          const center = Math.floor(current.data.board.length / 2);
          const point = legal.filter(point => !occupied.includes(point)).sort((a, b) => Math.abs(a - center) - Math.abs(b - center))[0];
          cell = `[data-${current.kind}-cell="${point}"]`;
        }
        if (await evaluate(`document.querySelector('.${current.kind}-board').dataset.overview==='true'`)) {
          const saved = await selection();
          await activate(cell, touch);
          await wait(`document.querySelector('.${current.kind}-board').dataset.overview!=='true'`);
          check(id + ' overview only zooms', await evaluate('window.fullscreenBoardProbe.actions.length') === before);
          if (current.kind === 'gomoku') check(id + ' overview retains selection', await selection() === saved);
        }
        await activate(cell, touch);
        if (current.kind === 'junqi') await activate('[data-controller-action="deploy-ready"]', touch);
        else { await wait(`!!document.querySelector('[data-${current.kind}-confirm]:not(:disabled)')`); await activate(`[data-${current.kind}-confirm]`, touch); }
        await activate(`.${current.kind}-map-tools button:last-of-type`, touch);
      }
      check(id + ' native game action', await evaluate('window.fullscreenBoardProbe.actions.length') === before + 1);
      return selection();
    };
    const fixtures = JSON.parse(await readFile(path.join(root, 'web/tests/fixtures/board-fullscreen.json'), 'utf8'));
    const views = red ? [[1280, 900, false], [320, 760, true]] : [[1280, 900, false], [320, 760, true], [890, 626, true], [1024, 1366, true]];
    for (const current of [...fixtures.cases, { id: 'corner-controls', kind: 'gomoku', collision: true }, { id: 'owned-overlay', kind: 'gomoku', collision: true, owned: true }, { id: 'safe-overlay', kind: 'gomoku', collision: true, safe: true }]) {
      const url = server.resolvedUrls.local[0] + '?case=' + (current.collision ? 'gomoku' : current.id) + (current.collision ? '&collision=1' : '') + (current.owned ? '&owned=1' : '') + (current.safe ? '&safe=1' : '');
      await call('Page.navigate', { url });
      await wait('window.fullscreenBoardProbe?.ready===true');
      await evaluate('window.fullscreenBoardIdentity=document.querySelector("[data-fullscreen-board]").firstElementChild;true');
      for (const [width, height, touch] of views) {
        await viewport(width, height, touch);
        await capture(current.id + '-' + width + '-normal');
        for (const mode of red ? ['page'] : ['native', 'page']) {
          await evaluate(`(()=>{window.fullscreenNativeRequest??=Element.prototype.requestFullscreen;Element.prototype.requestFullscreen=${mode === 'page' ? '()=>Promise.reject(Error("explicit page fallback"))' : 'window.fullscreenNativeRequest'};return true})()`);
          await activate('[data-enter]', touch);
          await wait('!!document.querySelector("[data-room-fullscreen]")');
          await viewport(width, height, touch);
          await frames();
          const geometry = await evaluate(`(()=>{const q=v=>Math.round(v*1000);const rect=e=>{const b=e.getBoundingClientRect();return {left:q(b.left),top:q(b.top),right:q(b.right),bottom:q(b.bottom),width:q(b.width),height:q(b.height)}};const surface=document.querySelector('[data-room-fullscreen]');const board=surface.querySelector('[data-fullscreen-board]');const toolbar=surface.querySelector('.game-fullscreen-toolbar');const game=board.firstElementChild;const map=game.querySelector('.gomoku-map,.weiqi-map,.chess-grid,.xiangqi-map,.junqi-map');const actions=game.querySelector('aside');const play=game.querySelector('[class$="-play-area"]');const status=surface.querySelector('.game-fullscreen-status');const exit=toolbar.querySelector('[data-fullscreen-toggle]');const corner=game.querySelector('[data-game-corner]');const hud=game.querySelector('[data-fullscreen-protected]');return {mode:surface.dataset.roomFullscreen,surface:rect(surface),board:rect(board),toolbar:rect(toolbar),status:status?rect(status):null,exit:rect(exit),game:rect(game),map:map?rect(map):null,actions:actions?rect(actions):null,play:play?rect(play):null,corner:corner?rect(corner):null,hud:hud?rect(hud):null,identity:window.fullscreenBoardIdentity===game,mounts:window.fullscreenBoardProbe.mounts,turn:status?.textContent??'',overflow:document.documentElement.scrollWidth>innerWidth}})()`);
          const id = current.id + '-' + width + '-' + mode;
          report.layouts.push({ id, width, height, touch, geometry });
          check(id + ' mode', geometry.mode === mode, geometry.mode);
          check(id + ' overlay height', geometry.board.top === geometry.surface.top && geometry.board.height === geometry.surface.height, geometry);
          check(id + ' same board', geometry.identity && geometry.mounts === 1);
          check(id + ' no overflow', !geometry.overflow);
          check(id + ' turn retained', geometry.turn.includes('1:59'));
          if (current.owned) check(id + ' explicit CS-style exit retained', geometry.exit.width === 32000 && geometry.exit.height === 32000, geometry.exit);
          if (current.safe) check(id + ' safe inset retained', geometry.exit.top >= 44000 && geometry.exit.right <= width * 1000 - 20000, geometry.exit);
          if (current.collision) {
            const overlaps = (a, b) => a.left < b.right && a.right > b.left && a.top < b.bottom && a.bottom > b.top;
            check(id + ' corner/HUD avoid', !overlaps(geometry.toolbar, geometry.corner) && !overlaps(geometry.toolbar, geometry.hud), geometry);
            await activate('[data-game-corner]', touch);
          } else {
            const inside = box => box.left >= geometry.board.left && box.top >= geometry.board.top && box.right <= geometry.board.right && box.bottom <= geometry.board.bottom;
            check(id + ' game contained', inside(geometry.game), geometry.game);
            check(id + ' actions reachable', inside(geometry.actions) && geometry.actions.height >= 96000, geometry.actions);
            const expected = await evaluate(`(()=>{const game=document.querySelector('[data-fullscreen-board]').firstElementChild;const map=game.querySelector('.gomoku-map,.weiqi-map,.chess-grid,.xiangqi-map,.junqi-map');const viewport=game.querySelector('[class$="-viewport"]');const play=game.querySelector('[class$="-play-area"]');const work=game.querySelector('[class$="-workspace"]');const actions=game.querySelector('aside');const b=document.querySelector('[data-fullscreen-board]').getBoundingClientRect();const p=play.getBoundingClientRect();const a=actions.getBoundingClientRect();const w=work.getBoundingClientRect();const m=map.getBoundingClientRect();const ratio=${current.kind === 'xiangqi' ? '9/10' : current.kind === 'junqi' ? 'parseFloat(map.style.width)/parseFloat(map.style.height)' : '1'};const gap=parseFloat(getComputedStyle(work).gap)||0;const stacked=a.top>=p.bottom-1;const availableWidth=stacked?w.width:w.width-a.width-gap;const extras=[...play.children].filter(e=>e!==map&&e!==viewport).reduce((n,e)=>{const s=getComputedStyle(e);return n+e.getBoundingClientRect().height+(parseFloat(s.marginTop)||0)+(parseFloat(s.marginBottom)||0)},0);const availableHeight=b.bottom-w.top-(stacked?a.height+gap:0)-extras-8;return {ratioMilli:Math.round(m.width/m.height*1000),expectedRatioMilli:Math.round(ratio*1000),actualWidth:Math.round(m.width*1000),idealWidth:Math.round(Math.min(availableWidth,availableHeight*ratio)*1000),viewport:viewport?{width:Math.round(viewport.clientWidth*1000),height:Math.round(viewport.clientHeight*1000)}:null}})()`);
            check(id + ' proportion', Math.abs(expected.ratioMilli - expected.expectedRatioMilli) <= 1, expected);
            check(id + ' maximized board', expected.actualWidth >= expected.idealWidth - 12000, expected);
            if (expected.viewport) check(id + ' initial whole board', geometry.map.width <= expected.viewport.width && geometry.map.height <= expected.viewport.height, expected);
          }
          await capture(id + '-fullscreen');
          const savedSelection = !red && !current.collision ? await operate(current, touch, id) : undefined;
          if (!red) {
            const before = await evaluate('window.fullscreenBoardProbe.actions.length');
            await activate('button[aria-label="保存"]', touch);
            check(id + ' save action retained', await evaluate('window.fullscreenBoardProbe.actions.length') === before + 1);
          }
          await activate('.chat-toggle', touch);
          await wait('!!document.querySelector(".chat-panel textarea")');
          await browser('fill', '.chat-panel textarea', '全屏草稿');
          await activate('button[aria-label="收起聊天"]', touch);
          if (!red && mode === 'page') {
            await evaluate('document.querySelector("[data-fullscreen-board]").focus({preventScroll:true});true');
            await call('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
            await call('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
            check(id + ' Escape forwarded', await evaluate('window.fullscreenBoardProbe.keys.includes("Escape")'));
          } else await activate('button[aria-label="退出全屏"]', touch);
          await wait('!document.querySelector("[data-room-fullscreen]")');
          check(id + ' cleanup', await evaluate('!document.querySelector("[data-platform]").inert&&document.documentElement.style.overflow===""&&window.fullscreenBoardProbe.mounts===1'));
          if (savedSelection !== undefined) check(id + ' selection retained', await selection() === savedSelection);
          await activate('.chat-toggle', touch);
          check(id + ' draft retained', await evaluate('document.querySelector(".chat-panel textarea").value==="全屏草稿"'));
          await activate('button[aria-label="收起聊天"]', touch);
        }
      }
    }
    report.passed = report.checks.every(value => value.passed);
    assert.ok(report.passed, JSON.stringify(report.checks.filter(value => !value.passed).slice(0, 8)));
  } catch (error) { report.error = error.stack ?? String(error); throw error; }
  finally {
    report.sourceChangedDuringRun = Object.entries(await sourceHashes()).filter(([name, hash]) => report.sources[name] !== hash).map(([name]) => name);
    report.passed = report.passed && report.sourceChangedDuringRun.length === 0;
    await writeFile(path.join(out, 'result.json'), JSON.stringify(report, null, 2) + '\n');
    socket?.close();
    await browser('close').catch(() => {});
    await server.close();
    await rm(temporary, { recursive: true, force: true });
    await rm(environment.TMPDIR, { recursive: true, force: true });
  }
});
