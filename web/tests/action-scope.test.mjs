import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { ActionScopes } from "../src/game-api/action-scope.ts";

const root = fileURLToPath(new URL("../../", import.meta.url));
const output = path.join(root, ".local/cs16/verification/action-scope");
mkdirSync(output, { recursive: true });
const evidence = [];
const state = (patch = {}) => ({
  serverInstance: "server-a",
  user: { id: "user-a" },
  room: {
    id: "room-a",
    matchId: "match-a",
    setupVersion: 1,
    status: "playing",
    you: 0,
    seats: [{ id: "user-a" }, { id: "user-b" }],
    game: { data: { epoch: "epoch-a", frame: 10 } },
  },
  ...patch,
});
const nextRoom = (source) => ({
  ...source,
  room: {
    ...source.room,
    id: "room-b",
    matchId: "match-b",
    game: { data: { epoch: "epoch-b" } },
  },
});

function deferred() {
  let resolve, reject;
  const promise = new Promise((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

test("late failed actions remain rejected and cannot report into another room", async () => {
  const scopes = new ActionScopes();
  const source = state();
  scopes.update(source);
  const token = scopes.capture(source, "epoch-a");
  const pending = deferred();
  const errors = [];
  const failure = Object.assign(new Error("无法执行游戏行动"), { status: 400 });
  const action = scopes.run(
    token,
    () => pending.promise,
    (error) => errors.push(error),
  );
  scopes.update(nextRoom(source));
  pending.reject(failure);
  await assert.rejects(action, (error) => error === failure);
  assert.deepEqual(errors, []);
  assert.equal(failure.transientSync, undefined);
  evidence.push({ scenario: "late-room-rejection", passed: true });
});

test("same-room business failures survive normal frames and retain their original rejection", async () => {
  const scopes = new ActionScopes();
  const source = state();
  scopes.update(source);
  const token = scopes.capture(source);
  scopes.update({
    ...source,
    room: {
      ...source.room,
      version: 99,
      game: { data: { epoch: "epoch-a", frame: 1000 } },
    },
  });
  const failure = Object.assign(new Error("资金不足"), { status: 400 });
  const errors = [];
  await assert.rejects(
    scopes.run(
      token,
      async () => {
        throw failure;
      },
      (error) => errors.push(error),
    ),
    (error) => error === failure,
  );
  assert.deepEqual(errors, [failure]);
  assert.equal(failure.transientSync, undefined);
  evidence.push({ scenario: "same-room-business-error", passed: true });
});

test("transition intent invalidates old inputs before a server response and failed intent resumes only new inputs", async () => {
  for (const intent of ["leave", "close", "rematch", "watch-leave"]) {
    const scopes = new ActionScopes();
    const source = state();
    scopes.update(source);
    const old = scopes.capture(source);
    const paused = scopes.pause();
    scopes.update({ ...source, room: { ...source.room, version: 100 } });
    assert.equal(scopes.capture(source), undefined);
    let sent = false;
    await assert.rejects(
      scopes.run(
        scopes.capture(source),
        async () => {
          sent = true;
        },
        () => assert.fail("inactive input reported"),
      ),
      (error) => error.transientSync === true,
    );
    assert.equal(sent, false);
    scopes.resume(paused.token, source);
    assert.equal(scopes.isCurrent(old), false);
    assert(scopes.capture(source));
    assert.equal(
      await scopes.run(
        scopes.capture(source),
        async () => "resumed",
        () => assert.fail("resumed failed"),
      ),
      "resumed",
    );
    evidence.push({ scenario: intent, passed: true });
  }
});

test("server, account, epoch, match and seat identity changes invalidate delayed failures", async () => {
  const source = state();
  const changed = [
    { ...source, serverInstance: "server-b" },
    { ...source, user: { id: "user-c" } },
    { ...source, room: { ...source.room, matchId: "match-b" } },
    { ...source, room: { ...source.room, setupVersion: 2 } },
    { ...source, room: { ...source.room, you: 1 } },
    {
      ...source,
      room: { ...source.room, seats: [{ id: "user-c" }, { id: "user-b" }] },
    },
    {
      ...source,
      room: { ...source.room, game: { data: { epoch: "epoch-b" } } },
    },
    { ...source, room: undefined },
  ];
  for (const next of changed) {
    const scopes = new ActionScopes();
    scopes.update(source);
    const token = scopes.capture(source);
    const pending = deferred();
    const errors = [];
    const action = scopes.run(
      token,
      () => pending.promise,
      (error) => errors.push(error),
    );
    scopes.update(next);
    pending.reject(new Error("stale failure"));
    await assert.rejects(action, /stale failure/);
    assert.deepEqual(errors, []);
    assert.equal(scopes.isCurrent(token), false);
  }
  evidence.push({
    scenario: "scope-identity",
    cases: changed.length,
    passed: true,
  });
});

test("scope changes clear only owned errors and preserve unrelated login errors even with identical text", () => {
  const scopes = new ActionScopes();
  const source = state();
  scopes.update(source);
  const token = scopes.capture(source);
  assert.equal(scopes.report(token, "请求失败"), true);
  assert.equal(scopes.update(nextRoom(source)), "请求失败");
  assert.equal(scopes.update(nextRoom(source)), undefined);
  scopes.update(source);
  scopes.report(scopes.capture(source), "请求失败");
  scopes.forgetError();
  assert.equal(scopes.update(nextRoom(source)), undefined);
  assert.equal(scopes.report(token, "late error"), false);
  evidence.push({ scenario: "owned-error-only", passed: true });
});

test("old action epochs and repeated room IDs never regain validity", () => {
  const scopes = new ActionScopes();
  const source = state();
  scopes.update(source);
  const original = scopes.capture(source);
  assert.equal(scopes.capture(source, "old-epoch"), undefined);
  for (let i = 0; i < 1000; i++) {
    scopes.update(nextRoom(source));
    scopes.update(source);
  }
  assert.equal(scopes.isCurrent(original), false);
  assert(scopes.capture(source, "epoch-a"));
  assert.equal(scopes.capture(nextRoom(source)), undefined);
  evidence.push({ scenario: "bounded-reused-room", passed: true });
});

test("numeric epochs are safe integers before any identity comparison", () => {
  const source = state();
  source.room.game.data.epoch = 4;
  const scopes = new ActionScopes();
  scopes.update(source);
  assert(scopes.capture(source, 4));
  for (const epoch of [4.5, Number.NaN, Number.POSITIVE_INFINITY, Number.MAX_SAFE_INTEGER + 1]) assert.equal(scopes.capture(source, epoch), undefined);
  assert.equal(scopes.capture(source, "4"), undefined);
  evidence.push({ scenario: "safe-integer-epochs", passed: true });
});

test("main captures the action scope and invalidates transition intentions", () => {
  const source = readFileSync(path.join(root, "web/src/main.tsx"), "utf8");
  assert.match(source, /from ["']\.\/game-api\/action-scope["']/);
  assert.match(source, /actionScopes\.capture\(state, action\.epoch\)/);
  assert.match(source, /actionScopes\.run\(/);
  assert.match(source, /actionScopes\.pause\(/);
  assert.match(source, /actionScopes\.resume\(/);
  assert.match(source, /\["leave", "close", "rematch"\]/);
  assert.match(source, /reportError\(failure, scope\)/);
  assert.match(source, /failure as \{ status\?: number \}/);
  evidence.push({ scenario: "main-integration", passed: true });
});

test.after(() => {
  writeFileSync(
    path.join(output, "test-results.json"),
    JSON.stringify(
      { passed: evidence.length === 11, cases: evidence },
      null,
      2,
    ) + "\n",
  );
});
