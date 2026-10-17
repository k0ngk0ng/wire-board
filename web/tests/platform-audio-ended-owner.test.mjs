// Failure observed with real finite basketball music75: next() drops the
// completed Task, so owner stop/dispose previously could not clear its source.
import test from 'node:test';
import assert from 'node:assert/strict';
import { GameAudio } from '../src/audio.ts';

class Media {
  dataset={};src='';muted=false;paused=true;currentTime=0;ended=false;onended=null;onerror=null;calls=[];
  play(){this.calls.push(this.src);this.paused=false;this.ended=false;return Promise.resolve();}
  pause(){this.paused=true;}
  removeAttribute(name){if(name==='src')this.src='';}
  load(){this.currentTime=0;this.ended=false;}
  remove(){}
  finish(){this.paused=true;this.ended=true;this.onended?.();}
}
function setup(){const voices=[];const audio=new GameAudio(()=>{const m=new Media();voices.push(m);return m;});return{audio,voices};}

test('owner stop clears an already-ended effect source without reviving it',async()=>{
  const {audio,voices}=setup(),p=audio.channel('room');
  await p.playWithFallback(['effect.wav']);voices[0].finish();await Promise.resolve();
  assert.equal(voices[0].src,'effect.wav');p.stop();
  assert.equal(voices[0].src,'');assert.equal(voices[0].paused,true);
  p.pause();assert.equal(await p.resume(),false);assert.equal(voices[0].calls.length,1);
});

test('finite background cleanup remains owner scoped after the Task ends',async()=>{
  const {audio,voices}=setup(),old=audio.channel('old'),next=audio.channel('next');
  await old.music(['ending.wav'],{loop:false});voices[0].finish();await Promise.resolve();
  next.stop();assert.equal(voices[0].src,'ending.wav');
  old.stopMusic();assert.equal(voices[0].src,'');
  await next.music(['next.wav']);old.stop();
  assert.equal(voices[0].src,'next.wav');assert.equal(voices[0].paused,false);
  voices[0].finish();await Promise.resolve();old.stop();assert.equal(voices[0].src,'next.wav');
  next.stop();assert.equal(voices[0].src,'');
});

test('mute and page/room cleanup clear completed media and resume cannot restore it',async()=>{
  for(const cleanup of ['mute','global-stop','owner-stop']) {
    const {audio,voices}=setup(),p=audio.channel('room');
    await p.music(['ending.wav'],{loop:false});voices[0].finish();await Promise.resolve();p.pause();
    if(cleanup==='mute'){audio.setEnabled(false);audio.setEnabled(true);}
    else if(cleanup==='global-stop')audio.stop();
    else p.stop();
    assert.equal(voices[0].src,'',cleanup);assert.equal(await p.resume(),false,cleanup);
  }
});

test('repeated unlock does not revive a completed clip or prevent its owner cleanup',async()=>{
  const {audio,voices}=setup(),old=audio.channel('old');
  await old.play(['effect.wav']);voices[0].finish();await Promise.resolve();
  const unlocking=audio.unlock();old.stop();
  assert.equal(await unlocking,true);assert.equal(voices[0].src,'');assert.equal(voices[0].calls.length,1);
});
