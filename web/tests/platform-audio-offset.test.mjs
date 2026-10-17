// Failure inventory before implementation: starting from zero before seeking;
// loop offsets beyond duration; replaying an ended finite track; stale metadata
// seeking a new owner's source; pause/resume losing the pending seek; duplicate
// state updates restarting music; invalid offsets interrupting valid playback.
import test from 'node:test';
import assert from 'node:assert/strict';
import {GameAudio} from '../src/audio.ts';
class Media {
  dataset={};paused=true;muted=false;loop=false;duration=NaN;readyState=0;onloadedmetadata=null;onended=null;onerror=null;heard=[];pending=[];position=0;source='';
  get src(){return this.source;}set src(v){this.source=v;this.readyState=0;this.duration=NaN;this.position=0;}
  get currentSrc(){return this.source;}
  get currentTime(){return this.position;}set currentTime(v){if(!this.readyState)throw Error('metadata not loaded');this.position=v;}
  play(){this.paused=false;return new Promise((resolve,reject)=>{this.pending.push({resolve,reject});if(this.readyState)this.finishPlay();});}
  finishPlay(){if(this.paused)return;this.heard.push(this.position);for(const p of this.pending.splice(0))p.resolve();}
  metadata(duration){this.duration=duration;this.readyState=1;this.onloadedmetadata?.();this.finishPlay();}
  pause(){this.paused=true;for(const p of this.pending.splice(0))p.reject(Error('AbortError'));}
  removeAttribute(name){if(name==='src')this.source='';}load(){this.readyState=0;this.position=0;}remove(){}
}
function setup(){const voices=[];const audio=new GameAudio(()=>{const m=new Media();voices.push(m);return m;});return{audio,voices,p:audio.channel('room')};}
const flush=async()=>{for(let i=0;i<8;i++)await Promise.resolve();};
test('music seeks before audible play and wraps by its actual media duration',async()=>{
  const{p,voices}=setup();const started=p.music(['loop.mp3'],{loop:true,startOffset:125.25,playbackID:'epoch-1'});
  voices[0].metadata(60);assert.equal(await started,true);assert.deepEqual(voices[0].heard,[5.25]);
  await p.music(['loop.mp3'],{loop:true,startOffset:130,playbackID:'epoch-1'});
  assert.deepEqual(voices[0].heard,[5.25]);
  const second=p.music(['loop.mp3'],{loop:true,startOffset:7,playbackID:'epoch-2'});voices[0].metadata(60);assert.equal(await second,true);assert.deepEqual(voices[0].heard,[5.25,7]);
});
test('finite music beyond its end is not replayed from the beginning',async()=>{
  const{p,voices}=setup();const started=p.music(['ending.mp3'],{loop:false,startOffset:90});
  voices[0].metadata(60);assert.equal(await started,false);assert.equal(voices[0].src,'');assert.deepEqual(voices[0].heard,[]);
});
test('pause during loading retains the offset and resume uses the same source',async()=>{
  const{p,voices}=setup();const started=p.music(['loop.mp3'],{startOffset:14});p.pause();await flush();voices[0].metadata(60);
  assert.equal(await started,false);assert.equal(voices[0].currentTime,14);assert.equal(voices[0].paused,true);
  assert.equal(await p.resume(),true);assert.deepEqual(voices[0].heard,[14]);
});
test('stale metadata cannot seek a newer owner or revive cancelled media',async()=>{
  const{audio,p,voices}=setup();const old=p.music(['old.mp3'],{startOffset:15});const stale=voices[0].onloadedmetadata;
  p.stop();const newer=audio.channel('new').music(['new.mp3'],{startOffset:3});voices[0].metadata(60);stale?.();
  assert.equal(await old,false);assert.equal(await newer,true);assert.equal(voices[0].currentTime,3);assert.deepEqual(voices[0].heard,[3]);
  const callback=voices[0].onloadedmetadata;audio.setEnabled(false);callback?.();assert.equal(voices[0].src,'');
});
test('invalid offsets do not interrupt an existing valid owner',{timeout:1000},async()=>{
  const{audio,p,voices}=setup();const started=p.music(['old.mp3']);voices[0].metadata(60);await started;
  for(const offset of [-1,NaN,Infinity])assert.equal(await audio.channel('new').music(['new.mp3'],{startOffset:offset}),false);
  assert.equal(voices[0].src,'old.mp3');assert.equal(voices[0].paused,false);
});

test('fallback retains the requested offset and ignores metadata from the failed source',async()=>{
  const{p,voices}=setup();const started=p.music(['cdn.mp3','local.mp3'],{startOffset:8});
  const stale=voices[0].onloadedmetadata;
  for(const pending of voices[0].pending.splice(0))pending.reject(Error('network'));
  await flush();assert.equal(voices[0].src,'local.mp3');stale?.();voices[0].metadata(60);
  assert.equal(await started,true);assert.deepEqual(voices[0].heard,[8]);
});
