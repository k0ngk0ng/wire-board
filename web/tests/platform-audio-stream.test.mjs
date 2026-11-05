// Controlled lifecycle/race tests, frozen before stream runtime implementation.
// Failures: missing API; caller mutation; hidden unbounded queues; duplicates,
// gaps and conflicting epochs; late sourceopen/updateend/play after stop;
// EOS confused with audible end; pause resets; back-buffer removal reaches the
// current frame; unsupported/invalid options interrupt an existing stream.
// Real Chromium/FLAC E2E is maintained separately; these fakes prove no PCM.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {GameAudio} from '../src/audio.ts';
const MIME='audio/mp4; codecs="fLaC"';
const flush=async()=>{for(let i=0;i<24;i++)await Promise.resolve();};
function ranges(values){return{length:values.length,start:index=>values[index][0],end:index=>values[index][1]};}
function setup(t){
 const old={MediaSource:globalThis.MediaSource,document:globalThis.document,create:URL.createObjectURL,revoke:URL.revokeObjectURL};
 const urls=new Map(),revoked=[],sources=[],media=[];
 class Buffer extends EventTarget{
  updating=false;auto=true;mode='segments';history=[];removals=[];values=[];
  get buffered(){return ranges(this.values);}
  appendBuffer(data){if(this.updating)throw Error('InvalidState');this.history.push(new Uint8Array(data).slice());this.updating=true;if(this.auto)queueMicrotask(()=>this.complete());}
  remove(start,end){if(this.updating)throw Error('InvalidState');this.removals.push([start,end]);this.values=this.values.map(([a,b])=>[Math.max(a,end),b]).filter(([a,b])=>a<b);this.updating=true;if(this.auto)queueMicrotask(()=>this.complete());}
  complete(){if(this.updating){this.updating=false;this.dispatchEvent(new Event('updateend'));}}
  abort(){this.updating=false;this.dispatchEvent(new Event('abort'));}
 }
 class Source extends EventTarget{
  static isTypeSupported=type=>type===MIME;
  readyState='closed';sourceBuffers=[];duration=NaN;ends=0;
  constructor(){super();sources.push(this);}
  addSourceBuffer(type){assert.equal(type,MIME);const buffer=new Buffer();this.sourceBuffers.push(buffer);return buffer;}
  removeSourceBuffer(buffer){this.sourceBuffers=this.sourceBuffers.filter(item=>item!==buffer);}
  endOfStream(){if(this.sourceBuffers.some(item=>item.updating))throw Error('updating');this.ends++;this.readyState='ended';this.dispatchEvent(new Event('sourceended'));}
  open(){this.readyState='open';this.dispatchEvent(new Event('sourceopen'));}
 }
 class Media extends EventTarget{
  dataset={};source='';paused=true;muted=false;volume=1;loop=false;currentTime=0;duration=NaN;readyState=4;ended=false;removed=false;plays=0;loads=0;
  onended=null;onerror=null;onloadedmetadata=null;behavior=()=>Promise.resolve();
  get src(){return this.source;}set src(value){this.source=value;this.currentTime=0;const source=urls.get(value);if(source)queueMicrotask(()=>source.open());}
  get currentSrc(){return this.source;}
  get buffered(){const source=urls.get(this.src);return source?.sourceBuffers[0]?.buffered??ranges([]);}
  play(){this.paused=false;this.plays++;this.dispatchEvent(new Event('playing'));return this.behavior();}
  pause(){this.paused=true;this.dispatchEvent(new Event('pause'));}
  load(){this.loads++;}
  removeAttribute(name){if(name==='src')this.source='';}
  remove(){this.removed=true;}
  finish(){this.ended=true;this.paused=true;this.onended?.(new Event('ended'));this.dispatchEvent(new Event('ended'));}
 }
 globalThis.MediaSource=Source;
 globalThis.document={hidden:false,body:{append(){}}};
 URL.createObjectURL=source=>{const key='blob:fake-'+urls.size;urls.set(key,source);return key;};
 URL.revokeObjectURL=value=>{revoked.push(value);};
 const audio=new GameAudio(()=>{const element=new Media();media.push(element);return element;});
 t.after(()=>{audio.dispose();globalThis.MediaSource=old.MediaSource;globalThis.document=old.document;URL.createObjectURL=old.create;URL.revokeObjectURL=old.revoke;});
 const config=(id='one',extra={})=>({playbackID:id,mimeType:MIME,sampleRate:44100,initSegment:Uint8Array.of(0x11,0x22),...extra});
 const segment=(sequence,startSample,sampleFrames=4410,tag=sequence)=>({sequence,startSample,sampleFrames,data:Uint8Array.of(0x31,tag)});
 return{audio,p:audio.channel('room'),other:audio.channel('other'),sources,media,revoked,config,segment};
}
test('native stream ready, contiguous commits and end distinguish EOS from playback end',async t=>{
 const{p,sources,media,config,segment}=setup(t);const h=p.openStream(config());assert.ok(h);assert.equal(await h.ready,true);
 const sb=sources[0].sourceBuffers[0];assert.deepEqual([...sb.history[0]],[0x11,0x22]);assert.equal(h.status().nextSeq,1);
 assert.equal(await h.append(segment(1,0)),true);assert.equal(h.status().acceptedEndSample,4410);assert.equal(h.status().nextSeq,2);assert.ok(media[0].plays>=1);assert.equal(media[0].loop,false);
 const ended=h.end();assert.equal(await ended,true);assert.equal(await h.end(),true);assert.equal(sources[0].ends,1);assert.equal(h.status().phase,'draining');
 assert.equal(await h.append(segment(2,4410)),false);media[0].finish();assert.equal(h.status().phase,'ended');
 assert.equal(p.openStream(config()),h);assert.equal(sources.length,1);
});
test('init and pending data are copied; only an identical in-flight segment shares a Promise',async t=>{
 const{p,sources,config,segment}=setup(t);const options=config();const h=p.openStream(options);options.initSegment[0]=0xff;assert.equal(await h.ready,true);
 const sb=sources[0].sourceBuffers[0];assert.equal(sb.history[0][0],0x11);sb.auto=false;
 const first=segment(1,0);const pending=h.append(first);const duplicate=h.append(segment(1,0));assert.equal(pending,duplicate);
 first.data[0]=0xff;first.startSample=99;assert.equal(await h.append(segment(2,4410)),false);assert.equal(h.status().nextSeq,1);
 await flush();assert.equal(sb.history.at(-1)[0],0x31);sb.complete();assert.equal(await pending,true);assert.equal(h.status().acceptedEndSample,4410);
 const count=sb.history.length;assert.equal(await h.append(segment(1,0)),true);assert.equal(sb.history.length,count);
 assert.equal(await h.append(segment(1,0,4410,9)),false);
});
test('invalid segments and forward backpressure preserve the current cursor and media',async t=>{
 const{p,media,config,segment}=setup(t);const h=p.openStream(config('bounded',{maxForwardSamples:4410}));await h.ready;
 assert.equal(await h.append(segment(1,0)),true);const before=h.status();
 for(const input of [segment(3,4410),segment(2,4411),segment(2,4410,0),segment(2,4410,-1),segment(2,4410,NaN),{...segment(2,4410),sequence:1.5},{...segment(2,4410),data:new Uint8Array(0)},{...segment(2,4410),data:new Uint8Array(1024*1024+1)}])assert.equal(await h.append(input),false);
 assert.equal(await h.append(segment(2,4410)),false);assert.equal(h.status().nextSeq,before.nextSeq);assert.equal(h.status().acceptedEndSample,before.acceptedEndSample);
 media[0].currentTime=.1;assert.equal(await h.append(segment(2,4410)),true);assert.equal(h.status().acceptedEndSample,8820);
});
test('unsupported, conflicting or malformed stream configurations are atomic',async t=>{
 const{p,sources,config}=setup(t);const h=p.openStream(config());await h.ready;
 for(const options of [config('one',{initSegment:Uint8Array.of(9)}),config('one',{maxForwardSamples:4410}),config('new',{mimeType:'not/supported'}),config('new',{sampleRate:0}),config('new',{playbackID:''}),config('new',{initSegment:new Uint8Array(0)}),config('new',{retainBackSamples:0}),config('new',{maxForwardSamples:Infinity}),config('new',{initSegment:new Uint8Array(65537)})])assert.equal(p.openStream(options),undefined);
 assert.equal(p.openStream(config()),h);assert.equal(sources.length,1);assert.notEqual(h.status().phase,'stopped');
});
test('replace during bootstrap or update cannot resurrect or stop another epoch',async t=>{
 const{p,other,sources,media,revoked,config,segment}=setup(t);
 const old=p.openStream(config('old'));old.stop();assert.equal(await old.ready,false);
 const current=p.openStream(config('current'));await current.ready;const sb=sources.at(-1).sourceBuffers[0];sb.auto=false;
 const pending=current.append(segment(1,0));await flush();const successor=other.openStream(config('successor'));assert.equal(await pending,false);await successor.ready;
 sb.complete();sources[0].open();current.stop();p.stop();await flush();assert.equal(successor.status().phase,'ready');assert.ok(media.at(-1).src);assert.equal(revoked.length,2);
});
test('pause preserves stream data and position; resume does not replace its URL',async t=>{
 const{p,media,config,segment}=setup(t);const h=p.openStream(config());await h.ready;p.pause();assert.equal(await h.append(segment(1,0)),true);
 assert.equal(media[0].paused,true);assert.equal(h.status().phase,'paused');const src=media[0].src;
 assert.equal(await p.resume(),true);assert.equal(media[0].src,src);media[0].currentTime=.07;p.pause();const at=h.status().currentTime;
 await flush();assert.equal(h.status().currentTime,at);assert.equal(await p.resume(),true);assert.equal(media[0].src,src);
});
test('end waits for an in-flight commit while stop/mute close all pending work exactly once',async t=>{
 const{audio,p,sources,media,revoked,config,segment}=setup(t);const h=p.openStream(config());await h.ready;
 const sb=sources[0].sourceBuffers[0];sb.auto=false;const pending=h.append(segment(1,0));await flush();const end=h.end();assert.equal(await h.append(segment(2,4410)),false);assert.equal(sources[0].ends,0);
 sb.complete();assert.equal(await pending,true);assert.equal(await end,true);assert.equal(sources[0].ends,1);
 const newer=p.openStream(config('new'));await newer.ready;const next=sources[1].sourceBuffers[0];next.auto=false;const pendingNew=newer.append(segment(1,0));await flush();audio.setEnabled(false);
 assert.equal(await pendingNew,false);next.complete();newer.stop();audio.setEnabled(true);await flush();assert.equal(newer.status().phase,'stopped');assert.equal(revoked.length,2);assert.ok(media.every(item=>item.paused&&!item.src&&item.removed));
});
test('back-buffer eviction keeps the current playback interval and a late play cannot revive stop',async t=>{
 const{p,sources,media,config,segment}=setup(t);const h=p.openStream(config('back',{maxForwardSamples:220500}));await h.ready;
 assert.equal(await h.append(segment(1,0,176400)),true);const sb=sources[0].sourceBuffers[0];sb.values=[[0,4]];media[0].currentTime=3;
 assert.equal(await h.append(segment(2,176400)),true);assert.ok(sb.removals.length>0);assert.ok(sb.removals.every(([start,end])=>start===0&&end<=1));
 h.stop();const fresh=p.openStream(config('late'));await fresh.ready;let finish;const element=media.at(-1);element.behavior=()=>new Promise(resolve=>{finish=()=>{element.paused=false;resolve();};});
 assert.equal(await fresh.append(segment(1,0)),true);await flush();fresh.stop();finish?.();await flush();assert.ok(element.paused&&!element.src&&element.removed);
});
