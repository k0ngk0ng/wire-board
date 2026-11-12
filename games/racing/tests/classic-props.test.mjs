// Written before the browser rule mirror. Failure inventory:
// classic-props-failures.md. This checks catalog/bit ordering, fixed walls,
// contact/height semantics and isolation of speculative prediction state.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,writeFileSync,mkdirSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import * as rules from '../web/classic-road.ts';
import {predictCar} from '../web/prediction.ts';
import {classicTerrainFromCatalog} from '../web/classic-terrain.ts';
import {createHash} from 'node:crypto';

const load=name=>JSON.parse(readFileSync(new URL(name,import.meta.url)));
const catalogs=new Map(),tracks=new Map();
for(const id of ['france','coastal','uk','park']){
 catalogs.set(id,load(`../assets/source/nfs4/props-rules/${id}.props.json`));
 const track=rules.classicTrackFromRoute(load(`../assets/public/racing/v3/tracks/${id}.route.json`));
 const collision=readFileSync(new URL(`../assets/public/racing/v3/tracks/${id}.collision.json`,import.meta.url));
 track.terrain=classicTerrainFromCatalog({...JSON.parse(collision),unit:'metres',geometry:track.geometry,sourceSha256:createHash('sha256').update(collision).digest('hex')},track);
 tracks.set(id,track);
}
const wire=(catalog,bits=new Uint8Array(Math.ceil(catalog.props.length/8)))=>({catalog:catalog.catalogHash,broken:Buffer.from(bits).toString('base64')});

test('frozen prop identities use compact LSB bits without changing road geometry',()=>{
 assert.equal(typeof rules.classicPropWorld,'function','missing confirmed prop-world decoder');
 for(const [id,catalog] of catalogs){
  const track=tracks.get(id),before=JSON.stringify(track),bits=new Uint8Array(Math.ceil(catalog.props.length/8));
  for(const index of [0,7,8,catalog.props.length-1])bits[index>>3]|=1<<(index&7);
  const world=rules.classicPropWorld(catalog,wire(catalog,bits),track);
  for(const prop of catalog.props)for(const edge of prop.ruleEdges)
   assert.equal(rules.classicBarrierBroken(world,edge),!!(bits[prop.index>>3]&(1<<(prop.index&7))));
  assert.equal(rules.classicBarrierBroken(world,track.barriers.findIndex(b=>b.kind==='wall')),false);
  assert.equal(JSON.stringify(track),before);
  assert.equal(rules.classicPropWorld(catalog,wire(catalog),track).broken[0],0,'independent worlds must not share mutable bits');
  assert.throws(()=>rules.classicPropWorld(catalog,{catalog:'missing',broken:wire(catalog).broken},track));
  assert.throws(()=>rules.classicPropWorld(catalog,{catalog:catalog.catalogHash,broken:'bad base64'},track));
  assert.throws(()=>rules.classicPropWorld(catalog,{catalog:catalog.catalogHash,broken:''},track));
  const invalid=new Uint8Array(bits.length);invalid[invalid.length-1]=0x80;
  assert.throws(()=>rules.classicPropWorld(catalog,wire(catalog,invalid),track));
 }
});

test('prediction never changes the externally confirmed prop bitset',()=>{
 assert.equal(typeof rules.classicPropWorld,'function');
 const catalog=catalogs.get('france'),track=tracks.get('france'),world=rules.classicPropWorld(catalog,wire(catalog),track);
 const edge=track.barriers[catalog.props[1].ruleEdges[0]],dx=edge.b.x-edge.a.x,dy=edge.b.y-edge.a.y,length=Math.hypot(dx,dy)||1,nx=-dy/length,ny=dx/length;
 const x=(edge.a.x+edge.b.x)/2+nx*3,y=(edge.a.y+edge.b.y)/2+ny*3,angle=Math.atan2(-ny,-nx);
 const car={x,y,vx:-nx*80,vy:-ny*80,angle,speed:80,vehicle:'p911',height:edge.bottom,routeIndex:8,rpm:2000,gear:1,resetUntil:0,finishAt:0,lastInputAt:0};
 const before=Buffer.from(world.broken).toString('hex');
 predictCar(car,{steer:0,throttle:0,brake:false,drift:false},track,20,20,world);
 assert.equal(Buffer.from(world.broken).toString('hex'),before,'prediction wrote authoritative broken bits');
});

test('prediction follows actual Module input across a real first-hit transition',()=>{
 const root=fileURLToPath(new URL('../../../',import.meta.url)),temp=path.join(root,'.local/tmp');mkdirSync(temp,{recursive:true});
 const env={...process.env,GOCACHE:path.join(root,'.local/go-build'),GOMODCACHE:path.join(root,'.local/go-mod'),GOTMPDIR:temp,TMPDIR:temp,GOTOOLCHAIN:'local'};
 execFileSync('go',['test','./games/racing/tests','-run','^TestClassicPropPredictionProductionTrace$','-count=1','-timeout=2m'],{cwd:root,env,encoding:'utf8',timeout:150000});
 const out=path.join(root,'.local/racing-nfs-backend/props-prediction'),data=JSON.parse(readFileSync(path.join(out,'production.json'))),catalog=catalogs.get('france'),track=tracks.get('france');
 let maximumPositionDM=0,maximumRPM=0,transition=false;
 for(const row of data.rows){
  const world=rules.classicPropWorld(catalog,row.propsBefore,track),bits=Buffer.from(world.broken).toString('hex');
  const predicted=predictCar(row.before,row.input,track,row.ms,row.at,world),actual=row.after;
  const position=Math.hypot(predicted.x-actual.x,predicted.y-actual.y),rpm=Math.abs(predicted.rpm-actual.rpm);
  maximumPositionDM=Math.max(maximumPositionDM,position);maximumRPM=Math.max(maximumRPM,rpm);
  assert(position<.02,`first-hit prediction differs ${position} dm`);assert(rpm<.1);
  assert.equal(Buffer.from(world.broken).toString('hex'),bits);
  if(row.propsBefore.broken!==row.propsAfter.broken)transition=true;
 }
 assert(transition,'missing actual first-hit transition');
 writeFileSync(path.join(out,'browser-parity.json'),JSON.stringify({passed:true,steps:data.rows.length,maximumPositionDM,maximumRPM,realFirstHitTransition:true},null,2)+'\n');
});
