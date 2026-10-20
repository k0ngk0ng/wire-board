// Failure inventory: two in-flight inputs, a bomb edge replayed behind a held
// direction, an epoch change replaying old sequences, and a D-pad dead zone.
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  LatestInput,
  NEUTRAL,
  isBombKey,
  keyMove,
  padDirection,
} from "../web/input.ts";

test("latest input keeps one request in flight and advances sequence", () => {
  const queue = new LatestInput();
  queue.sync({ scope: "room:epoch:0", epoch: "epoch", seq: 4, enabled: true });
  queue.set({ move: "up", bomb: false });
  const first = queue.next(0);
  assert.equal(first?.seq, 5);
  assert.equal(first?.move, "up");
  queue.set({ move: "down", bomb: true });
  assert.equal(queue.next(10), null, "a second request must not be created");
  queue.complete(first.token);
  const second = queue.next(100);
  assert.equal(second?.seq, 6);
  assert.equal(second?.move, "down");
  assert.equal(second?.bomb, true);
});

test("release is sent immediately and does not replay a later press", () => {
  const queue = new LatestInput();
  queue.sync({ scope: "room:epoch:0", epoch: "epoch", seq: 0, enabled: true });
  queue.set({ move: "left", bomb: true });
  const held = queue.next(0);
  queue.complete(held.token);
  queue.set({ ...NEUTRAL });
  const release = queue.next(20);
  assert.equal(release?.move, "none");
  assert.equal(release?.bomb, false);
  queue.complete(release.token);
  assert.equal(queue.next(30), null, "neutral heartbeat is not queued");
});

test("epoch changes reset local sequence and pending intent", () => {
  const queue = new LatestInput();
  queue.sync({ scope: "room:one:0", epoch: "one", seq: 9, enabled: true });
  queue.set({ move: "right", bomb: true });
  const pending = queue.next(0);
  queue.sync({ scope: "room:two:0", epoch: "two", seq: 2, enabled: true });
  assert.equal(queue.next(10)?.seq, 3);
  assert.equal(queue.value.move, "none");
  assert.equal(pending?.epoch, "one");
});

test("disabled input never emits and keyboard mapping is stable", () => {
  const queue = new LatestInput();
  queue.sync({ scope: "room:epoch:0", epoch: "epoch", seq: 0, enabled: false });
  queue.set({ move: "up", bomb: true });
  assert.equal(queue.next(0), null);
  assert.equal(keyMove("ArrowUp"), "up");
  assert.equal(keyMove("s"), "down");
  assert.equal(keyMove("A"), "left");
  assert.equal(keyMove("d"), "right");
  assert.equal(keyMove("q"), null);
  assert.equal(isBombKey(" "), true);
  assert.equal(isBombKey("j"), true);
  assert.equal(isBombKey("k"), true);
});

test("D-pad rejects the dead zone and selects the dominant axis", () => {
  const rect = { left: 0, top: 0, width: 100, height: 100 };
  assert.equal(padDirection(50, 50, rect), "none");
  assert.equal(padDirection(5, 50, rect), "left");
  assert.equal(padDirection(95, 50, rect), "right");
  assert.equal(padDirection(50, 5, rect), "up");
  assert.equal(padDirection(50, 95, rect), "down");
});

// Refactor failure cases: a short bomb press erased while another request is
// pending; held bomb replayed as multiple edges; releasing a newer direction
// forgets an older held key; opposite-key ordering and blur leave movement stuck.
test("bomb edge survives release behind an in-flight movement", () => {
  const q = new LatestInput();
  q.sync({ scope: "room", epoch: "a", seq: 0, enabled: true });
  q.set({ move: "right", bomb: false });
  const moving = q.next(0);
  q.set({ move: "right", bomb: true });
  q.set({ move: "right", bomb: false });
  q.complete(moving.token);
  assert.equal(q.next(160)?.bomb, true, "a tap must survive request latency");
});

test("one held bomb press emits one edge, not a repeated bomb heartbeat", () => {
  const q = new LatestInput();
  q.sync({ scope: "room", epoch: "a", seq: 0, enabled: true });
  q.set({ move: "right", bomb: true });
  const first = q.next(0);
  q.complete(first.token);
  const heartbeat = q.next(300);
  assert.equal(heartbeat?.bomb, false);
});

test("releasing the newest direction resumes the previous held key; blur clears all", async () => {
  const { HeldDirections } = await import("../web/input.ts");
  const keys = new HeldDirections();
  assert.equal(keys.press("ArrowRight"), "right");
  assert.equal(keys.press("ArrowUp"), "up");
  assert.equal(
    keys.press("ArrowRight"),
    "up",
    "OS key repeats must not steal direction",
  );
  assert.equal(keys.release("ArrowUp"), "right");
  assert.equal(keys.press("a"), "left");
  assert.equal(keys.release("a"), "right");
  keys.clear();
  assert.equal(keys.release("ArrowRight"), "none");
});
