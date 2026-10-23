"""Actual browser Go/Wasm spectator frames and prediction/correction acceptance."""
from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from urllib.parse import unquote
import json,os,subprocess,threading,uuid
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/basketball/network-scene';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkn-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=os.environ.get('AGENT_BROWSER_SOCKET_DIR',str(ROOT/'.local/bkn-sockets')),TMPDIR=str(ROOT/'.local/tmp'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 value=json.loads(r.stdout)
 if not value['success']:raise RuntimeError(value)
 return value['data']
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  route=unquote(self.path.split('?')[0]);path=None
  if route=='/':
   body=b'<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Basketball network scene acceptance</title><style>body{margin:16px;background:#172126;color:#f4eedb;font-family:sans-serif}canvas{display:block;width:min(100%,640px);image-rendering:pixelated;aspect-ratio:256/240}output{display:block;padding:12px 0}</style><canvas width="256" height="240"></canvas><output>Running</output><script type="module" src="/scene.js"></script>';mime='text/html'
  else:
   files={'/scene.js':OUT/'scene.js','/host-views.json':OUT/'host-views.json','/campaign-preparation-views.json':ROOT/'.local/campaign-host-preparation/host-views.json','/host-result-views.json':OUT/'host-result-views.json','/runtime/wasm_exec.js':ROOT/'web/dist/basketball/v1/runtime/wasm_exec.js','/runtime/build.json':ROOT/'web/dist/basketball/v1/runtime/build.json'}
   path=files.get(route)
   if route.startswith('/assets/'):
    rel=Path(route[8:])
    if rel.parts and '..' not in rel.parts and rel.parts[0] in ('court','courts','static-atlas','static-objects','raster','results') and rel.suffix in ('.png','.json'):
     allowed=(MEDIA/rel.parts[0]).resolve();candidate=(MEDIA/rel).resolve()
     if candidate.is_relative_to(allowed):path=candidate
   if path is None or not path.is_file():self.send_error(404);return
   body=path.read_bytes();mime={'.wasm':'application/wasm','.png':'image/png','.json':'application/json'}.get(path.suffix,'text/javascript')
  self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)

def main():
 global MEDIA
 media_root=os.environ.get('BASKETBALL_TEST_MEDIA_ROOT',str(ROOT))
 media_public=subprocess.check_output(['node','--input-type=module','-e','import {prepareBasketballAssets} from "./games/basketball/build/assets.mjs";console.log(prepareBasketballAssets('+json.dumps(media_root)+'))'],cwd=ROOT,env=env,text=True).strip()
 MEDIA=Path(media_public)/'basketball/v1/graphics'
 (OUT/'result.json').write_text('{"status":"running"}\n')
 subprocess.run(['go','test','./games/basketball/server','-run','^TestHostVideoHistoryReplaysExactInputAndHandoff$|^TestHostConfirmedResultAndTeamMapping$|^TestHostCampaignPreparationCommitRestore$','-count=1'],cwd=ROOT,env=env,check=True)
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/network_scene_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'scene.js')],cwd=ROOT,env=env,check=True)
 server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False
 try:
  browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/');opened=True
  browser('wait','--fn','!!window.sceneResult')
  result=browser('eval','window.sceneResult')['result'];(OUT/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n')
  browser('set','viewport','1100','820');browser('screenshot',str(OUT/'desktop.png'))
  browser('set','viewport','320','720');browser('screenshot',str(OUT/'mobile-320.png'))
  assert result['status']=='passed',result
  print(json.dumps(result,ensure_ascii=False))
 finally:
  if opened:browser('close')
  server.shutdown();server.server_close();thread.join(timeout=5)
if __name__=='__main__':main()
