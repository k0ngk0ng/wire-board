



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

test('immutable CDN entry assets participate in release detection', async () => {
 const old='https://cdn.ichenj.com/bga/web/'+'a'.repeat(40)+'/assets/index-abcdefgh.js';
 const next='https://cdn.ichenj.com/bga/web/'+'b'.repeat(40)+'/assets/index-ijklmnop.js';
 const notices=[];const m=new VersionMonitor([old],async()=>({version:'v2',assets:[next]}),x=>notices.push(x));await m.check();assert.equal(notices[0]?.version,'v2');
});

test("CDN-loaded entry and backwards-compatible asset identities match", async () => {
 const full='https://cdn.ichenj.com/bga/web/'+'a'.repeat(40)+'/assets/index-abcdefgh.js';
 const notices=[];let info={version:'v2',assets:['/assets/index-abcdefgh.js']};
 const m=new VersionMonitor([full],async()=>info,x=>notices.push(x));await m.check();assert.equal(notices.length,0);
 info={version:'v3',assets:['/assets/index-ijklmnop.js']};await m.check();assert.equal(notices[0]?.version,'v3');
});

test("canonical game namespace entries match compatible identities without a false update", async () => {
  const notices = [];
  let info = { version: "v1", assets: ["/assets/index-abcdefgh.js", "/assets/index-abcdefgh.css"] };
  const monitor = new VersionMonitor(["https://cdn.example/bga/_common/index-abcdefgh.js", "https://cdn.example/bga/_common/index-abcdefgh.css"], async () => info, value => notices.push(value));
  await monitor.check();
  assert.deepEqual(notices, []);
  info = { version: "v2", assets: ["/assets/index-ijklmnop.js", "/assets/index-abcdefgh.css"] };
  await monitor.check();
  assert.equal(notices.at(-1)?.version, "v2");
});
