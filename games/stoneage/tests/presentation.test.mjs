// Before the presentation implementation: early HP/death, stale packets,
// repeated movies, resetting a reconnect clock, mutation of authoritative data,
// and settlement opening before the last visual impact.
import test from "node:test";
import assert from "node:assert/strict";
import { createPlayback } from "../web/presentation.ts";

function snapshot(elapsed = 0, phase = "resolving", version = 3) {
  const before = [
    { seat: 0, body: { hp: 100 }, pet: { hp: 50 }, ride: { hp: 70 } },
    { seat: 1, body: { hp: 100 }, pet: { hp: 50 }, ride: { hp: 70 } },
  ];
  const after = structuredClone(before);
  after[1].body.hp = 40;
  after[1].ride.hp = 20;
  const data = {
    session: "arena",
    phase,
    window: 2,
    round: 1,
    players: after,
    events: [
      {
        id: 1,
        at: 0,
        duration: 480,
        kind: "attack",
        actor: 0,
        target: 2,
        amount: 60,
        rideAmount: 50,
        players: after,
      },
    ],
    playback:
      phase === "resolving"
        ? { startedAt: 10000, duration: 730, before }
        : undefined,
  };
  return {
    room: {
      id: "room",
      kind: "stoneage",
      version,
      status: phase === "finished" ? "finished" : "playing",
      you: 0,
      seats: [],
      game: {
        data,
        finished: phase === "finished",
        winners: phase === "finished" ? [0] : [],
        actors: [],
        log: [],
      },
    },
    clock: { serverNow: 10000 + elapsed, receivedAt: 100 },
  };
}
test("HP and riding HP change at impact; source snapshot is immutable", () => {
  const p = createPlayback(),
    source = snapshot(),
    original = structuredClone(source);
  const first = p.update(source, 100, true);
  assert.equal(first.room.game.data.players[1].body.hp, 100);
  assert.equal(first.blocked, true);
  assert.equal(p.sample(300).room.game.data.players[1].body.hp, 100);
  assert.equal(p.sample(350).room.game.data.players[1].body.hp, 40);
  assert.equal(p.sample(350).room.game.data.players[1].ride.hp, 20);
  assert.deepEqual(source, original);
});
test("duplicate snapshots retain the server timeline and stale versions are ignored", () => {
  const p = createPlayback();
  p.update(snapshot(), 100, true);
  assert.equal(
    p.update(snapshot(100), 200, false).room.game.data.players[1].body.hp,
    100,
  );
  assert.equal(
    p.update(snapshot(-100, "resolving", 2), 350, false).room.game.data
      .players[1].body.hp,
    40,
  );
});
test("reconnect resumes at the authoritative position without replaying earlier hits", () => {
  const p = createPlayback();
  const frame = p.update(snapshot(400), 100, true);
  assert.equal(frame.room.game.data.players[1].body.hp, 40);
  assert.ok(frame.wakeAt <= 430);
  assert.equal(p.sample(431).blocked, false);
});
test("next phase/result waits for the current movie; metadata remains live", () => {
  const p = createPlayback();
  p.update(snapshot(), 100, true);
  const end = snapshot(50, "finished", 4);
  end.room.name = "新名称";
  const held = p.update(end, 150, false);
  assert.equal(held.room.status, "playing");
  assert.equal(held.room.name, "新名称");
  assert.equal(held.room.game.finished, false);
  assert.equal(p.sample(831).room.status, "finished");
});
test("new match and closed room discard the previous movie", () => {
  const p = createPlayback();
  p.update(snapshot(), 100, true);
  const closed = snapshot(10);
  closed.room.status = "closed";
  assert.equal(p.update(closed, 110, false).blocked, false);
  const selection = snapshot(0, "selection", 10);
  selection.room.game.data.session = "new";
  assert.equal(p.update(selection, 200, false).blocked, false);
});
test("native contact timing controls HP rather than half the movement movie", () => {
  const p = createPlayback(),
    source = snapshot();
  source.room.game.data.events[0].duration = 2200;
  source.room.game.data.events[0].impactAt = 1400;
  source.room.game.data.playback.duration = 2450;
  p.update(source, 100, true);
  assert.equal(p.sample(1200).room.game.data.players[1].body.hp, 100);
  assert.equal(p.sample(1499).room.game.data.players[1].body.hp, 100);
  assert.equal(p.sample(1500).room.game.data.players[1].body.hp, 40);
});
