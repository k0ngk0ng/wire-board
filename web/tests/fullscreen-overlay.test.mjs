import test from 'node:test';
import assert from 'node:assert/strict';
import { placeFullscreenOverlay } from '../src/ui/fullscreen-overlay.ts';

const bounds = { left: 0, top: 0, right: 320000, bottom: 760000 };
const size = { width: 32000, height: 32000 };
test('fullscreen overlay starts at its safe top edge without reserving a row', () => {
  assert.deepEqual(placeFullscreenOverlay({ bounds, size, obstacles: [] }), { left: 280000, top: 8000, overlap: false });
});
test('right controls and full-width HUD move only the small overlay', () => {
  assert.deepEqual(placeFullscreenOverlay({ bounds, size, obstacles: [{ left: 250000, top: 0, right: 320000, bottom: 60000 }] }), { left: 212000, top: 8000, overlap: false });
  assert.deepEqual(placeFullscreenOverlay({ bounds, size, obstacles: [{ left: 0, top: 0, right: 320000, bottom: 60000 }] }), { left: 280000, top: 66000, overlap: false });
});
test('left status finds space beside an important HUD and respects integer edges', () => {
  assert.deepEqual(placeFullscreenOverlay({ bounds, size, edge: 'left', obstacles: [{ left: 0, top: 0, right: 120000, bottom: 60000 }] }), { left: 126000, top: 8000, overlap: false });
  const exact = [{ left: 0, top: 0, right: 274000, bottom: 60000 }];
  assert.deepEqual(placeFullscreenOverlay({ bounds, size, obstacles: exact }), { left: 280000, top: 8000, overlap: false });
});
test('occlusion is explicit and invalid geometry cannot be used in comparisons', () => {
  assert.equal(placeFullscreenOverlay({ bounds, size, obstacles: [{ ...bounds }] }).overlap, true);
  assert.throws(() => placeFullscreenOverlay({ bounds, size: { ...size, width: 1.5 }, obstacles: [] }));
  assert.throws(() => placeFullscreenOverlay({ bounds: { ...bounds, bottom: NaN }, size, obstacles: [] }));
});
