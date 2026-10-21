// Preimplementation cases: CAMERA_BOUNDARY_SCENARIOS.md. This changes camera
// policy only; target/trajectory inputs stay untouched above the map.
import test from 'node:test';
import assert from 'node:assert/strict';
import {boundCamera,centeredCamera,zoomAt,panCamera} from '../web/camera.ts';
const near=(a,b)=>assert.ok(Math.abs(a-b)<1e-8,`${a} != ${b}`);
function inside(c,w,h){
 assert.ok(c.x>=0 && c.y>=0,JSON.stringify(c));
 assert.ok(c.x+c.worldViewW<=w+1e-8 && c.y+c.worldViewH<=h+1e-8,JSON.stringify(c));
 assert.ok(c.worldViewW>0 && c.worldViewH>0);
}
test('high arc leaves the map while tracking stops at its top and resumes on descent',()=>{
 const target={x:700,y:-180};
 for(const [w,h] of [[800,450],[288,216]]){
  const top=centeredCamera(w,h,600,target,1600,960);
  assert.equal(top.y,0);assert.deepEqual(target,{x:700,y:-180});inside(top,1600,960);
  const back=centeredCamera(w,h,600,{x:700,y:700},1600,960);
  assert.ok(back.y>0);inside(back,1600,960);
  near(top.worldViewW/top.worldViewH,w/h);
 }
});
test('pan and zoom stay within every map edge without stretching the image',()=>{
 for(const [w,h] of [[1600,960],[960,300],[180,120]]){
  for(const [sw,sh] of [[800,450],[288,216]]){
   const c=centeredCamera(sw,sh,800,{x:w/2,y:h/2},w,h);
   inside(c,w,h);near(c.worldViewW/c.worldViewH,sw/sh);
   for(const dx of [-10000,10000])for(const dy of [-10000,10000]){
    const p=panCamera(c,dx,dy,w,h);inside(p,w,h);
    for(const factor of [.01,100]){
     const z=zoomAt(p,factor,{x:dx>0?1:0,y:dy>0?1:0},w,h);
     inside(z,w,h);near(z.worldViewW/z.worldViewH,sw/sh);
    }
   }
  }
 }
 const outside=boundCamera({x:-100,y:-320,worldViewW:1600,worldViewH:1200},900,400);
 inside(outside,900,400);near(outside.worldViewW/outside.worldViewH,4/3);
});
test('interior zoom preserves cursor world coordinates and does not mutate input',()=>{
 const c={x:200,y:100,worldViewW:800,worldViewH:450};
 const z=zoomAt(c,.8,{x:.25,y:.75},1600,960);
 near(c.x+c.worldViewW*.25,z.x+z.worldViewW*.25);
 near(c.y+c.worldViewH*.75,z.y+z.worldViewH*.75);
 assert.deepEqual(c,{x:200,y:100,worldViewW:800,worldViewH:450});
});
