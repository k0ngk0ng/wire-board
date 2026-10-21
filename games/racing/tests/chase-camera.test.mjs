// Failure inventory: docs/research/racing-remake/PLAN.md, written before implementation.
import test from 'node:test';
import assert from 'node:assert/strict';
import { chasePose, projectPoint, automaticGear, RearViewHold } from '../web/chase-camera.ts';
const car={x:100,y:200,angle:0,speed:180};
test('chase camera stays behind and above the car at every heading; forward road projects toward horizon',()=>{
 for(const angle of [0,Math.PI/2,Math.PI,-Math.PI/2]) {
  const c={...car,angle},p=chasePose(c,false,16/9);
  const forward={x:Math.cos(angle),z:Math.sin(angle)};
  assert.ok((p.eye.x-c.x)*forward.x+(p.eye.z-c.y)*forward.z<0);
  assert.ok(p.eye.y>10&&p.eye.y<90);
  const center=projectPoint({x:c.x,y:8,z:c.y},p,1280,720);
  assert.ok(center&&Math.abs(center.x-640)<.001&&center.y>360&&center.y<720,JSON.stringify(center));
  const road=projectPoint({x:c.x+forward.x*350,y:0,z:c.y+forward.z*350},p,1280,720);
  assert.ok(road&&road.y<center.y);
 }
});
test('rear view reverses only the camera, keeps vehicle state unchanged and is valid at 320px',()=>{
 const before=structuredClone(car),front=chasePose(car,false,320/430),back=chasePose(car,true,320/430);
 assert.ok(front.eye.x<car.x&&back.eye.x>car.x);assert.ok(back.target.x<car.x);
 assert.deepEqual(car,before);
 const p=projectPoint({x:car.x,y:8,z:car.y},back,320,430);assert.ok(p&&p.x>0&&p.x<320&&p.y>0&&p.y<430);
 assert.equal(projectPoint({x:back.eye.x+2000,y:10,z:car.y},back,320,430),null);
});
test('rear view holds are independent; release/cancel/blur cannot latch the view',()=>{
 const h=new RearViewHold();assert.equal(h.active,false);h.press('KeyB');h.press('pointer:1');assert.equal(h.active,true);
 h.release('pointer:2');h.release('KeyB');assert.equal(h.active,true);h.release('pointer:1');assert.equal(h.active,false);
 h.press('KeyB');h.clear();assert.equal(h.active,false);
});
test('automatic gearbox is bounded and has no player-controlled gear state',()=>{
 assert.equal(automaticGear(0),1);let previous=1;
 for(let speed=0;speed<500;speed+=1){const gear=automaticGear(speed);assert.ok(gear>=previous&&gear<=6);previous=gear}
 for(const speed of [NaN,-100,Infinity])assert.ok(Number.isInteger(automaticGear(speed)));
});

test('near-plane clipping keeps roads beneath the camera instead of dropping the whole segment',async()=>{
 const {clipPolygon}=await import('../web/chase-camera.ts');
 const pose={eye:{x:0,y:2,z:0},target:{x:10,y:2,z:0},fov:60,near:2};
 const points=[{x:1,y:0,z:-3},{x:8,y:0,z:-3},{x:8,y:0,z:3},{x:1,y:0,z:3}];
 const old=structuredClone(points),clipped=clipPolygon(points,pose);
 assert.equal(clipped.length,4);assert.ok(clipped.every(p=>projectPoint(p,pose,320,400)));assert.deepEqual(points,old);
 assert.equal(clipPolygon(points.map(p=>({...p,x:-p.x})),pose).length,0);
});

test('automatic gears and camera span the new road speed range',()=>{
 assert.equal(automaticGear(310),3);
 assert.equal(automaticGear(900),6);
 assert.ok(chasePose({...car,speed:600},false,16/9).fov < chasePose({...car,speed:900},false,16/9).fov);
});
