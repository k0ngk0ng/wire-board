import test from "node:test";
import assert from "node:assert/strict";
import { canQuickRematch, quickRematch } from "../src/ui/solo-rematch.ts";

const initial = () => ({ id: "same-room", host: "human", status: "finished", version: 3, seats: [{ id: "human" }, { id: "computer", bot: true }], options: { arena: 2 }, capacity: 2 });

test("quick rematch belongs to the only human host and preserves the room setup", async () => {
  let room = initial();
  const sent = [];
  assert.equal(canQuickRematch(room, "human"), true);
  const result = await quickRematch("same-room", "human", async () => room, async (current, type) => {
    sent.push({ type, version: current.version });
    room = { ...room, status: type === "start" ? "playing" : "waiting", version: room.version + 1 };
  });
  assert.deepEqual(sent, [{ type: "rematch", version: 3 }, { type: "ready", version: 4 }, { type: "start", version: 5 }]);
  assert.equal(result.status, "playing");
  assert.deepEqual(result.options, { arena: 2 });
  assert.equal(result.capacity, 2);
  assert.deepEqual(result.seats, initial().seats);
});

test("spectators, non-hosts, multiple humans and active games cannot quick rematch", () => {
  for (const [room, user] of [[{ ...initial(), spectating: true }, "human"], [initial(), "other"], [{ ...initial(), seats: [{ id: "human" }, { id: "other" }] }, "human"], [{ ...initial(), status: "playing" }, "human"], [{ ...initial(), status: "waiting" }, "human"]]) assert.equal(canQuickRematch(room, user), false);
});

test("a failed request or changed room stops the sequence", async () => {
  let room = initial();
  const sent = [];
  await assert.rejects(quickRematch("same-room", "human", async () => room, async (current, type) => {
    sent.push(type);
    throw Error("offline");
  }), /offline/);
  assert.deepEqual(sent, ["rematch"]);
  sent.length = 0;
  await assert.rejects(quickRematch("same-room", "human", async () => room, async (current, type) => {
    sent.push(type);
    room = { ...room, id: "another-room", status: "waiting" };
  }), /牌桌/);
  assert.deepEqual(sent, ["rematch"]);
});

test("a newly joined human is never readied or started by the shortcut", async () => {
  let room = initial();
  const sent = [];
  await assert.rejects(quickRematch("same-room", "human", async () => room, async (current, type) => {
    sent.push(type);
    room = { ...room, status: "waiting", seats: [...room.seats, { id: "new-human" }] };
  }), /玩家/);
  assert.deepEqual(sent, ["rematch"]);
});
