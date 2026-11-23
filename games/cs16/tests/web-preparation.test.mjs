import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const output = path.join(root, '.local/cs16/verification/web/small-maps');
mkdirSync(output, { recursive: true });
const { build } = createRequire(new URL('../../../web/package.json', import.meta.url))('esbuild');
const compiled = await build({
  stdin: {
    contents: `import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { Options, Preparation } from '../games/cs16/web/Preparation.tsx';
import { Rules } from '../games/cs16/web/Rules.tsx';
import { BuyMenu } from '../games/cs16/web/BuyMenu.tsx';
import { BattleRenderer } from '../games/cs16/web/renderer.ts';
import { TouchControls } from '../games/cs16/web/TouchControls.tsx';
import * as T from 'three';
import { defaultOptions, mapMode, normalizeOptions } from '../games/cs16/web/content.ts';
export { default as catalog } from '../games/cs16/web/catalog.tsx';
export function render(map, waiting = false) {
  const options = normalizeOptions({ ...defaultOptions, map, roundSeconds: 180 }, 10);
  const room = { capacity: 10, you: 0, spectating: false, seats: [{ name: '房主' }] };
  return renderToStaticMarkup(waiting ? React.createElement(Preparation, { room, options, host: true, busy: false, assets: '', prepare: () => {} }) : React.createElement(Options, { value: options, capacity: 10, disabled: false, onChange: () => {} }));
}
export function renderRules() { return renderToStaticMarkup(React.createElement(Rules)); }
export function renderTouch(weapon, zoom = 0) { return renderToStaticMarkup(React.createElement(TouchControls, { disabled: false, weapon, weaponState: { id: weapon, zoom }, useLabel: '使用', changed: () => {}, look: () => {}, native: () => {}, interrupted: () => {}, begin: () => {}, error: () => {} })); }
export function renderRoster() {
  const options = normalizeOptions(defaultOptions, 10);
  const room = { capacity: 10, you: 0, spectating: false, seats: [{ name: '房主', ready: true }, { name: '好友', ready: false }, { name: '电脑', bot: true, ready: true }] };
  return renderToStaticMarkup(React.createElement(Preparation, { room, options, host: true, busy: false, assets: '', prepare: () => {} }));
}
export function renderBuy(map) { return renderToStaticMarkup(React.createElement(BuyMenu, { player: { team: 'ct', money: 16000 }, prices: [], allowed: true, buy: () => {}, close: () => {}, error: '', mode: mapMode(map), category: 8 })); }
export function groundProjection(frames) {
  const renderer = Object.assign(Object.create(BattleRenderer.prototype), { assets: { models: new Map(['w-mp5', 'w-ak47'].map(key => [key, { scene: new T.Group() }])) }, world: new T.Group(), objects: new Map(), brushes: new Map(), smoke: new Map() });
  return frames.map(drops => {
    renderer.dynamic({ drops, hostages: [], grenades: [] });
    return { count: renderer.world.children.length, objects: [...renderer.objects].map(([id, object]) => ({ id, x: Math.round(object.position.x * 256), y: Math.round(object.position.y * 256), z: Math.round(object.position.z * 256), yaw: Math.round(object.rotation.z * 65536 / (Math.PI * 2)) })) };
  });
}`,
    resolveDir: path.join(root, 'web'), sourcefile: 'cs16-preparation-fixture.tsx', loader: 'tsx',
  },
  bundle: true, platform: 'node', format: 'cjs', jsx: 'automatic', write: false,
  nodePaths: [path.join(root, 'web/node_modules')], loader: { '.css': 'empty' },
});
const fixture = path.join(output, 'preparation-fixture.cjs');
writeFileSync(fixture, compiled.outputFiles[0].contents);
const { render, renderRules, renderBuy, groundProjection, renderRoster, catalog, renderTouch } = createRequire(import.meta.url)(fixture);
const evidence = [];
const mapIds = ['de_dust2', 'de_inferno', 'de_aztec', 'cs_assault', 'fy_iceworld', 'cs_bloodstrike'];

test('Counter-Strike has one platform name and ten seats remain a capacity', () => {
  assert.equal(catalog.name, '反恐精英');
  assert.equal(catalog.defaultPlayers, 10);
  assert.equal(catalog.minPlayers, 2);
  assert.notEqual(catalog.requiresFullTable, true);
});

test('right thumb keeps crouch jump and scope while screen aiming fires', () => {
  for (const weapon of ['ak47', 'awp']) {
    const html = renderTouch(weapon);
    const right = html.slice(html.indexOf('class="cs16-touch-actions"'));
    assert.ok(!right.includes('data-cs16-action="fire"'));
    assert.ok(html.includes('data-cs16-fire-area="true"'));
    assert.ok(html.includes('data-cs16-action="flashlight"'));
    assert.ok(right.includes('data-cs16-action="jump"'));
    assert.ok(right.includes('data-cs16-action="crouch"'));
    assert.equal(right.includes('data-cs16-action="secondary"'), weapon === 'awp');
    assert.ok(!right.includes('data-cs16-action="reload"'));
    assert.ok(!right.includes('data-cs16-action="use"'));
    assert.ok(html.includes('class="cs16-touch-utility"'));
    assert.ok(html.includes('data-cs16-controls-menu="true"'));
    evidence.push({ check: 'thumb action hierarchy', weapon, html });
  }
});

