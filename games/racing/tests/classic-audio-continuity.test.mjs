import assert from "node:assert/strict";
import test from "node:test";
import { ClassicEnginePlayback, classicEngineVoices } from "../web/classic-audio.ts";

const sequence=fn=>Array.from({length:512},(_,i)=>fn(i));
const meta={id:"test",idleRPM:1000,redlineRPM:8000,mapping:{index0RPM:1000,rpmPerIndex:100},samples:[
 {patch:1,path:"a.flac",timbreVolume:127,tables:[{kind:"load",volume:sequence(i=>Math.min(127,i*10)),rate:sequence(i=>.5+i*.01)}]},
 {patch:2,path:"b.flac",timbreVolume:127,tables:[{kind:"coast",volume:sequence(()=>100),rate:sequence(i=>.6+i*.01)}]},
]};

test("sub-index RPM changes interpolate source controls instead of stepping between nearest entries",()=>{
 const a=classicEngineVoices(meta,1125,1)[0],b=classicEngineVoices(meta,1175,1)[0];
 assert.ok(Math.abs(a.rate-.5125)<1e-10);
 assert.ok(Math.abs(b.rate-.5175)<1e-10);
 assert.ok(a.volume>0&&b.volume>a.volume);
});

test("muted RPM/load regions retain stable continuous phase voices without stop/start churn",()=>{
 const calls=[],stops=[];const playback=new ClassicEnginePlayback({loop(key,src,options){calls.push({key,src,options});return Promise.resolve(true);},stopLoop(key){stops.push(key);}});
 const engine={metadata:meta,sources:new Map([["a.flac","data:a"],["b.flac","data:b"]])};
 for(let i=0;i<50;i++)playback.update(engine,1000+i*2,i%2,true);
 assert.deepEqual(stops,[]);
 assert.deepEqual(new Set(calls.map(c=>c.key)),new Set(["racing-engine-test-1","racing-engine-test-2"]));
 assert.ok(calls.some(c=>c.options.volume===0));
 assert.ok(calls.every(c=>c.options.rate>=.25&&c.options.rate<=4&&c.options.volume>=0&&c.options.volume<=.25));
 assert.ok(calls.every(c=>c.options.continuous===true&&c.options.seamOverlapMS===5));
 playback.update(engine,1300,1,false);
 assert.deepEqual(new Set(stops),new Set(["racing-engine-test-1","racing-engine-test-2"]));
 playback.stop();assert.equal(stops.length,2);
});

test("source/vehicle replacement and disposal release all previous engine keys",()=>{
 const stops=[],playback=new ClassicEnginePlayback({loop(){return Promise.resolve(true);},stopLoop(key){stops.push(key);}});
 const engine={metadata:meta,sources:new Map([["a.flac","data:a"],["b.flac","data:b"]])};
 playback.update(engine,1800,1,true);
 playback.update({...engine,sources:new Map([["a.flac","data:new"],["b.flac","data:new-b"]])},1800,1,true);
 assert.equal(stops.length,2);
 playback.update(undefined,1800,1,true);assert.equal(stops.length,4);
 playback.stop();assert.equal(stops.length,4);
});
