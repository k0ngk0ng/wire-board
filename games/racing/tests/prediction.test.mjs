// Failure inventory BEFORE implementation: local steering waits for network;
// long network gaps let prediction escape the authoritative frame; drift and
// braking ignored; local extrapolation writes lap/rank; cars move in countdown,
// after finish or while reset; road rendering and offroad prediction disagree.
import test from "node:test";
import assert from "node:assert/strict";
import { predictCar, roadDistance } from "../web/prediction.ts";
const track = {
  width: 1000,
  height: 1000,
  roadWidth: 160,
  points: [
    { x: 100, y: 100 },
    { x: 900, y: 100 },
    { x: 900, y: 900 },
    { x: 100, y: 900 },
  ],
};
const c = {
  seat: 0,
  x: 300,
  y: 100,
  vx: 150,
  vy: 0,
  angle: 0,
  speed: 150,
  steer: 0,
  throttle: 1,
  brake: false,
  drift: false,
  skin: 0,
  seq: 1,
  lap: 2,
  checkpoint: 1,
  progress: 200,
  rank: 2,
  finishAt: 0,
  offroad: false,
  lastInputAt: 0,
  idleAt: 0,
  resetUntil: 0,
};
const input = { steer: 0, throttle: 1, brake: false, drift: false };
test('higher road speeds reduce yaw, with the same direction and analogue proportion',()=>{
 let previous=Infinity;
 for(const speed of [65,130,260,520,900]){
  const car={...c,vx:speed,speed};
  const right=predictCar(car,{...input,steer:1},track,20,1000).angle;
  assert.ok(right>0&&right<previous,{speed,right,previous});previous=right;
  assert.ok(Math.abs(predictCar(car,{...input,steer:-1},track,20,1000).angle+right)<1e-12);
  assert.ok(Math.abs(predictCar(car,{...input,steer:.25},track,20,1000).angle-right*.25)<1e-12);
 }
 assert.equal(predictCar({...c,vx:0,speed:0},{...input,throttle:0,steer:1},track,20,1000).angle,0);
});
test("prediction reacts immediately yet leaves all scoring and server inputs unchanged", () => {
  const original = JSON.stringify(c),
    n = predictCar(c, { ...input, steer: 1 }, track, 100, 1000);
  assert.ok(n.x > c.x);
  assert.ok(n.angle > 0.1);
  assert.equal(n.lap, 2);
  assert.equal(n.rank, 2);
  assert.equal(n.seq, 1);
  assert.equal(JSON.stringify(c), original);
});
test("250ms cap prevents a long outage predicting a whole race; braking slows and reset freezes", () => {
  assert.deepEqual(
    predictCar(c, input, track, 9000, 1000),
    predictCar(c, input, track, 250, 1000),
  );
  assert.ok(
    predictCar(c, { ...input, throttle: 0, brake: true }, track, 200, 1000)
      .speed < c.speed,
  );
  assert.equal(
    predictCar({ ...c, finishAt: 500 }, input, track, 200, 1000).x,
    c.x,
  );
  assert.equal(
    predictCar({ ...c, resetUntil: 2000 }, input, track, 200, 1000).x,
    c.x,
  );
});
test("closed road distance matches straight sections including last to first", () => {
  assert.equal(roadDistance(track, { x: 100, y: 300 }), 0);
  assert.equal(roadDistance(track, { x: 500, y: 500 }), 400);
  const grass = predictCar({ ...c, y: 400 }, input, track, 200, 1000);
  assert.equal(grass.offroad, true);
  assert.ok(grass.speed <= 110.01);
});
test("offroad sideways velocity is capped and reverse impacts do not produce reverse throttle", () => {
  const grass = predictCar(
    { ...c, y: 400, vx: 90, vy: 100 },
    { ...input, drift: true },
    track,
    20,
    1000,
  );
  assert.ok(grass.speed <= 110.00001);
  const backwards = predictCar(
    { ...c, vx: -100 },
    { ...input, throttle: 0 },
    track,
    20,
    1000,
  );
  assert.ok(backwards.vx >= 0);
});

test('visible roadside scenery constrains the cosmetic prediction without mutating the authority',()=>{
 const t={...track,props:[{x:250,y:200,radius:25,height:50,kind:'rock'}]};
 const car={...c,x:210,y:200,vx:200,vy:0,angle:0,speed:200,finishAt:0,resetUntil:0};
 const before=structuredClone(car),p=predictCar(car,{steer:0,throttle:1,brake:false,drift:false},t,200,1000);
 assert.ok(p.x<=202.01,JSON.stringify(p));assert.deepEqual(car,before);
});

test('handbrake preview slows the car as well as releasing lateral grip',()=>{
 const car={...c,x:500,y:100,vx:220,vy:0,angle:0,speed:220,finishAt:0,resetUntil:0};
 const coast=predictCar(car,{steer:0,throttle:0,brake:false,drift:false},track,250,1000);
 const hand=predictCar(car,{steer:0,throttle:0,brake:false,drift:true},track,250,1000);
 assert.ok(hand.speed<coast.speed*.8,{coast:coast.speed,hand:hand.speed});
});

test('automatic throttle cannot accelerate through the service brake',()=>{
 const car={...c,x:500,y:100,vx:0,vy:0,angle:0,speed:0,finishAt:0,resetUntil:0};
 const p=predictCar(car,{steer:0,throttle:1,brake:true,drift:false},track,250,1000);
 assert.equal(p.speed,0);assert.equal(p.x,500);
});

// Failure inventory recorded before changes: actual faster travel, same cap and braking as server.
test('sports-car prediction retains 324 km/h road speed and decelerates on grass',()=>{
 const straight={...track,width:30000,points:[{x:100,y:500},{x:29000,y:500}]};
 const fast={...c,x:200,y:500,vx:900,vy:0,speed:900};
 const result=predictCar(fast,input,straight,200,1000);
 assert.ok(result.speed>=899.9&&result.speed<=900.001,result.speed);
 assert.ok(Math.abs(result.x-fast.x-180)<.01,result.x);
 assert.ok(predictCar({...fast,y:100},input,straight,200,1000).speed<=110.01);
 assert.ok(predictCar(fast,{...input,brake:true},straight,200,1000).speed<300);
});

test('predicted brake pressure matches partial/full and preserves legacy braking',()=>{
 const light=predictCar(c,{...input,brake:true,brakeAmount:.25},track,200,1000);
 const full=predictCar(c,{...input,brake:true,brakeAmount:1},track,200,1000);
 const legacy=predictCar(c,{...input,brake:true},track,200,1000);
 assert.ok(light.speed>full.speed);assert.equal(full.speed,legacy.speed);
});
