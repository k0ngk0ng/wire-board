#!/usr/bin/env python3
"""Targeted native-browser verification of the production reset-password field."""
from pathlib import Path
import os,subprocess,json,threading,http.server,functools,shutil,hashlib
ROOT=Path(__file__).resolve().parents[1];OUT=ROOT/'.local/admin-password';PROFILE=OUT/'profile'
# Root task workspace owns this short socket path, including nested worktrees.
workspace=ROOT.parent.parent if ROOT.parent.name=='.local' else ROOT
SOCKET=workspace/'.local/apw'
for p in [OUT,PROFILE,SOCKET,OUT/'tmp']:p.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ,AGENT_BROWSER_SESSION='adminpw',AGENT_BROWSER_SOCKET_DIR=str(SOCKET),AGENT_BROWSER_PROFILE=str(PROFILE),TMPDIR=str(OUT/'tmp'))
subprocess.run(['node','--input-type=module','-e',"import {build} from './web/node_modules/esbuild/lib/main.js';await build({entryPoints:['web/tests/fixtures/admin-password.tsx'],bundle:true,outfile:'.local/admin-password/fixture.js',jsx:'automatic',nodePaths:['./web/node_modules']});"],cwd=ROOT,env=ENV,check=True)
(OUT/'index.html').write_text('<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="fixture.css"><body><div id="root"></div><script src="fixture.js"></script></body></html>')
class Quiet(http.server.SimpleHTTPRequestHandler):
 def log_message(self,*a):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Quiet,directory=str(OUT)));threading.Thread(target=server.serve_forever,daemon=True).start();url='http://127.0.0.1:'+str(server.server_port)
def b(*a):
 p=subprocess.run(['agent-browser',*map(str,a),'--json'],cwd=ROOT,env=ENV,capture_output=True,text=True,check=True,timeout=35);r=json.loads(p.stdout);assert r['success'],r;return r['data']
def js(s):return b('eval',s)['result']
report={'passed':False,'scope':'production password-field React component, real native clicks and form submission; no remote password reset','viewports':[]}
try:
 for w,h in [(1440,900),(320,760)]:
  b('open',url);b('set','viewport',w,h);b('wait','input[name=password]')
  assert js('window.submissions')==0
  b('click','.admin-password-row button');b('wait','--fn','document.querySelector("input[name=password]").value.length===20')
  one=js('document.querySelector("input[name=password]").value');assert len(one)==20 and js('window.submissions')==0
  b('click','.admin-password-row button');two=js('document.querySelector("input[name=password]").value');assert one!=two and js('window.submissions')==0
  geom=js('({overflow:document.documentElement.scrollWidth>innerWidth,input:document.querySelector("input[name=password]").getBoundingClientRect().toJSON(),generate:document.querySelector(".admin-password-row button").getBoundingClientRect().toJSON(),type:document.querySelector("input[name=password]").type})')
  assert not geom['overflow'] and geom['generate']['right']<=w and geom['input']['left']>=0 and geom['type']=='text',geom
  # A public dummy string, never a generated credential, appears in evidence.
  b('fill','input[name=password]','DemoPassword_4826');b('screenshot',OUT/('reset-'+str(w)+'.png'))
  b('click','button.primary');assert js('window.submissions')==1 and js('document.querySelector(".admin-password-row button").disabled')
  b('reload');b('wait','input[name=password]');assert js('document.querySelector("input[name=password]").value')==''
  report['viewports'].append({'width':w,'height':h,'generationDoesNotSubmit':True,'regenerationChangesValue':True,'confirmSubmitsOnce':True,'busyDisablesGeneration':True,'remountClearsValue':True,'geometry':geom})
 report['passed']=True
finally:
 try:b('close')
 except Exception:pass
 server.shutdown();server.server_close();shutil.rmtree(PROFILE,ignore_errors=True)
 for p in SOCKET.glob('adminpw.*'):p.unlink()
 if SOCKET.exists() and not any(SOCKET.iterdir()):SOCKET.rmdir()
 report['sourceSHA256']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [ROOT/'web/src/admin/password.ts',ROOT/'web/src/admin/password-field.tsx',ROOT/'web/src/admin.tsx',ROOT/'web/src/admin.css',ROOT/'web/tests/fixtures/admin-password.tsx',Path(__file__)]}
 (OUT/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':report['passed'],'viewports':len(report['viewports'])}))
