// Failure inventory: scripts/controllers/gamepad/SCENARIOS.md (written first).
import test from "node:test";
import assert from "node:assert/strict";
import {
  GamepadSampler,
  gamepadKey,
  defaultGamepadProfile,
  mergeLogicalButtons,
} from "../src/controllers/gamepad.ts";
const keyboard = {
  up: "KeyW",
  down: "KeyS",
  left: "KeyA",
  right: "KeyD",
  select: "KeyF",
  start: "Space",
  a: "KeyK",
  b: "KeyJ",
  turboA: "KeyI",
  turboB: "KeyU",
};
const profile = () => ({ revision: 0, keyboard: { ...keyboard }, turboHz: 10 });
const pad = (id = "Standard Pad", mapping = "standard", index = 0) => ({
  id,
  mapping,
  index,
  connected: true,
  axes: [0, 0],
  buttons: Array(17).fill(0),
});
const sample = (s, p, t = 0, enabled = true) =>
  [...s.sample(p, t, enabled)].sort();
test("standard face buttons, dpad, stick and diagonal produce logical names", () => {
  const p = pad(),
    s = new GamepadSampler(profile());
  assert.deepEqual(sample(s, [p]), []);
  p.buttons[0] = 1;
  p.buttons[12] = 1;
  p.axes[0] = 0.9;
  assert.deepEqual(sample(s, [p], 10), ["b", "right", "up"]);
  p.buttons.fill(0);
  p.axes = [0, 0];
  assert.deepEqual(sample(s, [p], 20), []);
  p.buttons[1] = 1;
  assert.deepEqual(sample(s, [p], 30), ["a"]);
});
test("sticks use dead zones and hysteresis and opposing directions cancel", () => {
  const p = pad(),
    s = new GamepadSampler(profile());
  sample(s, [p]);
  p.axes[0] = 0.2;
  assert.deepEqual(sample(s, [p], 1), []);
  p.axes[0] = 0.4;
  assert.deepEqual(sample(s, [p], 2), ["right"]);
  p.axes[0] = 0.21;
  assert.deepEqual(sample(s, [p], 3), ["right"]);
  p.axes[0] = 0.1;
  assert.deepEqual(sample(s, [p], 4), []);
  p.buttons[14] = p.buttons[15] = 1;
  assert.deepEqual(sample(s, [p], 5), []);
  assert.deepEqual(
    [
      ...mergeLogicalButtons(new Set(["left", "a"]), new Set(["right", "b"])),
    ].sort(),
    ["a", "b"],
  );
});
test("turbo retains phase and does not release a normal held button", () => {
  const p = pad(),
    s = new GamepadSampler(profile());
  sample(s, [p]);
  p.buttons[3] = 1;
  assert.deepEqual(sample(s, [p], 100), ["a"]);
  assert.deepEqual(sample(s, [p], 175), []);
  p.buttons[1] = 1;
  assert.deepEqual(sample(s, [p], 180), ["a"]);
  p.buttons[1] = 0;
  assert.deepEqual(sample(s, [p], 200), ["a"]);
});
test("connect, clear, disable and reconnect require neutral input before rearming", () => {
  const p = pad(),
    s = new GamepadSampler(profile());
  p.buttons[0] = 1;
  assert.deepEqual(sample(s, [p]), []);
  p.buttons[0] = 0;
  sample(s, [p], 1);
  p.buttons[0] = 1;
  assert.deepEqual(sample(s, [p], 2), ["b"]);
  s.clear();
  assert.deepEqual(sample(s, [p], 3), []);
  p.buttons[0] = 0;
  sample(s, [p], 4);
  p.buttons[0] = 1;
  assert.deepEqual(sample(s, [p], 5), ["b"]);
  assert.deepEqual(sample(s, [p], 6, false), []);
  assert.deepEqual(sample(s, [p], 7, true), []);
  p.buttons[0] = 0;
  sample(s, [p], 8);
  p.buttons[0] = 1;
  assert.deepEqual(sample(s, [p], 9), ["b"]);
  assert.deepEqual(sample(s, [], 10), []);
  assert.deepEqual(sample(s, [p], 11), []);
});
test("raw hardware stays silent until configured; each device keeps its own mapping", () => {
  const p = pad("USB Adapter", "", 2),
    other = pad("Second", "", 0),
    cfg = profile();
  const s = new GamepadSampler(cfg);
  sample(s, [p]);
  p.buttons[4] = 1;
  assert.deepEqual(sample(s, [p], 1), []);
  cfg.gamepads = {
    [gamepadKey(p)]: {
      deadzone: 0.35,
      bindings: {
        a: [{ type: "button", index: 4 }],
        left: [{ type: "axis", index: 0, direction: -1 }],
      },
    },
    [gamepadKey(other)]: {
      deadzone: 0.25,
      bindings: { b: [{ type: "button", index: 4 }] },
    },
  };
  cfg.gamepadDevice = gamepadKey(p);
  s.configure(cfg);
  assert.deepEqual(sample(s, [p, other], 2), []);
  p.buttons[4] = 0;
  sample(s, [p, other], 3);
  p.buttons[4] = 1;
  p.axes[0] = -0.8;
  assert.deepEqual(sample(s, [p, other], 4), ["a", "left"]);
  cfg.gamepadDevice = gamepadKey(other);
  s.configure(cfg);
  sample(s, [p, other], 5);
  other.buttons[4] = 1;
  assert.deepEqual(sample(s, [p, other], 6), ["b"]);
  cfg.gamepadDevice = "none";
  s.configure(cfg);
  assert.deepEqual(sample(s, [p, other], 7), []);
});
test("automatic selection sticks to its device, missing/invalid samples cannot latch input", () => {
  const a = pad("A", "standard", 2),
    b = pad("B", "standard", 0),
    s = new GamepadSampler(profile());
  sample(s, [a]);
  sample(s, [b, a], 1);
  b.buttons[0] = 1;
  a.buttons[1] = 1;
  assert.deepEqual(sample(s, [b, a], 2), ["a"]);
  a.connected = false;
  assert.deepEqual(sample(s, [b, a], 3), []);
  b.buttons[0] = 0;
  sample(s, [b], 4);
  b.axes = [NaN, Infinity];
  b.buttons[0] = NaN;
  assert.deepEqual(sample(s, [null, b], 5), []);
});
test("four-button defaults preserve X/Y and put turbo on triggers", () => {
  const p = pad(),
    m = defaultGamepadProfile(p, [
      "up",
      "down",
      "left",
      "right",
      "a",
      "b",
      "x",
      "y",
      "l",
      "r",
      "select",
      "start",
      "turboA",
      "turboB",
    ]);
  assert.deepEqual(m.bindings.x, [{ type: "button", index: 2 }]);
  assert.deepEqual(m.bindings.turboB, [{ type: "button", index: 6 }]);
  assert.notEqual(gamepadKey(p), gamepadKey({ ...p, mapping: "" }));
});
// An empty custom adapter must not occupy automatic selection ahead of a
// usable standard pad after its bindings are cleared in settings.
test("automatic selection skips raw devices with no assigned inputs", () => {
  const empty = pad("Empty adapter", "", 0),
    usable = pad("Usable", "standard", 2);
  const cfg = profile();
  cfg.gamepads = { [gamepadKey(empty)]: { deadzone: 0.25, bindings: {} } };
  const sampler = new GamepadSampler(cfg);
  sample(sampler, [empty, usable]);
  usable.buttons[0] = 1;
  assert.deepEqual(sample(sampler, [empty, usable], 1), ["b"]);
});
test("game-owned action presets do not require FC button names", () => {
  const p = pad(),
    cfg = {
      revision: 0,
      keyboard: { fire: "Space", repeatFire: "KeyF" },
      turboHz: 10,
    };
  const sampler = new GamepadSampler(cfg, {
    standardGamepad: {
      fire: [{ type: "button", index: 7 }],
      repeatFire: [{ type: "button", index: 5 }],
    },
    turboTargets: { repeatFire: "fire" },
  });
  sample(sampler, [p]);
  p.buttons[7] = 1;
  assert.deepEqual(sample(sampler, [p], 5), ["fire"]);
  p.buttons[7] = 0;
  p.buttons[5] = 1;
  assert.deepEqual(sample(sampler, [p], 10), ["fire"]);
  assert.deepEqual(sample(sampler, [p], 85), []);
});

