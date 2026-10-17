"""Browser E2E written before the Canvas renderer implementation.
Runs only a local verification page and retains exact original-pixel results.
"""
import base64,json,os,subprocess,threading,uuid,sys
from pathlib import Path
from http.server import ThreadingHTTPServer,BaseHTTPRequestHandler
from urllib.parse import unquote
from prepare_renderer_fixture import ROOT,prepare
from prepare_result_renderer_fixture import prepare as prepare_results
from prepare_courts_renderer_fixture import prepare as prepare_courts
from prepare_fade_fixture import prepare as prepare_fades
out=prepare();prepare_results();prepare_results(palette_variant=True);prepare_results(court_six=True);prepare_courts();prepare_courts(holdout=True);prepare_fades();legacy_only='--legacy-only' in sys.argv;profile=out/'profile';sockets=ROOT/'.local/bks';sockets.mkdir(exist_ok=True)
env=dict(os.environ,AGENT_BROWSER_SESSION='bkr-'+uuid.uuid4().hex[:4],AGENT_BROWSER_SOCKET_DIR=str(sockets),TMPDIR=str(ROOT/'.local/tmp'))
requests=[]
class Handler(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  route=unquote(self.path.split('?')[0]);requests.append(route);path=None
  files={'/':ROOT/'scripts/basketball/renderer_test_page.html','/acceptance.mjs':ROOT/'scripts/basketball/renderer_acceptance.mjs','/renderer.js':out/'renderer.js','/cases.json':out/'cases.json','/result-cases.json':out/'result-cases.json','/result-palette-cases.json':out/'result-palette-cases.json','/court-cases.json':out/'court-cases.json','/irq-cases.json':out/'irq-cases.json','/fade-cases.json':out/'fade-cases.json'}
  if route in files:path=files[route]
  elif route=='/result-court-six-cases.json':path=out/'result-court-six-cases.json'
  elif route.startswith('/result-court-six-reference/'):
   name=route.rsplit('/',1)[-1]
   if name.endswith('.png') and name[:-4].isdigit():path=ROOT/'.local/nes-basket/runs/court-six-result-observed'/('frame-'+name)
  elif route.startswith('/assets/'):
   rel=Path(route.removeprefix('/assets/'))
   if rel.parts and '..' not in rel.parts and rel.parts[0] in ('court','static-atlas','static-objects','raster','results','courts'):path=ROOT/'.local/basketball'/rel
  elif route.startswith('/fade-reference/'):
   parts=route.split('/')
   if len(parts)==4 and parts[2] in ('ny-open','ny-match','court4-match','court4-palette10') and parts[3].endswith('.png') and parts[3][:-4].isdigit():path=ROOT/f'.local/nes-basket/runs/fade-v2-{parts[2]}-observed'/('frame-'+parts[3])
  elif route.startswith('/irq-reference/'):
   parts=route.split('/')
   if len(parts)==4 and parts[2] in ('3','4') and parts[3].endswith('.png') and parts[3][:-4].isdigit():path=ROOT/f'.local/nes-basket/runs/court-{parts[2]}-irq-holdout-observed'/('frame-'+parts[3])
  elif route.startswith('/court-reference/'):
   parts=route.split('/')
   if len(parts)==4 and parts[2].isdigit() and 0<=int(parts[2])<8 and parts[3].endswith('.png') and parts[3][:-4].isdigit():path=ROOT/f'.local/nes-basket/runs/court-{parts[2]}-observed'/('frame-'+parts[3])
  elif route.startswith('/result-palette-reference/'):
   name=route.rsplit('/',1)[-1]
   if name.endswith('.png') and name[:-4].isdigit():path=ROOT/'.local/nes-basket/runs/court-4-result-observed'/('frame-'+name)
  elif route.startswith('/result-reference/'):
   name=route.rsplit('/',1)[-1]
   if name.endswith('.png') and name[:-4].isdigit():path=ROOT/'.local/nes-basket/runs/result-art-observed'/('frame-'+name)
  elif route.startswith('/reference/'):
   parts=route.split('/')
   if len(parts)==4 and parts[2].replace('-','').isalpha() and parts[3].endswith('.png') and parts[3][:-4].isdigit():path=ROOT/'.local/nes-basket/runs'/('renderer-'+parts[2])/('frame-'+parts[3])
  if path is None or not path.is_file():self.send_error(404);return
  content=path.read_bytes();mime='image/png' if path.suffix=='.png' else 'application/json' if path.suffix=='.json' else 'text/html' if path.suffix=='.html' else 'text/javascript'
  self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(content)));self.end_headers();self.wfile.write(content)
