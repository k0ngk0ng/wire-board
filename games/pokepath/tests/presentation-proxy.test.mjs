import test from "node:test";
import assert from "node:assert/strict";
import net from "node:net";
import readline from "node:readline";
import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";

const filename = new URL("./presentation-proxy.mjs", import.meta.url).pathname;
const wait = async predicate => {
  const until = Date.now() + 3000;
  while (!predicate()) {
    assert.ok(Date.now() < until, "TCP condition timed out");
    await new Promise(resolve => setTimeout(resolve, 5));
  }
};

test("presentation proxy rejects nonlocal targets", () => {
  for (const target of ["https://127.0.0.1:1234", "http://example.com:1234", "http://127.0.0.1:0"]) {
    const child = spawnSync(process.execPath, [filename, target], { encoding: "utf8" });
    assert.notEqual(child.status, 0);
  }
});

test("presentation proxy preserves order, propagation delay, outages and clean reconnect", { timeout: 12000 }, async () => {
  const peers = new Set();
  let received = "";
  const server = net.createServer(socket => {
    peers.add(socket);
    socket.on("close", () => peers.delete(socket));
    socket.on("error", () => {});
    socket.on("data", bytes => { received += bytes.toString(); socket.write(bytes); });
  });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  const child = spawn(process.execPath, [filename, `http://127.0.0.1:${server.address().port}`], { stdio: ["pipe", "pipe", "pipe"] });
  const lines = readline.createInterface({ input: child.stdout })[Symbol.asyncIterator]();
  const next = async () => {
    const line = await lines.next();
    assert.equal(line.done, false, "proxy exited before reporting its owned endpoint");
    return JSON.parse(line.value);
  };
  const command = async value => { child.stdin.write(JSON.stringify(value) + "\n"); return next(); };
  let first;
  let second;
  try {
    const target = new URL((await next()).url);
    const open = async () => {
      const socket = net.connect({ host: target.hostname, port: Number(target.port) });
      socket.on("error", () => {});
      await once(socket, "connect");
      return socket;
    };
    first = await open();
    let echoed = "";
    first.on("data", bytes => { echoed += bytes.toString(); });
    await command({ delayMS: 120 });
    const started = performance.now();
    first.write("alpha");
    await new Promise(resolve => setTimeout(resolve, 15));
    first.write("beta");
    await wait(() => echoed === "alphabeta");
    assert.ok(performance.now() - started >= 220, "delay was not applied in both directions");
    assert.ok(performance.now() - started < 600, "propagation delay accumulated per chunk");
    await command({ delayMS: 0, pauseDown: true });
    first.write("gamma");
    await wait(() => received === "alphabetagamma");
    assert.equal(echoed, "alphabeta");
    await command({ pauseDown: false });
    await wait(() => echoed === "alphabetagamma");
    await command({ pauseUp: true });
    first.write("discard");
    await new Promise(resolve => setTimeout(resolve, 20));
    const closed = new Promise(resolve => first.once("close", resolve));
    await command({ disconnect: true, pauseUp: false });
    await closed;
    second = await open();
    let newEcho = "";
    second.on("data", bytes => { newEcho += bytes.toString(); });
    second.write("fresh");
    await wait(() => newEcho === "fresh");
    assert.equal(received, "alphabetagammafresh");
    await command({ pauseUp: true });
    second.write(Buffer.alloc(6 * 1024 * 1024));
    await new Promise(resolve => setTimeout(resolve, 100));
    const stats = await command({});
    assert.ok(stats.maxQueueBytes <= 4 * 1024 * 1024 + 65536, "unbounded queued TCP bytes");
  } finally {
    first?.destroy();
    second?.destroy();
    if (child.exitCode === null) { child.kill("SIGTERM"); await once(child, "exit"); }
    for (const socket of peers) socket.destroy();
    await new Promise(resolve => server.close(resolve));
  }
});
