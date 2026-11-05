#!/usr/bin/env python3
"""Test-first real-Chrome native generated-audio feasibility harness.

No core, AudioContext, external capture, production player or browser install.
"""
import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import subprocess
import threading
import time
from urllib.parse import urlsplit
import wave

ROOT=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser()
parser.add_argument('--label',default='smoke-1024')
parser.add_argument('--chunk',type=int,choices=[441,1024],default=1024)
parser.add_argument('--seconds',type=int,choices=[2,4,12],default=12)
parser.add_argument('--scenario',choices=['smoke','jitter','pause','mute','stop','hidden'],default='smoke')
parser.add_argument('--ahead-chunks',type=int,choices=[1,2,4],default=2)
args=parser.parse_args();assert re.fullmatch(r'[a-zA-Z0-9_-]+',args.label)
PRIVATE=ROOT/'.local/audio-pcm-native-probe'
OUT=ROOT/'games/yuyuhakusho/verification/audio-pcm-native-probe'/args.label
for path in [PRIVATE,PRIVATE/'tmp',PRIVATE/'cache',PRIVATE/'config',PRIVATE/'s',OUT]:path.mkdir(parents=True,exist_ok=True)
sha=lambda raw:hashlib.sha256(raw).hexdigest()
old=json.loads((ROOT/'games/yuyuhakusho/data/audio-assets.json').read_text())
source=next(row for row in old['diagnostics']if row['code']==0x82)
wav_path=ROOT/source['media_file'];assert sha(wav_path.read_bytes())==source['wav_sha256']
with wave.open(str(wav_path),'rb')as wav:
 assert(wav.getnchannels(),wav.getsampwidth(),wav.getframerate())==(2,2,44100)
 pcm=wav.readframes(args.seconds*44100)
assert len(pcm)==args.seconds*44100*4
if args.seconds==12:assert sha(pcm)=='7801d4c5f7ae515e7819c69637f3974ed2feedfc0362df2370ec28f5b8d55f4c'
(PRIVATE/f'82-first-{args.seconds}s.pcm').write_bytes(pcm)
manifest={'code':0x82,'format':'s16','sampleRate':44100,'channels':2,'sampleFrames':len(pcm)//4,'pcmSHA256':sha(pcm),'url':'/pcm',
          'chunkFrames':args.chunk,'aheadChunks':args.ahead_chunks,'scenario':args.scenario,'inputWAVSHA256':source['wav_sha256']}
env=dict(os.environ,TMPDIR=str(PRIVATE/'tmp'),XDG_CACHE_HOME=str(PRIVATE/'cache'),XDG_CONFIG_HOME=str(PRIVATE/'config'),AGENT_BROWSER_SOCKET_DIR='s')
seed=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pcm-native-'+args.label],cwd=PRIVATE,env=env,text=True).strip()
env['AGENT_BROWSER_SESSION']='pn'+sha((seed+args.label).encode())[:6]
requests=[]
guard="""window.__forbiddenCalls=[];for(const name of ['AudioContext','webkitAudioContext']){const C=window[name];if(C)window[name]=new Proxy(C,{construct(){window.__forbiddenCalls.push(name);throw Error('Forbidden audio context');}});}for(const name of ['getUserMedia','getDisplayMedia']){if(navigator.mediaDevices?.[name])navigator.mediaDevices[name]=()=>{window.__forbiddenCalls.push(name);return Promise.reject(Error('Forbidden external capture'));};}window.__gestureEvidence=[];document.addEventListener('click',e=>{if(e.target instanceof HTMLButtonElement)window.__gestureEvidence.push({id:e.target.id,trusted:e.isTrusted,hidden:document.hidden,at:performance.now()});},true);"""
html='''<!doctype html><meta charset="utf-8"><title>Native generated PCM research</title><h1>Native generated PCM research</h1><p>Research only — generated track, native audio, no external capture</p><button id="start">Start generated PCM</button><button id="pause">Pause</button><button id="resume">Resume</button><button id="mute">Mute and release</button><button id="stop">Stop and release</button><pre id="status">Loading probe</pre>'''
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*items):pass
 def do_GET(self):
  route=urlsplit(self.path).path;requests.append(route)
  if route=='/':body=(html+'<script>'+guard+'</script><script type="module" src="/probe.js"></script>').encode();mime='text/html'
  elif route=='/blank':body=b'<!doctype html><title>Owned PCM visibility control</title>Owned blank control';mime='text/html'
  elif route=='/manifest':body=json.dumps(manifest).encode();mime='application/json'
  elif route=='/pcm':body=pcm;mime='application/octet-stream'
  elif route=='/probe.js':
   path=ROOT/'scripts/yuyuhakusho/audio_pcm_native_probe.js'
   if not path.exists():self.send_error(404);return
   body=path.read_bytes();mime='text/javascript'
  else:self.send_error(404);return
  self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(body)));self.send_header('Cache-Control','no-store');self.send_header('Permissions-Policy','microphone=(),camera=(),display-capture=()');self.end_headers()
  try:self.wfile.write(body)
  except(BrokenPipeError,ConnectionResetError):pass
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();url=f'http://127.0.0.1:{server.server_port}/'
def browser(*items):
 result=subprocess.run(['agent-browser',*items,'--json'],cwd=PRIVATE,env=env,capture_output=True,text=True,timeout=45)
 if result.returncode:raise RuntimeError(result.stdout+result.stderr)
 value=json.loads(result.stdout)
 if not value['success']:raise RuntimeError(str(value))
 return value['data']
