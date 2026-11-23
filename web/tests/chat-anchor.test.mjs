import test from 'node:test';
import assert from 'node:assert/strict';
import { defaultChatAnchor } from '../src/ui/chat-anchor.ts';

const bounds = { left: 0, top: 0, right: 800, bottom: 400 };
const overlaps = (point, rect) => point.x < rect.right && point.x + 44 > rect.left && point.y < rect.bottom && point.y + 44 > rect.top;

test('the default chat corner stays at the bottom when no control occupies it', () => {
  assert.deepEqual(defaultChatAnchor(bounds, 44, 44, []), { x: 756, y: 356 });
});

test('a preferred chat position is retained while it remains clear', () => {
  assert.deepEqual(defaultChatAnchor(bounds, 44, 44, [], { x: 100, y: 120 }), { x: 100, y: 120 });
});

test('a preferred chat position moves away from controls after a layout change', () => {
  const control = { left: 720, top: 180, right: 800, bottom: 260 };
  const preferred = { x: 756, y: 198 };
  const point = defaultChatAnchor(bounds, 44, 44, [control], preferred);
  assert.ok(!overlaps(point, control));
  assert.ok(point.x >= bounds.left && point.y >= bounds.top && point.x + 44 <= bounds.right && point.y + 44 <= bounds.bottom);
});

test('a fire control moves the default chat to the nearest free position', () => {
  const fire = { left: 720, top: 328, right: 800, bottom: 400 };
  const picture = { left: 0, top: 0, right: 650, bottom: 400 };
  const point = defaultChatAnchor(bounds, 44, 44, [fire, picture]);
  assert.deepEqual(point, { x: 756, y: 276 });
  assert.ok(!overlaps(point, fire) && !overlaps(point, picture));
});

test('stacked touch buttons and the game picture remain unblocked', () => {
  const obstacles = [{ left: 720, top: 328, right: 800, bottom: 400 }, { left: 720, top: 276, right: 800, bottom: 320 }, { left: 0, top: 0, right: 650, bottom: 400 }];
  const point = defaultChatAnchor(bounds, 44, 44, obstacles);
  assert.ok(obstacles.every(rect => !overlaps(point, rect)));
  assert.ok(point.x >= bounds.left && point.y >= bounds.top && point.x + 44 <= bounds.right && point.y + 44 <= bounds.bottom);
});

test('an occupied tiny viewport falls back inside its bounds without an error', () => {
  assert.deepEqual(defaultChatAnchor({ left: 0, top: 0, right: 44, bottom: 44 }, 44, 44, [{ left: 0, top: 0, right: 44, bottom: 44 }]), { x: 0, y: 0 });
});

test('CSS subpixel bounds use explicit sixty-fourth-pixel integer coordinates', () => {
  assert.deepEqual(defaultChatAnchor({ left: 0.25, top: 0.5, right: 320.25, bottom: 480.75 }, 44, 44, []), { x: 276.25, y: 436.75 });
});
