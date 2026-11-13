// Failure cases BEFORE implementation: opt-in repeated combat events allocate,
// unload and refetch media on every hit; new identities fail to rewind; duplicate
// identities rewind; pending old plays/metadata/ended callbacks resurrect or
// stop successors; cached fallback retries a failed CDN each shot; expired seeks,
// owner pause, mute/unmute, volume, source changes and 16/32 lane bounds leak.
// Native media E2E complements these isolated asynchronous failure boundaries.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {GameAudio} from '../src/audio.ts';
class Media {
 dataset={};paused=true;muted=false;loop=false;volume=1;duration=1;readyState=1;error=null;
 source='';position=0;assignments=[];loads=0;plays=[];removed=false;behavior=()=>Promise.resolve();
 get src(){return this.source}set src(v){this.source=v;this.position=0;this.assignments.push(v)}
 get currentSrc(){return this.source}get currentTime(){return this.position}set currentTime(v){this.position=v}
 play(){this.paused=false;this.plays.push([this.source,this.position]);return this.behavior()}
 pause(){this.paused=true}load(){this.loads++;this.position=0}removeAttribute(n){if(n==='src')this.source=''}remove(){this.removed=true}
}
function setup(){const voices=[];const audio=new GameAudio(()=>{const m=new Media();voices.push(m);return m});return{audio,p:audio.channel('room'),voices}}
const opts=id=>({playbackID:id,reuseLoaded:true});
test('opt-in finite sound reuses one loaded native player across new event identities',async()=>{
 const{audio,p,voices}=setup();await p.effect('arrow',['arrow.wav'],opts('1'));const m=voices[0],loads=m.loads,assigned=m.assignments.length;m.position=.08;
 for(let i=2;i<=30;i++){assert.equal(await p.effect('arrow',['arrow.wav'],opts(String(i))),true);assert.equal(m.currentTime,0)}
 assert.equal(voices.length,1);assert.equal(m.loads,loads);assert.equal(m.assignments.length,assigned);assert.equal(m.plays.length,30);
 m.position=.06;assert.equal(await p.effect('arrow',['arrow.wav'],opts('30')),true);assert.equal(m.position,.06);assert.equal(m.plays.length,30);
 m.paused=true;m.onended?.();await Promise.resolve();assert.equal(await p.effect('arrow',['arrow.wav'],opts('30')),false);await p.effect('arrow',['arrow.wav'],opts('31'));assert.equal(voices.length,1);assert.equal(m.position,0);audio.dispose();assert.ok(m.removed&&!m.src)
});
test('cached fallback remains loaded and an expired seek cannot replay',async()=>{
 const{audio,p,voices}=setup();await p.effect('other',['other.wav'],opts('other'));const n=voices.length;
 // Fail primary on first load, then reuse the successful local alternative.
 const original=audio.channel('room');let made;const a=new GameAudio(()=>{made=new Media();made.behavior=()=>made.src==='cdn.wav'?Promise.reject(Error('cdn')):Promise.resolve();return made});const port=a.channel('room');
 await port.effect('arrow',['cdn.wav','local.wav'],opts('1'));const m=made,loads=m.loads;assert.deepEqual(m.plays.map(x=>x[0]),['cdn.wav','local.wav']);
 await port.effect('arrow',['cdn.wav','local.wav'],opts('2'));assert.equal(made,m);assert.deepEqual(m.plays.map(x=>x[0]),['cdn.wav','local.wav','local.wav']);assert.equal(m.loads,loads);
 assert.equal(await port.effect('arrow',['cdn.wav','local.wav'],{...opts('expired'),startOffset:2}),false);assert.equal(m.plays.length,3);audio.dispose();a.dispose()
});
test('cached late callback, owner pause, mute and source replacement cannot revive old effects',async()=>{
 const{audio,p,voices}=setup();await p.effect('arrow',['arrow.wav'],opts('1'));const m=voices[0],oldEnd=m.onended;
 await p.effect('arrow',['arrow.wav'],opts('2'));oldEnd?.();await Promise.resolve();assert.equal(m.paused,false);
 p.pause();assert.equal(await p.effect('arrow',['arrow.wav'],opts('3')),false);await p.resume();assert.equal(m.paused,false);
 audio.setVolume(.2);assert.equal(m.volume,.2);audio.setEnabled(false);assert.ok(m.removed&&!m.src);audio.setEnabled(true);assert.ok(m.paused);
 await p.effect('arrow',['arrow.wav'],opts('4'));const newM=voices.at(-1);assert.notEqual(newM,m);await p.effect('arrow',['new.wav'],opts('5'));assert.ok(newM.removed&&!newM.src);assert.equal(voices.at(-1).src,'new.wav');audio.dispose()
});
test('a pending reused play cannot interrupt or clear its successor',async()=>{
 const{audio,p,voices}=setup();await p.effect('arrow',['arrow.wav'],opts('1'));const m=voices[0];let finish;m.behavior=()=>new Promise(r=>finish=r);const old=p.effect('arrow',['arrow.wav'],opts('2'));m.behavior=()=>Promise.resolve();assert.equal(await p.effect('arrow',['arrow.wav'],opts('3')),true);finish();assert.equal(await old,false);assert.equal(m.paused,false);assert.equal(voices.length,1);audio.dispose()
});
test('default finite replacement stays unchanged and bounded reuse lanes do not grow',async()=>{
 const{audio,p,voices}=setup();await p.effect('legacy',['one.wav'],{playbackID:'1'});await p.effect('legacy',['one.wav'],{playbackID:'2'});assert.equal(voices.length,2);assert.ok(voices[0].removed);
 for(let i=1;i<16;i++)await p.effect('lane'+i,['sound.wav'],opts(String(i)));assert.equal(await p.effect('extra',['extra.wav'],opts('17')),false);audio.dispose()
});
test('an already selected native playback session is not reconfigured per shot',async()=>{
 let writes=0,type='auto';const descriptor=Object.getOwnPropertyDescriptor(navigator,'audioSession');Object.defineProperty(navigator,'audioSession',{configurable:true,value:{get type(){return type},set type(v){writes++;type=v}}});
 try{const{audio,p}=setup();for(let i=0;i<8;i++)await p.effect('arrow',['arrow.wav'],opts(String(i)));assert.equal(writes,1);audio.dispose()}finally{if(descriptor)Object.defineProperty(navigator,'audioSession',descriptor);else delete navigator.audioSession}
});
