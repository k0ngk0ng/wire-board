import assert from 'node:assert/strict';

export async function auditClassicInteractions({ call, evaluate, view, nativeClick, wait, report }) {
  const evidence = { scope: 'production Board and trusted Chromium keyboard/mouse events', steps: [] };
  report.classicInteractions = evidence;
  const until = async predicate => {
    for (let attempt = 0; attempt < 150; attempt++) { const value = await predicate(); if (value) return value; await wait(20); }
    throw Error('classic interaction did not reach expected state');
  };
  const key = async (code, key) => {
    const windowsVirtualKeyCode = key.length === 1 ? key.toUpperCase().charCodeAt(0) : key === 'Escape' ? 27 : 0;
    await call('Input.dispatchKeyEvent', { type: 'keyDown', code, key, windowsVirtualKeyCode });
    await call('Input.dispatchKeyEvent', { type: 'keyUp', code, key, windowsVirtualKeyCode });
  };
  await nativeClick('[data-cs16-canvas]');
  await until(() => evaluate('!!document.pointerLockElement'));
  const before = await view();
  await key('KeyB', 'b');
  await until(() => evaluate('!!document.querySelector(".cs16-buy-menu")'));
  assert.ok(await evaluate('document.pointerLockElement===document.querySelector("[data-cs16-canvas]")'), 'numeric buying unnecessarily released mouse lock');
  await key('Digit3', '3');
  assert.ok(await evaluate('!!document.querySelector("[data-cs16-buy=mp5navy]")'), 'B3 did not open the original SMG category');
  await key('Digit0', '0');
  await until(() => evaluate('!document.querySelector(".cs16-buy-menu")&&!!document.pointerLockElement'));
  const closed = await view();
  assert.equal(closed.life, before.life);
  await call('Input.dispatchMouseEvent', { type: 'mouseMoved', x: 400, y: 300, movementX: 24, movementY: 0 });
  await until(async () => (await view()).yaw !== closed.yaw);
  const moved = await view();
  evidence.steps.push({ name: 'numeric menu closes with mouse aim immediately active', before, closed, moved });
  await key('KeyB', 'b');
  await until(() => evaluate('!!document.querySelector(".cs16-buy-menu")'));
  await key('Digit4', '4');
  const team = await evaluate('fetch("/api/state").then(r=>r.json()).then(s=>s.room.game.data.players.find(p=>p.seat===s.room.you).team)');
  assert.ok(await evaluate(`!!document.querySelector('[data-cs16-buy="${team === 'ct' ? 'm4a1' : 'scout'}"][data-cs16-buy-key="3"]')`), 'B43 lost the original team-specific weapon');
  await key('Escape', 'Escape');
  await until(() => evaluate('!document.querySelector(".cs16-buy-menu")&&!!document.pointerLockElement'));
  assert.equal(await evaluate('!!document.querySelector("[data-cs16-board] [role=alert],.toast.error")'), false);
  evidence.steps.push({ name: 'B43 team mapping and Escape restore the lock', team });
  const owner = () => evaluate('fetch("/api/state").then(response=>response.json()).then(state=>{const data=state.room.game.data;return {...data.players.find(player=>player.seat===state.room.you),frame:data.frame,mapId:data.mapId}})');
  const equipment = () => evaluate('[...document.querySelectorAll(".cs16-buy-menu [data-cs16-buy]")].map(button=>({key:Number(button.dataset.cs16BuyKey),item:button.dataset.cs16Buy}))');
  await key('KeyB', 'b');
  await until(() => evaluate('!!document.querySelector(".cs16-buy-menu")'));
  await key('Digit8', '8');
  const numbered = await equipment();
  await key('Digit0', '0');
  await key('KeyO', 'o');
  await until(() => evaluate('!!document.querySelector(".cs16-buy-menu [data-cs16-buy=vest]")'));
  assert.deepEqual(await equipment(), numbered, 'O and B8 do not share the same equipment menu');
  await key('Digit0', '0');
  evidence.steps.push({ name: 'O and B8 share original equipment digits', entries: numbered });
  if ((await owner()).money >= 10000) {
    for (const [digit, item] of [[1, 'vest'], [2, 'vesthelm'], [3, 'flashbang'], [4, 'hegrenade'], [5, 'smokegrenade'], [6, 'nvgs']]) {
      await key('KeyO', 'o');
      await until(() => evaluate(`!!document.querySelector('[data-cs16-buy="${item}"][data-cs16-buy-key="${digit}"]')`));
      await key('Digit' + digit, String(digit));
      await until(() => evaluate('!document.querySelector(".cs16-buy-menu")'));
      const purchased = await until(async () => {
        const player = await owner();
        return (item === 'vest' ? player.armor > 0 : item === 'vesthelm' ? player.helmet : item === 'nvgs' ? player.nightVisionOwned : player.inventory.some(weapon => weapon.id === item)) && player;
      });
      evidence.steps.push({ name: 'native O' + digit + ' purchase', item, money: purchased.money });
    }
    await key('KeyN', 'n');
    await until(async () => Boolean((await owner()).nightVision) && (await view()).effects.nightVision);
    const nightOn = await view();
    assert.ok(await evaluate('document.querySelector("[data-cs16-canvas]").classList.contains("cs16-night-vision")'));
    await wait(350);
    await key('KeyN', 'n');
    await until(async () => !(await owner()).nightVision && !(await view()).effects.nightVision);
    evidence.steps.push({ name: 'owned N toggles original night vision without a new HUD label', on: nightOn.effects, off: (await view()).effects });
    await key('KeyB', 'b');
    await until(() => evaluate('!!document.querySelector(".cs16-buy-menu")'));
    await key('Digit4', '4');
    await key('Digit' + (team === 'ct' ? 3 : 2), team === 'ct' ? '3' : '2');
    const primary = team === 'ct' ? 'm4a1' : 'ak47';
    await until(async () => { const current = await owner(); return current.weapon === primary && current.nextAttack <= current.frame; });
    const held = async (label, duration) => {
      const state = await owner();
      const before = await view();
      await evaluate(`(()=>{const seat=${state.seat};const samples=[];const voices=new Map();const initial=new Set();let active=true;for(const element of document.querySelectorAll('[data-spatial-voices]'))for(const voice of JSON.parse(element.dataset.spatialVoices||'[]'))initial.add(voice.playbackID);const collect=()=>{for(const element of document.querySelectorAll('[data-spatial-voices]'))for(const voice of JSON.parse(element.dataset.spatialVoices||'[]'))if(voice.key==='cs16:shot:'+seat&&voice.started&&!initial.has(voice.playbackID)&&!voices.has(voice.playbackID))voices.set(voice.playbackID,{id:voice.playbackID,at:Math.round(performance.now())})};const observer=new MutationObserver(collect);observer.observe(document.body,{attributes:true,subtree:true,attributeFilter:['data-spatial-voices']});const frame=()=>{if(!active)return;const value=document.querySelector('[data-cs16-canvas]').__cs16Frame;samples.push({at:Math.round(performance.now()),back:Math.round(value.effects.weaponBack*1000),pitch:Math.round(value.effects.cameraPitch*1000000),shots:value.effects.weaponShots});requestAnimationFrame(frame)};requestAnimationFrame(frame);window.__cs16HeldFire={stop:()=>{active=false;observer.disconnect();return {samples,voices:[...voices.values()]}}};return true})()`);
      const canvas = await evaluate('document.querySelector("[data-cs16-canvas]").getBoundingClientRect().toJSON()');
      const mouse = { x: canvas.x + canvas.width / 2, y: canvas.y + canvas.height / 2, button: 'left', clickCount: 1 };
      try {
        await call('Input.dispatchMouseEvent', { type: 'mousePressed', ...mouse, buttons: 1 });
        await wait(duration);
      } finally {
        await call('Input.dispatchMouseEvent', { type: 'mouseReleased', ...mouse, buttons: 0 });
      }
      const after = await until(async () => { const current = await owner(); return !current.input.fire && current; });
      await wait(120);
      const shown = await view();
      const trace = await evaluate('(()=>{const result=window.__cs16HeldFire.stop();delete window.__cs16HeldFire;return result})()');
      const shots = state.ammo - after.ammo;
      const animations = shown.effects.weaponShots - before.effects.weaponShots;
      const pulses = trace.samples.filter((sample, index) => index > 0 && sample.back - trace.samples[index - 1].back > 100).length;
      evidence.steps.push({ name: label, durationMS: duration, weapon: state.weapon, authorityShots: shots, animations, pulses, sounds: trace.voices, frames: trace.samples });
      return { shots, animations, pulses, trace };
    };
    const automatic = await held('native continuous automatic fire', 1200);
    assert.ok(automatic.shots >= 10 && automatic.shots <= 15, 'held rifle did not keep the original automatic cadence');
    assert.ok(automatic.animations >= automatic.shots - 1 && automatic.animations <= automatic.shots + 1, 'automatic shots lost or duplicated their model feedback');
    assert.ok(automatic.pulses >= automatic.shots - 2, 'held rifle did not kick on every shot');
    assert.ok(automatic.trace.voices.length >= automatic.shots - 1 && automatic.trace.voices.length <= automatic.animations, 'automatic shots lost or duplicated sound starts');
    assert.ok(new Set(automatic.trace.samples.map(sample => sample.pitch)).size >= 8, 'authority recoil did not move the view while holding fire');
    await key('Digit2', '2');
    await until(async () => { const current = await owner(); return current.weapon !== primary && current.nextAttack <= current.frame; });
    const semi = await held('native held semi-automatic pistol', 450);
    assert.equal(semi.shots, 1, 'holding a semi-automatic pistol became automatic');
    assert.equal(semi.animations, 1);
    assert.equal(semi.trace.voices.length, 1);
    await evaluate('document.exitPointerLock();true');
    await until(() => evaluate('!document.pointerLockElement'));
    await call('Emulation.setTouchEmulationEnabled', { enabled: true, maxTouchPoints: 3 });
    await call('Emulation.setEmitTouchEventsForMouse', { enabled: true, configuration: 'mobile' });
    const tap = async selector => {
      const geometry = await evaluate(`(()=>{const element=document.querySelector(${JSON.stringify(selector)});const box=element.getBoundingClientRect();const hit=document.elementFromPoint(box.x+box.width/2,box.y+box.height/2);return {box:box.toJSON(),hit:hit?.outerHTML.slice(0,800),visible:hit===element||element.contains(hit),viewport:{width:innerWidth,height:innerHeight,scrollY},point:{id:0,x:box.x+box.width/2,y:box.y+box.height/2,radiusX:4,radiusY:4,force:1}}})()`);
      evidence.steps.push({ name: 'touch target geometry', selector, ...geometry });
      assert.ok(geometry.visible, 'night vision touch control is covered: ' + JSON.stringify(geometry));
      const point = geometry.point;
      await call('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [point] });
      await call('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
    };
    await tap('[data-cs16-controls-menu]');
    await until(() => evaluate('!!document.querySelector(".cs16-touch-more-panel [data-cs16-action=nightVision]")'));
    assert.equal(await evaluate('!!document.querySelector(".cs16-touch-actions [data-cs16-action=nightVision]")'), false);
    await tap('.cs16-touch-more-panel [data-cs16-action=nightVision]');
    await until(async () => Boolean((await owner()).nightVision) && (await view()).effects.nightVision);
    await wait(350);
    await tap('.cs16-touch-more-panel [data-cs16-action=nightVision]');
    await until(async () => !(await owner()).nightVision && !(await view()).effects.nightVision && !(await owner()).input.nightVision);
    await tap('[data-cs16-controls-menu]');
    await call('Emulation.setTouchEmulationEnabled', { enabled: false, maxTouchPoints: 1 });
    await call('Emulation.setEmitTouchEventsForMouse', { enabled: false });
    await nativeClick('[data-cs16-canvas]');
    await until(() => evaluate('!!document.pointerLockElement'));
    evidence.steps.push({ name: 'owned night vision works through touch More without adding a right-side action', released: true });
  }
  if ((await owner()).mapId === 'cs_bloodstrike') assert.equal(await evaluate('!!document.querySelector(".cs16-match-hud")'), false, 'elimination objective text or unused clock remained in the battlefield');
  evidence.passed = true;
}
