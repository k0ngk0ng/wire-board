"""Actual native MP3 seeking from an authoritative host music checkpoint."""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import json,os,re,subprocess,threading,uuid
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'.local/basketball/audio-recovery-browser';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkar-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bkar-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
requests=[];checks=[];opened=False
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=40)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 v=json.loads(r.stdout)
 if not v['success']:raise RuntimeError(v)
 return v['data']
def js(source):return browser('eval',source)['result']
def check(name,value):assert value,name;checks.append(name)
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  route=self.path.split('?')[0];requests.append(route)
  if route=='/':body=b'<!doctype html><meta charset="utf-8"><title>Music recovery acceptance</title><button>Restore current music</button><script type="module" src="/harness.js"></script>';mime='text/html'
  else:
   files={'/harness.js':OUT/'harness.js','/host-music-view.json':ROOT/'.local/basketball/network-scene/host-music-view.json'};path=files.get(route)
   if re.fullmatch(r'/audio/music-(?:066-loop|075)\.mp3',route):path=ROOT/'.local/basketball/audio/export'/route.rsplit('/',1)[1]
   if path is None or not path.is_file():self.send_error(404);return
   body=path.read_bytes();mime='audio/mpeg' if path.suffix=='.mp3' else 'application/json' if path.suffix=='.json' else 'text/javascript'
  start=0;end=len(body)-1;match=re.fullmatch(r'bytes=(\d+)-(\d*)',self.headers.get('Range',''))
  if match:start=int(match[1]);end=min(end,int(match[2]) if match[2] else end)
  if start>end:self.send_error(416);return
  self.send_response(206 if match else 200);self.send_header('Content-Type',mime);self.send_header('Accept-Ranges','bytes')
  if match:self.send_header('Content-Range',f'bytes {start}-{end}/{len(body)}')
  self.send_header('Content-Length',str(end-start+1));self.end_headers()
  try:self.wfile.write(body[start:end+1])
  except (BrokenPipeError,ConnectionResetError):pass
subprocess.run(['go','test','./games/basketball/server','-run','^TestHostMusicPersistsWithoutReplayingOldEffects$','-count=1'],cwd=ROOT,env=env,check=True)
subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/audio_recovery_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'harness.js')],cwd=ROOT,env=env,check=True)
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
try:
 browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/');opened=True;browser('wait','--fn','!!window.audioRecovery')
 browser('click','button');browser('wait','--fn','audioRecovery.started')
 js('window.musicMedia=document.querySelector("[data-platform-music]")')
 first=js('({ok:audioRecovery.result,time:musicMedia.currentTime,duration:musicMedia.duration,paused:musicMedia.paused,expected:audioRecovery.data.music.positionMS/1000,src:musicMedia.currentSrc})')
 check('late join starts at the authoritative host position',first['ok'] and not first['paused'] and first['expected']<=first['time']<first['expected']+.5)
 js('recoveryCommand([253])');check('original pause freezes native media',js('musicMedia.paused'))
 js('recoveryCommand([66])');browser('wait','--fn','!musicMedia.paused && musicMedia.currentTime>0')
 check('new original music restarts and clears pause',js('musicMedia.currentTime<1'))
 wrapped=js('(async()=>{const ok=await restoreRecovery({...audioRecovery.data.music,positionMS:(musicMedia.duration+2)*1000,epoch:8});return{ok,time:musicMedia.currentTime}})()')
 check('recovery wraps by the decoded MP3 duration',wrapped['ok'] and 2<=wrapped['time']<2.5)
 js('restoreRecovery({...audioRecovery.data.music,positionMS:12500,paused:true,epoch:9})')
 browser('wait','--fn','musicMedia.readyState>=1')
 paused=js('({time:musicMedia.currentTime,paused:musicMedia.paused,src:musicMedia.currentSrc})')
 check('paused snapshot loads the correct position without starting playback',paused['paused'] and abs(paused['time']-12.5)<.001)
 js('recoveryCommand([255])');browser('wait','--fn','!musicMedia.paused && musicMedia.currentTime>12.5')
 check('resume keeps the restored source and position',js('musicMedia.currentSrc')==paused['src'] and js('musicMedia.currentTime')<13)
 finite=js('(async()=>{const ok=await restoreRecovery({...audioRecovery.data.music,trackID:75,assetSHA256:audioRecovery.delivery.music["75"].distribution_sha256,positionMS:9999999,epoch:10});return{ok,src:musicMedia.getAttribute("src"),paused:musicMedia.paused}})()')
 check('already-ended finite music stays silent',not finite['ok'] and not finite['src'] and finite['paused'])
 js('audioRecovery.audio.setEnabled(false)')
 muted=js('restoreRecovery({...audioRecovery.data.music,positionMS:8000,epoch:11})')
 check('recovery never overrides platform mute',not muted and not js('musicMedia.getAttribute("src")'))
 check('recovery fetches no old effects or WAV masters',not any('/sound-' in p or p.endswith('.wav') for p in requests))
 js('audioRecovery.adapter.dispose();audioRecovery.audio.dispose()')
 record=dict(status='passed',checks=checks,first=first,wrapped=wrapped,paused=paused,finite=finite,requests=requests,scope='real native MP3 recovery from Go host checkpoint, explicit pause/wrap/end boundary cases; not Safari or full room reconnect')
 (OUT/'result.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n');print(json.dumps(record,ensure_ascii=False))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,error=repr(error),requests=requests),ensure_ascii=False,indent=2)+'\n');raise
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=5)
