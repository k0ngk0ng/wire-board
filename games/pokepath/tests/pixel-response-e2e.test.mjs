import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createServer } from "node:http";
import { createHash, randomUUID } from "node:crypto";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { promisify } from "node:util";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const execute = promisify(execFile);
const output = join(root, ".local/pokepath/verification/pixel-response");
const id = randomUUID().slice(0, 8);
const profile = join(output, `profile-${id}`);
const environment = { ...process.env, TMPDIR: join(root, ".local/tmp"), AGENT_BROWSER_SOCKET_DIR: join(root, ".local/ab"), AGENT_BROWSER_PROFILE: profile, AGENT_BROWSER_NO_VERSION_CHECK: "1", AGENT_BROWSER_ARGS: "--mute-audio,--disable-breakpad,--disable-crash-reporter" };
const source = `const options=new URL(location.href).searchParams;window.pixelInputs=[];if(options.get('glyph')==='1')document.querySelector('button').textContent='||';if(options.get('dimming')==='1')document.querySelector('button').style.background='#70ac4c';document.querySelector('button').addEventListener('click',event=>{if(!event.isTrusted)return;window.pixelInputs.push({type:event.type,at:performance.now(),trusted:event.isTrusted});if(options.get('dimming')==='1'){document.querySelector('button').classList.add('feedback');setTimeout(()=>document.querySelector('button').classList.remove('feedback'),150)}if(options.get('freeze')==='1')return;setTimeout(()=>{if(options.get('glyph')==='1')document.querySelector('button').textContent='▶';else document.querySelector('button').style.background=options.get('dimming')==='1'?'#333335':'#d04d28';window.pixelChangedAt=performance.now()},Number(options.get('delay')??0))});document.documentElement.dataset.ready='true';`;
const page = `<!doctype html><html><head><style>body{margin:0;background:#162331}button{position:absolute;left:48px;top:54px;width:128px;height:48px;border:0;background:#1eaf65;color:white;font-size:18px;transition:background-color 180ms linear}button:focus{outline:4px solid white}button:active{filter:brightness(.8);transform:translateY(1px)}button.feedback{filter:brightness(.8)}</style></head><body><button>改变颜色</button><script src="/fixture.mjs"></script></body></html>`;

