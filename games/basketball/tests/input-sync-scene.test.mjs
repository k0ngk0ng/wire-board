// Written before the compact scene integration: only explicit bootstraps may
// import full state; pushes replay just new inputs, duplicates are inert,
// input/hash/revision gaps request one recovery, and native prediction forks
// must not round-trip simulation state through JavaScript.
import test from 'node:test';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {readFileSync} from 'node:fs';
import {fileURLToPath,pathToFileURL} from 'node:url';
import path from 'node:path';
const root=fileURLToPath(new URL('../../../',import.meta.url));
execFileSync('go',['test','./games/basketball/server','-run','^TestHostVideoHistoryReplaysExactInputAndHandoff$','-count=1'],{cwd:root,stdio:'pipe'});
const fixture=JSON.parse(readFileSync(path.join(root,'.local/basketball/network-scene/host-views.json'),'utf8'));
const runtime=execFileSync(process.execPath,['--input-type=module','-e','import {prepareBasketballRuntime} from "./games/basketball/build/runtime.mjs";console.log(prepareBasketballRuntime(process.cwd()))'],{cwd:root,encoding:'utf8'}).trim();
await import(pathToFileURL(path.join(runtime,'basketball/v1/runtime/wasm_exec.js')).href);
const go=new globalThis.Go();const loaded=await WebAssembly.instantiate(readFileSync(path.join(runtime,'basketball/v1/runtime/basketball.wasm')),go.importObject);void go.run(loaded.instance);
const {BasketballNetworkScene}=await import('../web/network-scene.ts');
const direct=command=>{const r=JSON.parse(globalThis.basketballWasm(JSON.stringify(command)));if(!r.ok)throw Error(r.error.detail);return r.data};
function fixtureScene(t,controller){
 const calls=[],frames=[],resync=[];const scene=new BasketballNetworkScene({engine:{request:command=>{calls.push(command);return direct(command)}},controller,onFrame:frame=>frames.push(frame),onResync:reason=>resync.push(reason)});
 t.after(()=>scene.dispose());return {scene,calls,frames,resync};
}
test('confirmed input windows match original state with one bootstrap only',t=>{
 const {scene,calls,resync}=fixtureScene(t);
 scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);
 for(let i=1;i<fixture.views.length;i++){
  assert.equal(scene.update(fixture.views[i],i+1,i*100),true);
  assert.deepEqual(scene.lastReplayState,fixture.checkpoints[i].state,`authority ${i}`);
  scene.tick(i*100+90);
 }
 assert.deepEqual(resync,[]);
 assert.equal(calls.filter(c=>c.op==='restore').length,1);
 assert.ok(calls.filter(c=>c.op==='replay').every(c=>c.compact&&c.window&&!c.checkpoint));
 const count=calls.length;scene.update(fixture.views.at(-1),fixture.views.length,950);assert.equal(calls.length,count,'duplicate performed work');
});
test('local prediction uses compact results and native forks',t=>{
 const {scene,calls}=fixtureScene(t,0);
 scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);
 scene.tick(1000/60,()=>1);
 const reference=direct({op:'restore',checkpoint:fixture.checkpoints[0]});
 t.after(()=>direct({op:'dispose',session:reference.session}));
 const expected=direct({op:'advance',session:reference.session,ticks:1,held:[1,0,0,0]});
 assert.deepEqual(scene.predictedState,expected.state);
 scene.update(fixture.views[1],2,100);
 assert.equal(calls.filter(c=>c.op==='restore').length,1,'reconciliation exported/reimported full state');
 assert.ok(calls.some(c=>c.op==='fork'&&c.target));
 assert.ok(calls.filter(c=>c.op==='advance').every(c=>c.compact));
 scene.cancel();assert.equal(scene.predictedState.held[0],0);
});
test('bad proof and missing history require explicit coalesced recovery',t=>{
 const {scene,frames,resync}=fixtureScene(t);
 scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);
 const before=frames.length;
 const wrong={...fixture.views[1],digest:'0'.repeat(64)};
 assert.equal(scene.update(wrong,2,100),false);
 assert.equal(frames.length,before,'unverified frame became visible');
 scene.update(fixture.views[2],3,200);assert.deepEqual(resync,['checksum']);
 scene.bootstrap(fixture.checkpoints[2],fixture.views[2],3,200);
 assert.equal(scene.update(fixture.views[3],4,300),true);
 const gap=structuredClone(fixture.views[4]);gap.replay.from=gap.frame+1;
 assert.equal(scene.update(gap,5,400),false);assert.deepEqual(resync,['checksum','gap']);
});
test('configuration and epoch changes do not reuse old simulation state',t=>{
 const {scene,resync}=fixtureScene(t,0);
 scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);
 assert.equal(scene.update({...fixture.views[1],syncRevision:1},2,100),false);
 assert.deepEqual(resync,['revision']);
 scene.bootstrap(fixture.checkpoints[1],fixture.views[1],2,100);
 assert.equal(scene.update({...fixture.views[2],epoch:'a'.repeat(24)},3,200),false);
 assert.deepEqual(resync,['revision','epoch']);
});
test('recovery catches up buffered authority without replaying stale presentation',t=>{
 const {scene,frames}=fixtureScene(t);
 scene.bootstrap(fixture.checkpoints[0],{...fixture.views[0],canInput:false},1,0,true);
 assert.equal(frames.length,0);
 for(let i=1;i<fixture.views.length;i++)assert.equal(scene.update({...fixture.views[i],canInput:false},i+1,i*100),true);
 assert.equal(frames.length,0);
 scene.finishRecovery(1000);
 assert.equal(frames.length,1);assert.equal(frames[0].frame,fixture.views.at(-1).frame);
 assert.deepEqual(scene.lastReplayState,fixture.checkpoints.at(-1).state);
});
