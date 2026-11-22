import test from "node:test";
import assert from "node:assert/strict";
import { RoomCommandQueue } from "../src/ui/room-command-queue.ts";

function deferred() {
  let finish;
  const promise = new Promise((resolve) => {
    finish = resolve;
  });
  return { promise, finish };
}

test("a password commit preserves the following click and uses the newly confirmed version", async () => {
  let room = { id: "a", status: "waiting", version: 1 };
  const queue = new RoomCommandQueue(() => room);
  const gate = deferred();
  const events = [];
  const password = queue.enqueue(
    room,
    async (current) => {
      events.push(["password", current.version]);
      await gate.promise;
      room = { ...room, version: 2 };
      return true;
    },
    true,
  );
  const add = queue.enqueue(room, async (current) => {
    events.push(["add", current.version]);
    return true;
  });
  gate.finish();
  assert.equal(await password, true);
  assert.equal(await add, true);
  assert.deepEqual(events, [
    ["password", 1],
    ["add", 2],
  ]);
});

test("failed or unrelated operations do not authorize a stale room version", async () => {
  for (const committed of [false, true]) {
    let room = { id: "a", status: "waiting", version: 1 };
    const queue = new RoomCommandQueue(() => room);
    const before = room;
    await queue.enqueue(before, async () => {
      room = { ...room, version: 2 };
      return committed;
    });
    let sent = false;
    assert.equal(
      await queue.enqueue(before, async () => {
        sent = true;
        return true;
      }),
      false,
    );
    assert.equal(sent, false);
  }
});

test("unknown concurrent changes, room departure and a started match cancel queued intents", async () => {
  for (const next of [
    { id: "b", status: "waiting", version: 2 },
    { id: "a", status: "playing", version: 2 },
    { id: "a", status: "waiting", version: 3 },
    undefined,
  ]) {
    let room = { id: "a", status: "waiting", version: 1 };
    const queue = new RoomCommandQueue(() => room);
    const before = room;
    await queue.enqueue(
      before,
      async () => {
        room = next;
        return true;
      },
      true,
    );
    let sent = false;
    assert.equal(
      await queue.enqueue(before, async () => {
        sent = true;
        return true;
      }),
      false,
    );
    assert.equal(sent, false);
  }
});

test("a thrown operation does not block a later operation on the unchanged room", async () => {
  const room = { id: "a", status: "waiting", version: 1 };
  const queue = new RoomCommandQueue(() => room);
  await assert.rejects(
    queue.enqueue(room, async () => {
      throw Error("offline");
    }),
    /offline/,
  );
  assert.equal(await queue.enqueue(room, async () => true), true);
});

test("a failed password commit cancels already queued intents but a later click can retry", async () => {
  const room = { id: "a", status: "waiting", version: 1 };
  const queue = new RoomCommandQueue(() => room);
  const first = queue.enqueue(room, async () => false, true);
  let sent = false;
  const queued = queue.enqueue(room, async () => {
    sent = true;
    return true;
  });
  assert.equal(await first, false);
  assert.equal(await queued, false);
  assert.equal(sent, false);
  assert.equal(await queue.enqueue(room, async () => true), true);
});
