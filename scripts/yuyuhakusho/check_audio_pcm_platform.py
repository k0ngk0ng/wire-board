"""Real production-platform PCM API E2E; source fidelity is not speaker fidelity."""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import argparse,hashlib,json,os,subprocess,threading,time,wave
ROOT=Path(__file__).resolve().parents[2];parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--label',required=True);parser.add_argument('--seconds',type=int,default=12);parser.add_argument('--chunk',type=int,choices=[441,1024],default=1024);parser.add_argument('--ahead',type=int,default=2048);parser.add_argument('--scenario',choices=['smoke','contract','jitter','pause','mute','stop','hidden','late'],default='smoke');args=parser.parse_args();assert 1<=args.seconds<=12 and 1<=args.ahead<=2048 and '/'not in args.label
OUT=ROOT/'games/yuyuhakusho/verification/audio-platform-pcm'/args.label;PRIVATE=ROOT/'.local/audio-platform-pcm';OUT.mkdir(parents=True,exist_ok=True)
for p in [PRIVATE,PRIVATE/'tmp',PRIVATE/'sock']:p.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(PRIVATE/'tmp'),AGENT_BROWSER_SOCKET_DIR='sock',NODE_COMPILE_CACHE=str(PRIVATE/'node-cache'));seed=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','public-pcm'],cwd=PRIVATE,env=env,text=True).strip();env['AGENT_BROWSER_SESSION']='pp'+seed.rsplit('-',1)[-1][:6]
source=ROOT/'.local/md-yuyuhakusho/audio-assets-final/isolated/music-excerpts/82.wav';assert hashlib.sha256(source.read_bytes()).hexdigest()=='85b105ce9ea41d49cf62da9b0a2612bbfdb436a77a9b6e4c0d72cd8ff3167c50'
with wave.open(str(source),'rb')as w:assert w.getnchannels()==2 and w.getframerate()==44100 and w.getsampwidth()==2;pcm=w.readframes(args.seconds*44100)
manifest={'sampleFrames':len(pcm)//4,'pcmSHA256':hashlib.sha256(pcm).hexdigest(),'chunk':args.chunk,'maxAhead':args.ahead,'scenario':args.scenario};mse=ROOT/'.local/md-yuyuhakusho/audio-encoder-plan/e2e/native/original12/small-bulk/init.mp4'
subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),'scripts/yuyuhakusho/audio_pcm_platform_harness.ts','--bundle','--format=esm','--target=es2022','--outfile='+str(PRIVATE/'harness.js')],cwd=ROOT,env=env,check=True)
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  if self.path=='/':data=b'<!doctype html><meta charset="utf-8"><title>Platform PCM API</title><button id="start">Start native platform PCM</button><button id="pause">Pause owner</button><button id="resume">Resume owner</button><button id="mute">Mute platform</button><button id="stop">Stop owner</button><script type="module" src="/harness.js"></script>';mime='text/html'
  elif self.path=='/harness.js':data=(PRIVATE/'harness.js').read_bytes();mime='text/javascript'
  elif self.path=='/manifest':data=json.dumps(manifest).encode();mime='application/json'
  elif self.path=='/source.pcm':data=pcm;mime='application/octet-stream'
  elif self.path=='/mse-init':data=mse.read_bytes();mime='audio/mp4'
  else:self.send_error(404);return
  self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();url=f'http://127.0.0.1:{server.server_port}/'
