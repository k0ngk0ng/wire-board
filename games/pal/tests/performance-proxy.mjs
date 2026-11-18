// Bounded FIFO TCP propagation-delay proxy. Chunks in one direction share a
// due-time queue; latency is not multiplied by serially sleeping per chunk.
import net from "node:net";
import readline from "node:readline";
const target = new URL(process.argv[2]);
if (target.protocol !== "http:" || target.hostname !== "127.0.0.1") throw Error("Expected local fixture URL");
let delayMS = 0;
const sockets = new Set();
const flushers = new Set();
const paused = { uploaded: false, downloaded: false };
let stats = { uploaded: 0, downloaded: 0, maxQueueBytes: 0, errors: 0 };
function pump(source, destination, counter) {
  const queue = [];
  let queued = 0, timer, blocked = false, ended = false;
  const flush = () => {
    clearTimeout(timer); timer = undefined;
    if (destination.destroyed || blocked || paused[counter]) return;
    while (queue.length && queue[0].due <= performance.now()) {
      const { bytes } = queue.shift(); queued -= bytes.length;
      stats[counter] += bytes.length;
      if (!destination.write(bytes)) {
        blocked = true;
        destination.once("drain", () => { blocked = false; flush(); });
        break;
      }
    }
    if (queued < 2 * 1024 * 1024 && !source.destroyed) source.resume();
    if (queue.length && !blocked) timer = setTimeout(flush, Math.max(0, queue[0].due - performance.now()));
    if (ended && !queue.length && !blocked) destination.end();
  };
  flushers.add(flush);
  source.on("data", bytes => {
    queue.push({ bytes, due: performance.now() + delayMS }); queued += bytes.length;
    stats.maxQueueBytes = Math.max(stats.maxQueueBytes, queued);
    if (queued >= 4 * 1024 * 1024) source.pause();
    if (!timer && !blocked) flush();
  });
  source.on("end", () => { ended = true; flush(); });
  source.on("close", () => { clearTimeout(timer); queue.length = 0; flushers.delete(flush); });
}
const server = net.createServer(client => {
  const upstream = net.connect({ host: target.hostname, port: Number(target.port) });
  sockets.add(client); sockets.add(upstream);
  client.setNoDelay(true); upstream.setNoDelay(true);
  for (const [socket, peer] of [[client, upstream], [upstream, client]]) {
    socket.on("error", () => { stats.errors++; socket.destroy(); peer.destroy(); });
    socket.on("close", () => sockets.delete(socket));
  }
  pump(client, upstream, "uploaded"); pump(upstream, client, "downloaded");
});
await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
console.log(JSON.stringify({ url: `http://127.0.0.1:${server.address().port}` }));
const input = readline.createInterface({ input: process.stdin });
input.on("line", line => {
  const value = JSON.parse(line);
  if (value.delayMS !== undefined) {
    if (!Number.isInteger(value.delayMS) || value.delayMS < 0 || value.delayMS > 1000) throw Error("Invalid propagation delay");
    delayMS = value.delayMS;
  }
  for (const [key, direction] of [["pauseUp", "uploaded"], ["pauseDown", "downloaded"]]) {
    if (value[key] !== undefined) {
      if (typeof value[key] !== "boolean") throw Error("Invalid outage flag");
      paused[direction] = value[key];
    }
  }
  if (value.reset) stats = { uploaded: 0, downloaded: 0, maxQueueBytes: 0, errors: 0 };
  if (value.disconnect) for (const socket of sockets) socket.destroy();
  for (const flush of flushers) flush();
  console.log(JSON.stringify({ delayMS, pauseUp: paused.uploaded, pauseDown: paused.downloaded, stats }));
});
await new Promise(resolve => { process.once("SIGTERM", resolve); process.once("SIGINT", resolve); });
input.close(); process.stdin.destroy();
for (const socket of sockets) socket.destroy();
await new Promise(resolve => server.close(resolve));
