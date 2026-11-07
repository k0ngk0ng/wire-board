// Failure inventory: stale runtime/source/loader, missing scene, altered gzip,
// local/remote audio mismatch, and compiled payload omitted from CDN inventory.
import test from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {existsSync,readFileSync} from 'node:fs';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {prepareYuYuDelivery,verifyYuYuDelivery} from './runtime-delivery.mjs';
const root=fileURLToPath(new URL('../../../',import.meta.url));
test('current source-bound runtime and all13 exact resources enter client delivery',()=>{
 const output=prepareYuYuDelivery(root),receipt=verifyYuYuDelivery(root,output);
 assert.match(receipt.sourceSHA256,/^[a-f0-9]{64}$/);
 assert.equal(receipt.resources,13);
 const runtime=JSON.parse(readFileSync(path.join(output,'yuyuhakusho/v1/runtime/build.json')));
 assert.equal(runtime.wasm.url,'./client.wasm');
 assert.equal(runtime.sourceSHA256,receipt.sourceSHA256);
 assert.ok(existsSync(path.join(output,'yuyuhakusho/v1/runtime/client.wasm')));
 assert.match(runtime.wasm.sha256,/^[a-f0-9]{64}$/);
 const result=JSON.parse(execFileSync(process.execPath,[path.join(root,'games/yuyuhakusho/build/smoke.mjs'),root,output],{cwd:root,encoding:'utf8'}));assert.equal(result.passed,true);assert.equal(result.checkpointRoundtrip,true);
});
