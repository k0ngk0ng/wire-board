from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import json,os,subprocess,threading
ROOT=Path(__file__).resolve().parents[2];OUT=ROOT/'games/yuyuhakusho/verification/audio-platform-stream/startup-probe';TMP=ROOT/'.local/md-yuyuhakusho/audio-stream-startup'
for p in [OUT,TMP,TMP/'tmp',ROOT/'.local/yt']:p.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,TMPDIR=str(TMP/'tmp'),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/yt'))
seed=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','stream-startup'],env=env,cwd=ROOT,text=True).strip();env['AGENT_BROWSER_SESSION']='su'+seed.rsplit('-',1)[-1][:6]
base=ROOT/'.local/md-yuyuhakusho/audio-encoder-plan/e2e/native/original12';manifests={name:json.loads((base/name/'manifest.json').read_text())for name in ['small-bulk','default-bulk']}
subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),'scripts/yuyuhakusho/audio_stream_startup_probe.ts','--bundle','--format=esm','--outfile='+str(TMP/'probe.js')],env=env,cwd=ROOT,check=True)
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  path=self.path
  if path=='/':raw=b'<!doctype html><title>Stream startup probe</title><button id="start">Start native threshold probe</button><script type="module" src="/probe.js"></script>';mime='text/html'
  elif path=='/probe.js':raw=(TMP/'probe.js').read_bytes();mime='text/javascript'
  elif path.startswith('/manifest/'):raw=json.dumps(manifests[path.rsplit('/',1)[-1]]).encode();mime='application/json'
  else:
   parts=path.strip('/').split('/');name=parts[0]
   if len(parts)==2 and parts[1]=='init':file=base/name/manifests[name]['init_file']
   elif len(parts)==3 and parts[1]=='fragment':file=base/name/manifests[name]['fragments'][int(parts[2])]['file']
   else:self.send_error(404);return
   raw=file.read_bytes();mime='audio/mp4'
  self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
 def do_POST(self):
  raw=self.rfile.read(int(self.headers['Content-Length']));(OUT/'progress.json').write_bytes(raw);r=json.loads(raw);last=r['results'][-1];print(json.dumps({'case':last.get('case'),'append':last.get('append'),'frames':last.get('frames'),'native':last.get('native'),'playPending':last.get('playPending'),'stage':last.get('stage'),'started':last.get('started')}),flush=True);self.send_response(204);self.end_headers()
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();url=f'http://127.0.0.1:{server.server_port}/'
def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],env=env,cwd=ROOT,text=True,capture_output=True,timeout=45)
 if p.returncode:raise RuntimeError(p.stdout+p.stderr)
 r=json.loads(p.stdout);assert r['success'],r;return r['data']
opened=False
try:
 browser('--profile',str(TMP/'profile'),'open',url);opened=True;endpoint=browser('get','cdp-url');endpoint=endpoint.get('url')or endpoint.get('cdpUrl')or endpoint.get('value')
 a=subprocess.run(['node',str(ROOT/'scripts/yuyuhakusho/activate_audio_media_page.mjs'),endpoint,url],env=env,cwd=ROOT,text=True,capture_output=True,check=True);target=json.loads(a.stdout);browser('tab',target['targetId']);browser('wait','--fn','!document.hidden && !!streamStartupProbe.ready');browser('click','#start');browser('wait','--fn','streamStartupProbe.done')
 result=browser('eval','({results:streamStartupProbe.results,events:streamStartupProbe.events,error:streamStartupProbe.error})')['result'];(OUT/'result.json').write_text(json.dumps(result,indent=2)+'\n');assert not result['error'],result['error']
finally:
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=2)
