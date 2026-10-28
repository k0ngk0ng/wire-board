"""Native platform/Contra adapter integration; predeclared failure coverage.

Checks source-bound media, per-slot pulse mute, original pause snapshots,
reset/replace, history-gap resync, old snapshots, global mute, hidden and dispose.
No fake media objects; no assertion of physical audibility or bit-identical mix.
"""
import argparse
import functools
import hashlib
import http.server
import json
import os
import re
from pathlib import Path
import shutil
import subprocess
import threading
import time
import traceback
import uuid

ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser();p.add_argument('--out',required=True);args=p.parse_args()
out=(ROOT/args.out).resolve();assert out.is_relative_to(ROOT) and not out.exists();out.mkdir(parents=True)
temp=out/'temporary';temp.mkdir();run='ca'+uuid.uuid4().hex[:6];sock=ROOT/'.s'/run;sock.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SESSION=run,AGENT_BROWSER_SOCKET_DIR=str(sock),AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter',TMPDIR=str(temp/'tmp'),GOTMPDIR=str(temp/'tmp'))
Path(env['TMPDIR']).mkdir();report={'passed':False,'checks':[],'scope':'Production Go AudioState snapshots → Contra adapter → platform GameAudio → native Chromium HTMLAudioElement. No ROM runtime, physical-device audio capture, listening or nonlinear-mix acceptance.'};server=None;opened=False

def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=ROOT,env=env,capture_output=True,text=True,timeout=45)
 assert r.returncode==0,r.stdout+r.stderr
 value=json.loads(r.stdout);assert value['success'],value;return value['data']
def js(code):return browser('eval',code)['result']
def check(name,code):
 value=js(code);assert value['passed'],(name,value);report['checks'].append({'name':name,**value})

