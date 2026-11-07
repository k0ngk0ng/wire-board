import test from "node:test";
import assert from "node:assert/strict";
import { KeyboardController, sampleLogicalControllerButton } from "../../../web/src/controllers/input.ts";
import { CONTRA_CONTROLLER_MODEL, contraControllerMask } from "./controller.ts";

test("Contra uses the existing classic-2 logical protocol", () => {
  assert.equal(CONTRA_CONTROLLER_MODEL, "classic-2");
  assert.equal(contraControllerMask(["right", "left", "down", "up", "start", "select", "b", "a"]), 255);
  assert.equal(contraControllerMask(["left", "right", "right"]), 3);
  assert.equal(contraControllerMask(["KeyJ", "turboB", "unmapped"]), 0);
});

test("saved platform mappings and profile changes control the game bits", () => {
  const keyboard = new KeyboardController({ revision: 2, keyboard: { right: "KeyL", b: "KeyZ", a: "KeyX" }, turboHz: 12 });
  keyboard.press("KeyJ", 0);
  assert.equal(contraControllerMask(keyboard.sample(0)), 0);
  for (const key of ["KeyL", "KeyZ", "KeyX"]) keyboard.press(key, 0);
  assert.equal(contraControllerMask(keyboard.sample(0)), 0xc1);
  keyboard.release("KeyZ");
  assert.equal(contraControllerMask(keyboard.sample(0)), 0x81);
  keyboard.configure({ revision: 3, keyboard: { b: "KeyN" }, turboHz: 12 });
  assert.equal(contraControllerMask(keyboard.sample(0)), 0);
  keyboard.press("KeyN", 10);
  assert.equal(contraControllerMask(keyboard.sample(10)), 0x40);
  keyboard.clear();
  assert.equal(contraControllerMask(keyboard.sample(20)), 0);
});

test("Turbo timing belongs to the shared platform sampler", () => {
  const sampled = (at) => [sampleLogicalControllerButton("turboB", 0, at, 10)].filter(Boolean);
  assert.equal(contraControllerMask(sampled(0)), 0x40);
  assert.equal(contraControllerMask(sampled(75)), 0);
  assert.equal(contraControllerMask(sampled(100)), 0x40);
});
