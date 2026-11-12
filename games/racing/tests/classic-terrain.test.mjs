// Failure inventory written before implementation in classic_terrain_test.go:
// distance cannot invent material/drag; true triangle/layer and wheel positions
// drive attachment; missing/unknown terrain is neutral.
import test from 'node:test';
import assert from 'node:assert/strict';
import {classicRoadSurface} from '../web/classic-road.ts';
import * as terrain from '../web/classic-terrain.ts';
const plane=(surface,height,left,right)=>({segment:0,surface,vertices:[[left,height,-100],[right,height,-100],[right,height,100],[left,height,100]],faces:[[0,1,2,3]]});
const wrap=surfaces=>({version:1,id:'test',unit:'decimetres',geometry:'a'.repeat(64),sourceSha256:'b'.repeat(64),surfaces});
test('distance does not impose grip or rolling drag',()=>{
 for(const side of [-10000,-300,0,300,10000]){const s=classicRoadSurface({ny:1,nx:0,nz:0,left:60,right:60,side},0);assert.equal(s.grip,1);assert.equal(s.resistance,0);}
});
test('triangle layers and original wheel contacts select material',()=>{
 const world=terrain.classicTerrainFromCatalog(wrap([plane(1,0,-100,100),plane(3,100,-100,100)]));
 for(const height of [0,100]){const hit=terrain.classicTerrainAt(world,10,20,height);assert(hit.found);assert.equal(hit.surface,height===0?1:3);assert(Math.abs(hit.height-height)<1e-6);}
 assert.equal(terrain.classicTerrainAt(world,1000,1000,0).found,false);
 const split=terrain.classicTerrainFromCatalog(wrap([plane(1,0,-100,0),plane(3,0,0,100)]));
 const base={grip:1,normalY:1,resistance:0,grade:0};
 const actual=terrain.classicTerrainSurface(split,{x:0,y:0,height:0,angle:0,vehicle:'p911'},base);
 assert(actual.frontGrip<1);assert.equal(actual.rearGrip,1);assert.equal(actual.grip,1);assert.equal(actual.resistance,0);
 const unknown=terrain.classicTerrainSurface(split,{x:1000,y:1000,height:0,angle:0,vehicle:'p911'},base);
 assert.equal(unknown.frontGrip,1);assert.equal(unknown.rearGrip,1);assert.equal(terrain.classicMaterialGrip(999),1);
});
test('actual contact normals provide grade and normal load',()=>{
 const slope=plane(1,0,-100,100);for(const v of slope.vertices)v[1]=v[0]*.5;
 const world=terrain.classicTerrainFromCatalog(wrap([slope]));
 const surface=terrain.classicTerrainSurface(world,{x:0,y:0,height:0,angle:0,vehicle:'p911'},{grip:1,normalY:1,grade:0});
 assert(surface.grade>.4);assert(surface.normalY<.95);
});
test('not-driveable roofs and near-vertical faces are not wheel floors',()=>{
 let world=terrain.classicTerrainFromCatalog(wrap([plane(1,0,-100,100),plane(0,10,-100,100)]));
 let hit=terrain.classicTerrainAt(world,10,20,10);assert(hit.found);assert.equal(hit.surface,1);assert.equal(hit.height,0);
 world=terrain.classicTerrainFromCatalog(wrap([plane(0,10,-100,100)]));assert.equal(terrain.classicTerrainAt(world,10,20,10).found,false);
 world=terrain.classicTerrainFromCatalog(wrap([{segment:0,surface:1,vertices:[[0,0,-100],[.0001,100,-100],[.0001,100,100],[0,0,100]],faces:[[0,1,2,3]]}]));assert.equal(terrain.classicTerrainAt(world,.00005,0,50).found,false);
});
