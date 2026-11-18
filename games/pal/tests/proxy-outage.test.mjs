// Test first. The network E2E must preserve byte order under an upstream or
// downstream outage, resume the same queued bytes, keep bounded queues, and
// destroy old buffered bytes on a deliberate disconnect before reconnecting.
import test from "node:test";
import assert from "node:assert/strict";
import net from "node:net";
import readline from "node:readline";
import { spawn } from "node:child_process";
import { once } from "node:events";

test("owned TCP outage proxy preserves FIFO and discards disconnected buffers", { timeout: 12000 }, async () => {
  let received = "";
  const peers = new Set();
  const server = net.createServer(socket => {
    peers.add(socket); socket.on("close",()=>peers.delete(socket)); socket.on("error",()=>{});
    socket.on("data",bytes=>{ received+=bytes.toString(); socket.write(bytes); });
  });
  await new Promise(resolve=>server.listen(0,"127.0.0.1",resolve));
  const child=spawn(process.execPath,[new URL("./performance-proxy.mjs",import.meta.url).pathname,`http://127.0.0.1:${server.address().port}`],{stdio:["pipe","pipe","pipe"]});
  const lines=readline.createInterface({input:child.stdout})[Symbol.asyncIterator]();
  const next=async()=>{const line=await lines.next();assert.ok(!line.done);return JSON.parse(line.value);};
  const command=async value=>{child.stdin.write(JSON.stringify(value)+"\n");return next();};
  const wait=async predicate=>{const end=Date.now()+2000;while(!predicate()){assert.ok(Date.now()<end,"TCP predicate timed out");await new Promise(r=>setTimeout(r,10));}};
  let socket, second;
  try {
    const {url}=await next(), address=new URL(url);
    socket=net.connect({host:address.hostname,port:Number(address.port)});socket.on("error",()=>{});await once(socket,"connect");
    let echoed="";socket.on("data",bytes=>echoed+=bytes.toString());
    await command({delayMS:0,pauseUp:true});socket.write("one");
    await new Promise(r=>setTimeout(r,80));assert.equal(received,"");assert.equal(echoed,"");
    await command({pauseUp:false});await wait(()=>echoed==="one");
    await command({pauseDown:true});socket.write("two");await wait(()=>received==="onetwo");
    await new Promise(r=>setTimeout(r,60));assert.equal(echoed,"one");
    await command({pauseDown:false});await wait(()=>echoed==="onetwo");
    await command({pauseUp:true});socket.write("discard-me");
    // A deliberate TCP teardown may report ECONNRESET before close. Observe
    // the terminal close itself; events.once(close) rejects on that expected
    // error and can prevent the reconnect/FIFO assertions from running.
    const closed=new Promise(resolve=>socket.once("close",resolve));await command({pauseUp:false,pauseDown:false,disconnect:true});await closed;
    second=net.connect({host:address.hostname,port:Number(address.port)});second.on("error",()=>{});await once(second,"connect");
    let newEcho="";second.on("data",bytes=>newEcho+=bytes.toString());second.write("new");await wait(()=>newEcho==="new");
    assert.equal(received,"onetwonew","old connection bytes replayed after reconnect");
    await command({pauseUp:true});second.write(Buffer.alloc(6*1024*1024));
    await new Promise(r=>setTimeout(r,100));
    const sample=await command({});
    assert.ok(sample.stats.maxQueueBytes<=4*1024*1024+65536,"outage queue grew without bounds");
  } finally {
    socket?.destroy();second?.destroy();child.kill("SIGTERM");await once(child,"exit");
    for(const peer of peers)peer.destroy();await new Promise(resolve=>server.close(resolve));
  }
});