test('mobile sniper control identifies the next magnification and then leaving the scope', () => {
  assert.ok(renderTouch('awp', 0).includes('aria-label="开镜"'));
  assert.ok(renderTouch('awp', 40).includes('aria-label="放大"'));
  assert.ok(renderTouch('awp', 10).includes('aria-label="退镜"'));
});

test('teams count actual participants and expose a clear available destination', () => {
  const html = renderRoster();
  assert.ok(html.includes('T · 恐怖分子'));
  assert.ok(html.includes('CT · 反恐精英'));
  assert.ok(html.includes('2 / 5'));
  assert.ok(html.includes('1 / 5'));
  assert.equal((html.match(/class="cs16-seat"/g) ?? []).length, 3);
  assert.ok(!html.includes('等待玩家'));
  assert.ok(/<button[^>]*>加入 CT<\/button>/.test(html));
  assert.ok(!/<button[^>]*disabled[^>]*>加入 CT<\/button>/.test(html));
  evidence.push({ check: 'actual roster and team destination', html });
});

test('create and waiting render six maps with three distinct objective labels', () => {
  for (const waiting of [false, true]) {
    const html = render('fy_iceworld', waiting);
    for (const id of mapIds) assert.ok(html.includes(`data-cs16-map="${id}"`), id);
    assert.equal((html.match(/<span>爆破<\/span>/g) ?? []).length, 3);
    assert.equal((html.match(/<span>人质营救<\/span>/g) ?? []).length, 1);
    assert.equal((html.match(/<span>回合歼灭<\/span>/g) ?? []).length, 2);
    assert.ok(html.includes('冰雪世界'));
    assert.ok(html.includes('血战回廊'));
    evidence.push({ check: 'six-map labels', waiting, html });
  }
});

test('elimination hides ineffective round duration in create and waiting while retaining other settings', () => {
  for (const waiting of [false, true]) {
    for (const map of mapIds) {
      const html = render(map, waiting);
      assert.equal(html.includes('aria-label="每回合时长"'), !['fy_iceworld', 'cs_bloodstrike'].includes(map), `${map}/${waiting}`);
      if (html.includes('aria-label="每回合时长"')) assert.ok(html.includes('<option value="180" selected="">3 分钟</option>'));
      assert.ok(html.includes('aria-label="比赛局数"'));
      assert.ok(html.includes('aria-label="允许队友伤害"'));
      evidence.push({ check: 'effective settings', map, waiting, html });
    }
  }
});

test('only bomb maps offer a CT defuser and rules explain classic elimination rounds', () => {
  for (const map of mapIds) assert.equal(renderBuy(map).includes('data-cs16-buy="defuser"'), ['de_dust2', 'de_inferno', 'de_aztec'].includes(map), map);
  const html = renderRules();
  for (const text of ['冰雪世界', '血战回廊', '回合歼灭', '地面武器', '中央购买区', '下一回合']) assert.ok(html.includes(text), text);
  evidence.push({ check: 'mode-specific equipment and rules', html });
});

test('actual Three dynamic entities retain armoury yaw and clear picked guns before round respawn', () => {
  const drop = { id: 100, x: 256, y: -512, z: 768, weapon: { id: 'mp5navy' }, spawnId: 42, remaining: 2, yaw: 16384 };
  const views = groundProjection([[drop], [{ ...drop, x: 1024, yaw: 32768, remaining: 1 }], [], [{ ...drop, id: 200, yaw: 49152 }], [{ ...drop, id: 200, yaw: undefined }]]);
  assert.deepEqual(views, [
    { count: 1, objects: [{ id: 'drop:100', x: 256, y: -512, z: 768, yaw: 16384 }] },
    { count: 1, objects: [{ id: 'drop:100', x: 1024, y: -512, z: 768, yaw: 32768 }] },
    { count: 0, objects: [] },
    { count: 1, objects: [{ id: 'drop:200', x: 256, y: -512, z: 768, yaw: 49152 }] },
    { count: 1, objects: [{ id: 'drop:200', x: 256, y: -512, z: 768, yaw: 0 }] },
  ]);
  evidence.push({ check: 'Three dynamic ground guns', views });
});

test.after(() => writeFileSync(path.join(output, 'ui-render.json'), JSON.stringify({ scope: 'Actual React server rendering of CS16 Options, Preparation, BuyMenu and Rules plus Three dynamic entities; browser geometry is verified separately', checks: evidence }, null, 2) + '\n'));
