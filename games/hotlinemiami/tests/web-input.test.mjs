import test from "node:test";
import assert from "node:assert/strict";
import { InputChannel, emptyInput } from "../web/input.ts";

function binding(extra = {}) {
  return { identity: "room:0", epoch: "epoch", life: 1, receivedSeq: 0, enabled: true, window: 8, ...extra };
}

test("continuous movement and attack do not wait for a previous ACK", () => {
  const sent = [];
  const channel = new InputChannel(a => { sent.push(a); return new Promise(() => {}); });
  channel.update(binding());
  channel.push({ ...emptyInput(), moveX: 1000 });
  channel.push({ ...emptyInput(), moveY: 1000, fire: true });
  assert.equal(sent.length, 2);
  assert.equal(sent[1].seq, 2);
  assert.equal(sent[1].fire, true);
});

test("queued discrete edges survive coalesced movement", async () => {
  const sent = [];
  let release;
  const channel = new InputChannel(a => { sent.push(a); return new Promise(resolve => { release = resolve; }); });
  channel.update(binding({ window: 1 }));
  channel.push({ ...emptyInput(), moveX: 1000 });
  channel.push({ ...emptyInput(), throw: true });
  channel.push({ ...emptyInput(), moveY: 1000 });
  channel.push({ ...emptyInput(), pickup: true });
  release();
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(sent[1].throw, true);
  release();
  await Promise.resolve();
  await Promise.resolve();
  release();
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(sent.some(a => a.pickup), true);
});

test("transport readiness and new life discard stale operations", () => {
  const sent = [];
  const channel = new InputChannel(async a => { sent.push(a); });
  channel.update(binding({ enabled: false, window: 0 }));
  assert.equal(channel.push({ ...emptyInput(), fire: true }), undefined);
  channel.update(binding({ life: 2 }));
  channel.push(emptyInput());
  assert.equal(sent.length, 1);
  assert.equal(sent[0].life, 2);
  assert.equal(sent[0].fire, false);
});

test("late receipt does not lower sequence or show errors in a new identity", async () => {
  const sent = [], errors = [];
  let reject;
  const channel = new InputChannel(a => { sent.push(a); return new Promise((_, r) => { reject = r; }); }, m => errors.push(m));
  channel.update(binding({ receivedSeq: 10 }));
  channel.push(emptyInput());
  const oldReject = reject;
  channel.update(binding({ identity: "new-room", receivedSeq: 0 }));
  channel.push(emptyInput());
  oldReject(Error("stale"));
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(errors.length, 0);
  assert.equal(sent[0].seq, 11);
  assert.equal(sent[1].seq, 1);
});

test("release contains no discrete attack and leaves no queued operations", () => {
  const sent = [];
  const channel = new InputChannel(a => { sent.push(a); return new Promise(() => {}); });
  channel.update(binding());
  channel.push({ ...emptyInput(), fire: true, moveX: 1000 });
  channel.release();
  assert.equal(sent.at(-1).fire, false);
  assert.equal(sent.at(-1).moveX, 0);
  assert.equal(sent.at(-1).throw, false);
  assert.equal(channel.queued, 0);
});

test("each input carries the latest authoritative view frame", () => {
  const sent = [];
  const channel = new InputChannel(async action => { sent.push(action); });
  channel.update(binding({ frame: 64 }));
  channel.push(emptyInput());
  assert.equal(sent[0].viewFrame, 64);
});

test("ownership suspension isolates old errors and in-flight credits from resumed control", async () => {
  const sent = [], pending = [], errors = [];
  const channel = new InputChannel(action => {
    sent.push(action);
    return new Promise((resolve, reject) => pending.push({ resolve, reject }));
  }, message => errors.push(message));
  channel.update(binding({ window: 1 }));
  channel.push({ ...emptyInput(), fire: true });
  channel.push({ ...emptyInput(), throw: true });
  channel.update(binding({ enabled: false, window: 0 }));
  assert.equal(channel.queued, 0);
  channel.update(binding({ enabled: true, window: 1 }));
  channel.push({ ...emptyInput(), moveX: 1000 });
  assert.equal(sent.length, 2);
  assert.equal(sent[1].seq, 3);
  channel.push({ ...emptyInput(), pickup: true });
  pending[0].reject(Error("当前由电脑托管，请先取消托管再手动操作"));
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  assert.deepEqual(errors, []);
  assert.equal(sent.length, 2);
  assert.equal(channel.queued, 1);
  pending[1].resolve();
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(sent.length, 3);
  assert.equal(sent[2].pickup, true);
  assert.equal(sent[2].seq, 4);
});

test("local final-pose preparation completes before the same input reaches the wire", () => {
  const sent = [];
  const channel = new InputChannel(async action => { sent.push({ ...action }); });
  channel.update(binding());
  channel.push(emptyInput(), action => { action.aim = 875; });
  assert.equal(sent.length, 1);
  assert.equal(sent[0].aim, 875);
  assert.equal(sent[0].seq, 1);
});