def browser(*items):
 p=subprocess.run(['agent-browser',*items,'--json'],cwd=PRIVATE,env=env,text=True,capture_output=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(code):return browser('eval',code)['result']
def activate(action='activate'):
 ep=browser('get','cdp-url');ep=ep.get('url')or ep.get('cdpUrl')or ep.get('value');p=subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/activate_public_pcm.mjs'),ep,url,action],cwd=PRIVATE,env=env,text=True,capture_output=True,timeout=25);assert p.returncode==0,p.stderr;return json.loads(p.stdout)
opened=False
try:
 chrome=Path('/Applications/Google Chrome.app/Contents/MacOS/Google Chrome');launch=['--profile',str(PRIVATE/'profile'),'--args',f'--disk-cache-dir={PRIVATE/"cache"},--disable-component-update,--no-first-run,--no-default-browser-check,--enable-automation'];launch+=['--executable-path',str(chrome)]if chrome.exists()else[]
 browser(*launch,'open',url);opened=True;setup=activate();browser('tab',setup['targetId']);browser('wait','--fn','publicPCM.ready || !!publicPCM.error');(OUT/'setup.json').write_text(json.dumps(setup,indent=2)+'\n');browser('click','#start');browser('wait','--fn','publicPCM.firstPlaying || publicPCM.complete || !!publicPCM.error')
 if args.scenario in ['pause','mute','stop','hidden']:
  browser('wait','--fn','publicPCM.snapshot()?.nativeCurrentTimeSeconds > .45 || publicPCM.complete || !!publicPCM.error')
  assert not js('publicPCM.error'),js('publicPCM.error')
  if args.scenario=='pause':
   browser('click','#pause');time.sleep(.03);before=js('publicPCM.snapshot()');time.sleep(.3);after=js('publicPCM.snapshot()');assert before['phase']==after['phase']=='paused'and before['sourceClockSamples']==after['sourceClockSamples']and before['acceptedEndSample']==after['acceptedEndSample'];(OUT/'pause.json').write_text(json.dumps({'before':before,'after':after},indent=2)+'\n');browser('click','#resume')
  elif args.scenario=='hidden':hidden=activate('hide');(OUT/'hidden.json').write_text(json.dumps(hidden,indent=2)+'\n')
  else:browser('click','#'+args.scenario)
 if args.scenario=='hidden':r=hidden['result']
 else:browser('wait','--fn','publicPCM.complete || !!publicPCM.error');r=js('publicPCM.result()')
 (OUT/'observed.json').write_text(json.dumps(r,indent=2)+'\n');assert not r['error'],r['error'];assert not r['forbidden'];assert r['firstPlaying']and r['firstPlaying']['status']['nativePlayedSamples']is None and r['firstPlaying']['status']['nativeDroppedSamples']is None
 cursor=0
 for row in r['packets']:
  if row['accepted']:assert row['startSample']==cursor;cursor+=row['frames']
 if args.scenario in ['smoke','contract','jitter','pause']:assert cursor==manifest['sampleFrames']and r['reason']=='input-ended'and r['inputEnded']['status']['phase']=='input-ended'and not r['inputEnded']['released']
 else:assert r['reason']==args.scenario
 frames=[f for f in r['frames']if f['stage']=='main'];assert all(f['closes']==1 for f in r['frames'])
 start=0
 for f in frames:
  assert f['format']=='s16'and f['sampleRate']==44100 and f['channels']==2 and f['timestamp']==start*1000000//44100
  value=2166136261
  for b in pcm[start*4:(start+f['frames'])*4]:value=((value^b)*16777619)&0xffffffff
  assert f['copyFNV']==value;start+=f['frames']
 assert r['after']['framesCreated']==r['after']['framesClosed']and all(t['state']=='ended'and not t['locked']for t in r['after']['tracks'])and all(m['srcObjectCleared']and m['paused']and m['removed']for m in r['after']['media'])
 r.update(pass_=True,scope='Real platform-owned native PCM input/lifecycle/admission; native played/dropped counts and speaker output are unobservable',source_sha256={name:hashlib.sha256((ROOT/name).read_bytes()).hexdigest()for name in ['web/src/audio.ts','web/src/game-api/module.ts','web/src/game-api/audio-pcm-stream.ts','scripts/yuyuhakusho/audio_pcm_platform_harness.ts','scripts/yuyuhakusho/check_audio_pcm_platform.py']},speaker_bit_exact=False,original_mix_equivalent=False,manifest=manifest);r['pass']=r.pop('pass_');(OUT/'result.json').write_text(json.dumps(r,indent=2)+'\n')
 if args.scenario=='hidden':shown=activate();browser('tab',shown['targetId'])
 browser('screenshot',str(OUT/'desktop.png'));print(json.dumps({'pass':True,'scenario':args.scenario,'accepted':cursor,'firstPlayingMs':r['firstPlaying']['gestureDelayMs']}))
except Exception as e:
 (OUT/'failure.json').write_text(json.dumps({'error':repr(e)},indent=2)+'\n')
 if opened:
  try:(OUT/'failure-state.json').write_text(json.dumps(js('typeof publicPCM!=="undefined"?publicPCM.result():null'),indent=2)+'\n')
  except Exception:pass
 raise
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=3)
