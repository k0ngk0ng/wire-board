// Failure scenarios before implementation: hardware-dependent Web Audio route;
// mute leaves playing/queued sound alive; pending play revives after mute;
// unmute replays old sound; a departed game stops preview/new-room audio;
// a blocked media request breaks actions; common CDN cues have no fallback.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {GameAudio} from '../src/audio.ts';
import {soundWav, soundNames} from '../src/audio-tones.ts';
class Media {
 dataset={};src='';muted=false;paused=true;currentTime=0;preload='';onended=null;onerror=null;calls=[];removed=false;
 behavior=()=>Promise.resolve();
 play(){this.paused=false;this.calls.push(this.src);return this.behavior();}
 pause(){this.paused=true;}
 load(){this.currentTime=0;}
 removeAttribute(name){if(name==='src')this.src='';}
 remove(){this.removed=true;}
}
function setup(){const media=new Media();const audio=new GameAudio(()=>media);audio.mount();return {media,audio};}
test('all common cues are valid non-silent PCM WAVs',()=>{
 assert.equal(soundNames.length,7);
 for(const name of soundNames){const wav=soundWav(name),v=new DataView(wav.buffer);assert.equal(new TextDecoder().decode(wav.slice(0,4)),'RIFF');assert.equal(new TextDecoder().decode(wav.slice(8,12)),'WAVE');assert.equal(v.getUint32(4,true),wav.length-8);assert.equal(v.getUint32(40,true),wav.length-44);assert.equal(v.getUint16(34,true),16);assert.ok(wav.slice(44).some(x=>x));}
});
test('common cues use the media player and configured CDN',async()=>{
 const {audio,media}=setup();audio.setAssetsBase('https://cdn.example.test/base/');assert.equal(await audio.play('preview'),true);assert.match(media.src,/\/base\/platform\/audio\/v1\/preview.wav$/);assert.equal(media.muted,false);assert.equal(media.dataset.platformAudio,'');
});
test('mute stops current media and drops the rest of a game queue',async()=>{
 const {audio,media}=setup(),game=audio.channel('room-a');await game.play(['one.mp3','two.mp3']);const ended=media.onended;audio.setEnabled(false);assert.equal(media.paused,true);assert.equal(media.muted,true);ended?.();assert.deepEqual(media.calls,['one.mp3']);assert.equal(await audio.play('preview'),false);audio.setEnabled(true);assert.equal(media.paused,true);assert.deepEqual(media.calls,['one.mp3']);
});
test('late play resolution cannot revive a muted player',async()=>{
 const {audio,media}=setup();let finish;media.behavior=()=>new Promise(r=>finish=()=>{media.paused=false;r()});const p=audio.channel('room').play(['roll.mp3']);audio.setEnabled(false);finish();assert.equal(await p,false);assert.equal(media.paused,true);assert.equal(media.muted,true);
});
test('channel cleanup never stops another room or explicit preview',async()=>{
 const {audio,media}=setup();const old=audio.channel('old'),next=audio.channel('next');await old.play(['old.mp3']);await next.play(['next.mp3']);old.stop();assert.equal(media.paused,false);assert.equal(media.src,'next.mp3');await audio.play('preview');next.stop();assert.equal(media.paused,false);
});
test('queued game clips share the same media element',async()=>{
 const {audio,media}=setup();await audio.channel('room').play(['roll.mp3','card.mp3']);media.onended();await Promise.resolve();assert.deepEqual(media.calls,['roll.mp3','card.mp3']);audio.dispose();assert.equal(media.paused,true);assert.equal(media.removed,true);
});
test('a failed common CDN cue falls back to the same synthesized WAV',async()=>{
 const {audio,media}=setup();audio.setAssetsBase('https://cdn.example.test');media.behavior=()=>media.src.startsWith('https:')?Promise.reject(Error('network')):Promise.resolve();assert.equal(await audio.play('take'),true);assert.equal(media.calls.length,2);assert.ok(media.src.startsWith('data:audio/wav;base64,'));
});
test('blocked custom audio returns false without throwing or replaying the queue',async()=>{
 const {audio,media}=setup();media.behavior=()=>Promise.reject(Error('autoplay denied'));assert.equal(await audio.channel('room').play(['roll.mp3','card.mp3']),false);assert.equal(media.paused,true);assert.deepEqual(media.calls,['roll.mp3']);
});
test('explicit preview and victory are not cut off by a late action cue',async()=>{
 const {audio,media}=setup();await audio.play('finish');const src=media.src;assert.equal(await audio.play('move'),false);assert.equal(media.src,src);audio.stop();await audio.play('preview');const preview=media.src;assert.equal(await audio.channel('room').play(['roll.mp3']),false);assert.equal(media.src,preview);
});
// New channel operation: CDN and local URLs are alternatives, never two
// consecutive copies. A delayed failure must not restart audio after mute.
test('game fallback sources play exactly one successful clip',async()=>{
 const {audio,media}=setup();const game=audio.channel('poker');assert.equal(await game.playWithFallback(['cdn.mp3','local.mp3']),true);media.onended();await Promise.resolve();assert.deepEqual(media.calls,['cdn.mp3']);
});
test('a failed game CDN clip uses its local alternative once',async()=>{
 const {audio,media}=setup();media.behavior=()=>media.src==='cdn.mp3'?Promise.reject(Error('network')):Promise.resolve();assert.equal(await audio.channel('poker').playWithFallback(['cdn.mp3','local.mp3']),true);assert.deepEqual(media.calls,['cdn.mp3','local.mp3']);media.onended();await Promise.resolve();assert.equal(media.calls.length,2);
});
test('late game fallback cannot resume a muted or replaced action',async()=>{
 const {audio,media}=setup();let reject;media.behavior=()=>new Promise((_,r)=>reject=r);const p=audio.channel('poker').playWithFallback(['cdn.mp3','local.mp3']);audio.setEnabled(false);audio.setEnabled(true);reject(Error('late'));assert.equal(await p,false);assert.deepEqual(media.calls,['cdn.mp3']);
});

