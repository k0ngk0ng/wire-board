import test from 'node:test';
import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import { mkdirSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const output = path.join(root, '.local/cs16/verification/classic-ui');
mkdirSync(output, { recursive: true });
const require = createRequire(new URL('../../../web/package.json', import.meta.url));
const { build } = require('esbuild');
const compiled = await build({ stdin: { contents: `import React from 'react'; import {renderToStaticMarkup} from 'react-dom/server'; import {ClassicStatus,HudNumber} from '../games/cs16/web/ClassicHud.tsx'; export function render(props){return renderToStaticMarkup(React.createElement(ClassicStatus,props))} export function number(props){return renderToStaticMarkup(React.createElement(HudNumber,props))}`, resolveDir: path.join(root, 'web'), loader: 'tsx' }, bundle: true, platform: 'node', format: 'cjs', jsx: 'automatic', write: false, nodePaths: [path.join(root, 'web/node_modules')] });
const fixture = path.join(output, 'classic-status.cjs');
writeFileSync(fixture, compiled.outputFiles[0].contents);
const { render, number } = require(fixture);
const sprite = { x: 2, y: 3, width: 32, height: 32, blend: 'additive' };
const hud = { schema: 1, atlas: { file: 'hud/atlas.png' }, width: 256, height: 256, sprites: { 640: { c4: sprite, buyzone: sprite, number_1: sprite, number_0: sprite } } };

test('C4 and buy-zone icons use original sprites and geographic HUD flags', () => {
  const props = { hud, base: '', player: { seat: 0, alive: true }, view: { buyZone: true, plantZone: true, buyAllowed: false, bomb: { carrier: 0 } }, frame: 100 };
  const html = render(props);
  assert.ok(html.includes('data-cs16-status="c4"'));
  assert.ok(html.includes('data-cs16-status="buyzone"'));
  assert.ok(html.includes('data-flashing="true"'));
  assert.ok(html.includes('/cs16/hud/atlas.png'));
  assert.ok(!render({ ...props, player: { seat: 1, alive: true }, view: { ...props.view, buyZone: false } }).includes('data-cs16-status='));
  assert.ok(!render({ ...props, player: { ...props.player, alive: false } }).includes('data-cs16-status='));
});

test('original bitmap digits preserve an accessible numeric value', () => {
  const html = number({ hud, base: '', value: 100, label: '生命 100' });
  assert.ok(html.includes('aria-label="生命 100"'));
  assert.equal((html.match(/data-cs16-sprite="number_/g) ?? []).length, 3);
});
