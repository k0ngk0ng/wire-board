// Failure cases before implementation: cooldown jumps backwards on packet updates;
// pause/finished/hidden time advances; arrow stacks are mislabeled as seconds;
// ready icon stays partially covered; corrupt/zero durations yield NaN.
import test from 'node:test';
import assert from 'node:assert/strict';
import { skillProgress } from '../web/skill-progress.ts';
const timed={cooldownMS:6000,readyInMS:4500,stacks:0,maxStacks:0};
test('cooldown interpolates elapsed battle time and reaches ready',()=>{
 assert.deepEqual(skillProgress(timed,0,true),{progress:.25,remainingMS:4500,ready:false,stacks:false});
 assert.equal(skillProgress(timed,1500,true).progress,.5);
 assert.equal(skillProgress(timed,5000,true).progress,1);
 assert.equal(skillProgress(timed,5000,true).ready,true);
});
test('paused time freezes while new snapshot resumes from authoritative remainder',()=>{
 assert.equal(skillProgress(timed,5000,false).remainingMS,4500);
 assert.equal(skillProgress({...timed,readyInMS:3000},0,true).progress,.5);
});
test('arrow stacks use shots, not a fake cooldown',()=>{
 const result=skillProgress({cooldownMS:0,readyInMS:0,stacks:2,maxStacks:3},5000,true);
 assert.equal(result.progress,2/3);assert.equal(result.stacks,true);assert.equal(result.ready,false);
});
test('zero, out-of-range and invalid time are bounded',()=>{
 for(const value of [NaN,-50,Infinity,0,99999]){
  const result=skillProgress({...timed,readyInMS:value},100,true);
  assert.ok(Number.isFinite(result.progress)&&result.progress>=0&&result.progress<=1);
 }
 assert.equal(skillProgress({...timed,cooldownMS:0,readyInMS:0},0,true).ready,true);
});
