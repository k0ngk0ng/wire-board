#!/usr/bin/env python3
"""Browser contract for the shared menu guard; independent of game builds.
Failure inventory: docs/research/mobile-controls/CONTEXT_MENU_SCENARIOS.md.
This is Chromium DOM/native mouse coverage, not a physical iOS menu test.
"""
import argparse
import functools
import http.server
import json
import os
from pathlib import Path
import shutil
import subprocess
import threading
import traceback
import uuid

parser=argparse.ArgumentParser();parser.add_argument('--out',required=True);args=parser.parse_args()
root=Path(__file__).resolve().parents[2];out=(root/args.out).resolve()
assert out.is_relative_to(root) and not out.exists();out.mkdir(parents=True)
temp=out/'temporary';temp.mkdir();run='cm'+uuid.uuid4().hex[:5];sockets=root/'.s'/run;sockets.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SESSION=run,AGENT_BROWSER_SOCKET_DIR=str(sockets),AGENT_BROWSER_PROFILE=str(temp/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report={'passed':False,'physicalIOSVerified':False,'scope':'Real Chromium DOM + native mouse/keyboard; explicitly dispatched contextmenu contract; actual production source/CSS'}
server=thread=None;opened=False

def browser(*args):
 p=subprocess.run(['agent-browser',*args,'--json'],env=env,cwd=root,text=True,capture_output=True,timeout=45)
 assert p.returncode==0,p.stdout+p.stderr
 j=json.loads(p.stdout);assert j['success'],j
 return j['data']
def js(code):return browser('eval',code)['result']

try:
 subprocess.run([str(root/'web/node_modules/.bin/esbuild'),'web/src/control-context-menu.ts','--bundle','--format=esm','--outfile='+str(temp/'guard.js')],cwd=root,env=env,check=True,capture_output=True)
 shutil.copyfile(root/'web/src/style.css',temp/'style.css')
 (temp/'index.html').write_text('''<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><link rel="stylesheet" href="style.css"><style>main{padding:16px;max-width:600px}section{margin:12px 0}button,[role=button]{padding:12px;background:#e4ece0;color:#183c31}input,textarea{width:100%;margin:4px 0}svg{width:20px;height:20px}</style><main>
 <h1 data-probe="heading">长按控件验收</h1>
 <section><button data-probe="button"><span data-probe="button-label">保持按住</span><svg><path data-probe="button-svg" d="M2 2L18 18" stroke="currentColor"/></svg></button><input type="button" data-probe="input-button" value="原生按钮"></section>
 <section><div role="button" tabindex="0" data-probe="role"><span data-probe="role-label">自定义按键</span></div></section>
 <section data-game-controls data-probe="controls"><span data-probe="control-label">摇杆与蓄力区域</span><input data-probe="input" value="聊天文字可以复制"><textarea data-probe="textarea">可编辑文本</textarea><select data-probe="select"><option>下拉选项</option></select><div contenteditable="true" data-probe="editor"><span data-probe="editor-child">可编辑区域</span></div></section>
 <p data-probe="copy">普通文字仍可选择和复制。</p></main>
 <script type="module">import {installControlContextMenuGuard} from './guard.js';
 let remove=installControlContextMenuGuard(document);window.removeGuard=()=>remove();window.installGuard=()=>{remove();remove=installControlContextMenuGuard(document)};
 window.audit={held:false,clicks:0,up:0,cancel:0};const button=document.querySelector('button');button.addEventListener('pointerdown',()=>window.audit.held=true);button.addEventListener('pointerup',()=>{window.audit.held=false;window.audit.up++});button.addEventListener('pointercancel',()=>{window.audit.held=false;window.audit.cancel++});button.addEventListener('click',()=>window.audit.clicks++);
 </script>''')
 class Quiet(http.server.SimpleHTTPRequestHandler):
  def log_message(self,*args):pass
 server=http.server.ThreadingHTTPServer(('127.0.0.1',0),functools.partial(Quiet,directory=str(temp)))
 thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
 browser('open','http://127.0.0.1:'+str(server.server_port));opened=True;browser('set','viewport','320','844');browser('wait','button')
 code='''(()=>{return [...document.querySelectorAll('[data-probe]')].map(e=>{const event=new MouseEvent('contextmenu',{bubbles:true,cancelable:true,composed:true});e.dispatchEvent(event);return {name:e.dataset.probe,prevented:event.defaultPrevented,userSelect:getComputedStyle(e).userSelect}})})()'''
 protected={'button','button-label','button-svg','input-button','role','role-label','controls','control-label'}
 rows=js(code)
 for row in rows:
  assert row['prevented']==(row['name'] in protected),row
  assert (row['userSelect']=='none')==(row['name'] in protected),row
 report['boundaries']=rows
 js('window.removeGuard()');assert all(not r['prevented'] for r in js(code))
 js('window.installGuard();window.installGuard()');assert sum(r['prevented'] for r in js(code))==len(protected)
 report['lifecycle']='cleanup removes guard; remount restores exactly scoped behavior'
 button=js("(()=>{const r=document.querySelector('button').getBoundingClientRect();return{x:r.x+r.width/2,y:r.y+r.height/2}})()")
 browser('mouse','move',str(round(button['x'])),str(round(button['y'])));browser('mouse','down')
 assert js('window.audit.held')
 menu=js("(()=>{const e=new MouseEvent('contextmenu',{bubbles:true,cancelable:true});document.querySelector('button span').dispatchEvent(e);return {prevented:e.defaultPrevented,held:window.audit.held}})()")
 assert menu['prevented'] and menu['held'],menu
 browser('mouse','up');assert not js('window.audit.held') and js('window.audit.up')==1
 clicks=js('window.audit.clicks');browser('focus','button');browser('press','Space');assert js('window.audit.clicks')==clicks+1
 report['nativeMouseAndKeyboard']={'heldDuringMenu':menu,'afterReleaseAndSpace':js('window.audit')}
 browser('fill','textarea','聊天文字可以复制');browser('focus','textarea');browser('press','Shift+ArrowLeft')
 selection=js("(()=>{const e=document.querySelector('textarea');return {start:e.selectionStart,end:e.selectionEnd,length:e.value.length}})()")
 assert selection['start']<selection['end']==selection['length'],selection
 report['editorSelection']=selection
 browser('screenshot',str(out/'contract-320.png'),'--full')
 report['passed']=True
except BaseException:
 report['error']=traceback.format_exc();print(report['error'])
finally:
 if opened:
  try:browser('close')
  except BaseException:report['passed']=False
 if server:server.shutdown();server.server_close()
 if thread:thread.join(timeout=5)
 for path in [temp,sockets]:
  if path.exists():shutil.rmtree(path)
 (out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 print(out/'report.json')
 if not report['passed']:raise SystemExit(1)
