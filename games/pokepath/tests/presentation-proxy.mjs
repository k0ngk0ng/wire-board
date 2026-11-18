import net from "node:net";
import readline from "node:readline";

const target = new URL(process.argv[2]);
if (target.protocol !== "http:" || target.hostname !== "127.0.0.1" || !Number.isInteger(Number(target.port)) || Number(target.port) < 1 || Number(target.port) > 65535) throw Error("Expected a local HTTP fixture with an explicit port");
let delayMS = 0;
const peers = new Set();
const pumps = new Set();
const paused = { up: false, down: false };
let stats = { upBytes: 0, downBytes: 0, maxQueueBytes: 0, connections: 0 };

function connectDirection(source, destination, direction) {
  let queue = [];
  let bytes = 0;
  let timer;
  let blocked = false;
  let ended = false;
  const flush = () => {
    clearTimeout(timer);
    timer = undefined;
    if (source.destroyed || destination.destroyed || blocked || paused[direction]) return;
    while (queue.length && queue[0].due <= performance.now()) {
      const entry = queue.shift();
      bytes -= entry.data.length;
      stats[`${direction}Bytes`] += entry.data.length;
      if (!destination.write(entry.data)) {
        blocked = true;
        destination.once("drain", () => { blocked = false; flush(); });
        break;
      }
    }
    if (bytes < 2 * 1024 * 1024) source.resume();
    if (queue.length && !blocked) timer = setTimeout(flush, Math.max(0, queue[0].due - performance.now()));
    if (ended && !queue.length && !blocked) destination.end();
  };
  pumps.add(flush);
  source.on("data", data => {
    queue.push({ data, due: performance.now() + delayMS });
    bytes += data.length;
    stats.maxQueueBytes = Math.max(stats.maxQueueBytes, bytes);
    if (bytes >= 4 * 1024 * 1024) source.pause();
    if (!timer && !blocked) flush();
  });
  source.on("end", () => { ended = true; flush(); });
  source.on("close", () => {
    clearTimeout(timer);
    queue = [];
    pumps.delete(flush);
  });
}

const server = net.createServer({ allowHalfOpen: true }, client => {
  const upstream = net.connect({ host: target.hostname, port: Number(target.port), allowHalfOpen: true });
  stats.connections++;
  for (const [socket, other] of [[client, upstream], [upstream, client]]) {
    peers.add(socket);
    socket.setNoDelay(true);
    socket.on("error", () => { socket.destroy(); other.destroy(); });
    socket.on("close", () => { peers.delete(socket); other.destroy(); });
  }
  connectDirection(client, upstream, "up");
  connectDirection(upstream, client, "down");
});
await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
process.stdout.write(JSON.stringify({ url: `http://127.0.0.1:${server.address().port}` }) + "\n");
const input = readline.createInterface({ input: process.stdin });
input.on("line", line => {
  const value = JSON.parse(line);
  if (value.delayMS !== undefined) {
    if (!Number.isInteger(value.delayMS) || value.delayMS < 0 || value.delayMS > 1000) throw Error("Invalid propagation delay");
    delayMS = value.delayMS;
  }
  for (const [key, direction] of [["pauseUp", "up"], ["pauseDown", "down"]]) {
    if (value[key] !== undefined) {
      if (typeof value[key] !== "boolean") throw Error("Invalid outage flag");
      paused[direction] = value[key];
    }
  }
  if (value.reset) stats = { upBytes: 0, downBytes: 0, maxQueueBytes: 0, connections: 0 };
  if (value.disconnect) for (const socket of peers) socket.destroy();
  for (const pump of pumps) pump();
  process.stdout.write(JSON.stringify({ delayMS, pauseUp: paused.up, pauseDown: paused.down, ...stats }) + "\n");
});
await new Promise(resolve => {
  process.once("SIGTERM", resolve);
  process.once("SIGINT", resolve);
  input.once("close", resolve);
});
input.close();
process.stdin.destroy();
for (const socket of peers) socket.destroy();
await new Promise(resolve => server.close(resolve));