test("native frame timestamps distinguish input delay from message delivery", { timeout: 90000 }, async () => {
  const { recordNativePixelInput, inspectNativePixelInput } = await import("./pixel-response.mjs");
  await mkdir(profile, { recursive: true });
  const report = { command: "node --test games/pokepath/tests/pixel-response-e2e.test.mjs", scope: "A muted owned calibration page and actual native compositor frames; not a game performance result", passed: false, cases: [] };
  const server = createServer((request, response) => { response.writeHead(200, { "Content-Type": request.url.startsWith("/fixture.mjs") ? "text/javascript" : "text/html; charset=utf-8" }); response.end(request.url.startsWith("/fixture.mjs") ? source : page); });
  await new Promise(resolve => server.listen(0, "127.0.0.1", resolve));
  let name, socket, session;
  const listeners = new Set();
  const frameErrors = [];
  const pending = new Map();
  let serial = 0;
  const browser = async (...args) => {
    const { stdout } = await execute("agent-browser", ["--session", name, ...args, "--json"], { cwd: root, env: environment, timeout: 45000 });
    const result = JSON.parse(stdout);
    assert.ok(result.success, JSON.stringify(result));
    return result.data;
  };
  const call = (method, params = {}, target = session) => new Promise((resolve, reject) => { const id = ++serial; pending.set(id, { resolve, reject }); socket.send(JSON.stringify({ id, method, params, ...(target ? { sessionId: target } : {}) })); });
  const evaluate = async expression => { const result = await call("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true }); if (result.exceptionDetails) throw Error(JSON.stringify(result.exceptionDetails)); return result.result.value; };
  try {
    name = (await execute("agent-browser", ["session", "id", "--scope", "worktree", "--prefix", `pixel-${id}`], { cwd: root, env: environment })).stdout.trim();
    const base = `http://127.0.0.1:${server.address().port}`;
    await browser("open", base);
    await browser("set", "viewport", "400", "240");
    socket = new WebSocket((await browser("get", "cdp-url")).cdpUrl);
    await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
    socket.onmessage = ({ data }) => {
      const message = JSON.parse(data);
      if (message.id) { const request = pending.get(message.id); pending.delete(message.id); if (request) message.error ? request.reject(Error(JSON.stringify(message.error))) : request.resolve(message.result); return; }
      if (message.sessionId === session) {
        if (message.method === "Page.screencastFrame") void call("Page.screencastFrameAck", { sessionId: message.params.sessionId }).catch(error => frameErrors.push(String(error)));
        for (const listener of listeners) listener(message.method, message.params);
      }
    };
    const { targetInfos } = await call("Target.getTargets", {}, null);
    const target = targetInfos.find(value => value.type === "page" && value.url.startsWith(base));
    assert.ok(target);
    ({ sessionId: session } = await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }, null));
    await call("Runtime.enable");
    for (const scenario of [{ name: "immediate", delay: 0 }, { name: "late", delay: 250 }, { name: "frozen", freeze: 1 }, { name: "delivered-late", delay: 0, delivery: 250 }, { name: "glyph-immediate", glyph: 1 }, { name: "glyph-late", glyph: 1, delay: 250 }, { name: "dimming-immediate", dimming: 1 }, { name: "dimming-late", dimming: 1, delay: 250 }, { name: "dimming-frozen", dimming: 1, freeze: 1 }]) {
      await browser("open", `${base}/?${new URLSearchParams({ delay: String(scenario.delay ?? 0), freeze: String(scenario.freeze ?? 0), glyph: String(scenario.glyph ?? 0), dimming: String(scenario.dimming ?? 0) })}`);
      await browser("wait", "--fn", "document.documentElement.dataset.ready==='true'");
      const rect = await evaluate("(()=>{const r=document.querySelector('button').getBoundingClientRect();return{x:r.x,y:r.y,width:r.width,height:r.height}})()");
      const listen = listener => {
        const subscribed = (method, params) => { if (method === "Page.screencastFrame" && scenario.delivery) setTimeout(() => listener(method, params), scenario.delivery); else listener(method, params); };
        listeners.add(subscribed);
        return () => listeners.delete(subscribed);
      };
      const trace = await recordNativePixelInput({ call, evaluate, listen, rect, readInput: () => evaluate("window.pixelInputs.at(-1)") });
      assert.deepEqual(frameErrors, []);
      if (scenario.name === "immediate") {
        const { decodePNG, cropPNG } = await import("./native-png.mjs");
        const png = Buffer.from(trace.frames[0].data, "base64");
        assert.throws(() => decodePNG(png.subarray(0, png.length - 1)));
        const broken = Buffer.from(png);
        broken[0] ^= 1;
        assert.throws(() => decodePNG(broken));
        const corrupt = Buffer.from(png);
        corrupt[corrupt.length - 1] ^= 1;
        assert.throws(() => decodePNG(corrupt));
        const decoded = decodePNG(png);
        for (const bounds of [{ left: -1, top: 0, width: 1, height: 1 }, { left: 0, top: 0, width: decoded.width + 1, height: 1 }, { left: 0, top: 0, width: 0, height: 1 }, { left: 0.5, top: 0, width: 1, height: 1 }]) assert.throws(() => cropPNG(decoded, bounds));
      }
      const result = await inspectNativePixelInput({ trace, evaluate: () => { throw Error("Native PNG analysis must not execute in the measured browser"); }, output, label: scenario.name });
      for (const id of new Set([result.baselineFrame, result.finalFrame])) {
        const frame = result.frames.find(value => value.id === id);
        const source = trace.frames.find(value => value.id === id);
        const expected = await evaluate(`(async()=>{const bytes=Uint8Array.from(atob(${JSON.stringify(source.data)}),character=>character.charCodeAt(0)),image=await createImageBitmap(new Blob([bytes],{type:'image/png'}));const canvas=document.createElement('canvas');canvas.width=image.width;canvas.height=image.height;const context=canvas.getContext('2d');context.drawImage(image,0,0);image.close();const result={};for(const[key,rect]of Object.entries(${JSON.stringify({ crop: frame.bounds, content: frame.contentBounds })})){const values=context.getImageData(rect.left,rect.top,rect.width,rect.height).data;result[key]=Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',values))).map(value=>value.toString(16).padStart(2,'0')).join('')}return result})()`);
        assert.equal(frame.rgbaSHA256, expected.crop);
        assert.equal(frame.contentSHA256, expected.content);
        assert.equal(createHash("sha256").update(await readFile(frame.screenshot.path)).digest("hex"), frame.screenshot.sha256);
      }
      if (scenario.name === "immediate") {
        const sourcePath = join(output, "audit-source.json");
        const sourceBytes = JSON.stringify({ pixelResponse: result }, null, 2) + "\n";
        await writeFile(sourcePath, sourceBytes);
        const { stdout } = await execute(process.execPath, [join(root, "games/pokepath/tests/presentation-audit.mjs"), sourcePath], { cwd: root, env: environment });
        const audit = JSON.parse(await readFile(JSON.parse(stdout).path, "utf8"));
        assert.deepEqual(audit.pixelResponse.firstChangedCaptureMS, result.firstChangedCaptureMS);
        assert.equal(audit.originalReport.sha256, createHash("sha256").update(sourceBytes).digest("hex"));
        assert.deepEqual(audit.pixelResponse.frames.map(frame => frame.contentSHA256), result.frames.map(frame => frame.contentSHA256));
      }
      report.cases.push({ ...scenario, result });
      if (scenario.freeze) assert.equal(result.available, false);
      else {
        assert.ok(result.available, `${scenario.name}: no native result pixels`);
        if (scenario.delay >= 200) assert.ok(result.firstChangedCaptureMS.upper >= 240 && result.firstChangedCaptureMS.upper <= scenario.delay + 100);
        else assert.ok(result.firstChangedCaptureMS.upper <= 100, `${scenario.name}: native result exceeded 100 ms (${result.firstChangedCaptureMS.upper})`);
        if (scenario.delivery) assert.ok(result.firstChangedReceiptMS - result.firstChangedCaptureMS.upper >= 200);
      }
    }
    report.passed = true;
  } finally {
    if (socket) { try { await call("Target.detachFromTarget", { sessionId: session }, null); } catch {} socket.close(); }
    if (name) { try { await browser("close"); } catch {} }
    await new Promise(resolve => server.close(resolve));
    await rm(profile, { recursive: true, force: true });
    await writeFile(join(output, "result.json"), JSON.stringify(report, null, 2) + "\n");
  }
});