// Music migration failure inventory (before implementation): game creates its
// own audio route; effects cancel background music; global mute or room cleanup
// leaves music playing; denied/CDN music throws; stale room stops newer music.
function musicSetup(){const voices=[];const audio=new GameAudio(()=>{const m=new Media();voices.push(m);return m});audio.mount();return {audio,voices};}
test('game music uses a separate looping native platform channel, effects coexist',async()=>{
 const {audio,voices}=musicSetup(),port=audio.channel('pairs');
 assert.equal(await port.music(['bg.mp3','local.mp3']),true);
 assert.equal(voices.length,2);assert.equal(voices[1].dataset.platformAudio,'');assert.equal(voices[1].loop,true);assert.equal(voices[1].volume,.15);
 await port.play(['match.wav']);assert.equal(voices[1].paused,false);assert.equal(voices[0].src,'match.wav');
 await port.music(['bg.mp3','local.mp3']);assert.deepEqual(voices[1].calls,['bg.mp3']);
 port.stopMusic();assert.equal(voices[1].paused,true);assert.equal(voices[0].paused,false);
});
test('global mute cancels music and effects; unmute never replays either',async()=>{
 const {audio,voices}=musicSetup(),port=audio.channel('pairs');await port.music(['bg.mp3']);await port.play(['match.wav']);audio.setEnabled(false);
 assert.ok(voices.every(m=>m.paused&&m.muted));audio.setEnabled(true);assert.ok(voices.every(m=>m.paused));
 audio.dispose();assert.ok(voices.every(m=>m.removed));
});
test('music belongs to its room and falls back to local media',async()=>{
 const {audio,voices}=musicSetup(),old=audio.channel('old'),next=audio.channel('new');await old.music(['old.mp3']);
 voices[1].behavior=()=>voices[1].src==='cdn.mp3'?Promise.reject(Error('cdn')):Promise.resolve();
 assert.equal(await next.music(['cdn.mp3','local.mp3']),true);assert.equal(voices[1].src,'local.mp3');old.stop();assert.equal(voices[1].paused,false);next.stop();assert.equal(voices[1].paused,true);
});
test('pending music cannot return after mute',async()=>{
 const {audio,voices}=musicSetup();await audio.channel('pairs').music(['bg.mp3']);let finish;
 voices[1].behavior=()=>new Promise(r=>finish=r);const p=audio.channel('pairs').music(['next.mp3']);audio.setEnabled(false);finish();assert.equal(await p,false);assert.equal(voices[1].paused,true);
});
