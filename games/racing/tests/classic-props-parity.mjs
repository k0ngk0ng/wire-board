import assert from 'node:assert/strict';
import {readFileSync,writeFileSync} from 'node:fs';
import * as api from '../web/classic-road.ts';
const [fixture,report]=process.argv.slice(2),rows=JSON.parse(readFileSync(fixture)),tracks=new Map(),catalogs=new Map();
for(const id of ['france','coastal','uk','park']){
 tracks.set(id,api.classicTrackFromRoute(JSON.parse(readFileSync(new URL(`../assets/public/racing/v3/tracks/${id}.route.json`,import.meta.url)))));
 catalogs.set(id,JSON.parse(readFileSync(new URL(`../assets/source/nfs4/props-rules/${id}.props.json`,import.meta.url))));
}
assert.equal(typeof api.classicPropWorld,'function','browser prop-world rules missing');
let maximumError=0,events=0;
for(const row of rows){
 const track=tracks.get(row.track),catalog=catalogs.get(row.track),world=api.classicPropWorld(catalog,{catalog:catalog.catalogHash,broken:row.before},track),hits=[];
 const motion=api.collideClassicRoad(track,row.motion,row.vehicle,row.height,world,(prop,vx,vz,closing)=>hits.push({prop,dir:Math.round(Math.atan2(vz,vx)*1000)/1000,high:closing>=8,intensity:Math.max(0,Math.min(1,closing/20))}));
 for(const key of Object.keys(row.expected)){const delta=Math.abs(motion[key]-row.expected[key]);maximumError=Math.max(maximumError,delta);assert(delta<1e-7,`${row.track}/${row.prop}/${row.mode} ${key}: ${motion[key]} vs ${row.expected[key]}`);}
 assert.equal(Buffer.from(world.broken).toString('base64'),row.after);
 assert.equal(hits.length,row.events.length);
 for(let i=0;i<hits.length;i++){
  const hit=hits[i],expected=row.events[i];
  assert.equal(hit.prop,expected.prop);assert.equal(hit.high,expected.high??false);
  assert(Math.abs(hit.dir-(expected.dir??0))<1e-9);
  assert(Math.abs(hit.intensity-expected.intensity)<1e-12);
 }
 events+=hits.length;
}
writeFileSync(report,JSON.stringify({passed:true,probes:rows.length,firstHitEvents:events,maximumError},null,2)+'\n');
console.log(`Verified ${rows.length} original-prop/vehicle/state collision probes and ${events} first-hit events.`);
