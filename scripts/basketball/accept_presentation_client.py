"""Actual browser Go/Wasm earned reward, projection, replay and resume acceptance.
Run source scripts/env.sh before this script. Artifacts stay in this worktree.
"""
from pathlib import Path
from http.server import SimpleHTTPRequestHandler,ThreadingHTTPServer
import json,os,subprocess,threading,uuid
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/presentation-client';OUT.mkdir(parents=True,exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bpc-'+uuid.uuid4().hex[:6],AGENT_BROWSER_SOCKET_DIR=str((ROOT/'.local/basketball').resolve()/'bkn-sockets'))
Path(env['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True,exist_ok=True)
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=60)
 if r.returncode:raise RuntimeError(r.stdout+r.stderr)
 data=json.loads(r.stdout)
 if not data.get('success'):raise RuntimeError(data)
 return data['data']
def main():
 subprocess.run(['go','test','./games/basketball/runtimebridge','-run','^TestBridgeEarnedPresentationLifecycle$','-count=1','-v'],cwd=ROOT,env=env,check=True)
 subprocess.run(['go','test','./games/basketball/engine','-run','^TestRuntimePresentationDisplayLifecycle$','-count=1'],cwd=ROOT,env=dict(env,BASKETBALL_PRESENTATION_ARTIFACT_DIR=str(OUT)),check=True)
 runtime=OUT/'runtime';runtime.mkdir(exist_ok=True)
 subprocess.run(['go','build','-o',str(runtime/'basketball.wasm'),'./games/basketball/cmd/browser-wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
 goroot=Path(subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip());(runtime/'wasm_exec.js').write_bytes((goroot/'lib/wasm/wasm_exec.js').read_bytes())
 subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(ROOT/'scripts/basketball/presentation_client_harness.ts'),'--bundle','--format=esm','--target=es2022','--outfile='+str(OUT/'client.js')],cwd=ROOT,env=env,check=True)
 (OUT/'index.html').write_text('<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><style>body{margin:12px;background:#172126;color:#f4eedb;font-family:sans-serif}canvas{display:block;width:min(100%,640px);image-rendering:pixelated;aspect-ratio:256/240}output{display:block;padding:12px 0}</style><canvas width="256" height="240"></canvas><output>Running</output><script type="module" src="client.js"></script>')
 pin=json.loads((ROOT/'games/basketball/assets/release-manifest.json').read_text());assets=ROOT/'.local/basketball-asset-build'/pin['contentSHA256']/'public/basketball/v1/graphics'
 class Handler(SimpleHTTPRequestHandler):
  def __init__(self,*a,**kw):super().__init__(*a,directory=str(OUT),**kw)
  def log_message(self,*a):pass
  def translate_path(self,path):
   if path.startswith('/assets/'):
    relative=Path(path[8:]);candidate=(assets/relative).resolve()
    if candidate.is_relative_to(assets.resolve()):return str(candidate)
   return super().translate_path(path)
 server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False
 try:
  browser('--profile',str(OUT/'profile'),'open',f'http://127.0.0.1:{server.server_port}/');opened=True
  browser('wait','--fn','!!window.result','--timeout','60000');result=browser('eval','window.result')['result'];(OUT/'result.json').write_text(json.dumps(result,indent=2)+'\n')
  browser('set','viewport','1100','820');browser('screenshot',str(OUT/'desktop.png'))
  browser('set','viewport','320','720');browser('screenshot',str(OUT/'mobile-320.png'))
  if result['status']=='passed':
   browser('eval','window.showEnding()');browser('screenshot',str(OUT/'ending-mobile-320.png'))
   browser('set','viewport','1100','820');browser('screenshot',str(OUT/'ending-desktop.png'))
  print(json.dumps(result));assert result['status']=='passed',result
 finally:
  if opened:browser('close')
  server.shutdown();server.server_close();thread.join(timeout=5)
if __name__=='__main__':main()
