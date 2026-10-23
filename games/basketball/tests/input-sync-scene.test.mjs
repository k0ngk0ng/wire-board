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
const direct=command=>{const r=JSON.parse(globalThis.basketballWasm(JSON.stringify(command)));if(!r.ok)throw Object.assign(Error(r.error.detail),{code:r.error.code});return r.data};
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

// Failure cases specified before implementation: deterministic native rule
// errors must not download the same checkpoint/restart audio; duplicate views,
// new envelope versions and RAF gaps cannot clear the fault; authority that
// actually advances may recover; unknown errors may try one checkpoint but
// cannot restart that allowance by reinstalling the same state. Existing
// checksum/history/epoch tests above retain normal recoverable behavior.
test('native rule failure freezes presentation without checkpoint or audio-reset loops',t=>{
 let broken=true;const resets=[],faults=[],frames=[],resync=[],calls=[];
 const scene=new BasketballNetworkScene({controller:0,engine:{request:c=>{
  calls.push(c.op);if(broken&&c.op==='advance')throw Object.assign(Error('unrestored original action'),{code:'rule_unavailable'});return direct(c);
 }},onFrame:f=>frames.push(f),onReset:seq=>resets.push(seq),onFault:f=>faults.push(f),onResync:r=>resync.push(r)});
 t.after(()=>scene.dispose());scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);
 scene.tick(20,()=>1);
 assert.equal(resync.length,0,'rule errors must not request recovery snapshots');
 assert.equal(faults.length,1);assert.match(faults[0],/游戏/);assert.equal(scene.predicting,false);
 const count=calls.length;
 for(let n=1;n<=100;n++){scene.update({...fixture.views[0],serverAt:fixture.views[0].serverAt+n},n+1,n*100);scene.tick(n*100,()=>1);}
 scene.tick(50000,()=>1);
 assert.equal(calls.length,count,'same authority retried the failed simulation');
 assert.equal(resets.length,1,'repeated failure reset the music');assert.equal(frames.length,1);
 broken=false;assert.equal(scene.update(fixture.views[1],102,50100),true);
 assert.equal(faults.at(-1),undefined);assert.equal(scene.predicting,true);scene.tick(50120,()=>0);
 assert.equal(resets.length,1,'fault recovery treated the paused clock as a new RAF gap and stopped music');
 assert.equal(calls.filter(c=>c==='restore').length,1,'advancing authority downloaded a checkpoint');
});

test('reinstalling the same checkpoint does not reset the automatic recovery allowance',t=>{
 const resync=[],faults=[];let broken=true;
 const scene=new BasketballNetworkScene({controller:0,engine:{request:c=>{
  if(broken&&c.op==='advance')throw Error('unexpected replay failure');return direct(c);
 }},onFrame:()=>{},onResync:r=>resync.push(r),onFault:f=>faults.push(f)});
 t.after(()=>scene.dispose());scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);
 scene.tick(20,()=>0);assert.equal(resync.length,1);
 scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,100);scene.tick(120,()=>0);
 assert.equal(resync.length,1,'same failure started another full recovery cycle');assert.equal(faults.length,1);
 broken=false;scene.bootstrap(fixture.checkpoints[1],fixture.views[1],2,200);scene.tick(220,()=>0);
 assert.equal(scene.predicting,true);assert.equal(faults.at(-1),undefined);
});

test('authoritative replay rule failure is not classified as a network gap',t=>{
 const faults=[],resync=[];const scene=new BasketballNetworkScene({engine:{request:c=>{
  if(c.op==='replay')throw Object.assign(Error('unrestored authoritative action'),{code:'rule_unavailable'});return direct(c);
 }},onFrame:()=>{},onFault:f=>faults.push(f),onResync:r=>resync.push(r)});
 t.after(()=>scene.dispose());scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);
 assert.equal(scene.update(fixture.views[1],2,100),false);assert.equal(resync.length,0);assert.equal(faults.length,1);
});

test('a stopped Wasm runtime is reported once and can be safely disposed',()=>{
 let stopped=false;const faults=[],reloads=[],resync=[];const scene=new BasketballNetworkScene({controller:0,engine:{request:c=>{
  if(stopped)throw Object.assign(Error('runtime exited'),{code:'runtime_stopped'});return direct(c);
 }},onFrame:()=>{},onFault:(f,reload)=>{faults.push(f);reloads.push(reload);},onResync:r=>resync.push(r)});
 scene.bootstrap(fixture.checkpoints[0],fixture.views[0],1,0);stopped=true;scene.tick(20,()=>0);scene.tick(40,()=>0);
 for(let i=1;i<fixture.views.length;i++)scene.update(fixture.views[i],i+1,i*100);
 scene.update({...fixture.views.at(-1),epoch:'b'.repeat(24)},fixture.views.length+1,2000);
 assert.equal(faults.length,1);assert.match(faults[0],/刷新/);assert.equal(resync.length,0);
 assert.deepEqual(reloads,[true],'stopped runtime must request a real page reload');
 assert.doesNotThrow(()=>scene.dispose());
});
