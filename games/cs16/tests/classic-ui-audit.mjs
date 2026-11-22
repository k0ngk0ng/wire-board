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
  evidence.passed = true;
}
