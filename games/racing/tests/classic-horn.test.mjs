import assert from "node:assert/strict";
import test from "node:test";
import { createHash } from "node:crypto";
import { ClassicHornPlayback, classicHornAssets, loadClassicHorn } from "../web/classic-horn.ts";

// Before implementation: local horn must override both delayed authority-on and
// authority-off states, while remote horns use applied authority. Protection,
// finishing, removal, stopped race, missing source, changing pack and disposal
// must release owned loop keys. No private player or unverified byte may play.
const flac=new TextEncoder().encode("verified horn bytes").buffer;
const sha=b=>createHash("sha256").update(new Uint8Array(b)).digest("hex");
const metadata={version:1,path:"audio/horn/horn.flac",origin:"original-synthesis",description:"自制双音喇叭，非极品飞车原声",sampleRate:22050,channels:1,bits:16,frames:4410,loopStartFrame:0,loopEndFrameExclusive:4410,sha256:sha(flac)};
function pack(meta=metadata){const json=new TextEncoder().encode(JSON.stringify(meta)).buffer;return {base:"",manifest:{version:3,assets:[{path:"audio/horn/horn.flac",bytes:flac.byteLength,sha256:sha(flac),mime:"audio/flac"},{path:"audio/horn/horn.json",bytes:json.byteLength,sha256:sha(json),mime:"application/json"}]},files:new Map([["audio/horn/horn.flac",flac],["audio/horn/horn.json",json]])};}
function playback(){const calls=[],stops=[];const controller=new ClassicHornPlayback({loop(key,sources,options){calls.push({key,sources,options});return Promise.resolve(true);},stopLoop(key){stops.push(key);}});return {controller,calls,stops};}
const own={seat:0,x:0,y:0,horn:false,finishAt:0,protected:false},other={seat:1,x:100,y:0,horn:true,finishAt:0,protected:false};

test("horn source is mandatory, SHA-verified, self-created and fully loopable",async()=>{
 const p=pack();assert.equal(classicHornAssets(p.manifest).length,2);
 const result=await loadClassicHorn(p,new AbortController().signal);
 assert.deepEqual(result.bytes,flac);assert.equal(result.metadata.origin,"original-synthesis");
 const invalid=pack();invalid.files.set("audio/horn/horn.flac",new Uint8Array(flac.byteLength).buffer);
 await assert.rejects(loadClassicHorn(invalid,new AbortController().signal),/校验/);
 const missing=pack();missing.files.delete("audio/horn/horn.json");
 await assert.rejects(loadClassicHorn(missing,new AbortController().signal),/缺失/);
 const controller=new AbortController();controller.abort();
 await assert.rejects(loadClassicHorn(pack(),controller.signal),e=>e.name==="AbortError");
 assert.throws(()=>classicHornAssets({version:3,assets:[]}),/喇叭/);
 await assert.rejects(loadClassicHorn(pack({...metadata,origin:"nfs-original"}),new AbortController().signal),/来源/);
 await assert.rejects(loadClassicHorn(pack({...metadata,loopEndFrameExclusive:0}),new AbortController().signal),/循环/);
});

test("local key press and release immediately override stale authority without silencing remote horns",()=>{
 const {controller,calls,stops}=playback();
 controller.update("blob:horn",[own,other],0,true,true);
 assert.deepEqual(new Set(calls.map(c=>c.key)),new Set(["racing-horn-0","racing-horn-1"]));
 controller.update("blob:horn",[{...own,horn:true},other],0,false,true);
 assert.ok(stops.includes("racing-horn-0"));assert.ok(!stops.includes("racing-horn-1"));
 assert.ok(calls.every(c=>c.sources[0]==="blob:horn"&&c.options.rate===1&&c.options.volume>0&&c.options.volume<=.35));
});

test("authorized local intention overrides a stale protected snapshot, remote protection still silences",()=>{
 const {controller,calls}=playback();
 controller.update("blob:horn",[{...own,protected:true},{...other,protected:true}],0,true,true);
 assert.deepEqual(calls.map(c=>c.key),["racing-horn-0"]);
});

test("protection, finish, removal, no source, race stop and disposal release owned keys",()=>{
 const {controller,calls,stops}=playback();
 controller.update("blob:horn",[own,other],0,true,true);
 controller.update("blob:horn",[{...own,protected:true},{...other,finishAt:1}],0,false,true);
 assert.deepEqual(new Set(stops),new Set(["racing-horn-0","racing-horn-1"]));
 controller.update("blob:horn",[own,other],0,true,true);controller.update("blob:horn",[],0,true,true);
 assert.equal(stops.length,4);
 controller.update("blob:horn",[own],0,true,true);controller.update(undefined,[own],0,true,true);
 assert.equal(stops.length,5);
 controller.update("blob:horn",[own],0,true,true);controller.update("blob:horn",[own],0,true,false);
 assert.equal(stops.length,6);
 controller.update("blob:horn",[own],0,true,true);controller.stop();controller.stop();
 assert.equal(stops.length,7);
 assert.ok(calls.every(c=>/^racing-horn-\d+$/.test(c.key)));
});

test("spectators hear authority, changing sources stops the old voice, and car data stays immutable",()=>{
 const cars=[{...own,horn:true},other],before=JSON.stringify(cars),{controller,calls,stops}=playback();
 controller.update("blob:first",cars,null,false,true);assert.equal(calls.length,2);
 controller.update("blob:second",cars,null,false,true);assert.equal(stops.length,2);
 assert.ok(calls.slice(2).every(c=>c.sources[0]==="blob:second"));
 assert.equal(JSON.stringify(cars),before);
});
