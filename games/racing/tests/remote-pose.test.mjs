// Before implementation: the 3D driver view must not snap opponents at the
// snapshot rate; observers keep their delayed interpolation, scoring untouched.
import test from 'node:test';
import assert from 'node:assert/strict';
import { remotePose } from '../web/remote-pose.ts';
const track={width:1000,height:1000,roadWidth:180,points:[{x:100,y:100},{x:900,y:100},{x:900,y:900},{x:100,y:900}]};
const car={seat:1,x:300,y:100,vx:100,vy:0,angle:0,speed:100,steer:0,throttle:0,brake:false,drift:false,skin:1,lap:2,checkpoint:3,rank:2,progress:900,finishAt:0,resetUntil:0};
test('remote pose advances between identical authoritative snapshots and remains bounded',()=>{
 const sample={older:{...car,x:290},current:car,track,previousAt:900,currentAt:1000,visualAt:1100,driving:true};
 const first=remotePose({...sample,visualAt:1010}),later=remotePose(sample);
 assert.ok(later.x>first.x+5);assert.ok(later.x<325);assert.equal(later.lap,2);assert.equal(car.x,300);
 assert.deepEqual(remotePose({...sample,visualAt:5000}),remotePose({...sample,visualAt:1250}));
});
test('observer interpolation and reset/finish do not invent progress or sweep across respawn',()=>{
 const sample={older:{...car,x:290},current:car,track,previousAt:900,currentAt:1000,visualAt:950,driving:false};
 assert.equal(remotePose(sample).x,295);
 for(const change of [{finishAt:1000},{resetUntil:2000}])assert.equal(remotePose({...sample,current:{...car,...change},visualAt:1200,driving:true}).x,300);
});
