#!/usr/bin/env python3
"""Real React component/HTTP/browser acceptance with controlled release metadata.
The server fixture changes only version metadata and entry asset names; the
production monitor, notice, timers, fetch and native clicks run unchanged.
"""
import argparse, functools, http.server, json, os, shutil, subprocess, threading, time, uuid
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--out',required=True);args=p.parse_args()
root=Path(__file__).resolve().parents[1];out=(root/args.out).resolve();assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
temp=out/'temporary';temp.mkdir();(temp/'assets').mkdir();sockets=root/'.s'/('ver'+uuid.uuid4().hex[:6]);sockets.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SESSION=sockets.name,AGENT_BROWSER_SOCKET_DIR=str(sockets),AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--disable-breakpad,--disable-crash-reporter',TMPDIR=str(temp))
report={'passed':False,'scope':__doc__,'checks':[]};server=None;opened=False
state={'version':'v1','remote':'v1','html':'v1','calls':0,'loads':0,'failure':False}
def browser(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],cwd=root,env=env,text=True,capture_output=True,timeout=45);assert r.returncode==0,r.stdout+r.stderr
 d=json.loads(r.stdout);assert d['success'],d;return d['data']
def js(code):return browser('eval',code)['result']
def wait(test,seconds=12):
 deadline=time.monotonic()+seconds
 while time.monotonic()<deadline:
  v=test()
  if v:return v
  time.sleep(.15)
 raise AssertionError('condition timed out')
def capture(name):browser('screenshot',str(out/(name+'.png')))
def assets(version):return ['/assets/app-'+version+'.js','/assets/style.css']
def check(name,**extra):report['checks'].append({'name':name,**extra});print('PASS',name,flush=True)
try:
 entry=temp/'entry.tsx'
 entry.write_text('import React from '+json.dumps(str(root/'web/node_modules/react/index.js'))+'; import {createRoot} from '+json.dumps(str(root/'web/node_modules/react-dom/client.js'))+'; import {VersionUpdateNotice} from '+json.dumps(str(root/'web/src/VersionUpdateNotice.tsx'))+';const lazy=document.createElement("link");lazy.rel="modulepreload";lazy.href="/assets/optional.js";document.head.append(lazy);createRoot(document.getElementById("notices")).render(<VersionUpdateNotice/>);')
 subprocess.run([str(root/'web/node_modules/.bin/esbuild'),str(entry),'--bundle','--format=esm','--jsx=automatic','--alias:react='+str(root/'web/node_modules/react'),'--alias:react-dom='+str(root/'web/node_modules/react-dom'),'--outfile='+str(temp/'assets/app.js')],cwd=root,env=env,check=True,capture_output=True)
 shutil.copyfile(root/'web/src/style.css',temp/'assets/style.css')
 class Handler(http.server.SimpleHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_GET(self):
   if self.path=='/api/version':
    state['calls']+=1;self.send_response(503 if state['failure'] else 200);self.send_header('Content-Type','application/json');self.send_header('Cache-Control','no-store');self.end_headers();self.wfile.write(json.dumps({'version':state['version'],'assets':assets(state['remote'])}).encode());return
   if self.path=='/':
    state['loads']+=1;self.send_response(200);self.send_header('Content-Type','text/html; charset=utf-8');self.send_header('Cache-Control','no-store');self.end_headers()
    self.wfile.write(('''<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="/assets/style.css"><style>main{margin:160px 12px 0}#board{height:260px;background:#214d40;color:#fff;padding:20px;border-radius:12px}input{max-width:100%;margin-top:20px}</style><div id="notices"></div><main><h1>版本更新验收</h1><div id="board">对局区域<input aria-label="聊天输入" placeholder="保留当前输入"></div></main><script type="module" src="/assets/app-'''+state['html']+'''.js"></script>''').encode());return
   if self.path.startswith('/assets/app-'):self.path='/assets/app.js'
   return super().do_GET()
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Handler,directory=str(temp)));threading.Thread(target=server.serve_forever,daemon=True).start()
 browser('open','http://127.0.0.1:'+str(server.server_port));opened=True;browser('set','viewport','320','844');browser('wait','#board');wait(lambda:state['calls']>0)
 assert not js('!!document.querySelector("[data-version-update]")');before=js('document.querySelector("#board").getBoundingClientRect().toJSON()');loads=state['loads'];calls=state['calls'];browser('fill','input','未发送的聊天')
 state['version']='v2'
 wait(lambda:js('document.querySelector("[data-version-update]")?.dataset.versionUpdate')=='v2',70)
 assert state['calls']>calls and state['loads']==loads
 assert js('document.querySelector("#board").getBoundingClientRect().toJSON()')==before
 assert js('document.querySelector("input").value')=='未发送的聊天'
 assert not js('document.documentElement.scrollWidth>innerWidth')
 capture('mobile-update');check('actual 60-second poll detects backend-only change without navigation or layout shift',requests=state['calls']-calls)
 calls=state['calls'];js('Object.defineProperty(document,"visibilityState",{value:"hidden",configurable:true});document.dispatchEvent(new Event("visibilitychange"))');time.sleep(.5);assert state['calls']==calls
 js('delete document.visibilityState;document.dispatchEvent(new Event("visibilitychange"))');wait(lambda:state['calls']>calls);check('hidden visibility signal makes no request; visible resume checks immediately')
 browser('click','.version-update-actions .outline');assert js('!!document.querySelector(".version-update-chip")')
 js('document.dispatchEvent(new Event("visibilitychange"))');time.sleep(.5);assert js('!!document.querySelector(".version-update-chip")')
 browser('click','.version-update-chip');state['failure']=True;js('document.dispatchEvent(new Event("visibilitychange"))');time.sleep(.5);assert js('document.querySelector("[data-version-update]").dataset.versionUpdate')=='v2';state['failure']=False
 state['version']='v3';state['remote']='v3';js('document.dispatchEvent(new Event("visibilitychange"))');wait(lambda:js('document.querySelector("[data-version-update]")?.dataset.versionUpdate')=='v3')
 browser('set','viewport','1280','900');capture('desktop-update');check('visibility check, defer/reopen, duplicate suppression and temporary failure preserve update')
 state['html']='v3';browser('click','.version-update-actions .primary');wait(lambda:state['loads']>loads);browser('wait','#board');time.sleep(.5);assert not js('!!document.querySelector("[data-version-update]")');check('only user refresh navigates; matching release after reload clears notice')
 state['html']='v1';browser('reload');wait(lambda:js('document.querySelector("[data-version-update]")?.dataset.versionUpdate')=='v3');check('cached old entry detects mismatch on its first successful metadata response')
 report['passed']=True
finally:
 if opened:
  try:browser('close')
  except Exception:pass
 if server:server.shutdown();server.server_close()
 shutil.rmtree(temp,ignore_errors=True);shutil.rmtree(sockets,ignore_errors=True)
 (out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
