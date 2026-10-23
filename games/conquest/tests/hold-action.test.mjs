import test from "node:test";
import assert from "node:assert/strict";
import { createHoldActionController } from "../web/hold-action-controller.ts";

function fixture() {
  let now = 0,
    serial = 0;
  const jobs = new Map(),
    calls = [],
    holding = [];
  let callbacks = {
    onTap: () => calls.push("tap"),
    onHold: () => calls.push("hold"),
  };
  const c = createHoldActionController({
    getCallbacks: () => callbacks,
    onHoldingChange: (v) => holding.push(v),
    timers: {
      set: (fn, ms) => {
        const id = ++serial;
        jobs.set(id, { fn, at: now + ms });
        return id;
      },
      clear: (id) => jobs.delete(id),
    },
  });
  c.configure({ enabled: true, identity: "room:1:province" });
  const tick = (ms) => {
    now += ms;
    for (const [id, j] of [...jobs])
      if (j.at <= now) {
        jobs.delete(id);
        j.fn();
      }
  };
  return { c, calls, holding, jobs, tick, callbacks: (v) => (callbacks = v) };
}
const down = (f) => f.c.pointerDown(1, 20, 20);
const up = (f) => f.c.pointerUp(1, 20, 20, true);

test("a short pointer press executes only from its click", () => {
  const f = fixture();
  assert.equal(down(f), true);
  f.tick(649);
  assert.deepEqual(f.calls, []);
  up(f);
  f.c.click(1);
  f.tick(1000);
  assert.deepEqual(f.calls, ["tap"]);
  assert.deepEqual(f.holding, [true, false]);
});
test("650ms long press fires exactly once and consumes the following click", () => {
  const f = fixture();
  down(f);
  f.tick(650);
  f.tick(5000);
  up(f);
  f.c.click(1);
  assert.deepEqual(f.calls, ["hold"]);
});
test("failed long action never falls back to the short action", async () => {
  const f = fixture();
  f.callbacks({
    onTap: () => f.calls.push("tap"),
    onHold: () => {
      f.calls.push("hold");
      return Promise.reject(new Error("game reports insufficient resources"));
    },
  });
  down(f);
  f.tick(650);
  up(f);
  f.c.click(1);
  await Promise.resolve();
  assert.deepEqual(f.calls, ["hold"]);
});
for (const mode of [
  "move",
  "outside",
  "cancel",
  "lost",
  "blur",
  "hidden",
  "dispose",
])
  test(mode + " cancels a pending hold and its release click", () => {
    const f = fixture();
    down(f);
    if (mode === "move") f.c.pointerMove(1, 29, 20, true);
    else if (mode === "outside") f.c.pointerMove(1, 23, 20, false);
    else if (mode === "lost" || mode === "cancel") f.c.cancelPointer(1);
    else if (mode === "dispose") f.c.dispose();
    else f.c.resetInputs();
    f.tick(1000);
    up(f);
    f.c.click(1);
    assert.deepEqual(f.calls, []);
  });
