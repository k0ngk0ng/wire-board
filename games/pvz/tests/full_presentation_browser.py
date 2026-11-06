#!/usr/bin/env python3
"""Rebuild actual Go Wasm + production Predictor; drive its browser boundary."""
import functools
import hashlib
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/pvz/full-presentation'
OUT.mkdir(parents=True,exist_ok=True)
ENV=dict(os.environ)
for key,path in {'GOCACHE':'.local/go-build','GOMODCACHE':'.local/go-mod','GOTMPDIR':'.local/tmp','TMPDIR':'.local/tmp'}.items():
    ENV[key]=str(ROOT/path);Path(ENV[key]).mkdir(parents=True,exist_ok=True)
ENV.update(GOWORK='off',GOTOOLCHAIN='local',GOENV='off',PVZ_PRESENTATION_OUT=str(OUT))
def command(args,**kwargs):return subprocess.run(args,cwd=ROOT,env=ENV,text=True,capture_output=True,check=True,**kwargs).stdout
command(['go','test','./games/pvz/tests','-run','^TestPvZFullPresentationFixtures$','-count=1'])
runtime=Path(command(['node','--input-type=module','-e','import {preparePvZRuntime} from "./games/pvz/build/runtime.mjs"; console.log(preparePvZRuntime(process.cwd()))']).strip())
command(['node','--input-type=module','-e','import {build} from "./web/node_modules/esbuild/lib/main.js";await build({entryPoints:["games/pvz/tests/full_presentation_browser.mjs"],bundle:true,format:"esm",outfile:".local/pvz/full-presentation/browser.js",define:{"import.meta.env.BASE_URL":JSON.stringify("/")}})'])
(OUT/'index.html').write_text('<!doctype html><html><meta charset="utf-8"><title>PvZ presentation acceptance</title><body><button>Plant two cards</button><canvas width="900" height="600"></canvas><output></output><script type="module" src="/browser.js"></script></body></html>')
class Handler(http.server.SimpleHTTPRequestHandler):
    def translate_path(self,path):
        if path.startswith('/pvz/'):return str(runtime/path.lstrip('/'))
        return super().translate_path(path)
    def log_message(self,*args):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(OUT)))
thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
ENV.update(AGENT_BROWSER_SESSION='p'+uuid.uuid4().hex[:5],AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/ab'),AGENT_BROWSER_PROFILE=str(OUT/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True,exist_ok=True)
def browser(*args):
    r=json.loads(command(['agent-browser',*map(str,args),'--json'],timeout=60));assert r['success'],r;return r['data']
report={'passed':False}
try:
    browser('open','http://127.0.0.1:'+str(server.server_port))
    browser('wait','--fn','window.presentationReport?.ready===true')
    browser('click','button')
    browser('wait','--fn','window.presentationReport?.done===true')
    report=browser('eval','window.presentationReport')['result']
    label=os.environ.get('PVZ_PRESENTATION_LABEL','after')
    browser('screenshot',str(OUT/(label+'.png')))
    report['command']='python3 -B games/pvz/tests/full_presentation_browser.py'
    report['runtime']=json.loads((runtime/'pvz/v1/runtime/build.json').read_text())
    report['sources']={str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest() for p in [ROOT/'games/pvz/web/runtime.ts',ROOT/'games/pvz/runtimebridge/bridge.go',*sorted((ROOT/'games/pvz/engine').glob('*.go'))]}
    (OUT/(label+'.json')).write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'passed':report['passed'],'cases':report['cases']},ensure_ascii=False,indent=2))
finally:
    browser('close');server.shutdown();server.server_close();shutil.rmtree(OUT/'profile',ignore_errors=True)
assert report['passed'],'presentation acceptance failed; see retained report'
