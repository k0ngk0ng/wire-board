// Failure inventory precedes implementation: cached old page's first response
// is already new; CSS-only deploy; backend-only version changes; rollbacks;
// overlapping checks, failed requests, malformed data, and late replies after
// teardown. Detection must never reload the page itself.
import test from "node:test";
import assert from "node:assert/strict";
import { VersionMonitor } from "../src/version-update.ts";
const assets = ["/assets/a.css", "/assets/a.js"];
test("compares the loaded assets immediately, including stale first load and CSS-only updates", async () => {
  let info = { version: "v2", assets: ["/assets/b.css", "/assets/a.js"] };
  const updates = [];
  const m = new VersionMonitor(
    assets,
    async () => info,
    (x) => updates.push(x),
  );
  await m.check();
  assert.equal(updates[0].version, "v2");
  await m.check();
  assert.equal(updates.length, 1);
  info = { version: "v1", assets: [...assets].reverse() };
  await m.check();
  assert.equal(updates.at(-1), null);
});
test("remembers the initial release for backend-only updates and handles rollback", async () => {
  let info = { version: "v1", assets };
  const updates = [];
  const m = new VersionMonitor(
    assets,
    async () => info,
    (x) => updates.push(x),
  );
  await m.check();
  assert.equal(updates.length, 0);
  info = { version: "v2", assets };
  await m.check();
  assert.equal(updates[0].version, "v2");
  info = { version: "v1", assets };
  await m.check();
  assert.equal(updates.at(-1), null);
});
test("failed and malformed polls do not erase a pending update or change the baseline", async () => {
  let info = { version: "v1", assets };
  const updates = [];
  const m = new VersionMonitor(
    assets,
    async () => {
      if (info instanceof Error) throw info;
      return info;
    },
    (x) => updates.push(x),
  );
  await m.check();
  info = { version: "v2", assets };
  await m.check();
  info = Error("offline");
  await m.check();
  info = { version: "v3" };
  await m.check();
  assert.equal(updates.length, 1);
  assert.equal(updates[0].version, "v2");
  info = { version: "v3", assets };
  await m.check();
  assert.equal(updates.at(-1).version, "v3");
});
test("checks never overlap and stopped monitors abort and ignore late responses", async () => {
  let resolve,
    calls = 0,
    signal;
  const updates = [];
  const m = new VersionMonitor(
    assets,
    (s) => {
      signal = s;
      calls++;
      return new Promise((r) => (resolve = r));
    },
    (x) => updates.push(x),
  );
  const pending = m.check();
  await m.check();
  assert.equal(calls, 1);
  m.stop();
  assert.ok(signal.aborted);
  resolve({ version: "v2", assets: ["/assets/b.js"] });
  await pending;
  assert.deepEqual(updates, []);
  await m.check();
  assert.equal(calls, 1);
});