def browser(*args):
 result=subprocess.run(['agent-browser',*args,'--json'],env=env,capture_output=True,text=True,timeout=90)
 if result.returncode:raise RuntimeError(result.stdout+result.stderr)
 data=json.loads(result.stdout)
 if not data.get('success'):raise RuntimeError(data)
 return data.get('data')
source=ROOT/'games/basketball/web/renderer.ts'
if source.exists():subprocess.run([str(ROOT/'web/node_modules/.bin/esbuild'),str(source),'--bundle','--format=esm','--target=es2022','--outfile='+str(out/'renderer.js')],env=env,check=True,capture_output=True)
server=ThreadingHTTPServer(('127.0.0.1',0),Handler);thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start();opened=False
try:
 browser('--profile',str(profile),'open','http://127.0.0.1:'+str(server.server_port)+('/?legacy-only=1' if legacy_only else '/'));opened=True
 result=browser('eval','(async()=>await window.rendererAcceptance)()')['result']
 (out/('legacy-result.json' if legacy_only else 'result.json')).write_text(json.dumps(result,indent=2)+'\n')
 if result.get('firstFailure'):
  data=browser('eval','window.rendererFailurePNG')['result'];(out/'pixel-failure.png').write_bytes(base64.b64decode(data.split(',')[1]))
  (out/'pixel-failure-state.json').write_text(json.dumps(browser('eval','window.rendererFailureSnapshot')['result'],indent=2)+'\n')
 if result.get('resultFrames'):
  data=browser('eval','window.resultScenePNG')['result'];(out/'result-canvas.png').write_bytes(base64.b64decode(data.split(',')[1]))
 if result.get('fadeFrames'):
  for name,data in browser('eval','window.fadeScenePNGs')['result'].items():(out/f'fade-{name}.png').write_bytes(base64.b64decode(data.split(',')[1]))
 if result.get('resultPaletteFrames'):
  data=browser('eval','window.resultPaletteScenePNG')['result'];(out/'result-palette-canvas.png').write_bytes(base64.b64decode(data.split(',')[1]))
 if result.get('resultCourtSixFrames'):
  data=browser('eval','window.resultCourtSixScenePNG')['result'];(out/'result-court-six-canvas.png').write_bytes(base64.b64decode(data.split(',')[1]))
 if result.get('courtFrames'):
  for court,data in browser('eval','window.courtScenePNGs')['result'].items():(out/f'court-{court}.png').write_bytes(base64.b64decode(data.split(',')[1]))
 browser('set','viewport','1280','1000')
 browser('screenshot',str(out/'desktop.png'))
 data=browser('eval','document.querySelector("canvas").toDataURL()')['result'];(out/'canvas.png').write_bytes(base64.b64decode(data.split(',')[1]))
 browser('set','viewport','320','720');browser('screenshot',str(out/'mobile-320.png'))
 responsive=browser('eval','(()=>{const c=document.querySelector("canvas"),r=c.getBoundingClientRect();return {overflow:document.documentElement.scrollWidth>innerWidth,left:r.left,right:r.right,width:r.width,height:r.height,intrinsic:[c.width,c.height],smoothing:getComputedStyle(c).imageRendering}})()')['result']
 (out/'responsive.json').write_text(json.dumps(responsive,indent=2)+'\n')
 assert not responsive['overflow'] and responsive['right']<=320 and responsive['intrinsic']==[256,240] and responsive['smoothing']=='pixelated',responsive
 assert result['status']=='passed',result.get('firstFailure',result)
 assert not any('.nes' in route or '.bin' in route for route in requests),'Runtime requested original binary data'
 print(json.dumps({k:v for k,v in result.items() if not k.endswith('Detail')}))
finally:
 (out/'requests.json').write_text(json.dumps(requests,indent=2)+'\n')
 if opened:browser('close')
 server.shutdown();server.server_close();thread.join(timeout=5)
