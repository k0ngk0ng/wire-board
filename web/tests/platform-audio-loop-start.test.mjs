// Failure inventory: repeat intro, wrong cumulative-offset mapping, stale epochs,
// pause at ended, cancelled/stale callback restart, metadata validation, fallback
// offsets and regressions to native whole-resource looping. Tests precede code.
import test from 'node:test';
import assert from 'node:assert/strict';
import {GameAudio} from '../src/audio.ts';
class Media {
 dataset={};paused=true;muted=false;loop=false;duration=NaN;readyState=0;onloadedmetadata=null;onended=null;onerror=null;
 heard=[];pending=[];position=0;source='';sources=[];ended=false;deferPlay=false;abortOnPause=true;
 get src(){return this.source;}set src(v){this.source=v;this.sources.push(v);this.readyState=0;this.duration=NaN;this.position=0;this.ended=false;}
 get currentSrc(){return this.source;}
 get currentTime(){return this.position;}set currentTime(v){if(!this.readyState)throw Error('metadata not loaded');this.position=v;this.ended=false;}
 play(){if(this.ended&&this.position>=this.duration){this.position=0;this.ended=false;}this.paused=false;return new Promise((resolve,reject)=>{this.pending.push({resolve,reject});if(this.readyState&&!this.deferPlay)this.finishPlay();});}
 finishPlay(){if(this.paused)return;this.heard.push(this.position);for(const p of this.pending.splice(0))p.resolve();}
 metadata(duration){this.duration=duration;this.readyState=1;this.onloadedmetadata?.();if(!this.deferPlay)this.finishPlay();}
 end(){this.position=this.duration;this.ended=true;this.paused=true;this.onended?.();}
 pause(){this.paused=true;if(this.abortOnPause)for(const p of this.pending.splice(0))p.reject(Error('AbortError'));}
 removeAttribute(n){if(n==='src')this.source='';}load(){this.readyState=0;this.position=0;this.ended=false;}remove(){}
}
const flush=async()=>{for(let i=0;i<12;i++)await Promise.resolve();};
function setup(){const voices=[];const audio=new GameAudio(()=>{const m=new Media();voices.push(m);return m;});return{audio,voices,p:audio.channel('room')};}
async function start(options={}){const h=setup();const ready=h.p.music(['compound.mp3'],{loop:true,loopStartSeconds:5,playbackID:'epoch-1',...options});h.voices[0].metadata(10);assert.equal(await ready,true);return{...h,m:h.voices[0]};}
test('suffix music maps cumulative offsets without repeating the intro',{timeout:1000},async()=>{
 for(const [offset,want] of [[0,0],[4.9,4.9],[5,5],[9.9,9.9],[10,5],[20,5],[120.5,5.5]]){
  const{m}=await start({startOffset:offset});assert.equal(m.loop,false);assert.deepEqual(m.heard,[want]);
 }
});
test('native ended reuses one loaded source and repeats only its suffix',{timeout:1000},async()=>{
 const{m}=await start();m.end();await flush();m.end();await flush();assert.deepEqual(m.heard,[0,5,5]);assert.deepEqual(m.sources,['compound.mp3']);assert.equal(m.paused,false);assert.equal(m.ended,false);
});
test('stable epochs do not restart; a new epoch can play the intro again',{timeout:1000},async()=>{
 const{p,m}=await start();await p.music(['compound.mp3'],{loopStartSeconds:5,startOffset:8,playbackID:'epoch-1'});assert.deepEqual(m.heard,[0]);
 const next=p.music(['compound.mp3'],{loopStartSeconds:5,startOffset:0,playbackID:'epoch-2'});m.metadata(10);assert.equal(await next,true);assert.deepEqual(m.heard,[0,0]);
});
test('a pause racing with ended retains the suffix restart for resume',{timeout:1000},async()=>{
 const{p,m}=await start();p.pause();m.end();await flush();assert.deepEqual(m.heard,[0]);assert.equal(m.paused,true);
 assert.equal(await p.resume(),true);assert.deepEqual(m.heard,[0,5]);assert.equal(m.src,'compound.mp3');
});
test('pause during a pending suffix play preserves the mapped position',{timeout:1000},async()=>{
 const{p,m}=await start();m.deferPlay=true;m.end();p.pause();await flush();assert.equal(m.currentTime,5);assert.equal(m.paused,true);
 m.deferPlay=false;assert.equal(await p.resume(),true);assert.equal(m.currentTime,5);assert.equal(m.src,'compound.mp3');
});
test('stale ended callbacks cannot revive cancelled sources or seek a new owner',{timeout:1000},async()=>{
 for(const mode of ['stop','stopMusic','mute','dispose']){
  const{audio,p,m}=await start();const stale=m.onended;
  if(mode==='stop')p.stop();else if(mode==='stopMusic')p.stopMusic();else if(mode==='mute')audio.setEnabled(false);else audio.dispose();
  stale?.();await flush();assert.equal(m.src,'');assert.equal(m.paused,true);assert.deepEqual(m.heard,[0]);
 }
 const{audio,m}=await start();const stale=m.onended;const next=audio.channel('other').music(['other.mp3'],{startOffset:3});m.metadata(10);await next;stale?.();await flush();assert.equal(m.src,'other.mp3');assert.equal(m.currentTime,3);assert.equal(m.paused,false);
});
test('hidden-page cancellation and unmute require a fresh current-state restore',{timeout:1000},async()=>{
 const previous=globalThis.document;globalThis.document={hidden:false,body:{append(){}}};
 try{
  const{audio,p,m}=await start();const stale=m.onended;globalThis.document.hidden=true;audio.stop();stale?.();await flush();assert.equal(m.src,'');
  globalThis.document.hidden=false;audio.setEnabled(false);audio.setEnabled(true);assert.equal(m.src,'');
  const ready=p.music(['compound.mp3'],{loopStartSeconds:5,startOffset:27,playbackID:'epoch-1'});m.metadata(10);assert.equal(await ready,true);assert.equal(m.currentTime,7);
 }finally{if(previous===undefined)delete globalThis.document;else globalThis.document=previous;}
});
test('invalid suffix options do not interrupt an existing owner',{timeout:1000},async()=>{
 const{audio,m}=await start();for(const value of [-1,NaN,Infinity])assert.equal(await audio.channel('other').music(['bad.mp3'],{loopStartSeconds:value}),false);
 assert.equal(await audio.channel('other').music(['bad.mp3'],{loop:false,loopStartSeconds:5}),false);assert.equal(m.src,'compound.mp3');assert.equal(m.paused,false);
});
test('loop start outside decoded duration fails and cleans the source',{timeout:1000},async()=>{
 for(const [anchor,duration]of [[10,10],[11,10],[5,NaN],[5,Infinity]]){
  const{p,voices}=setup();const ready=p.music(['bad.mp3'],{loopStartSeconds:anchor});voices[0].metadata(duration);assert.equal(await ready,false);assert.equal(voices[0].src,'');
 }
});
test('fallback keeps suffix offset mapping and rejects stale metadata',{timeout:1000},async()=>{
 const{p,voices}=setup();const ready=p.music(['cdn.mp3','local.mp3'],{loopStartSeconds:5,startOffset:27});const m=voices[0],stale=m.onloadedmetadata;
 for(const q of m.pending.splice(0))q.reject(Error('network'));await flush();assert.equal(m.src,'local.mp3');stale?.();m.metadata(10);assert.equal(await ready,true);assert.deepEqual(m.heard,[7]);m.end();await flush();assert.deepEqual(m.heard,[7,5]);
});
test('a late loop play completion does not pause or rewind a successor',{timeout:1000},async()=>{
 const{audio,m}=await start();m.deferPlay=true;m.abortOnPause=false;m.end();const pending=m.pending.splice(0);m.deferPlay=false;
 const next=audio.channel('other').music(['other.mp3'],{startOffset:3});m.metadata(10);await next;for(const q of pending)q.resolve();await flush();assert.equal(m.src,'other.mp3');assert.equal(m.currentTime,3);assert.equal(m.paused,false);
});
test('omitted or zero loop start keeps native whole-resource semantics',{timeout:1000},async()=>{
 for(const anchor of [undefined,0]){const{p,voices}=setup();const ready=p.music(['loop.mp3'],{startOffset:27,loopStartSeconds:anchor});voices[0].metadata(10);await ready;assert.equal(voices[0].loop,true);assert.equal(voices[0].currentTime,7);}
});
