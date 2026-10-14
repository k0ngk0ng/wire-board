// Before distance-based cue control: time alone must not produce power; forward
// or sideways movement must not shoot; portrait must use the rotated shot axis;
// pulling farther increases power, returning cancels; cue-tip radius is bounded.
import test from 'node:test';
import assert from 'node:assert/strict';
import {pullPower,cueTip} from '../web/cue-input.ts';
test('pull distance sets stable power and caps at full strength',()=>{
 const a={x:100,y:100};assert.equal(pullPower(a,a,0,false,100),0);
 assert.equal(pullPower(a,{x:120,y:100},0,false,100),0);
 assert.equal(pullPower(a,{x:100,y:30},0,false,100),0);
 const low=pullPower(a,{x:80,y:100},0,false,100),high=pullPower(a,{x:30,y:100},0,false,100);
 assert.ok(low>0&&high>low);assert.equal(high,pullPower(a,{x:30,y:100},0,false,100));assert.equal(pullPower(a,{x:-300,y:100},0,false,100),1);
 assert.equal(pullPower(a,{x:98,y:100},0,false,100),0);
});
test('portrait pulling follows the rotated cue direction',()=>{
 const a={x:100,y:100};assert.equal(pullPower(a,{x:100,y:40},0,true,100),pullPower(a,{x:40,y:100},0,false,100));
 assert.equal(pullPower(a,{x:100,y:160},0,true,100),0);
});
test('white ball tip selection preserves signs and stays within the legal circle',()=>{
 assert.deepEqual(cueTip(0,0),{side:0,top:0});assert.deepEqual(cueTip(.3,.4),{side:.3,top:.4});
 for(const x of [-3,-.9,.9,3])for(const y of [-3,-.9,.9,3]){const p=cueTip(x,y);assert.ok(p.side*p.side+p.top*p.top<=.81+1e-12);assert.equal(Math.sign(p.side),Math.sign(x));assert.equal(Math.sign(p.top),Math.sign(y));}
});
