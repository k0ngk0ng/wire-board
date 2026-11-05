// Failure inventory frozen before implementation: absent finite lane API;
// one lane cuts a peer/music/legacy effect; same instance rewinds or conflicts;
// completed instance replays; replacement/stop/mute/late metadata resurrects an
// obsolete voice; fallback loses offset or becomes a playlist; expired seeks
// replay; pause-aborted loading falls through; owner/volume/dispose leakage;
// unstable lane names retain unbounded voices (16/owner,32/platform maximum).
// These controlled Promise tests complement the real Chromium E2E contract.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {GameAudio} from '../src/audio.ts';
class Media {
  dataset={};paused=true;muted=false;loop=false;volume=1;duration=8;readyState=1;
  onloadedmetadata=null;onended=null;onerror=null;removed=false;plays=[];position=0;source='';
  behavior=()=>Promise.resolve();
  get src(){return this.source;}set src(value){this.source=value;this.position=0;}
  get currentSrc(){return this.source;}
  get currentTime(){return this.position;}set currentTime(value){if(!this.readyState)throw Error('metadata pending');this.position=value;}
  play(){this.paused=false;this.plays.push({src:this.src,offset:this.currentTime});return this.behavior();}
  pause(){this.paused=true;}load(){this.position=0;}remove(){this.removed=true;}
  removeAttribute(name){if(name==='src')this.source='';}
}
function setup(make=()=>new Media()){
  const voices=[];const audio=new GameAudio(()=>{const media=make();voices.push(media);return media;});
  return {audio,voices,p:audio.channel('room'),other:audio.channel('other')};
}
const flush=async()=>{for(let i=0;i<8;i++)await Promise.resolve();};
const live=voices=>voices.filter(media=>media.src&&!media.paused&&!media.removed);
test('finite lanes coexist with music and the legacy single effect task',async()=>{
  const{audio,p,voices}=setup();await p.music(['music'],{loop:false});await p.play(['legacy']);
  assert.equal(await p.effect('pcm-main',['voice-a'],{playbackID:'one'}),true);
  assert.equal(await p.effect('pcm-alt',['voice-b'],{playbackID:'two'}),true);
  assert.equal(live(voices).length,4);assert.ok(voices.every(media=>!media.loop));
  await p.play(['next-legacy']);assert.equal(live(voices).length,4);
  p.stopEffect('pcm-main','one');assert.equal(live(voices).length,3);
  p.stopMusic();assert.equal(live(voices).length,2);audio.dispose();assert.ok(voices.every(media=>media.removed&&!media.src));
});
test('finite instance identity is stable through refresh, conflict, end and replacement',async()=>{
  const{p,voices}=setup();await p.effect('main',['a'],{playbackID:'id1'});const first=voices[0];first.currentTime=2;
  assert.equal(await p.effect('main',['a'],{playbackID:'id1',startOffset:0}),true);assert.equal(first.currentTime,2);assert.equal(first.plays.length,1);
  assert.equal(await p.effect('main',['b'],{playbackID:'id1'}),false);assert.equal(first.src,'a');assert.equal(first.paused,false);
  first.paused=true;first.onended?.();await flush();await p.effect('main',['a'],{playbackID:'id1'});assert.equal(first.plays.length,1);
  assert.equal(await p.effect('main',['a'],{playbackID:'id2'}),true);const second=live(voices)[0];assert.notEqual(second,first);assert.ok(first.removed);assert.equal(second.currentTime,0);
  p.stopEffect('main','id1');assert.equal(second.paused,false);p.stopEffect('main','id2');assert.equal(second.src,'');
});
test('invalid finite requests are atomic and source alternatives are not a playlist',async()=>{
  const{p,voices}=setup();await p.effect('main',['good'],{playbackID:'current'});const media=voices[0];
  for(const [lane,sources,options]of [['', ['a'],{playbackID:'x'}],['main',[],{playbackID:'x'}],['main',[''],{playbackID:'x'}],['main',['a'],{playbackID:''}],['main',['a'],{playbackID:'x',startOffset:-1}],['main',['a'],{playbackID:'x',startOffset:Infinity}],['main',['a'],{playbackID:'x',startOffset:NaN}],['main',['a'],{playbackID:'x'.repeat(257)}]])assert.equal(await p.effect(lane,sources,options),false);
  assert.equal(media.src,'good');assert.equal(media.paused,false);assert.equal(voices.length,1);
  p.stop();const next=setup(()=>{const media=new Media();media.behavior=()=>media.src==='a'||media.src==='b'?Promise.reject(Error('network')):Promise.resolve();return media;});
  assert.equal(await next.p.effect('main',['a','b','c'],{playbackID:'fallback',startOffset:2.5}),true);
  assert.deepEqual(next.voices[0].plays,[{src:'a',offset:2.5},{src:'b',offset:2.5},{src:'c',offset:2.5}]);
  next.voices[0].paused=true;next.voices[0].onended?.();await flush();assert.equal(next.voices[0].plays.length,3);
});
test('expired restore does not replay or interrupt a peer',async()=>{
  const{p,voices}=setup();await p.effect('peer',['peer'],{playbackID:'peer'});
  assert.equal(await p.effect('main',['expired'],{playbackID:'expired',startOffset:8}),false);
  assert.deepEqual(live(voices).map(media=>media.src),['peer']);
  assert.ok(!voices.some(media=>media.plays.some(play=>play.src==='expired')));
});
test('late replaced or muted plays cannot revive detached voices or stop a successor',async()=>{
  const{audio,p,voices}=setup();let finish;
  const old=p.effect('main',['old'],{playbackID:'old'});const oldMedia=voices[0];
  // Start another deferred instance using a fresh controlled factory.
  p.stop();
  const state=setup(()=>{const media=new Media();media.behavior=()=>media.src==='slow'?new Promise(resolve=>{finish=()=>{media.paused=false;resolve();};}):Promise.resolve();return media;});
  const slow=state.p.effect('main',['slow'],{playbackID:'slow'});const obsolete=state.voices[0];
  assert.equal(await state.p.effect('main',['fresh'],{playbackID:'fresh'}),true);const fresh=live(state.voices)[0];finish();assert.equal(await slow,false);
  assert.ok(obsolete.paused&&obsolete.removed&&!obsolete.src);assert.equal(fresh.paused,false);
  const stale=state.p.effect('alt',['slow'],{playbackID:'mute'});const muted=state.voices.at(-1);state.audio.setEnabled(false);finish();assert.equal(await stale,false);assert.ok(muted.paused&&muted.removed&&!muted.src);assert.equal(live(state.voices).length,0);
  state.audio.setEnabled(true);assert.equal(live(state.voices).length,0);await old;assert.ok(oldMedia.paused&&oldMedia.removed);audio.dispose();
});
test('pause during finite metadata load preserves offset and does not trigger fallback',async()=>{
  let ready;
  const{p,voices}=setup(()=>{const media=new Media();media.readyState=0;media.duration=NaN;
    media.behavior=()=>media.readyState?Promise.resolve():new Promise(resolve=>{ready=()=>{media.readyState=1;media.duration=8;media.onloadedmetadata?.();resolve();};});return media;});
  const started=p.effect('main',['cdn','fallback'],{playbackID:'paused',startOffset:3});const media=voices[0];p.pause();ready();assert.equal(await started,false);assert.equal(media.currentTime,3);assert.equal(media.paused,true);
  assert.equal(await p.resume(),true);assert.equal(media.src,'cdn');assert.equal(media.currentTime,3);assert.ok(!media.plays.some(play=>play.src==='fallback'));
});
test('finite owner cancellation, volume and stale callbacks stay isolated',async()=>{
  const{audio,p,other,voices}=setup();await p.effect('main',['old'],{playbackID:'old'});await other.effect('main',['new'],{playbackID:'new'});
  const old=voices[0],newer=voices[1],ended=old.onended,metadata=old.onloadedmetadata;
  audio.setVolume(.25);assert.equal(old.volume,.25);assert.equal(newer.volume,.25);p.pause();assert.equal(old.paused,true);assert.equal(newer.paused,false);
  p.stop();ended?.();metadata?.();assert.equal(newer.paused,false);assert.equal(newer.src,'new');assert.ok(old.removed&&!old.src);
  audio.setVolume(0);assert.equal(live(voices).length,0);audio.setVolume(1);assert.equal(live(voices).length,0);audio.dispose();assert.ok(voices.every(media=>media.removed));
});
test('finite stable lane capacity is bounded and replacement does not grow live media',async()=>{
  const{audio,p,other,voices}=setup();
  for(let i=0;i<16;i++)assert.equal(await p.effect('lane-'+i,['a'],{playbackID:'id-'+i}),true);
  assert.equal(await p.effect('overflow',['a'],{playbackID:'overflow'}),false);
  for(let i=0;i<16;i++)assert.equal(await other.effect('lane-'+i,['b'],{playbackID:'id-'+i}),true);
  assert.equal(await audio.channel('third').effect('overflow',['c'],{playbackID:'overflow'}),false);
  for(let i=0;i<120;i++)assert.equal(await p.effect('lane-0',['a'],{playbackID:'replacement-'+i}),true);
  assert.equal(live(voices).length,32);assert.equal(voices.filter(media=>!media.removed).length,32);
  p.stop();assert.equal(live(voices).length,16);assert.equal(await audio.channel('third').effect('free',['c'],{playbackID:'free'}),true);
  audio.dispose();assert.ok(voices.every(media=>media.removed&&!media.src));
});
