// Before implementation: center jitter must not move; outer drag must stay
// captured/clamped; diagonal jitter must not alternate axes; no diagonal action.
import test from 'node:test';
import assert from 'node:assert/strict';
import { joystickIntent } from '../web/joystick.ts';
test('dead zone stops and all four outside directions remain cardinal',()=>{
 assert.equal(joystickIntent(2,3,50,'right').move,'none');
 for(const [x,y,move] of [[100,0,'right'],[-100,0,'left'],[0,100,'down'],[0,-100,'up']]){
  const p=joystickIntent(x,y,50,'none');assert.equal(p.move,move);assert.ok(Math.hypot(p.x,p.y)<=1.00001);
 }
});
test('diagonal noise retains the held axis until a deliberate turn; center releases it',()=>{
 assert.equal(joystickIntent(30,31,50,'right').move,'right');
 assert.equal(joystickIntent(30,40,50,'right').move,'down');
 assert.equal(joystickIntent(31,30,50,'down').move,'down');
 assert.equal(joystickIntent(40,30,50,'down').move,'right');
 assert.equal(joystickIntent(0,0,50,'up').move,'none');
});
test('bad geometry never creates non-finite CSS coordinates or gameplay inputs',()=>{
 for(const args of [[NaN,0,50],[0,Infinity,50],[20,20,0]]){const p=joystickIntent(...args,'none');assert.deepEqual(p,{x:0,y:0,move:'none'})}
});