test('opposite navigation directions cancel across input sources', () => {
  assert.deepEqual([...mergeLogicalButtons(['navLeft', 'confirm'], ['navRight'])], ['confirm']);
  assert.deepEqual([...mergeLogicalButtons(['navUp'], ['navDown'])], []);
});

test('charged actions distinguish cancellation from a normal device release', () => {
  const p=pad(),s=new GamepadSampler(profile());
  sample(s,[p]);const initial=s.cancelVersion;
  p.buttons[0]=1;sample(s,[p]);p.buttons[0]=0;sample(s,[p]);
  assert.equal(s.cancelVersion,initial);
  p.buttons[0]=1;sample(s,[p]);sample(s,[]);
  assert.equal(s.cancelVersion,initial+1);
  sample(s,[]);sample(s,[],10,false);
  assert.equal(s.cancelVersion,initial+1,'idle absence is not another interrupted hold');
  p.buttons[0]=0;sample(s,[p]);p.buttons[0]=1;sample(s,[p]);s.configure(profile());
  assert.equal(s.cancelVersion,initial+2);
});

test('continuous axes and triggers preserve values without changing digital buttons', () => {
  const p=pad(),profile={revision:0,keyboard:{left:'KeyA',right:'KeyD',throttle:'KeyW',fire:'KeyJ'},turboHz:10};
  const sampler=new GamepadSampler(profile,{continuous:['left','right','throttle'],standardGamepad:{left:[{type:'axis',index:0,direction:-1}],right:[{type:'axis',index:0,direction:1}],throttle:[{type:'button',index:7}],fire:[{type:'button',index:0}]}});
  sample(sampler,[p]);p.axes[0]=.625;p.buttons[7]=.3;p.buttons[0]=.2;p.pressed=Array(17).fill(false);p.pressed[0]=true;
  assert.deepEqual(sample(sampler,[p],10),['fire','right','throttle']);
  assert.equal(sampler.values.get('right'),.5);assert.equal(sampler.values.get('throttle'),.3);assert.equal(sampler.values.get('fire'),1);
  p.axes[0]=.2;p.buttons[7]=.02;sample(sampler,[p],20);assert.equal(sampler.values.has('right'),false);assert.equal(sampler.values.has('throttle'),false);
  sample(sampler,[],30);assert.equal(sampler.values.size,0);
  sample(sampler,[p],40);assert.equal(sampler.values.size,0,'reconnect held fire remains disarmed');
});