try:
 fixture=subprocess.run(['go','run','scripts/contra/audio_fixture.go'],cwd=ROOT,env=env,capture_output=True,text=True,check=True)
 (temp/'views.json').write_text(fixture.stdout)
 source=json.loads((ROOT/'games/contra/assets/audio-source.json').read_text());loops=json.loads((ROOT/'games/contra/assets/audio-loops.json').read_text())
 paths={}
 for sound in source['sounds']:
  if sound['id']==110:paths[sound['researchDirectory']+'/'+sound['file']]=sound['sha256']
  for stem in sound['stems']:
   path=stem['researchDirectory']+'/'+stem['file'];paths[path]=stem['sha256']
 for song in loops['songs']:
  for stem in song['stems']:paths[stem['file']]=stem['sha256']
 # Serve only SHA-verified declared media files; never expose the workspace.
 (temp/'catalogs.json').write_text(json.dumps({'source':source,'loops':loops}))
 (temp/'fixture.ts').write_text('''import {GameAudio} from "'''+str(ROOT/'web/src/audio.ts')+'''";
import {ContraAudioAdapter,createContraAudioResources} from "'''+str(ROOT/'games/contra/web/audio.ts')+'''";
const [views,catalogs]=await Promise.all([fetch('./views.json').then(r=>r.json()),fetch('./catalogs.json').then(r=>r.json())]);
const a=new GameAudio();a.mount();const port=a.channel('contra-native');
let slow=false;const resources=createContraAudioResources(catalogs.source,catalogs.loops,(path,sha)=>['/media/'+sha+(slow?'?slow=1':'')]);
const adapter=new ContraAudioAdapter(port,resources,'native-audio-fixture');
const t=(window as any).fixture={a,port,adapter,views,resources,setSlow:(value:boolean)=>slow=value,apply:(key:string,resync=false)=>adapter.update(views[key],{resync}),media:(key:string)=>document.querySelector(`[data-platform-track="contra/${key}"]`),events:[]};
document.querySelector('#start')!.addEventListener('click',async()=>{await a.unlock();t.apply('empty');t.apply('music');t.started=true;});
document.querySelector('#mute')!.addEventListener('click',()=>a.setEnabled(false));
document.querySelector('#unmute')!.addEventListener('click',()=>a.setEnabled(true));
document.addEventListener('visibilitychange',()=>{t.events.push(document.hidden?'hidden':'visible');if(document.hidden){adapter.suspend();a.stop()}});
window.addEventListener('beforeunload',()=>{adapter.dispose();a.dispose()});t.ready=true;
''')
 bundle=subprocess.run(['node','--input-type=module','-e',"import{build}from'./web/node_modules/esbuild/lib/main.js';await build({entryPoints:[process.argv[1]],outfile:process.argv[2],bundle:true,format:'esm',target:'es2022',platform:'browser'});",str(temp/'fixture.ts'),str(temp/'fixture.js')],cwd=ROOT,env=env,capture_output=True,text=True,check=True)
 (temp/'index.html').write_text('''<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>魂斗罗原生音频验收</title><style>body{font:16px system-ui;color:#202d28;background:#f4f1e6;margin:24px}button{font:inherit;padding:12px;margin:4px;color:#202d28;background:#fff;border:1px solid #596b61}pre{white-space:pre-wrap;overflow-wrap:anywhere}</style><h1>魂斗罗音频接入</h1><button id="start">开始验证</button><button id="mute">静音</button><button id="unmute">取消静音</button><pre id="status">等待原版控制快照</pre><script type="module" src="fixture.js"></script></html>''')
 bysha={sha:ROOT/path for path,sha in paths.items()}
 class Handler(http.server.SimpleHTTPRequestHandler):
  def log_message(self,*_):pass
  def do_GET(self):
   if self.path.startswith('/media/'):
    sha=self.path.removeprefix('/media/').split('?')[0];path=bysha.get(sha)
    if not path:self.send_error(404);return
    if self.path.endswith('?slow=1'):time.sleep(.35)
    raw=path.read_bytes();assert hashlib.sha256(raw).hexdigest()==sha
    total=len(raw);start=0;end=total-1;status=200
    requested=self.headers.get('Range')
    if requested:
     match=re.fullmatch(r'bytes=(\d+)-(\d*)',requested);assert match,requested
     start=int(match.group(1));end=min(total-1,int(match.group(2)) if match.group(2) else total-1);status=206
    self.send_response(status);self.send_header('Content-Type','audio/wav');self.send_header('Accept-Ranges','bytes')
    if status==206:self.send_header('Content-Range',f'bytes {start}-{end}/{total}')
    self.send_header('Content-Length',str(end-start+1));self.end_headers();self.wfile.write(raw[start:end+1])
   else:super().do_GET()
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=temp));threading.Thread(target=server.serve_forever,daemon=True).start();base=f'http://127.0.0.1:{server.server_port}'
 browser('--profile',str(temp/'profile'),'open',base);opened=True;browser('wait','--fn','window.fixture?.ready===true');browser('snapshot','-i');browser('click','#start');browser('wait','--fn','window.fixture.started&&fixture.media("slot-0")?.currentTime>.06')
 check('original music starts three independent tonal lanes', '''(()=>{const t=fixture,m=['slot-0','slot-1','slot-2'].map(t.media);return {passed:m.every(x=>x instanceof HTMLAudioElement&&!x.paused)&&t.adapter.missingResources.length===0,tracks:m.map(x=>({key:x.dataset.platformTrack,time:x.currentTime,src:x.currentSrc}))}})()''')
 check('accepted gun mutes pulse1 without pausing its clock', '''(()=>{const t=fixture;t.music=t.media('slot-0');t.before=t.music.currentTime;t.apply('shot');return {passed:t.music.muted&&!t.music.paused&&!!t.media('slot-4')&&!!t.media('slot-5'),time:t.before}})()''')
 browser('wait','--fn','fixture.music.currentTime>fixture.before+.08')
 check('pause freezes background and old noise while cue74 plays', '''(()=>{const t=fixture;t.apply('pause');t.pauseTime=t.music.currentTime;const cue=t.media('slot-4'),noise=t.media('slot-5');return {passed:t.music.paused&&noise.paused&&cue&&!cue.paused,time:t.pauseTime}})()''')
 browser('wait','--fn','fixture.media("slot-4")?.currentTime>.08')
 check('cue expiry releases its lane and keeps prior noise frozen', '''(()=>{const t=fixture;t.apply('pauseEnd');return {passed:t.music.paused&&t.music.currentTime===t.pauseTime&&!t.media('slot-4')&&t.media('slot-5').paused,time:t.music.currentTime}})()''')
 check('resume accepts a new gun and keeps music identity', '''(()=>{const t=fixture;t.apply('resume');return {passed:t.music===t.media('slot-0')&&!t.music.paused&&t.music.muted&&!!t.media('slot-4')&&!!t.media('slot-5')}})()''')
 check('reset stops old song before replacing all tonal lanes', '''(()=>{const t=fixture;const old=t.music;t.apply('reset');t.music=t.media('slot-0');return {passed:old.paused&&!old.isConnected&&t.music!==old&&!t.music.paused&&!t.music.muted}})()''')
 check('history gap restores song position and never missed effects', '''(()=>{const t=fixture;const old=t.music;t.apply('gap');return {passed:t.media('slot-0')===old&&!t.media('slot-4')&&!t.media('slot-5'),time:old.currentTime,sequence:t.views.gap.sequence}})()''')
 browser('wait','--fn','fixture.music.currentTime>2')
 check('old snapshot and old epoch cannot rewind or replace', '''(()=>{const t=fixture,before=t.music.currentTime;t.apply('shot');t.adapter.update({...t.views.gap,epoch:'old'});return {passed:t.music===t.media('slot-0')&&t.music.currentTime>=before&&!t.media('slot-4'),time:t.music.currentTime}})()''')
 browser('click','#mute');browser('click','#unmute')
 check('unmute resync cannot resurrect stopped playback IDs', '''(()=>{const t=fixture;t.apply('gap',true);return {passed:[...document.querySelectorAll('[data-platform-track]')].every(x=>x.paused)&&t.music.paused&&!t.music.isConnected}})()''')
 # New matches use explicit epochs in this integration fixture; the adapter
 # itself never invents event IDs to recover a stopped lane.
 js("fixture.adapter.bindEpoch('new-match');fixture.adapter.update({...fixture.views.empty,epoch:'new-match'});fixture.currentNew={...fixture.views.music,epoch:'new-match',voices:fixture.views.music.voices.map(v=>({...v,playbackID:'new-'+v.playbackID})),events:fixture.views.music.events.map(e=>({...e,voice:e.voice?{...e.voice,playbackID:'new-'+e.voice.playbackID}:undefined}))};fixture.adapter.update(fixture.currentNew);true")
 browser('wait','--fn','fixture.media("slot-0")?.currentTime>.06')
 js("fixture.beforeHidden=fixture.media('slot-0');true")
 browser('tab','new',base);browser('wait','--fn','window.fixture?.ready===true');browser('tab','t1')
 check('native page hiding stops all lanes before explicit recovery', '''(()=>{const t=fixture;return {passed:t.events.includes('hidden')&&t.beforeHidden.paused&&!t.beforeHidden.isConnected,events:t.events}})()''')
 js("fixture.returned={...fixture.currentNew,tick:601,voices:fixture.currentNew.voices.map(v=>({...v,positionTicks:600}))};fixture.adapter.update(fixture.returned,{resync:true});true")
 browser('wait','--fn','fixture.media("slot-0")?.currentTime>9')
 check('same-ID background recovery restores current music only and preserves source rate', '''(()=>{const t=fixture,m=t.media('slot-0'),v=t.returned.voices.find(v=>v.key==='slot-0'),r=t.resources.resolve(v),expected=600*r.tickSeconds;return {passed:m!==t.beforeHidden&&!m.paused&&Math.abs(m.currentTime-expected)<.25&&Math.abs(m.playbackRate-r.tickSeconds*60)<1e-9&&!t.media('slot-4')&&!t.media('slot-5'),position:m.currentTime,expected,rate:m.playbackRate}})()''')
 # Near-end authoritative positions verify the new source rate survives the
 # native prefix/suffix restart, without waiting a full 50-second song period.
 js("fixture.loopVoice=fixture.returned.voices.find(v=>v.key==='slot-0');fixture.loopResource=fixture.resources.resolve(fixture.loopVoice);fixture.loopAge=Math.floor((fixture.media('slot-0').duration-.12)/fixture.loopResource.tickSeconds);fixture.adapter.update({...fixture.returned,tick:4000,voices:fixture.returned.voices.map(v=>({...v,positionTicks:fixture.loopAge}))},{resync:true});true")
 browser('wait','--fn','fixture.media("slot-0").currentTime<fixture.loopResource.loopStartSeconds+.4')
 check('rate-corrected native loop restarts at verified suffix with rate preserved', '''(()=>{const t=fixture,m=t.media('slot-0'),r=t.loopResource;return {passed:!m.paused&&m.currentTime>=r.loopStartSeconds-.02&&Math.abs(m.playbackRate-r.tickSeconds*60)<1e-9,position:m.currentTime,loopStart:r.loopStartSeconds,rate:m.playbackRate}})()''')
 # Real delayed HTTP audio response, while accepted views continue to draw.
 js("fixture.adapter.bindEpoch('late-match');fixture.setSlow(true);fixture.adapter.update({...fixture.views.empty,epoch:'late-match'});fixture.late={...fixture.currentNew,epoch:'late-match',voices:fixture.currentNew.voices.map(v=>({...v,playbackID:'late-'+v.playbackID})),events:fixture.currentNew.events.map(e=>({...e,voice:e.voice?{...e.voice,playbackID:'late-'+e.voice.playbackID}:undefined}))};fixture.adapter.update(fixture.late);fixture.lateTick=0;fixture.painter=setInterval(()=>{fixture.lateTick++;fixture.latestLate={...fixture.late,tick:1+fixture.lateTick,voices:fixture.late.voices.map(v=>({...v,positionTicks:fixture.lateTick}))};fixture.adapter.update(fixture.latestLate)},1000/60);true")
 browser('wait','--fn','fixture.media("slot-0")?.currentTime>.25&&fixture.lateTick>20')
 check('delayed native media aligns to latest visible voice after load', '''(()=>{const t=fixture,m=t.media('slot-0'),v=t.latestLate.voices.find(v=>v.key==='slot-0'),expected=v.positionTicks*t.resources.resolve(v).tickSeconds;clearInterval(t.painter);return {passed:Math.abs(m.currentTime-expected)<.08,position:m.currentTime,expected,tick:t.lateTick,rate:m.playbackRate}})()''')
 # Real final handoff: server audio updates end before age7 percussion, but
 # the source-verified complete finite cue contains all 39 original requests.
 js("fixture.adapter.bindEpoch('native-terminal');fixture.setSlow(false);fixture.adapter.update(fixture.views.terminalEmpty);fixture.adapter.update(fixture.views.terminalStart);fixture.terminalStems=['slot-0','slot-1','slot-2'].map(fixture.media);fixture.adapter.update(fixture.views.terminal,{finalized:true});true")
 browser('wait','--fn','fixture.media("terminal-cue")?.currentTime>.07')
 check('live terminal 6E replaces all tonal stems with one source-verified complete finite cue', '''(()=>{const t=fixture,m=t.media('terminal-cue'),v=t.views.terminal.voices.find(v=>v.key==='slot-0'),expected=v.positionTicks*.016639;t.terminalMedia=m;return {passed:t.terminalStems.every(x=>x.paused&&!x.isConnected)&&m instanceof HTMLAudioElement&&!m.loop&&!m.paused&&m.currentSrc.endsWith('c5c16a2b174c64c4a2d2a2fef2ba5507311c8db66f09efd01ed62240731f3fe8')&&Math.abs(m.currentTime-expected)<.3&&Math.abs(m.playbackRate-.99834)<1e-9&&!t.media('slot-0')&&!t.media('slot-1')&&!t.media('slot-2'),position:m.currentTime,expected,duration:m.duration,rate:m.playbackRate}})()''')
 browser('wait','--fn','fixture.terminalMedia.ended')
 check('completed terminal cue and duplicate finalized views never replay', '''(()=>{const t=fixture;t.adapter.update(t.views.terminal,{finalized:true});return {passed:t.media('terminal-cue')===t.terminalMedia&&t.terminalMedia.ended&&t.terminalMedia.paused,duration:t.terminalMedia.duration}})()''')
 browser('click','#mute');browser('click','#unmute')
 js("fixture.adapter.update(fixture.views.terminal,{finalized:true,resync:true});true")
 check('global unmute and finalized reconnect cannot replay the finite terminal cue', '''(()=>{const t=fixture;return {passed:t.terminalMedia.paused&&!t.terminalMedia.isConnected&&[...document.querySelectorAll('[data-platform-track]')].every(x=>x.paused)}})()''')
 check('dispose and late updates cannot restart media', '''(()=>{const t=fixture;t.adapter.dispose();t.apply('gap',true);return {passed:[...document.querySelectorAll('[data-platform-track]')].every(x=>x.paused)}})()''')
 js("document.querySelector('#status').textContent='原生媒体验证通过：原版声音快照、声道抢占、暂停恢复、重连、静音与离桌清理。';true")
 browser('set','viewport','1100','780');browser('screenshot',str(out/'desktop.png'));browser('set','viewport','320','740');browser('screenshot',str(out/'mobile-320.png'))
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc()
 if opened:
  try:report['browserErrors']=browser('errors');report['page']=js('({text:document.body.innerText,tracks:[...document.querySelectorAll("[data-platform-track]")].map(x=>({key:x.dataset.platformTrack,time:x.currentTime,paused:x.paused,muted:x.muted,error:x.error?.message}))})')
  except BaseException:pass
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:server.shutdown();server.server_close()
 for path in [temp,sock]:
  if path.exists():shutil.rmtree(path)
 report['sources']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [ROOT/'web/src/audio.ts',ROOT/'games/contra/web/audio.ts',ROOT/'games/contra/engine/audio_control.go',ROOT/'games/contra/engine/audio_control_data.json',ROOT/'scripts/contra/audio_fixture.go',Path(__file__)]}
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps(report,ensure_ascii=False,indent=2))
raise SystemExit(0 if report['passed'] else 1)
