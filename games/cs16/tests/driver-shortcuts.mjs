import assert from "node:assert/strict";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const { endpoint } = JSON.parse(readFileSync(0, "utf8"));
const output = resolve(".local/cs16/verification/driver-shortcuts.json");
mkdirSync(resolve(".local/cs16/verification"), { recursive: true });
const socket = new WebSocket(endpoint);
const pending = new Map();
let ordinal = 0;
let session;
await new Promise((accept, reject) => { socket.onopen = accept; socket.onerror = reject; });
socket.onmessage = event => {
  const message = JSON.parse(event.data);
  const selected = pending.get(message.id);
  if (!selected) return;
  pending.delete(message.id);
  clearTimeout(selected.timer);
  message.error ? selected.reject(message.error) : selected.accept(message.result);
};
const call = (method, params = {}, sessionId = session) => new Promise((accept, reject) => {
  const id = ++ordinal;
  const timer = setTimeout(() => reject(Error(method)), 10000);
  pending.set(id, { accept, reject, timer });
  socket.send(JSON.stringify({ id, method, params, sessionId }));
});
const evaluate = async expression => {
  const result = await call("Runtime.evaluate", { expression, returnByValue: true });
  assert.ok(!result.exceptionDetails, JSON.stringify(result.exceptionDetails));
  return result.result.value;
};
const report = { scope: "plain textarea without application code, raw CDP key mapping against native edit command" };
try {
  report.browser = await call("Browser.getVersion", {}, undefined);
  const targets = await call("Target.getTargets", {}, undefined);
  const target = targets.targetInfos.find(item => item.type === "page" && item.url === "about:blank");
  assert.ok(target);
  session = (await call("Target.attachToTarget", { targetId: target.targetId, flatten: true }, undefined)).sessionId;
  await call("Page.bringToFront");
  await evaluate("document.body.innerHTML='<textarea id=editor></textarea>';window.keys=[];document.addEventListener('keydown',event=>queueMicrotask(()=>window.keys.push({code:event.code,trusted:event.isTrusted,ctrl:event.ctrlKey,meta:event.metaKey,prevented:event.defaultPrevented})));true");
  report.platform = await evaluate("navigator.platform");
  report.cases = [];
  for (const commands of [undefined, ["selectAll"]]) {
    await evaluate("(()=>{const editor=document.querySelector('#editor');editor.value='cs16 edit shortcut';editor.focus();editor.setSelectionRange(editor.value.length,editor.value.length);window.keys=[];return true})()");
    const mac = /Mac/.test(report.platform);
    const modifier = { code: mac ? "MetaLeft" : "ControlLeft", key: mac ? "Meta" : "Control", windowsVirtualKeyCode: mac ? 91 : 17, modifiers: mac ? 4 : 2 };
    await call("Input.dispatchKeyEvent", { type: "keyDown", ...modifier });
    await call("Input.dispatchKeyEvent", { type: "keyDown", code: "KeyA", key: "a", windowsVirtualKeyCode: 65, modifiers: modifier.modifiers, commands });
    await call("Input.dispatchKeyEvent", { type: "keyUp", code: "KeyA", key: "a", windowsVirtualKeyCode: 65, modifiers: modifier.modifiers });
    await call("Input.dispatchKeyEvent", { type: "keyUp", ...modifier, modifiers: 0 });
    const observed = await evaluate("(()=>{const editor=document.querySelector('#editor');return {selected:editor.selectionEnd-editor.selectionStart,length:editor.value.length,keys:window.keys,focused:document.activeElement===editor}})()");
    report.cases.push({ commands: commands ?? [], ...observed });
  }
  report.nativeCommandPassed = report.cases[1].selected === report.cases[1].length;
  assert.ok(report.nativeCommandPassed);
} finally {
  if (session) await call("Target.detachFromTarget", { sessionId: session }, undefined).catch(() => {});
  writeFileSync(output, JSON.stringify(report, null, 2) + "\n");
  socket.close();
}