def js(expression):return browser('eval',expression)['result']
def visibility(action='activate'):
 endpoint=browser('get','cdp-url');endpoint=endpoint.get('url')or endpoint.get('cdpUrl')or endpoint.get('value')
 result=subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/activate_pcm_native_probe.mjs'),endpoint,url,action],cwd=PRIVATE,env=env,capture_output=True,text=True,timeout=20)
 assert result.returncode==0,result.stderr
 return json.loads(result.stdout)
def expect(value,message):
 if not value:raise AssertionError(message)
opened=False
try:
 chrome=Path('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome')
 launch=['--profile',str(PRIVATE/'profile'),'--args',f'--disk-cache-dir={PRIVATE/"cache"},--disable-component-update,--no-first-run,--no-default-browser-check,--enable-automation']
 if chrome.exists():launch+=['--executable-path',str(chrome)]
 browser(*launch,'open',url);opened=True;browser('set','viewport','960','720');browser('snapshot','-i')
 setup=visibility();browser('tab',setup['targetId']);(OUT/'setup.json').write_text(json.dumps(setup,indent=2)+'\n')
 exists=js('typeof window.pcmNativeProbe!=="undefined"')
 expect(exists,'Missing native generated-track probe implementation')
 browser('wait','--fn','pcmNativeProbe.ready || !!pcmNativeProbe.error')
 expect(js('pcmNativeProbe.error')=='','Probe input or initialization failed')
 browser('click','#start');browser('wait','--fn','pcmNativeProbe.snapshot().playingCount > 0 || !!pcmNativeProbe.error')
 expect(js('pcmNativeProbe.error')=='','Native generator did not reach playing')
 browser('wait','--fn','pcmNativeProbe.snapshot().maxCurrentTime > 0.45 || !!pcmNativeProbe.error')
 expect(js('pcmNativeProbe.error')=='','Native media clock did not progress')
 if args.scenario=='pause':
  browser('click','#pause');browser('wait','--fn','pcmNativeProbe.snapshot().phase==="paused"')
  before=js('pcmNativeProbe.snapshot()');time.sleep(.3);after=js('pcmNativeProbe.snapshot()')
  expect(abs(after['currentTime']-before['currentTime'])<.04,'Native pause clock continued')
  expect(after['acceptedSamples']==before['acceptedSamples'],'Paused producer kept filling')
  (OUT/'pause.json').write_text(json.dumps({'before':before,'after':after},indent=2)+'\n');browser('click','#resume')
 elif args.scenario in ['mute','stop']:
  browser('click','#'+args.scenario)
 elif args.scenario=='hidden':
  hidden=visibility('hide');(OUT/'hidden.json').write_text(json.dumps(hidden,indent=2)+'\n')
 if args.scenario=='hidden':result=hidden['probe']
 else:
  browser('wait','--fn','pcmNativeProbe.complete || !!pcmNativeProbe.error')
  result=js('pcmNativeProbe.result()')
 (OUT/'observed.json').write_text(json.dumps(result,indent=2)+'\n')
 expect(not result['error'],result['error'])
 run=result['run'];expect(result['forbiddenCalls']==[],'Probe used forbidden audio/capture API')
 expect(any(item['id']=='start'and item['trusted']and not item['hidden']for item in result['gestures']),'Missing real visible trusted start gesture')
 expect(run['playingCount']>0 and run['maxCurrentTime']>.45,'Accepted writes without actual native playing/currentTime')
 expect(run['maxPendingWrites']<=1 and run['maxAheadSamples']<=args.chunk*args.ahead_chunks+args.chunk,'Unbounded producer queue/ahead')
 expect(run['audioDataCreated']==run['audioDataClosed'],'AudioData leak')
 expect(run['resources']['trackEnded']and run['resources']['writerUnlocked']and run['resources']['srcObjectCleared']and run['resources']['elementRemoved'],'Native resources were not released')
 cursor=0
 for packet in run['packets']:
  if not packet['accepted']:continue
  expect(packet['startSample']==cursor and packet['timestamp']==cursor*1000000//44100,'Accepted source gap/duplicate/timestamp drift')
  expect(packet['frames']>0 and packet['frames']<=args.chunk and packet['dataFrames']==packet['frames'],'AudioData frame count differs')
  expected_pcm=pcm[cursor*4:(cursor+packet['frames'])*4]
  checksum=2166136261
  for byte in expected_pcm:checksum=((checksum^byte)*16777619)&0xffffffff
  expect(packet['dataCopyFNV']==checksum,'Constructed AudioData copy differs from original packet')
  cursor+=packet['frames']
 expect(cursor==run['acceptedSamples'],'Accepted packet ledger differs from source cursor')
 if args.scenario in ['smoke','jitter','pause']:
  expect(run['acceptedSamples']==manifest['sampleFrames'],'Source sample cursor is incomplete')
  expect(run['maxCurrentTime']>=args.seconds-.25,'Native played only an initial prefix')
  expect(run['inputContiguous']and run['audioDataCopyExact'],'Timestamp/source PCM input changed')
 else:expect(run['reason']==args.scenario,'Lifecycle cancellation reason differs')
 result.update(pass_=True,scope='Research-only generated PCM track into native HTMLAudio; input continuity, not speaker sample identity',manifest=manifest,requests=requests,setup=setup,
  source_sha256={name:sha((ROOT/name).read_bytes())for name in ['scripts/yuyuhakusho/check_audio_pcm_native_probe.py','scripts/yuyuhakusho/activate_pcm_native_probe.mjs','scripts/yuyuhakusho/audio_pcm_native_probe.js','docs/research/md-yuyuhakusho/AUDIO_PCM_NATIVE_PROBE_FAILURES.md']},speaker_bit_exact=False,original_mix_equivalent=False,safari_mobile_verified=False)
 result['pass']=result.pop('pass_');(OUT/'result.json').write_text(json.dumps(result,indent=2)+'\n')
 if args.scenario=='hidden':
  shown=visibility();browser('tab',shown['targetId'])
 browser('screenshot',str(OUT/'desktop.png'));js('pcmNativeProbe.dispose()');print(json.dumps({'pass':True,'firstPlayingMs':run['firstPlaying']['delayMs'],'acceptedSamples':run['acceptedSamples'],'maxCurrentTime':run['maxCurrentTime'],'scenario':args.scenario}))
except Exception as error:
 (OUT/'failure.json').write_text(json.dumps({'error':repr(error),'requests':requests,'manifest':manifest},indent=2)+'\n')
 if opened:
  try:
   (OUT/'failure-state.json').write_text(json.dumps(js('typeof pcmNativeProbe!=="undefined"?pcmNativeProbe.result():{apiExists:false,gestures:window.__gestureEvidence,forbiddenCalls:window.__forbiddenCalls}'),indent=2)+'\n');browser('screenshot',str(OUT/'failure.png'))
  except Exception:pass
 raise
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=5)
