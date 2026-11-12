// Real native-loop red evidence: native-container-probe-01 has124..126ms
// relative-RMS holes with identical FLAC/WAV, with and without capture. Before
// implementation: preserve phase/initial pitch at zero gain, never connect the
// physical context destination, honor media/master/session/CSP, cancel old
// decode/play work on mute/hidden/owner exit, and explicitly mark fallback.
import assert from "node:assert/strict";
import test from "node:test";
import { GameAudio } from "../src/audio.ts";
const source="data:audio/flac;base64,ZkxhQw==";
const flush=async()=>{for(let i=0;i<30;i++)await Promise.resolve();};
function setup(t,{deferDecode=false,unsupported=false,decodeError=false,denyPlay=false}={}){
 const names=["AudioContext","webkitAudioContext","document","navigator","fetch"],old=new Map(names.map(n=>[n,Object.getOwnPropertyDescriptor(globalThis,n)]));
 const contexts=[],elements=[],connections=[],decodes=[],tracks=[],api={allowPlay:!denyPlay};
 class Param{value=1;targets=[];setValueAtTime(v){this.value=v;}cancelScheduledValues(){}cancelAndHoldAtTime(){}setTargetAtTime(v,time,constant){this.targets.push({v,time,constant});this.value=v;}}
 class Sample{constructor(channels=1,length=4096,rate=44100){this.numberOfChannels=channels;this.length=length;this.sampleRate=rate;this.duration=length/rate;this.data=Array.from({length:channels},()=>Float32Array.from({length},(_,i)=>Math.sin(i*.03)*.7));}getChannelData(i){return this.data[i];}copyToChannel(v,i){this.data[i].set(v);}}
 class Context{
  currentTime=2;state="running";destination={physicalDevice:true};nodes=[];decodeCount=0;closed=false;
  constructor(){contexts.push(this);}
  createBuffer(c,n,r){return new Sample(c,n,r);}
  decodeAudioData(bytes){this.decodeCount++;assert.deepEqual([...new Uint8Array(bytes)],[102,76,97,67]);if(decodeError)return Promise.reject(Error('decode'));return deferDecode?new Promise(resolve=>decodes.push(()=>resolve(new Sample()))):Promise.resolve(new Sample());}
  createGain(){const node={gain:new Param(),connect(target){connections.push({node,target});},disconnect(){}};this.nodes.push(node);return node;}
  createBufferSource(){const node={buffer:null,loop:false,playbackRate:new Param(),starts:0,stops:0,connect(target){connections.push({node,target});},disconnect(){},start(){this.starts++;},stop(){this.stops++;}};this.nodes.push(node);return node;}
  createMediaStreamDestination(){const track={stopped:false,stop(){this.stopped=true;}};tracks.push(track);return {stream:{getTracks:()=>[track],getAudioTracks:()=>[track]}};}
  resume(){this.state="running";return Promise.resolve();}suspend(){this.state="suspended";return Promise.resolve();}close(){this.closed=true;this.state="closed";return Promise.resolve();}
 }
 class Media{dataset={};paused=true;muted=false;volume=1;playbackRate=1;src="";srcObject=null;calls=0;removed=false;loop=false;play(){this.calls++;if(this.srcObject&&!api.allowPlay)return Promise.reject(Error('NotAllowedError'));this.paused=false;return Promise.resolve();}pause(){this.paused=true;}load(){}removeAttribute(n){if(n==="src")this.src="";}remove(){this.removed=true;}}
 const document=new EventTarget();document.hidden=false;document.body={append(){}};
 for(const[n,value]of Object.entries({AudioContext:unsupported?undefined:Context,webkitAudioContext:undefined,document,navigator:{audioSession:{type:"auto"}},fetch:()=>{throw Error('inline data must not use connect-src fetch')}}))Object.defineProperty(globalThis,n,{value,writable:true,configurable:true});
 const audio=new GameAudio(()=>{const m=new Media();elements.push(m);return m;}),port=audio.channel('race-a');audio.mount();
 t.after(async()=>{audio.dispose();await flush();for(const[n,d]of old){if(d)Object.defineProperty(globalThis,n,d);else delete globalThis[n];}});
 return {audio,port,contexts,elements,connections,decodes,tracks,document,api};
}
test('continuous source phases keep one native mixed stream without short-file seeks or device connections',async t=>{
 const x=setup(t);assert.equal(await x.port.loop('a',[source],{continuous:true,volume:.2,rate:.69}),true);assert.equal(await x.port.loop('b',[source],{continuous:true,volume:0,rate:1.1}),true);
 assert.equal(x.contexts.length,1);const ctx=x.contexts[0],media=x.elements.find(m=>m.dataset.platformContinuous!==undefined);assert.ok(media?.srcObject);assert.equal(media.loop,false);assert.equal(media.playbackRate,1);assert.equal(ctx.decodeCount,1);
 const nodes=ctx.nodes.filter(n=>'starts'in n);assert.equal(nodes.length,2);assert.ok(nodes.every(n=>n.loop&&n.starts===1));assert.equal(nodes[0].playbackRate.value,.69);
 await x.port.loop('a',[source],{continuous:true,volume:0,rate:1.5});await x.port.loop('a',[source],{continuous:true,volume:.25,rate:2});assert.ok(nodes.every(n=>n.starts===1&&n.stops===0));assert.equal(media.calls,1);
 assert.ok(x.connections.every(c=>c.target!==ctx.destination));assert.equal(navigator.audioSession.type,'playback');x.audio.setVolume(.4);assert.equal(media.volume,.4);
 x.port.stopLoop('a');assert.equal(nodes[0].stops,1);assert.equal(nodes[1].stops,0);x.port.stop();assert.equal(nodes[1].stops,1);assert.ok(x.tracks.every(t=>t.stopped));assert.ok(media.paused&&media.srcObject===null&&media.removed);
});
test('pause/resume preserves phases and old owner cleanup cannot stop its successor',async t=>{
 const x=setup(t);await x.port.loop('engine',[source],{continuous:true});const ctx=x.contexts[0],media=x.elements.at(-1);x.port.pause();await flush();assert.equal(ctx.state,'suspended');assert.equal(media.paused,true);assert.equal(await x.port.resume(),true);assert.equal(ctx.nodes.filter(n=>'starts'in n)[0].starts,1);
 const next=x.audio.channel('race-b');await next.loop('engine',[source],{continuous:true});assert.equal(ctx.closed,true);const current=x.elements.at(-1);x.port.stop();x.port.stopLoop('engine');assert.equal(current.paused,false);
});
for(const kind of ['mute','hidden','stop'])test('late decoding cannot revive after '+kind,async t=>{
 const x=setup(t,{deferDecode:true}),pending=x.port.loop('engine',[source],{continuous:true});
 const deadline=Date.now()+1000;while(!x.decodes.length&&Date.now()<deadline)await new Promise(resolve=>setImmediate(resolve));assert.equal(x.decodes.length,1);
 if(kind==='mute')x.audio.setEnabled(false);else if(kind==='hidden'){x.document.hidden=true;x.document.dispatchEvent(new Event('visibilitychange'));}else x.port.stop();
 assert.equal(await pending,false);x.decodes[0]();await flush();assert.equal(x.contexts[0].nodes.filter(n=>'starts'in n).length,0);assert.ok(x.tracks.every(t=>t.stopped));assert.ok(x.elements.every(m=>m.paused));
});
test('a fresh trusted platform unlock retries blocked output without recreating source phases',async t=>{
 const x=setup(t,{denyPlay:true});assert.equal(await x.port.loop('engine',[source],{continuous:true}),false);x.api.allowPlay=true;await x.audio.unlock();await flush();assert.equal(await x.port.loop('engine',[source],{continuous:true}),true);assert.equal(x.contexts[0].nodes.filter(n=>'starts'in n)[0].starts,1);assert.equal(x.elements.at(-1).paused,false);
});
test('empty continuous requests release the requested lane and never allocate an empty output context',async t=>{
 const x=setup(t);assert.equal(await x.port.loop('missing',[],{continuous:true}),false);assert.equal(x.contexts.length,0);
 await x.port.loop('engine',[source],{continuous:true});assert.equal(await x.port.loop('engine',[],{continuous:true}),false);assert.equal(x.contexts[0].closed,true);
});
for(const options of [{unsupported:true},{decodeError:true}])test('explicit native fallback for '+Object.keys(options)[0],async t=>{
 const x=setup(t,options);assert.equal(await x.port.loop('engine',[source],{continuous:true,rate:1.2}),true);const media=x.elements.find(m=>m.dataset.continuousFallback);assert.ok(media);assert.equal(media.src,source);assert.equal(media.loop,true);x.port.stop();
});
test('decoded overlap smooths an artificial seam without mutating input or adding a silence fade',async()=>{
 const {overlapContinuousLoop}=await import('../src/audio-continuous.ts');const original=Float32Array.from({length:4096},(_,i)=>Math.sin(i*.013)*.6);original.fill(.8,0,16);original.fill(-.8,original.length-16);const before=original.slice(),result=overlapContinuousLoop(original,44100,5);
 assert.deepEqual(original,before);assert.ok(result.length<original.length&&result.length>original.length*.98);assert.ok(Math.abs(result.at(-1)-result[0])<.05);assert.ok(result.every(Number.isFinite));assert.ok(Math.max(...result)<=.8&&Math.min(...result)>=-.8);
});
