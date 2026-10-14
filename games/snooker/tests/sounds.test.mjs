// Written before strength/variation mapping: every hit sounds identical;
// mapping requests missing files or unknown events, or changes across viewers.
import test from 'node:test';
import assert from 'node:assert/strict';
import {soundForImpact} from '../web/sounds.ts';
test('collision energy chooses a file tier and stroke chooses a stable variation',()=>{
 for(const kind of ['ball','cushion']){
  assert.equal(soundForImpact({kind,time:1,ball:0,speed:.1},4),`${kind}-soft-1`);
  assert.equal(soundForImpact({kind,time:1,ball:0,speed:.7},5),`${kind}-medium-2`);
  assert.equal(soundForImpact({kind,time:1,ball:0,speed:2},6),`${kind}-hard-0`);
 }
 assert.equal(soundForImpact({kind:'pot',time:1,ball:19},4),'pocket');
 assert.equal(soundForImpact({kind:'cue',time:0,ball:0},4),'cue');
 assert.equal(soundForImpact({kind:'unknown',time:0,ball:0},4),undefined);
});
