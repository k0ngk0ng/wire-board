// Failure inventory: realtime-efficiency/MOTION.md. Variable source frame sizes,
// fixed team palettes, no replay cursor on refresh, missing CDN/local fallback.
import test from 'node:test';
import assert from 'node:assert/strict';
import { frameAt, clipDuration, avatarSourceCandidates, normalizeAvatar } from '../web/avatar-animation.ts';
import { avatarCatalog, loadAvatar, getAvatarImage } from '../web/avatar-art.ts';
test('original five-frame walk follows its six-step sequence; death keeps variable frame rectangles',()=>{
 const a=avatarCatalog.avatars[0];const walk=a.clips.down;
 assert.deepEqual([0,60,120,180,240,300,360].map(t=>frameAt(walk,t,true)),[0,1,2,0,3,4,0]);
 const die=a.clips.death;
 assert.equal(die.frames[0].h,70);assert.equal(die.frames[2].h,136);
 assert.equal(frameAt(die,clipDuration(die)+500,false),13);
 assert.equal(frameAt(die,-100,false),0);
});
test('all four appearances have fixed red/blue resources for every direction and life state',()=>{
 assert.equal(avatarCatalog.avatars.length,4);
 for(const a of avatarCatalog.avatars)for(const c of Object.values(a.clips)){
  assert.match(c.red,/-red\.png$/);assert.match(c.blue,/-blue\.png$/);
  assert.notEqual(c.red,c.blue);assert.ok(c.frames.length>0);
 }
 for(const invalid of [-1,4,NaN,1.5,undefined])assert.equal(normalizeAvatar(invalid),0);
});
test('avatar atlas caches downloads, preserves blue selection and falls back locally',async()=>{
 const seen=[];
 globalThis.Image=class{complete=false;naturalWidth=0;set src(url){seen.push(url);queueMicrotask(()=>{if(url.startsWith('https:'))this.onerror?.();else{this.complete=true;this.naturalWidth=174;this.onload?.()}})}};
 const result=await loadAvatar('https://example.invalid/atlas',1,1);
 assert.equal(result.loaded,6);assert.ok(getAvatarImage('https://example.invalid/atlas',1,1,'right'));
 assert.ok(seen.every(url=>url.endsWith('-blue.png')));
 const count=seen.length;await loadAvatar('https://example.invalid/atlas',1,1);assert.equal(seen.length,count);
 assert.deepEqual(avatarSourceCandidates('', 'avatar-0-up-red.png'),['/bubbleparty/v3/avatars/avatar-0-up-red.png']);
});
test('fully failed images stay unavailable for the team-colored vector fallback',async()=>{
 globalThis.Image=class{complete=false;naturalWidth=0;set src(url){queueMicrotask(()=>this.onerror?.())}};
 const result=await loadAvatar('https://example.invalid/failure',2,0);
 assert.equal(result.loaded,0);assert.equal(getAvatarImage('https://example.invalid/failure',2,0,'idle'),null);
});
