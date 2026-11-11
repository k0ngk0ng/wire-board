import assert from "node:assert/strict";
import test from "node:test";
import { classicCameraClearance, advanceClassicCameraLift } from "../web/classic-camera-clearance.ts";

// Written before the neighbour-camera implementation. Failure cases: the
// renderer follows its own bounds, a bridge-level car raises a lower camera,
// rear view is treated differently, a near-edge lift jumps, or lift never falls.
const eye = {x:0,y:25.3,z:-69}; // decimetres: current 2.53m-high chase camera.
const rearCar = {seat:2,min:{x:-10,y:0,z:-75},max:{x:10,y:13,z:-29}};
const ownCar = {seat:0,min:{x:-10,y:0,z:-23},max:{x:10,y:13,z:23}};

test("actual rear-car roof receives safe clearance without moving or hiding vehicles",()=>{
  const original=JSON.stringify([rearCar,ownCar]);
  const result=classicCameraClearance(eye,0,[rearCar,ownCar]);
  assert.ok(Math.abs(result.targetLift-9.7)<1e-10);
  assert.ok(Math.abs(result.minimumLift-2.7)<1e-10);
  assert.deepEqual(result.neighbors.map(x=>x.seat),[2]);
  assert.equal(result.neighbors[0].roof,13);
  assert.equal(result.neighbors[0].footprintDistance,0);
  assert.equal(JSON.stringify([rearCar,ownCar]),original);
});

test("self, clear space, and a separate bridge level do not lift the camera",()=>{
  assert.equal(classicCameraClearance({...eye,z:0},0,[ownCar]).targetLift,0);
  assert.equal(classicCameraClearance({...eye,x:30},0,[rearCar]).targetLift,0);
  const bridge={seat:3,min:{x:-10,y:45,z:-75},max:{x:10,y:58,z:-29}};
  assert.equal(classicCameraClearance(eye,0,[bridge]).targetLift,0);
  assert.equal(classicCameraClearance({...eye,y:40},0,[rearCar]).targetLift,0);
});

test("front view uses the same physical bounds and edge influence is continuous",()=>{
  const front={seat:4,min:{x:-10,y:0,z:29},max:{x:10,y:13,z:75}};
  assert.equal(classicCameraClearance({...eye,z:69},0,[front]).targetLift,
               classicCameraClearance(eye,0,[rearCar]).targetLift);
  const inside=classicCameraClearance({...eye,x:10},0,[rearCar]).targetLift;
  const edge=classicCameraClearance({...eye,x:12},0,[rearCar]).targetLift;
  const outside=classicCameraClearance({...eye,x:14},0,[rearCar]).targetLift;
  assert.ok(inside>edge&&edge>outside);
  assert.ok(Math.abs(edge-inside*.5)<1e-10);
  assert.equal(outside,0);
});

test("lift is bounded, rises safely, and returns continuously after leaving",()=>{
  const target=classicCameraClearance(eye,0,[rearCar]);
  let lift=advanceClassicCameraLift(0,target,1/60,false);
  assert.ok(lift>=target.minimumLift&&lift<target.targetLift);
  for(let i=0;i<60;i++)lift=advanceClassicCameraLift(lift,target,1/60,false);
  assert.ok(Math.abs(lift-target.targetLift)<.001);
  const before=lift;
  lift=advanceClassicCameraLift(lift,{targetLift:0,minimumLift:0,neighbors:[]},1/60,false);
  assert.ok(lift>0&&lift<before);
  const huge=classicCameraClearance(eye,0,[{...rearCar,max:{...rearCar.max,y:60}}]);
  assert.equal(huge.targetLift,24);
  assert.equal(advanceClassicCameraLift(0,target,1/60,true),target.targetLift);
});
