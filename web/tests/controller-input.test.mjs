// Before implementation: exact default logical mapping, held-key repeats
// resetting turbo, turbo gaps cancelling a normal button, stuck keys after
// blur/profile changes, and sharing state between independent users/devices.
import test from "node:test";
import assert from "node:assert/strict";
import { KeyboardController } from "../src/controllers/input.ts";
const profile = () => ({
  revision: 0,
  turboHz: 10,
  keyboard: {
    up: "KeyW",
    down: "KeyS",
    left: "KeyA",
    right: "KeyD",
    select: "KeyF",
    start: "Space",
    b: "KeyJ",
    a: "KeyK",
    turboB: "KeyU",
    turboA: "KeyI",
  },
});
test("all default buttons have the requested direction and A/B identity", () => {
  for (const [code, button] of Object.entries({
    KeyW: "up",
    KeyS: "down",
    KeyA: "left",
    KeyD: "right",
    KeyF: "select",
    Space: "start",
    KeyJ: "b",
    KeyK: "a",
    KeyU: "b",
    KeyI: "a",
  })) {
    const input = new KeyboardController(profile());
    input.press(code, 0);
    assert.deepEqual([...input.sample(0)], [button]);
    input.release(code);
    assert.equal(input.sample(1).size, 0);
  }
});
test("turbo begins immediately and holds its phase through OS key repeats", () => {
  const input = new KeyboardController(profile());
  input.press("KeyU", 100);
  assert.ok(input.sample(100).has("b"));
  assert.ok(input.sample(149).has("b"));
  input.press("KeyU", 150);
  assert.equal(input.sample(150).has("b"), false);
  assert.equal(input.sample(199).has("b"), false);
  assert.ok(input.sample(200).has("b"));
  input.release("KeyU");
  assert.equal(input.sample(220).size, 0);
});
test("normal A stays held during a turbo A gap and A+B can coexist", () => {
  const input = new KeyboardController(profile());
  input.press("KeyI", 0);
  input.press("KeyK", 0);
  input.press("KeyJ", 0);
  assert.deepEqual([...input.sample(75)].sort(), ["a", "b"]);
  input.release("KeyK");
  assert.deepEqual([...input.sample(75)], ["b"]);
});
test("clear and profile replacement release old keys without affecting another controller", () => {
  const first = new KeyboardController(profile()),
    second = new KeyboardController(profile());
  first.press("KeyD", 0);
  second.press("KeyW", 0);
  first.clear();
  assert.equal(first.sample(5).size, 0);
  assert.ok(second.sample(5).has("up"));
  first.press("KeyD", 10);
  const changed = profile();
  changed.keyboard.right = "ArrowRight";
  first.configure(changed);
  assert.equal(first.sample(15).size, 0);
  first.press("KeyD", 20);
  assert.equal(first.sample(20).size, 0);
  first.press("ArrowRight", 20);
  assert.ok(first.sample(20).has("right"));
});
test("OS repeats after clearing cannot reactivate a key without a fresh press", () => {
  const input = new KeyboardController(profile());
  input.press("KeyD", 0);
  input.clear();
  input.press("KeyD", 30, true);
  assert.equal(input.sample(30).size, 0);
  input.release("KeyD");
  input.press("KeyD", 40, false);
  assert.ok(input.sample(40).has("right"));
});
