import test from "node:test";
import assert from "node:assert/strict";
import { networkQuality } from "../src/network-quality.ts";
import {
  quietSyncError,
  isTransientSyncError,
} from "../src/game-api/sync-error.ts";
import { mergeNetworkState, mergeStreamState } from "../src/realtime-stream.ts";
test("network packets update peers without moving the game clock or reviving old rooms", () => {
  const before = {
    user: { id: "me" },
    serverInstance: "one",
    serverNow: 100,
    receivedAt: 20,
    room: {
      id: "room",
      version: 7,
      seats: [{ id: "me" }, { id: "peer", bot: false }],
      game: { data: { tick: 12 } },
    },
  };
  const packet = {
    type: "network",
    userId: "me",
    serverInstance: "one",
    serverNow: 200,
    roomId: "room",
    self: { connected: true, rttMs: 30 },
    players: { peer: { connected: true, rttMs: 450 } },
  };
  const next = mergeNetworkState(before, packet);
  assert.equal(next.serverNow, 100);
  assert.equal(next.receivedAt, 20);
  assert.equal(next.room.game, before.room.game);
  assert.equal(next.room.seats[1].network.rttMs, 450);
  assert.equal(mergeNetworkState(next, { ...packet, serverNow: 150 }), next);
  assert.equal(
    mergeNetworkState(next, { ...packet, serverInstance: "old" }),
    next,
  );
  const other = mergeNetworkState(
    { ...next, room: { id: "new", seats: [], version: 1 } },
    { ...packet, serverNow: 300 },
  );
  assert.equal(other.room.id, "new");
  assert.equal(other.room.seats.length, 0);
});
test("signal bars reflect RTT rather than connection alone and zero RTT is valid", () => {
  assert.equal(networkQuality({ connected: true, rttMs: 0 }).bars, 4);
  assert.equal(networkQuality({ connected: true, rttMs: 120 }).bars, 3);
  assert.equal(networkQuality({ connected: true, rttMs: 280 }).bars, 2);
  assert.equal(networkQuality({ connected: true, rttMs: 700 }).bars, 1);
  assert.equal(networkQuality({ connected: true, rttMs: null }).bars, 0);
  assert.equal(networkQuality({ connected: false, rttMs: 10 }).bars, 0);
});
test("late HTTP snapshots cannot overwrite newer measured latency", () => {
  const initial = {
    user: { id: "me" },
    serverInstance: "one",
    serverNow: 100,
    receivedAt: 20,
    room: { id: "room", version: 7, seats: [{ id: "peer" }] },
  };
  const fresh = mergeNetworkState(initial, {
    type: "network",
    userId: "me",
    serverInstance: "one",
    serverNow: 300,
    roomId: "room",
    self: { connected: true, rttMs: 200 },
    players: { peer: { connected: true, rttMs: 450 } },
  });
  const old = {
    ...initial,
    serverNow: 200,
    network: { connected: true, rttMs: 20 },
    room: {
      ...initial.room,
      seats: [{ id: "peer", network: { connected: true, rttMs: 15 } }],
    },
  };
  const next = mergeStreamState(fresh, old, 40);
  assert.equal(next.network.rttMs, 200);
  assert.equal(next.room.seats[0].network.rttMs, 450);
  assert.equal(next.serverNow, 200);
});
test("transient sync rejection stays rejected but has no dismissible message", () => {
  for (const original of [
    Object.assign(Error("局面已更新"), { status: 409 }),
    Object.assign(Error("timeout"), { transientSync: true }),
    new TypeError("Failed to fetch"),
  ]) {
    const error = quietSyncError(original);
    assert.ok(isTransientSyncError(error));
    assert.equal(error.message, "");
  }
  const permission = Object.assign(Error("权限不足"), { status: 403 });
  assert.equal(quietSyncError(permission), permission);
  assert.equal(isTransientSyncError(permission), false);
  const invalid = Object.assign(Error("不能这样出牌"), { status: 400 });
  assert.equal(quietSyncError(invalid), invalid);
});
