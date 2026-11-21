import test from 'node:test';
import assert from 'node:assert/strict';
import { decodeStreamGamePacket, mergeStreamGame } from '../src/realtime-stream.ts';

const state = () => ({ user: { id: 'u' }, serverInstance: 'server', serverNow: 100, room: { id: 'room', kind: 'custom', setupVersion: 3, version: 8, status: 'playing', game: { data: { count: 1 } } } });
const packet = changes => ({ type: 'game', serverInstance: 'server', roomId: 'room', setupVersion: 3, version: 9, status: 'playing', turnDeadline: 0, serverNow: 120, game: { data: { add: 2 } }, ...changes });

test('existing scope and version guards run before resolver or game decoder', () => {
  let resolved = 0;
  const resolver = () => { resolved++; throw Error('obsolete packet must not resolve'); };
  for (const value of [packet({ roomId: 'old' }), packet({ serverInstance: 'old' }), packet({ setupVersion: 2 }), packet({ version: 8 }), packet({ version: 7 })]) assert.equal(decodeStreamGamePacket(state(), value, 20, resolver), undefined);
  assert.equal(decodeStreamGamePacket(undefined, packet(), 20, resolver), undefined);
  assert.equal(resolved, 0);
});

test('decode executes once synchronously and complete packet can feed action scope and deferred React updates', () => {
  let calls = 0;
  const resolver = kind => { assert.equal(kind, 'custom'); return (previous, incoming) => { calls++; return { count: previous.count + incoming.add }; }; };
  let action = state();
  const reactUpdates = [];
  for (const value of [packet(), packet({ version: 10, game: { data: { add: 4 } } })]) {
    const decoded = decodeStreamGamePacket(action, value, 20, resolver);
    action = mergeStreamGame(action, decoded, 20);
    reactUpdates.push(previous => mergeStreamGame(previous, decoded, 20));
  }
  let react = state();
  for (const update of reactUpdates) react = update(react);
  assert.equal(calls, 2);
  assert.equal(action.room.game.data.count, 7);
  assert.deepEqual(react, action);
});

test('missing callback retains every legacy game field and malformed current delta throws for reconnect', () => {
  const raw = packet();
  assert.equal(decodeStreamGamePacket(state(), raw, 20, () => undefined), raw);
  assert.throws(() => decodeStreamGamePacket(state(), raw, 20, () => () => { throw Error('baseline gap'); }), /baseline gap/);
});

test('an ahead HTTP snapshot does not replace the connection-specific delta baseline', () => {
  const current = { ...state(), room: { ...state().room, version: 9, game: { data: { count: 50 } } } };
  const baseline = { serverInstance: 'server', roomId: 'room', setupVersion: 3, data: { count: 1 } };
  const decoded = decodeStreamGamePacket(current, packet({ version: 10 }), 20, () => (previous, incoming) => ({ count: previous.count + incoming.add }), baseline);
  assert.equal(decoded.game.data.count, 3);
  assert.equal(mergeStreamGame(current, decoded, 20).room.game.data.count, 3);
});

test('non-integer setup metadata cannot authorize reuse of a connection baseline', () => {
  const current = { ...state(), room: { ...state().room, setupVersion: 3.5 } };
  const raw = packet({ setupVersion: 3.5 });
  const baseline = { serverInstance: 'server', roomId: 'room', setupVersion: 3.5, data: { count: 1 } };
  const decoded = decodeStreamGamePacket(current, raw, 20, () => previous => ({ baselineMissing: previous === undefined }), baseline);
  assert.equal(decoded.game.data.baselineMissing, true);
});
