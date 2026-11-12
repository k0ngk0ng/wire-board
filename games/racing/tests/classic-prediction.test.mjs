// Before implementation: compare browser prediction against actual Module
// input/timer trajectories on the real France road, not another JS equation.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,mkdirSync,writeFileSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
import {predictCar} from '../web/prediction.ts';
import {classicTrackFromRoute} from '../web/classic-road.ts';
import {classicTerrainFromCatalog} from '../web/classic-terrain.ts';
import {createHash} from 'node:crypto';
import {interpolateCar} from '../web/presentation.ts';

test('browser motion follows production Module snapshots for all five prepared cars',()=>{
 const root=fileURLToPath(new URL('../../../',import.meta.url)),out=path.join(root,'.local/racing-nfs-v3/prediction');
 const temp=path.join(root,'.local/tmp');mkdirSync(temp,{recursive:true});mkdirSync(out,{recursive:true});
 const env={...process.env,GOCACHE:path.join(root,'.local/go-build'),GOMODCACHE:path.join(root,'.local/go-mod'),GOTMPDIR:temp,TMPDIR:temp,GOTOOLCHAIN:'local',WIRE_BOARD_RACING_PREDICTION_EVIDENCE:out};
 execFileSync('go',['test','./games/racing/tests','-run','^TestClassicPredictionProductionTrace$','-count=1','-timeout=2m'],{cwd:root,env,encoding:'utf8',timeout:150000});
 const raw=JSON.parse(readFileSync(path.join(root,'games/racing/assets/public/racing/v3/tracks/france.route.json'))),track=classicTrackFromRoute(raw);
 const collision=readFileSync(path.join(root,'games/racing/assets/public/racing/v3/tracks/france.collision.json'));
 track.terrain=classicTerrainFromCatalog({...JSON.parse(collision),unit:'metres',geometry:track.geometry,sourceSha256:createHash('sha256').update(collision).digest('hex')},track);
 const data=JSON.parse(readFileSync(path.join(out,'production-trajectories.json')));
 let largestPosition=0,largestRPM=0;
 for(const row of data.rows){
   const predicted=predictCar(row.before,row.input,track,row.ms,row.at),actual=row.after;
   const position=Math.hypot(predicted.x-actual.x,predicted.y-actual.y);
   largestPosition=Math.max(largestPosition,position);largestRPM=Math.max(largestRPM,Math.abs((predicted.rpm??0)-actual.rpm));
   assert(position<.015,`${row.vehicle} browser/server position differs ${position} dm`);
   assert(Math.abs((predicted.rpm??0)-actual.rpm)<.1,`${row.vehicle} RPM/filter state lost`);
   assert.equal(predicted.gear,actual.gear);
   assert(Math.abs(predicted.motion.steering-actual.motion.steering)<1e-5);
   assert(Math.abs(predicted.motion.brake-actual.motion.brake)<1e-5);
 }
 writeFileSync(path.join(out,'browser-parity.json'),JSON.stringify({passed:true,steps:data.rows.length,largestPositionDM:largestPosition,largestRPM,scope:data.scope},null,2)+'\n');
});

test('classic spectator interpolation includes road pose and filtered steering',()=>{
 const before={x:0,y:0,angle:0,height:10,pitch:.1,roll:.02,rpm:1000,gear:1,vehicle:'p911',resetUntil:0,motion:{steering:0,rpm:1000,gear:1}};
 const after={...before,x:10,height:20,pitch:.2,roll:.04,rpm:2000,gear:2,motion:{steering:1,rpm:2000,gear:2}};
 const result=interpolateCar(before,after,.5);
 assert.equal(result.height,15);assert(Math.abs(result.pitch-.15)<1e-10);assert.equal(result.rpm,1500);assert.equal(result.motion.steering,.5);assert.equal(result.gear,2);
});
