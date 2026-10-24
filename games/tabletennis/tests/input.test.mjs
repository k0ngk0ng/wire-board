// Before implementation: gesture cancellation, body-free input, device scaling,
// stale requests/ACKs, and loss of discrete swings behind in-flight requests.
import test from "node:test";
import assert from "node:assert/strict";
import { classifyStroke, StrokeChannel } from "../web/input.ts";
const binding = (extra = {}) => ({
  identity: "room:user:0:epoch:connection",
  epoch: "epoch",
  rally: 1,
  shot: 2,
  seq: 0,
  enabled: true,
  serverAt: 1000,
  ...extra,
});
test("direct swipes distinguish drive/chop/block and normalize across viewport sizes", () => {
  const a = classifyStroke(40, -80, 150, 400);
  const b = classifyStroke(80, -160, 150, 800);
  assert.deepEqual(a, b);
  assert.equal(a.kind, "drive");
  assert.ok(a.aim > 0 && a.power > 0);
  assert.equal(classifyStroke(-40, 80, 150, 400).kind, "chop");
  assert.equal(classifyStroke(2, 1, 100, 400).kind, "block");
});
test("one outstanding swing per incoming ball; snapshot can acknowledge before response", async () => {
  let resolve;
  const sent = [];
  const c = new StrokeChannel({
    send: (a) => {
      sent.push(a);
      return new Promise((r) => (resolve = r));
    },
  });
  c.update(binding());
  assert.equal(c.swing({ kind: "drive", aim: 0.3, power: 0.6 }, 1020), true);
  assert.equal(c.swing({ kind: "chop", aim: 0, power: 0.5 }, 1021), false);
  assert.equal(sent.length, 1);
  c.update(binding({ seq: 1, shot: 3 }));
  assert.equal(c.swing({ kind: "drive", aim: 0, power: 0.6 }, 1100), true);
  assert.equal(sent[1].seq, 2);
  resolve();
});
test("cancel overtakes a pending swing and carries a higher sequence", () => {
  const sent = [];
  const c = new StrokeChannel({
    send: (a) => {
      sent.push(a);
      return new Promise(() => {});
    },
  });
  c.update(binding());
  c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1020);
  c.cancel();
  assert.equal(sent.length, 2);
  assert.equal(sent[1].type, "cancel");
  assert.ok(sent[1].seq > sent[0].seq);
});
test("old rejection cannot reopen a newer ball or notify a different seat", async () => {
  let reject;
  const errors = [];
  const sent = [];
  const c = new StrokeChannel({
    send: (a) => {
      sent.push(a);
      return new Promise((_, r) => (reject = r));
    },
    onError: (e) => errors.push(e),
  });
  c.update(binding());
  c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1020);
  const oldReject = reject;
  c.update(binding({ identity: "other-user", seq: 0 }));
  c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1030);
  oldReject(Error("old"));
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(errors.length, 0);
  assert.equal(c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1040), false);
});
test("same-ball rejection allows a fresh gesture; stale snapshots cannot lower sequence", async () => {
  const errors = [];
  let calls = 0;
  const c = new StrokeChannel({
    send: async () => {
      calls++;
      throw Error("too early");
    },
    onError: (e) => errors.push(e),
  });
  c.update(binding());
  c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1020);
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(errors.length, 1);
  assert.equal(c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1120), true);
  c.update(binding({ seq: 0, serverAt: 900 }));
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(calls, 2);
});
test("spectator/autoplay input cannot be queued or replayed on re-enable", () => {
  const sent = [];
  const c = new StrokeChannel({
    send: async (a) => {
      sent.push(a);
    },
  });
  c.update(binding({ enabled: false }));
  assert.equal(c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1020), false);
  c.update(binding());
  assert.equal(sent.length, 0);
  c.dispose();
  assert.equal(c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1030), false);
});

test("discarding local input on unmount does not send a command to a closed room", () => {
  const sent = [];
  const c = new StrokeChannel({
    send: (a) => {
      sent.push(a);
      return new Promise(() => {});
    },
  });
  c.update(binding());
  c.swing({ kind: "drive", aim: 0, power: 0.5 }, 1020);
  c.cancel(false);
  assert.equal(sent.length, 1);
});

test("a confirmed whiff allows a fresh swing without an older receipt unlocking a newer one", async () => {
  const sent = [];
  const c = new StrokeChannel({
    send: (a) => {
      sent.push(a);
      return new Promise(() => {});
    },
  });
  c.update(binding());
  c.swing({ kind: "drive", aim: 0, power: 0.5, racketX: -0.8 }, 1020);
  c.update(binding({ seq: 1, retrySeq: 1, serverAt: 1050 }));
  assert.equal(
    c.swing({ kind: "drive", aim: 0, power: 0.5, racketX: 0.6 }, 1060),
    true,
  );
  c.update(binding({ seq: 1, retrySeq: 1, serverAt: 1070 }));
  assert.equal(
    c.swing({ kind: "drive", aim: 0, power: 0.5, racketX: 0.6 }, 1080),
    false,
  );
  assert.equal(sent[1].seq, 2);
});
