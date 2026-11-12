import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { classicTerrainForRoute } from '../web/classic-terrain-loading.ts';
const read=p=>JSON.parse(readFileSync(new URL(p,import.meta.url)));
const hash=b=>createHash('sha256').update(b).digest('hex');

test('all four native ground catalogs bind to exact selected route without changing vertices',()=>{
 for(const id of ['france','coastal','uk','park']){
  const bytes=readFileSync(new URL(`../assets/public/racing/v3/tracks/${id}.collision.json`,import.meta.url));
  const raw=JSON.parse(bytes),route=read(`../assets/public/racing/v3/tracks/${id}.route.json`),before=JSON.stringify(raw);
  const world=classicTerrainForRoute(raw,route,hash(bytes),{geometry:route.geometryHash});
  assert.equal(world.geometry,route.geometryHash);assert(world.triangles.length>5000);assert(world.grid.size>100);
  assert.equal(JSON.stringify(raw),before,'verified source was mutated');
 }
});

test('wrong source version, route, origin, transform and digest cannot become ready',()=>{
 const id='uk',bytes=readFileSync(new URL(`../assets/public/racing/v3/tracks/${id}.collision.json`,import.meta.url)),raw=JSON.parse(bytes),route=read(`../assets/public/racing/v3/tracks/${id}.route.json`),track={geometry:route.geometryHash};
 for(const bad of [null,{...raw,version:2},{...raw,id:'france'},{...raw,origin:[0,0,0]},{...raw,sourceTransform:[1,1,1]},{...raw,surfaces:null},{...raw,surfaces:[]},{...raw,surfaces:[{segment:0,surface:1,vertices:[],faces:[]}]}])assert.throws(()=>classicTerrainForRoute(bad,route,hash(bytes),track),/地表/);
 assert.throws(()=>classicTerrainForRoute(raw,route,'bad',track),/地表/);
 assert.throws(()=>classicTerrainForRoute(raw,route,hash(bytes),{geometry:'f'.repeat(64)}),/地表/);
});
