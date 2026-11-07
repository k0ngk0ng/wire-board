// Failure cases before manifest normalization: leading slash escapes the
// immutable manifest directory; media paths use page origin; absolute CDN URLs
// are corrupted; only the initial map is described; invalid URL schemes pass.
import test from 'node:test';import assert from 'node:assert/strict';
import {resolveRuntimeManifest,resolveRuntimeURL, runtimeStageManifest, runtimePresentationManifest, fetchRuntimeManifest} from './runtime-bridge.ts';
test('all runtime asset URLs bind to the immutable manifest parent',()=>{
 const entry=url=>({url,bytes:1,sha256:'a'.repeat(64)}),base='https://assets.test/game/contra/runtime-code/manifest.json';
 const manifest=resolveRuntimeManifest({version:1,rulesBuildID:'c'.repeat(64),resources:entry('/stage-1.json'),stages:{'0':entry('/stage-1.json'),'1':entry('stage-2.json')},rgb:entry('/graphics/colors.rgb'),pages:{'0':entry('/graphics/0.idx')},wasm:entry('/contra.wasm'),loader:entry('/wasm_exec.js'),audio:'/audio.json'},base);
 for(const value of [manifest.resources,manifest.stages['1'],manifest.rgb,manifest.pages['0'],manifest.wasm,manifest.loader])assert.ok(value.url.startsWith('https://assets.test/game/contra/runtime-code/'),value);
 assert.equal(manifest.audio,'https://assets.test/game/contra/runtime-code/audio.json');assert.equal(resolveRuntimeURL('/audio/a.wav',base),'https://assets.test/game/contra/runtime-code/audio/a.wav');
 assert.equal(resolveRuntimeURL('https://cdn.test/immutable/a.wav',base),'https://cdn.test/immutable/a.wav');
 for(const url of ['javascript:alert(1)','data:text/javascript,x','file:///etc/hosts'])assert.throws(()=>resolveRuntimeURL(url,base));
});
// Rules, actual executable/loader artifacts, and normalized pixels have separate
// identities. A same-rule wrapper/toolchain update must never overwrite a URL,
// and a swapped or stale manifest must fail before the Wasm is instantiated.
test('exact manifest bytes bind the runtime URL independently of rule and visual identities',async()=>{
 const entry=url=>({url,bytes:1,sha256:'a'.repeat(64)});
 const manifest={version:1,rulesBuildID:'c'.repeat(64),visualsID:'d'.repeat(64),resources:entry('stage-1.json'),rgb:entry('colors'),pages:{'0':entry('page')},wasm:entry('runtime.wasm'),loader:entry('loader.js'),audio:'audio.json'};
 const raw=JSON.stringify(manifest),hash=async s=>Buffer.from(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(s))).toString('hex'),runtimeBuildID=await hash(raw),expected={runtimeBuildID,rulesBuildID:manifest.rulesBuildID,visualsID:manifest.visualsID};
 const before=globalThis.fetch;globalThis.fetch=async()=>new Response(raw);
 try {
  const resolved=await fetchRuntimeManifest('https://assets.test/contra/runtime-'+runtimeBuildID+'/manifest.json',expected);assert.ok(resolved.wasm.url.includes('/runtime-'+runtimeBuildID+'/'));
  await assert.rejects(()=>fetchRuntimeManifest('https://assets.test/manifest.json',{...expected,runtimeBuildID:'e'.repeat(64)}));
  await assert.rejects(()=>fetchRuntimeManifest('https://assets.test/manifest.json',{...expected,visualsID:'e'.repeat(64)}));
  const changed=JSON.stringify({...manifest,wasm:{...manifest.wasm,sha256:'f'.repeat(64)}});assert.notEqual(await hash(changed),runtimeBuildID);
  globalThis.fetch=async()=>new Response(changed);await assert.rejects(()=>fetchRuntimeManifest('https://assets.test/manifest.json',expected));
 } finally {globalThis.fetch=before}
});
test('restore loads actual applied graphics while a previous interlude survives the stage boundary',()=>{
 const manifest={pages:{'0':{url:'current'},'18':{url:'previous'}}};
 assert.deepEqual(Object.keys(runtimePresentationManifest(manifest,{Patterns:[18,0,18]}).pages),['0','18']);
 assert.throws(()=>runtimePresentationManifest(manifest,{Patterns:[99]}));
});
// Per-stage selection must not preload other stages, lose valid page zero,
// accept incomplete catalogues, or treat the cleared level-eight state as a
// ninth level. Page bytes still pass the ordinary SHA-256 fetch verification.
test('stage graphics include only the bound current-stage scene pages',()=>{
 const manifest={pages:{'0':{url:'zero'},'1':{url:'one'},'2':{url:'two'}},stagePages:{'0':[0,1],'1':[0,2],'7':[2]}};
 assert.deepEqual(Object.keys(runtimeStageManifest(manifest,0).pages),['0','1']);
 assert.deepEqual(Object.keys(runtimeStageManifest(manifest,1).pages),['0','2']);
 assert.deepEqual(Object.keys(runtimeStageManifest(manifest,8).pages),['2']);
 assert.throws(()=>runtimeStageManifest({...manifest,stagePages:{'0':[4]}},0));
 assert.throws(()=>runtimeStageManifest(manifest,2));
});
