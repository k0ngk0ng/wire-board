import assert from "node:assert/strict";
import test from "node:test";
import { classicGaugeReading, classicGaugeAngle, classicGaugeTicks, classicGaugeMode } from "../web/classic-gauges.ts";
import { classicSkidStrength, classicSkidQuad, classicSkidOnSurface, ClassicSkidTrail } from "../web/classic-skids.ts";

test("speed and tachometer needles follow independent quantities with bounded sweep and reverse gear",()=>{
  const reading=classicGaugeReading({speed:500,rpm:6200,gear:3},6500);
  assert.equal(reading.speed,180);assert.equal(reading.rpm,6200);assert.equal(reading.gear,"3");
  assert.equal(reading.speedAngle,classicGaugeAngle(180,400));assert.equal(reading.rpmAngle,classicGaugeAngle(6200,10000));
  assert.equal(reading.redlineAngle,classicGaugeAngle(6500,10000));
  assert.equal(classicGaugeReading({speed:200,rpm:2300,gear:-1},7500).gear,"R");
  assert.equal(classicGaugeAngle(0,400),-130);assert.equal(classicGaugeAngle(400,400),130);
  assert.equal(classicGaugeAngle(Infinity,400),-130);assert.equal(classicGaugeAngle(-10,400),-130);
  assert.equal(classicGaugeAngle(999,400),130);
});
test("dial ticks have readable major values and finite positions at both ends",()=>{
  const ticks=classicGaugeTicks(400,40,10);
  assert.deepEqual(ticks.filter(t=>t.major).map(t=>t.value),[0,40,80,120,160,200,240,280,320,360,400]);
  assert.equal(ticks[0].angle,-130);assert.equal(ticks.at(-1).angle,130);
  assert(ticks.every(t=>Number.isFinite(t.angle)));
});
test("narrow portrait screens prioritize compact driving readouts while landscape keeps analogue dials",()=>{
  for(const width of [320,390,430,480])assert.equal(classicGaugeMode(width,844),"compact");
  for(const [width,height]of [[481,844],[844,390],[932,430],[1280,1000],[1920,1080]])assert.equal(classicGaugeMode(width,height),"analog");
});
test("gear is a small independent number or reverse with no drive prefix",()=>{
  for(const gear of [1,2,3,4,5,6,7,8])assert.equal(classicGaugeReading({speed:0,gear},6500).gear,String(gear));
  for(const gear of [undefined,0,NaN])assert.equal(classicGaugeReading({speed:0,gear},6500).gear,"1");
  assert.equal(classicGaugeReading({speed:0,gear:-1},6500).gear,"R");
});
const car=(patch={})=>({vx:250,vy:0,angle:0,speed:250,brake:false,brakeAmount:0,drift:false,offroad:false,finishAt:0,protected:false,resetUntil:0,motion:{longAccel:0,handbrake:0,brake:0},...patch});
test("ordinary aligned turning, coasting, gentle braking and parking do not paint",()=>{
  for(const c of [car(),car({steer:1,motion:{longAccel:-.5,lateralAccel:8,yawRate:.4}}),car({brake:true,brakeAmount:.3,motion:{longAccel:-2,brake:.3}}),car({speed:10,vx:10,drift:true})])
    assert.deepEqual(classicSkidStrength(c,1000),{front:0,rear:0,cause:"none"});
});
test("actual sideslip, hard braking and handbrake have independent axle activation",()=>{
  const slide=classicSkidStrength(car({vy:70}),1000);assert(slide.rear>0);assert.equal(slide.cause,"slip");
  const braking=classicSkidStrength(car({brake:true,brakeAmount:1,motion:{longAccel:-9,brake:1}}),1000);assert(braking.front>.5&&braking.rear>.5);assert.equal(braking.cause,"brake");
  const hand=classicSkidStrength(car({drift:true,motion:{handbrake:1,longAccel:-1}}),1000);assert.equal(hand.front,0);assert(hand.rear>.5);assert.equal(hand.cause,"handbrake");
});
test("protection, finish and reset suppress tracks even during a skid",()=>{
  for(const patch of [{protected:true},{finishAt:10},{resetUntil:2000}]) {
    const s=classicSkidStrength(car({vy:90,drift:true,...patch}),1000);assert.equal(s.front+s.rear,0);
  }
});
test("a car outside VROAD width still marks actual asphalt during a genuine skid",()=>{
  const road=classicSkidStrength(car({vy:90,drift:true}),1000);
  const shoulderAsphalt=classicSkidStrength(car({vy:90,drift:true,offroad:true}),1000);
  assert(road.rear>0);assert.deepEqual(shoulderAsphalt,road,"offroad is distance to VROAD, not terrain material");
});
test("only known loose materials suppress black rubber; unknown material never means grass",()=>{
  for(const surface of [2,3,4,11,5,13,9,15])assert.equal(classicSkidOnSurface(true,surface),false,`surface ${surface}`);
  for(const surface of [0,1,6,7,8,10,12,14,16,255,-1])assert.equal(classicSkidOnSurface(true,surface),true,`surface ${surface}`);
  assert.equal(classicSkidOnSurface(false,0),false);assert.equal(classicSkidOnSurface(false,255),false);
});
const point=(x,z=0,y=0,n={x:0,y:1,z:0})=>({x,y,z,nx:n.x,ny:n.y,nz:n.z});
test("a wheel ribbon lies on the supplied compound road plane and keeps its physical width",()=>{
  const normal={x:0,y:Math.SQRT1_2,z:-Math.SQRT1_2};
  const a=point(0,0,0,normal),b=point(10,4,4,normal),q=classicSkidQuad(a,b,2.2);
  assert.equal(q.length,18);assert(q.every(Number.isFinite));
  const vertices=Array.from({length:6},(_,i)=>q.slice(i*3,i*3+3));
  assert.ok(Math.abs(Math.hypot(...vertices[0].map((v,i)=>v-vertices[1][i]))-2.2)<1e-8);
  for(const [x,y,z] of vertices)assert.ok(Math.abs(y-z)<1e-8);
  assert.equal(classicSkidQuad(a,a,2.2),null);
});
const contact=(wheel,x,front=false)=>({wheel,front,width:2.2,point:point(x,wheel===0?-8:8)});
const active={front:0,rear:1,cause:"handbrake"};
test("two original wheel centres create separate tracks without inventing a shared axle width",()=>{
  const trail=new ClassicSkidTrail(8,1000);
  assert.equal(trail.sample(2,[contact(0,0),contact(1,0)],active,100).length,0);
  const marks=trail.sample(2,[contact(0,4),contact(1,4)],active,120);
  assert.equal(marks.length,2);assert.equal(marks[0].seat,2);assert.notDeepEqual(marks[0].vertices,marks[1].vertices);
  assert.equal(marks[0].wheel,0);assert.equal(marks[1].wheel,1);
});
test("marking stops cleanly and teleports or stale sample gaps do not draw connecting lines",()=>{
  const trail=new ClassicSkidTrail(8,1000);
  trail.sample(0,[contact(0,0)],active,100);
  assert.equal(trail.sample(0,[contact(0,500)],active,120).length,0);
  assert.equal(trail.sample(0,[contact(0,502)],active,1000).length,0);
  trail.sample(0,[contact(0,503)],{front:0,rear:0,cause:"none"},1020);
  assert.equal(trail.sample(0,[contact(0,504)],active,1040).length,0);
});
test("missing terrain or a loose surface cuts wheel continuity rather than floating or bridging a gap",()=>{
  const trail=new ClassicSkidTrail(8,1000);
  trail.sample(0,[contact(0,0)],active,100);
  assert.equal(trail.sample(0,[{...contact(0,2),valid:classicSkidOnSurface(false,0)}],active,120).length,0);
  assert.equal(trail.sample(0,[contact(0,4)],active,140).length,0);
  assert.equal(trail.sample(0,[{...contact(0,6),valid:classicSkidOnSurface(true,3)}],active,160).length,0);
  assert.equal(trail.sample(0,[contact(0,8)],active,180).length,0);
});
test("cache is a bounded ring, old segments expire and reduced motion releases trails",()=>{
  const trail=new ClassicSkidTrail(4,1000);
  for(let i=0;i<12;i++)trail.sample(0,[contact(0,i*2)],active,100+i*20);
  assert.equal(trail.live(400).length,4);assert(trail.written>4);assert.equal(trail.live(2000).length,0);
  trail.clear();assert.equal(trail.live(400).length,0);assert.equal(trail.written,0);
  assert.equal(trail.sample(0,[contact(0,25)],active,500,true).length,0);
  assert.equal(trail.sample(0,[contact(0,27)],active,520).length,0);
});
test("one observer sees independent trails for all vehicles and removing a vehicle drops only its continuity",()=>{
  const trail=new ClassicSkidTrail(16,1000);
  for(const seat of [0,1])trail.sample(seat,[contact(0,0)],active,100);
  assert.equal(trail.sample(0,[contact(0,3)],active,120).length,1);
  assert.equal(trail.sample(1,[contact(0,4)],active,120).length,1);
  trail.remove(0);
  assert.equal(trail.sample(0,[contact(0,6)],active,140).length,0);
  assert.equal(trail.sample(1,[contact(0,7)],active,140).length,1);
});
