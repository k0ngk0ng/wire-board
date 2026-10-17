"""Verify supplementary original effects through the real platform media port.

No Audio/AudioContext mock and no physical-device listening claim.
"""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import hashlib,json,os,re,subprocess,threading,uuid
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/audio-effects-browser';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkae-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/bkae-sockets'),TMPDIR=str(ROOT/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
requests=[];checks=[];observations=[];opened=False
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=40)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 value=json.loads(r.stdout)
 if not value['success']:raise RuntimeError(value)
 return value['data']
def js(code):return browser('eval',code)['result']
def check(label,value):
 assert value,label
 checks.append(label)
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  route=self.path.split('?')[0];requests.append(route)
  if route=='/':body=b'<!doctype html><meta charset="utf-8"><title>Original effect acceptance</title><button>Play original effect</button><script type="module" src="/harness.js"></script>';mime='text/html'
  else:
   file=OUT/'harness.js' if route=='/harness.js' else ROOT/'.local/basketball/audio/export'/route.rsplit('/',1)[1] if re.fullmatch(r'/audio/sound-(001|023|048)\.wav',route) else None
   if file is None or not file.is_file():self.send_error(404);return
   body=file.read_bytes();mime='audio/wav' if file.suffix=='.wav' else 'text/javascript'
  self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(body)));self.end_headers()
  try:self.wfile.write(body)
  except (BrokenPipeError,ConnectionResetError):pass
subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/audio_effects_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'harness.js')],cwd=ROOT,env=env,check=True)
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
try:
 browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/');opened=True;browser('wait','--fn','!!window.effects')
 js('window.media=document.querySelector("audio[data-platform-audio]");window.mediaEvents=[];["playing","ended","error","timeupdate"].forEach(name=>media.addEventListener(name,()=>mediaEvents.push({event:name,time:media.currentTime,src:media.currentSrc})))')
 for ident in [1,23,48]:
  js(f'effects.id={ident};effects.completed=false;mediaEvents.length=0');browser('click','button');browser('wait','--fn','effects.completed');browser('wait','--fn','media.ended')
  result=js('({report:effects.report,src:media.currentSrc,duration:media.duration,time:media.currentTime,events:mediaEvents})');observations.append(dict(id=ident,**result))
  check(f'ID {ident}: native WAV decodes and reaches ended',result['duration']>0 and result['time']>=result['duration'] and any(e['event']=='playing' for e in result['events']) and any(e['event']=='ended' for e in result['events']))
  check(f'ID {ident}: unavailable CDN uses exact local original file',result['src'].endswith(f'/audio/sound-{ident:03}.wav') and f'/unavailable/basketball/v1/audio/sound-{ident:03}.wav' in requests)
  check(f'ID {ident}: adapter reports no missing or failed effect',result['report']==dict(ignored=False,unsupported=[],failed=[]))
  count=len(requests)
  duplicate=js(f'effects.adapter.consume([{ident}],effects.sequence)')
  check(f'ID {ident}: duplicate event does not replay',duplicate['ignored'] and js('media.ended') and len(requests)==count)
 js('effects.audio.setEnabled(false)')
 count=len(requests);muted=js('effects.adapter.consume([48],++effects.sequence)')
 check('platform mute stops media and refuses a new effect',muted['failed']==[48] and js('media.paused && !media.getAttribute("src")') and len(requests)==count)
 unknown=js('effects.adapter.consume([254],++effects.sequence)')
 check('unknown request is explicit and does not fetch a guessed file',unknown['unsupported']==[254] and len(requests)==count)
 js('effects.adapter.dispose();effects.audio.dispose()')
 check('dispose removes the platform media element',js('document.querySelectorAll("audio").length')==0)
 assets={f'sound-{i:03}.wav':hashlib.sha256((ROOT/'.local/basketball/audio/export'/f'sound-{i:03}.wav').read_bytes()).hexdigest() for i in [1,23,48]}
 result=dict(status='passed',checks=checks,observations=observations,requests=requests,assets=assets,scope='native desktop Chromium platform media; synthetic original IDs; not a full gameplay timing or physical phone listening acceptance')
 (OUT/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
except Exception as error:
 (OUT/'result.json').write_text(json.dumps(dict(status='failed',checks=checks,observations=observations,requests=requests,error=repr(error)),indent=2)+'\n');raise
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=5)
