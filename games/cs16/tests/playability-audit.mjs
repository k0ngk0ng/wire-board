import assert from 'node:assert/strict';
import { writeFileSync } from 'node:fs';
import { resolve, sep } from 'node:path';

export async function auditPlayability({ call, evaluate, view, photograph, nativeClick, settle, waitMode, wait, report, output }) {
  assert.ok(resolve(output).startsWith(resolve('.local/cs16') + sep));
  const original = await evaluate('({width:innerWidth,height:innerHeight,ratio:devicePixelRatio})');
  const evidence = { scope: 'functional production-page audit after the unchanged input-latency measurement; Chromium emulation, not physical iPhone or iPad certification', tablets: [] };
  report.playability = evidence;
  const authority = async predicate => {
    for (let attempt = 0; attempt < 300; attempt++) {
      const state = report.network.lastAuthority;
      if (state && !state.partial && predicate(state)) return state;
      await wait(20);
    }
    throw Error('playability authority did not reach the requested state');
  };
  const noError = async () => assert.equal(await evaluate('!!document.querySelector(".cs16-error-banner,[data-cs16-board] [role=alert],.toast.error")'), false, 'game displayed an operation error');
  const point = async selector => evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});if(!element)throw Error('missing touch control');element.scrollIntoView({block:'center',inline:'nearest'});const box=element.getBoundingClientRect();const hit=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);if(!hit||hit!==element&&!element.contains(hit))throw Error('touch control is covered');return {id:0,x:box.x+box.width/2,y:box.y+box.height/2,radiusX:4,radiusY:4,force:1}})()`);
  const tap = async selector => {
    const current = await point(selector);
    await call('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [current] });
    await call('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [current] });
    await settle();
  };
  const key = async (code, value, windowsVirtualKeyCode) => {
    await call('Input.dispatchKeyEvent', { type: 'keyDown', code, key: value, windowsVirtualKeyCode });
    await call('Input.dispatchKeyEvent', { type: 'keyUp', code, key: value, windowsVirtualKeyCode });
  };
  await evaluate('document.exitPointerLock();true');
  await evaluate(`(()=>{const canvas=document.querySelector('[data-cs16-canvas]');window.__cs16PointerDescriptor=Object.getOwnPropertyDescriptor(canvas,'requestPointerLock');Object.defineProperty(canvas,'requestPointerLock',{configurable:true,value:()=>{throw new DOMException('test browser denial','NotAllowedError')}});return true})()`);
  await nativeClick('[data-cs16-canvas]');
  await settle();
  await noError();
  await evaluate(`(()=>{const canvas=document.querySelector('[data-cs16-canvas]');if(window.__cs16PointerDescriptor)Object.defineProperty(canvas,'requestPointerLock',window.__cs16PointerDescriptor);else delete canvas.requestPointerLock;delete window.__cs16PointerDescriptor;return true})()`);
  await nativeClick('[data-cs16-canvas]');
  for (let attempt = 0; attempt < 100 && !await evaluate('!!document.pointerLockElement'); attempt++) await wait(20);
  assert.ok(await evaluate('document.pointerLockElement===document.querySelector("[data-cs16-canvas]")'), 'a real click did not retry pointer lock');
  evidence.pointerDenial = { silent: true, retryAcquired: true };
  await key('Digit2', '2', 50);
  let ready = await authority(state => state.player.alive && state.player.weapon !== 'knife' && state.player.nextAttack <= state.frame);
  if (ready.player.ammo === 0) {
    await key('KeyR', 'r', 82);
    ready = await authority(state => state.player.ammo > 0 && state.player.nextAttack <= state.frame);
  }
  assert.ok(ready.player.ammo > 0, 'shooting visual audit has no ammunition');
  const before = await view();
  const canvas = await evaluate('document.querySelector("[data-cs16-canvas]").getBoundingClientRect().toJSON()');
  const mouse = { x: canvas.x + canvas.width / 2, y: canvas.y + canvas.height / 2, button: 'left', clickCount: 1 };
  try {
    await call('Input.dispatchMouseEvent', { type: 'mousePressed', ...mouse, buttons: 1 });
    const visible = await evaluate(`new Promise(accept=>{const end=performance.now()+1500;const next=()=>{const frame=document.querySelector('[data-cs16-canvas]').__cs16Frame;if(frame.effects?.muzzle&&frame.effects.tracers>${before.effects.tracers})accept(frame);else if(performance.now()>end)accept(null);else requestAnimationFrame(next)};requestAnimationFrame(next)})`);
    assert.ok(visible && visible.effects.impacts > before.effects.impacts, 'a live shot had no muzzle, tracer or wall impact');
    assert.ok(visible.effects.muzzleAttachmentCount > 0, 'muzzle flash is not attached to the original weapon muzzle');
    assert.equal(visible.effects.lightmapColorSpace, 'srgb-linear');
    await photograph('live-shot-effects');
    evidence.shooting = { before: before.effects, visible: visible.effects, lightmap: visible.effects.lightmapColorSpace };
  } finally {
    await call('Input.dispatchMouseEvent', { type: 'mouseReleased', ...mouse, buttons: 0 });
  }
  await authority(state => !state.player.input?.fire);
  await call('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 5 });
  await call('Emulation.setEmitTouchEventsForMouse', { enabled: true, configuration: 'mobile' });
  await settle();
  const previous = (await authority(state => state.player.alive)).player.weapon;
  const alternate = await evaluate(`(()=>{const buttons=[...document.querySelectorAll('[data-cs16-select]')];return buttons.find(button=>button.getAttribute('aria-pressed')!=='true')?.dataset.cs16Select})()`);
  assert.ok(alternate);
  await tap('[data-cs16-select="' + alternate + '"]');
  await authority(state => state.player.weapon === alternate);
  assert.equal(await evaluate('!!document.pointerLockElement'), false, 'first touch selection retained the old mouse lock');
  await tap('[data-cs16-select="' + previous + '"]');
  await authority(state => state.player.weapon === previous);
  evidence.leftWeaponSwitch = { from: previous, selected: alternate, restored: previous, nativeTouch: true, mouseHandoff: true };
  await tap('button[aria-label="全屏"]');
  await waitMode(null);
  await evaluate('window.__cs16PlayabilityCanvas=document.querySelector("[data-cs16-canvas]");true');
  for (const [width, height] of [[466, 678], [678, 466], [626, 890], [890, 626], [744, 1133], [1133, 744], [820, 1180], [1180, 820], [1024, 1366], [1366, 1024]]) {
    await call('Emulation.setDeviceMetricsOverride', { width, height, screenWidth: width, screenHeight: height, deviceScaleFactor: 1, mobile: true, screenOrientation: { type: width > height ? 'landscapePrimary' : 'portraitPrimary', angle: width > height ? 90 : 0 } });
    await settle();
    const geometry = await evaluate(`(()=>{const box=element=>element.getBoundingClientRect().toJSON();const canvas=document.querySelector('[data-cs16-canvas]');return {width:innerWidth,height:innerHeight,canvas:box(canvas),inventory:box(document.querySelector('.cs16-inventory')),move:box(document.querySelector('[data-cs16-stick=move]')),right:[...document.querySelectorAll('.cs16-touch-actions button')].map(button=>({action:button.dataset.cs16Action,rect:box(button)}))}})()`);
    assert.equal(geometry.width, width);
    assert.equal(geometry.height, height);
    assert.ok(geometry.inventory.right <= geometry.canvas.x + geometry.canvas.width / 2, 'weapon switching left the left half');
    assert.ok(geometry.canvas.bottom - geometry.move.bottom <= 64, 'movement stick is too far above the bottom');
    assert.ok(geometry.right.length <= 4 && geometry.right.every(button => ['fire', 'jump', 'crouch', 'secondary'].includes(button.action)), 'right thumb has too many actions');
    await point('[data-cs16-stick=move]');
    await point('[data-cs16-action=fire]');
    await photograph('tablet-' + width + 'x' + height);
    evidence.tablets.push(geometry);
  }
  await tap('[data-cs16-controls-menu]');
  assert.equal(await evaluate('getComputedStyle(document.querySelector(".cs16-touch-more-panel")).touchAction'), 'pan-y');
  await tap('.cs16-touch-more-panel label');
  assert.ok(await evaluate('!!document.querySelector("[data-cs16-fire-area=true]")&&!document.querySelector("[data-cs16-action=fire]")'), 'aim-area fire mode did not replace the fire button');
  const move = await point('[data-cs16-stick=move]');
  const aim = await evaluate(`(()=>{const element=document.querySelector('[data-cs16-fire-area=true]');const box=element.getBoundingClientRect();for(const fraction of [.2,.35,.5]){const x=box.x+box.width*.6;const y=box.y+box.height*fraction;if(document.elementFromPoint(x,y)===element)return {id:1,x,y,radiusX:4,radiusY:4,force:1}}throw Error('aim area covered')})()`);
  await call('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [move, aim] });
  const held = [{ ...move, y: move.y - 24 }, { ...aim, x: aim.x + 12 }];
  await call('Input.dispatchTouchEvent', { type: 'touchMove', touchPoints: held });
  evidence.aimArea = await authority(state => state.player.input?.fire && state.player.input?.moveY > 0);
  await call('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: held });
  await authority(state => !state.player.input?.fire && state.player.input?.moveX === 0 && state.player.input?.moveY === 0);
  await tap('[data-cs16-controls-menu]');
  assert.equal(await evaluate('getComputedStyle(document.querySelector(".cs16-touch-more-panel")).touchAction'), 'pan-y');
  await tap('.cs16-touch-more-panel label');
  assert.ok(await evaluate('!!document.querySelector("[data-cs16-action=fire]")'), 'button fire mode did not return');
  await call('Emulation.setDeviceMetricsOverride', { width: 890, height: 626, screenWidth: 890, screenHeight: 626, deviceScaleFactor: 2, mobile: true, screenOrientation: { type: 'landscapePrimary', angle: 90 } });
  await settle();
  const density = await evaluate(`(()=>{const canvas=document.querySelector('[data-cs16-canvas]');return {ratio:devicePixelRatio,cssWidth:canvas.clientWidth,cssHeight:canvas.clientHeight,width:canvas.width,height:canvas.height}})()`);
  assert.equal(density.width, density.cssWidth * 2);
  assert.equal(density.height, density.cssHeight * 2);
  const start = await view();
  await wait(2000);
  const end = await view();
  evidence.highDensity = { ...density, visibleFPS: (end.drawCount - start.drawCount) * 1000 / (end.at - start.at), physicalDevice: false };
  assert.ok(evidence.highDensity.visibleFPS >= 55, 'high-density tablet drawing fell below 55 FPS');
  const image = await call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
  writeFileSync(output.replace(/\.json$/, '') + '-tablet-dpr2.png', Buffer.from(image.data, 'base64'));
  await noError();
  await tap('button[aria-label="退出全屏"]');
  await waitMode('');
  await call('Emulation.setTouchEmulationEnabled', { enabled: false, maxTouchPoints: 1 });
  await call('Emulation.setEmitTouchEventsForMouse', { enabled: false });
  await call('Emulation.setDeviceMetricsOverride', { width: original.width, height: original.height, screenWidth: original.width, screenHeight: original.height, deviceScaleFactor: original.ratio, mobile: false });
  await settle();
  assert.ok(await evaluate('window.__cs16PlayabilityCanvas===document.querySelector("[data-cs16-canvas]")'), 'tablet transitions remounted the canvas');
  await evaluate('delete window.__cs16PlayabilityCanvas;true');
  evidence.passed = true;
}
