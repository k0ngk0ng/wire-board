import test from 'node:test';
import assert from 'node:assert/strict';
import { racingResetFeedback, racingResetSeconds } from '../web/racing-reset.ts';
const ready={active:true,canReset:true,physics:3,phase:'racing',spectating:false,autoPlay:false,finished:false,busy:false};
test('v3 countdown ends with reset protection and legacy keeps its old cooldown',()=>{
 assert.equal(racingResetSeconds(3000,1000,3),2);
 assert.equal(racingResetSeconds(3000,2500,3),1);
 assert.equal(racingResetSeconds(3000,3000,3),0);
 assert.equal(racingResetSeconds(3000,4000,3),0);
 assert.equal(racingResetSeconds(3000,4000,0),2);
 assert.equal(racingResetSeconds(0,1000,3),0);
});
test('valid authority permission has no client speed or distance gate',()=>{
 assert.equal(racingResetFeedback(ready),'');
 assert.equal(racingResetFeedback({...ready,phase:'finishing'}),'');
});
test('unavailable default R explains countdown or current reset without legacy limits',()=>{
 assert.equal(racingResetFeedback({...ready,canReset:false,phase:'countdown'}),'发车后可复位');
 assert.equal(racingResetFeedback({...ready,canReset:false}),'正在复位，请稍候');
 assert.equal(racingResetFeedback({...ready,canReset:false,physics:0}),'请减速或等待复位冷却');
});
test('ownership and completion reasons do not silently swallow R',()=>{
 assert.equal(racingResetFeedback({...ready,finished:true,active:false}),'已经完赛');
 assert.equal(racingResetFeedback({...ready,autoPlay:true,active:false}),'恢复手动驾驶后可复位');
 assert.equal(racingResetFeedback({...ready,spectating:true,active:false}),'观战时无法复位');
 assert.equal(racingResetFeedback({...ready,busy:true,active:false}),'操作正在提交，请稍候');
 assert.equal(racingResetFeedback({...ready,active:false}),'当前无法复位');
});
