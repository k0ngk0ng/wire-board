import test from 'node:test';
import assert from 'node:assert/strict';
import { mergeCS16StreamData } from '../web/stream-delta.ts';

const player = (seat, changes = {}) => ({ seat, team: seat ? 'ct' : 't', character: seat ? 'urban' : 'terror', life: 1, alive: true, visible: true, x: seat * 100, y: 0, z: 0, yaw: 0, pitch: 0, kills: 0, deaths: 0, ...changes });
const full = changes => ({ schema: 1, epoch: 'epoch', frame: 100, serverAt: 1000, startAt: 0, eventId: 10, mapId: 'de_dust2', rulesSHA256: 'a'.repeat(64), options: { map: 'de_dust2', roundSeconds: 120, maxRounds: 10, friendlyFire: false, teams: ['t', 'ct'], characters: ['terror', 'urban'] }, round: 1, phase: 'live', phaseEndsAt: 12000, buyEndsAt: 9000, buyAllowed: true, teamScores: [0, 0], bomb: { carrier: 0, planted: false, dropped: false, x: 0, y: 0, z: 0, explodesAt: 0, defuser: -1, defuseAt: 0 }, players: [player(0, { inventory: [{ id: 'glock18', ammo: 20, reserve: 120 }], appliedSeq: 1 }), player(1)], hostages: [], grenades: [], drops: [{ id: 3, x: 10, y: 0, z: 0 }], brushes: [{ id: 7, open: false, health: 100 }], events: [{ id: 10, type: 'round_start', frame: 0, seat: -1 }], finished: false, you: 0, canAct: true, recover: true, ...changes });
const delta = changes => ({ schema: 2, epoch: 'epoch', baseFrame: 100, baseEventId: 10, frame: 103, serverAt: 1030, eventId: 12, events: [{ id: 12, type: 'shot', seat: 0, frame: 102 }], ...changes });

test('full baseline and nested delta recreate public state without mutating either input', () => {
  const baseline = full();
  const original = structuredClone(baseline);
  const patch = delta({ global: { values: { buyAllowed: false }, objects: { bomb: { values: { planted: true }, remove: ['dropped'] } } }, collections: { players: { upsert: [{ id: 0, patch: { values: { x: 50, appliedSeq: 3, inventory: [{ id: 'glock18', ammo: 19, reserve: 120 }] } } }] }, brushes: { upsert: [{ id: 7, patch: { values: { open: true } } }] } } });
  const patchOriginal = structuredClone(patch);
  assert.deepEqual(mergeCS16StreamData(undefined, baseline), baseline);
  const next = mergeCS16StreamData(baseline, patch);
  assert.equal(next.schema, 1);
  assert.equal(next.frame, 103);
  assert.equal(next.eventId, 12);
  assert.equal(next.players[0].x, 50);
  assert.equal(next.players[0].inventory.length, 1);
  assert.equal(next.players[0].inventory[0].ammo, 19);
  assert.equal(next.brushes[0].open, true);
  assert.equal(next.bomb.planted, true);
  assert.equal(Object.hasOwn(next.bomb, 'dropped'), false);
  assert.equal(next.recover, false);
  assert.deepEqual(next.events, patch.events);
  assert.deepEqual(baseline, original);
  assert.deepEqual(patch, patchOriginal);
});

test('full player replacement removes hidden position and old private fields, while remove deletes entities', () => {
  const baseline = full();
  const hidden = { seat: 1, team: 'ct', character: 'urban', life: 1, alive: true, visible: false, kills: 0, deaths: 0 };
  const next = mergeCS16StreamData(baseline, delta({ collections: { players: { upsert: [{ id: 1, full: hidden }] }, drops: { remove: [3] }, grenades: { upsert: [{ id: 9, full: { id: 9, kind: 'hegrenade', x: 10, y: 0, z: 20 } }] } } }));
  assert.deepEqual(next.players[1], hidden);
  assert.equal('x' in next.players[1], false);
  assert.deepEqual(next.drops, []);
  assert.equal(next.grenades[0].id, 9);
});

test('same-frame metadata updates and visibility-filtered event gaps are valid', () => {
  const next = mergeCS16StreamData(full(), delta({ frame: 100, eventId: 20, events: [], collections: { players: { upsert: [{ id: 0, patch: { values: { receivedSeq: 8 } } }] } } }));
  assert.equal(next.players[0].receivedSeq, 8);
  assert.equal(next.eventId, 20);
  assert.deepEqual(next.events, []);
});

test('missing or mismatched baseline, identity and regressing headers fail explicitly', () => {
  for (const [baseline, patch] of [[undefined, delta()], [full(), delta({ epoch: 'other' })], [full(), delta({ baseFrame: 99 })], [full(), delta({ baseEventId: 9 })], [full(), delta({ frame: 99 })], [full(), delta({ eventId: 9 })]]) assert.throws(() => mergeCS16StreamData(baseline, patch));
  assert.throws(() => mergeCS16StreamData(undefined, full({ brushes: undefined })));
});

test('unknown entity patch, duplicate IDs, ambiguous operations and reserved headers are rejected', () => {
  const patches = [
    delta({ collections: { players: { upsert: [{ id: 4, patch: { values: { x: 1 } } }] } } }),
    delta({ collections: { players: { upsert: [{ id: 0, full: player(0) }, { id: 0, full: player(0) }] } } }),
    delta({ collections: { players: { upsert: [{ id: 0, full: player(0), patch: {} }] } } }),
    delta({ collections: { players: { remove: [0], upsert: [{ id: 0, full: player(0) }] } } }),
    delta({ collections: { players: { upsert: [{ id: 1, full: player(0) }] } } }),
    delta({ collections: { players: { upsert: [{ id: 20, full: player(20) }] } } }),
    delta({ collections: { players: { upsert: [{ id: 1, patch: { values: { visible: false } } }] } } }),
    delta({ collections: { players: { upsert: [{ id: 1, patch: { values: { life: 2 } } }] } } }),
    delta({ global: { values: { schema: 99 } } }),
    delta({ global: { values: { round: 2 }, remove: ['round'] } }),
    delta({ global: { objects: { phase: { values: { invalid: true } } } } }),
    delta({ events: [{ id: 10, type: 'shot' }] }),
    delta({ events: [{ id: 12 }, { id: 11 }] }),
  ];
  for (const patch of patches) assert.throws(() => mergeCS16StreamData(full(), patch));
});

test('prototype keys and non-integer public numeric state cannot pollute the decoder', () => {
  const malicious = delta({ global: JSON.parse('{"values":{"__proto__":{"polluted":true}}}') });
  assert.throws(() => mergeCS16StreamData(full(), malicious));
  assert.throws(() => mergeCS16StreamData(full(), delta({ collections: { players: { upsert: [{ id: 0, patch: { values: { x: 1.25 } } }] } } })));
  assert.equal({}.polluted, undefined);
});
