import { GameAudio } from "../../web/src/audio";
import type { GameAudioStreamConfig, GameAudioStreamHandle, GameAudioStreamSegment } from "../../web/src/game-api/audio-stream";
type Port = ReturnType<GameAudio["channel"]> & { openStream(config: GameAudioStreamConfig): GameAudioStreamHandle | undefined };
type Fragment = { sequence: number; startSample: number; sampleFrames: number; url: string };
type Fixture = { mimeType: string; sampleRate: number; totalSamples: number; sourcePCMHash: string; initURL: string; fragments: Fragment[] };
const h = { ready: true, complete: false, error: "", checks: [] as unknown[], events: [] as any[], clocks: [] as any[], appends: [] as any[], removals: [] as any[], urls: [] as any[], crossings: [] as any[], visibility: [] as any[], firstPlaying: undefined as any, current: undefined as GameAudioStreamHandle | undefined, phase: "setup", capture: undefined as any };
Object.assign(window, { audioStreamProbe: h });
const now = () => performance.now();
const assert = (value: unknown, message: string) => { if (!value) throw Error(message); };
const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
const until = async (predicate: () => boolean, limit = 15000) => { const start = now();while(!predicate()){if(now()-start>limit)throw Error("native stream predicate timed out");await delay(5);} };
const bounded = async <T>(promise: Promise<T>, label: string, ms = 5000) => Promise.race([promise,new Promise<never>((_, reject)=>setTimeout(()=>reject(Error(label+" timed out")),ms))]);
const check = (kind: string, value: object = {}) => h.checks.push({ kind, ...value });
const nativeURL = URL.createObjectURL.bind(URL), revokeURL = URL.revokeObjectURL.bind(URL);
URL.createObjectURL = (value) => { const url = nativeURL(value);h.urls.push({kind:"create",url,at:now()});return url; };
URL.revokeObjectURL = (url) => { h.urls.push({kind:"revoke",url,at:now()});revokeURL(url); };
const nativeAppend = SourceBuffer.prototype.appendBuffer, nativeRemove = SourceBuffer.prototype.remove;
SourceBuffer.prototype.appendBuffer = function(data) { h.appends.push({at:now(),bytes:data.byteLength,phase:h.phase});return nativeAppend.call(this,data); };
SourceBuffer.prototype.remove = function(start,end) { h.removals.push({at:now(),start,end,phase:h.phase,currentTime:h.current?.status().currentTime});return nativeRemove.call(this,start,end); };
const audioElements: HTMLAudioElement[] = [];
const platform = new GameAudio(() => {
 const media = new Audio();const id = audioElements.push(media)-1;
 for(const kind of ["loadstart","loadedmetadata","playing","waiting","stalled","ended","error","seeking","seeked","pause"])
  media.addEventListener(kind,()=>{const row={kind,id,at:now(),time:media.currentTime,src:media.currentSrc,phase:h.phase,status:h.current?.status()};h.events.push(row);if(kind==="playing"&&h.phase==="main"&&!h.firstPlaying)h.firstPlaying=row;},{capture:true});
 return media;
});
platform.mount();
const p = platform.channel("stream-owner") as Port, other = platform.channel("stream-other") as Port;
const media = () => audioElements.find((item)=>item.isConnected&&item.getAttribute("src")?.startsWith("blob:"))!;
const cache = new Map<number, Uint8Array>();
async function bytes(fragment: Fragment) { let data=cache.get(fragment.sequence);if(!data){data=new Uint8Array(await fetch(fragment.url).then(response=>{if(!response.ok)throw Error("fixture fragment fetch failed");return response.arrayBuffer();}));cache.set(fragment.sequence,data);}return data.slice(); }
const segment = async (fragment: Fragment): Promise<GameAudioStreamSegment> => ({sequence:fragment.sequence,startSample:fragment.startSample,sampleFrames:fragment.sampleFrames,data:await bytes(fragment)});
const stateSafe = (handle: GameAudioStreamHandle) => {const s=handle.status();assert(Number.isSafeInteger(s.nextSeq)&&Number.isSafeInteger(s.acceptedEndSample)&&Number.isFinite(s.currentTime)&&s.pendingBytes<=1048576,"unbounded or invalid stream status");assert(s.buffered.length<=2,"unbounded/discontinuous native range list");return s;};
const cursor = (handle: GameAudioStreamHandle) => {const s=stateSafe(handle);return [s.nextSeq,s.acceptedEndSample];};
const sameCursor = (handle: GameAudioStreamHandle, value: number[]) => {const s=cursor(handle);return s[0]===value[0]&&s[1]===value[1];};
let fixture: Fixture, init: Uint8Array;
const config = (id: string, extra: Partial<GameAudioStreamConfig> = {}): GameAudioStreamConfig => ({playbackID:id,mimeType:fixture.mimeType,sampleRate:fixture.sampleRate,initSegment:init.slice(),maxForwardSamples:22050,retainBackSamples:11025,...extra});
const open = (id: string, port=p, extra: Partial<GameAudioStreamConfig> = {}) => {const handle=port.openStream(config(id,extra));assert(handle,"valid native stream was rejected");return handle!;};
async function feed(handle: GameAudioStreamHandle, piece: GameAudioStreamSegment) {const start=now();while(!await handle.append(piece)){const s=stateSafe(handle);if(["failed","stopped","ended","draining"].includes(s.phase))throw Error("valid append reached terminal stream: "+JSON.stringify(s));if(now()-start>8000)throw Error("valid stream backpressure never cleared");await delay(5);} }
document.addEventListener("visibilitychange",()=>{h.visibility.push({hidden:document.hidden,at:now()});if(document.hidden)platform.stop();});
document.querySelector<HTMLButtonElement>("#start")!.onclick = async event => {
 try {
  assert(event.isTrusted&&!document.hidden,"stream verifier lacks real visible native gesture");
  await platform.unlock();
  assert(typeof p.openStream==="function","platform openStream method is unavailable");
  fixture=await fetch('/manifest').then(response=>response.json());init=new Uint8Array(await fetch(fixture.initURL).then(response=>response.arrayBuffer()));
  assert(fixture.sampleRate===44100&&fixture.fragments.length>=4,"invalid native stream fixture");
  assert(Math.max(...fixture.fragments.map(row=>row.sampleFrames))<=4096,"low-buffer acceptance requires small Go fragments, not the old multi-second preload fixture");
  const first=await segment(fixture.fragments[0]), second=await segment(fixture.fragments[1]);

  h.phase="opening-lifecycle";
  assert(!p.openStream(config('invalid',{mimeType:'audio/mp4; codecs="definitely-invalid"'})),"unsupported MIME created a stream");
  assert(!p.openStream(config('invalid',{sampleRate:48000})),"unsupported sample rate created a stream");
  assert(!p.openStream(config('invalid',{retainBackSamples:11024})),"unsafe back retention accepted");
  assert(!p.openStream(config('invalid',{maxForwardSamples:441001})),"unbounded ahead accepted");
  const replaced=open('opening-old');const current=open('opening-new');
  assert(!await bounded(replaced.ready,"replaced ready"),"replaced opening stream reported ready");
  assert(await bounded(current.ready,"init-only ready"),"init-only ready failed or waited for media play");
  replaced.stop();assert(!["failed","stopped"].includes(current.status().phase),"old stop cancelled new stream");
  assert(!await replaced.append(first)&&!await replaced.end(),"old handle accepted operations");
  check("opening-replacement-and-old-epoch",{state:current.status()});current.stop();

  h.phase="append-contract";
  const mutableInit=init.slice(), owned=open('owned-init',p,{initSegment:mutableInit});mutableInit.fill(0);
  assert(await bounded(owned.ready,"copied init ready"),"caller init mutation poisoned SourceBuffer");
  assert(p.openStream(config('owned-init'))===owned,"stable init/config did not reuse handle");
  assert(!p.openStream(config('owned-init',{retainBackSamples:22050})),"conflicting stable config accepted");
  const before=cursor(owned);
  for(const bad of [{...first,sequence:2},{...first,startSample:1},{...first,sampleFrames:NaN},{...first,startSample:Number.MAX_SAFE_INTEGER},{...first,data:new Uint8Array(1048577)}])assert(!await owned.append(bad)&&sameCursor(owned,before),"invalid metadata changed the receiving cursor");
  const original=first.data.slice();const pending=owned.append(first);const duplicate=owned.append({...first,data:original.slice()});
  assert(pending===duplicate,"identical pending segment did not reuse its Promise");
  const busy=owned.append(second);first.data.fill(0);
  assert(await bounded(pending,"first append")&&await duplicate&&!await busy,"pending copy or bounded busy rejection failed");
  const after=cursor(owned), nativeCalls=h.appends.length;
  assert(await owned.append({...first,data:original.slice()})&&sameCursor(owned,after)&&h.appends.length===nativeCalls,"last identical fragment was appended again");
  const changed=original.slice();changed[changed.length-1]^=1;
  assert(!await owned.append({...first,data:changed})&&sameCursor(owned,after),"same sequence accepted a changed payload");
  assert(!await owned.append({...second,sequence:3})&&sameCursor(owned,after),"gap sequence affected the active stream");
  await feed(owned,second);const afterSecond=cursor(owned);assert(!await owned.append({...first,data:original.slice()})&&sameCursor(owned,afterSecond),"older history was accepted after the last-segment cache moved");
  check("copied-bytes-sequence-and-single-flight",{state:owned.status(),nativeCalls});owned.stop();

  h.phase="pause-stop";
  const paused=open('paused');await bounded(paused.ready,"pause ready");h.current=paused;
  let bootstrap=0;const bootstrapStarted=now();
  while(paused.status().currentTime<=0.03){
   assert(bootstrap<fixture.fragments.length&&now()-bootstrapStarted<5000,"bounded native pause bootstrap did not reach playing");
   const row=fixture.fragments[bootstrap];
   if((row.startSample+row.sampleFrames)/fixture.sampleRate-paused.status().currentTime<=0.26){await feed(paused,await segment(row));bootstrap++;}
   else await delay(5);
  }
  const src=media().src;p.pause();const at=paused.status().currentTime;await delay(250);
  assert(paused.status().phase==="paused"&&Math.abs(paused.status().currentTime-at)<0.005,"pause did not freeze native stream clock");
  assert(await p.resume(),"native stream resume failed");assert(media().src===src&&paused.status().currentTime>=at&&paused.status().currentTime<at+0.15,"resume changed source or position");
  check("native-stream-pause-resume",{state:paused.status()});paused.stop();
  const stopped=open('stop-pending');await bounded(stopped.ready,"stop pending ready");const writing=stopped.append(await segment(fixture.fragments[0]));stopped.stop();
  assert(!await bounded(writing,"stopped append")&&stopped.status().phase==="stopped","updating stop committed a cancelled append");
  const successor=open('next-owner',other);assert(await bounded(successor.ready,"new owner ready"),"new owner failed ready");stopped.stop();p.stop();
  assert(!["stopped","failed"].includes(successor.status().phase),"old owner cleanup stopped new owner stream");successor.stop();
  const muted=open('mute-pending');platform.setEnabled(false);assert(!await bounded(muted.ready,"muted ready"),"mute let opening ready revive");platform.setEnabled(true);
  assert(audioElements.every(item=>!item.getAttribute('src')&&item.paused),"unmute revived a cancelled stream");
  check("stop-owner-and-mute-cleanup");

  h.phase="main";h.events=[];h.clocks=[];h.appends=[];h.removals=[];h.firstPlaying=undefined;
  const main=open('low-buffer-main');h.current=main;assert(await bounded(main.ready,"main init ready"),"main init failed");
  const mainURL=media().src;let previous=0;let clockActive=true;
  const sample=()=>{if(!clockActive)return;const s=stateSafe(main);h.clocks.push({at:now(),...s});for(let i=0;i<fixture.fragments.length-1;i++){const f=fixture.fragments[i],edge=(f.startSample+f.sampleFrames)/fixture.sampleRate;if(previous<edge&&s.currentTime>=edge)h.crossings.push({index:i,boundary:edge,before:previous,after:s.currentTime,at:now()});}previous=s.currentTime;requestAnimationFrame(sample);};requestAnimationFrame(sample);
  let eos:Promise<boolean>|undefined;
  for(let i=0;i<fixture.fragments.length;i++){
   const row=fixture.fragments[i], piece=await segment(row);
   await until(()=>{const s=stateSafe(main);return !s.updating&&(row.startSample+row.sampleFrames)/fixture.sampleRate-s.currentTime<=0.26;});
   if(i===fixture.fragments.length-1){const tail=main.append(piece);eos=main.end();assert(main.end()===eos,"duplicate end created another operation");assert(!await main.append({...piece,sequence:piece.sequence+1,startSample:fixture.totalSamples}),"end request accepted another segment");assert(await bounded(tail,"tail append"),"end dropped the current tail");}
   else await feed(main,piece);
   assert(media().src===mainURL,"stream changed native source during append");
  }
  assert(eos&&await bounded(eos,"EOS commit"),"end failed to commit EOS");
  const draining=main.status();assert(draining.phase==="draining"||draining.phase==="ended"&&Math.abs(draining.currentTime-12)<0.1,"EOS was confused with completed playback");
  await until(()=>main.status().phase==="ended",18000);clockActive=false;
  const final=stateSafe(main);assert(final.acceptedEndSample===fixture.totalSamples&&final.nextSeq===fixture.fragments.length+1,"final receive cursor lost samples");
  assert(Math.abs(final.currentTime-fixture.totalSamples/fixture.sampleRate)<0.1,"native playback did not drain to the final sample time");
  assert(h.crossings.length>=2&&h.firstPlaying,"actual native sample boundaries were not crossed");
  assert(h.firstPlaying.status.buffered.length&&h.firstPlaying.status.buffered.at(-1)[1]-h.firstPlaying.time<=0.5,"first native playing required preload above the configured half-second bound");
  const bad=h.events.filter(row=>row.at>h.firstPlaying.at&&["waiting","stalled","seeking","error"].includes(row.kind));assert(!bad.length,"native stream rebuffered, sought or failed after playing");
  assert(h.appends.length>=fixture.fragments.length,"only a prefix was really appended");
  assert(h.clocks.every(row=>row.buffered.length<=1&&row.buffered.every((range:number[])=>range[1]-row.currentTime<=0.55)),"native forward buffer exceeded the bounded stream window");
  assert(h.removals.length>0&&h.removals.every(row=>row.end<row.currentTime),"back cleanup removed current/future audio or never ran");
  assert(h.clocks.every(row=>row.currentTime<1.5||row.buffered.every((range:number[])=>range[0]>=row.currentTime-1.5)),"native backward buffer grew beyond the configured retention plus bounded trim margin");
  assert(p.openStream(config('low-buffer-main'))===main,"ended stable instance reopened a new stream");
  check("real-low-buffer-incremental-playback",{final,draining,firstPlaying:h.firstPlaying,crossings:h.crossings.length,fragmentCount:fixture.fragments.length,sourcePCMHash:fixture.sourcePCMHash});main.stop();
  for(const row of h.urls.filter(row=>row.kind==='create'))assert(h.urls.filter(other=>other.kind==='revoke'&&other.url===row.url).length===1,"ObjectURL was leaked or revoked more than once");
  const hidden=open('host-hidden');h.current=hidden;await bounded(hidden.ready,"hidden fixture ready");await feed(hidden,await segment(fixture.fragments[0]));
  (h as any).hiddenHandle=hidden;(h as any).platform=platform;(h as any).audioElements=audioElements;
  h.complete=true;document.querySelector('pre')!.textContent=JSON.stringify({complete:true,checks:h.checks.length,crossings:h.crossings.length,firstPlayingBuffer:h.firstPlaying.status.buffered,speakerSampleExact:false},null,2);
 }catch(error){h.error=String(error);platform.stop();document.querySelector('pre')!.textContent=h.error;}
};