test("identity changes cannot redirect an old hold or release to another province", () => {
  const f = fixture();
  down(f);
  f.tick(300);
  f.c.configure({ enabled: true, identity: "room:1:other" });
  f.tick(800);
  up(f);
  f.c.click(1);
  assert.deepEqual(f.calls, []);
  down(f);
  up(f);
  f.c.click(1);
  assert.deepEqual(f.calls, ["tap"]);
});
test("identity change after pointerup but before click consumes the old gesture", () => {
  const f = fixture();
  down(f);
  up(f);
  f.c.configure({ enabled: true, identity: "room:2:other" });
  f.c.click(1);
  assert.deepEqual(f.calls, []);
});
test("fast disable and re-enable does not erase long-click suppression", () => {
  const f = fixture();
  down(f);
  f.tick(650);
  f.c.configure({ enabled: false, identity: "room:1:province" });
  f.c.configure({ enabled: true, identity: "room:1:province" });
  up(f);
  f.c.click(1);
  assert.deepEqual(f.calls, ["hold"]);
});
test("disabled short press does not become active when re-enabled before release", () => {
  const f = fixture();
  f.c.configure({ enabled: false, identity: "room:1:province" });
  down(f);
  f.c.configure({ enabled: true, identity: "room:1:province" });
  up(f);
  f.c.click(1);
  assert.deepEqual(f.calls, []);
});
test("a second pointer cancels both gestures and the next fresh gesture works", () => {
  const f = fixture();
  down(f);
  f.c.additionalPointer(2);
  assert.equal(f.c.pointerDown(2, 21, 21, true, 0, true), false);
  f.tick(1000);
  up(f);
  f.c.pointerUp(2, 21, 21, true);
  f.c.click(1);
  assert.deepEqual(f.calls, []);
  down(f);
  up(f);
  f.c.click(1);
  assert.deepEqual(f.calls, ["tap"]);
});
test("secondary mouse buttons and non-primary pointers never invoke an action", () => {
  const f = fixture();
  assert.equal(f.c.pointerDown(1, 0, 0, true, 2), false);
  f.tick(1000);
  assert.deepEqual(f.calls, []);
  assert.equal(f.c.pointerDown(2, 0, 0, false), false);
  f.tick(1000);
  assert.deepEqual(f.calls, []);
});
test("ordinary keyboard activation is independent of stale pointer suppression", () => {
  const f = fixture();
  down(f);
  f.tick(650);
  up(f);
  assert.equal(f.c.keyDown("Enter", false, false), false);
  f.c.click(0);
  f.c.keyUp("Enter");
  assert.deepEqual(f.calls, ["hold", "tap"]);
});
test("Shift+Enter runs hold once and repeat events cannot create more actions", () => {
  const f = fixture();
  assert.equal(f.c.keyDown("Enter", true, false), true);
  assert.equal(f.c.keyDown("Enter", true, true), true);
  assert.equal(f.c.keyDown("Enter", true, false), true);
  f.c.click(0);
  f.c.keyUp("Enter");
  assert.deepEqual(f.calls, ["hold"]);
  assert.equal(f.c.keyDown("Enter", false, false), false);
  f.c.click(0);
  f.c.keyUp("Enter");
  assert.deepEqual(f.calls, ["hold", "tap"]);
});
test("normal Enter repeats are prevented and Space remains a short activation", () => {
  const f = fixture();
  assert.equal(f.c.keyDown("Enter", false, false), false);
  f.c.click(0);
  assert.equal(f.c.keyDown("Enter", false, true), true);
  f.c.keyUp("Enter");
  assert.equal(f.c.keyDown(" ", false, false), false);
  f.c.keyUp(" ");
  f.c.click(0);
  assert.deepEqual(f.calls, ["tap", "tap"]);
});
test("changing identity while Space is held does not activate the new selection", () => {
  const f = fixture();
  f.c.keyDown(" ", false, false);
  f.c.configure({ enabled: true, identity: "new" });
  f.c.keyUp(" ");
  f.c.click(0);
  assert.deepEqual(f.calls, []);
});
test("the timer invokes current callbacks rather than a stale render closure", () => {
  const f = fixture();
  down(f);
  f.tick(300);
  f.callbacks({
    onTap: () => f.calls.push("new tap"),
    onHold: () => f.calls.push("new hold"),
  });
  f.tick(350);
  assert.deepEqual(f.calls, ["new hold"]);
});
test("a cancelled already-queued timer is inert", () => {
  const f = fixture();
  down(f);
  const queued = [...f.jobs.values()][0].fn;
  f.c.configure({ enabled: false, identity: "room:1:province" });
  queued();
  assert.deepEqual(f.calls, []);
});
test("dispose blocks future events; activation supports a clean StrictMode remount", () => {
  const f = fixture();
  down(f);
  f.c.dispose();
  f.tick(700);
  assert.equal(down(f), false);
  f.c.click(0);
  assert.deepEqual(f.calls, []);
  f.c.activate();
  f.c.configure({ enabled: true, identity: "room:1:province" });
  assert.equal(f.c.keyDown("Enter", false, false), false);
  f.c.click(0);
  f.c.keyUp("Enter");
  assert.deepEqual(f.calls, ["tap"]);
});
