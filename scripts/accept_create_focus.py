#!/usr/bin/env python3
"""Local E2E: mobile creation must not focus an editor or summon a keyboard.
Failures: search/name autofocus on either step or returning to choices; desktop
autofocus lost; focus leaves the dialog; explicit tap cannot edit the name.
"""
import argparse, json, os, shutil, subprocess, time, uuid
from pathlib import Path
from urllib.parse import urlsplit
root=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('--url',default='http://127.0.0.1:18099');p.add_argument('--baseline',action='store_true');a=p.parse_args()
assert urlsplit(a.url).hostname in ('localhost','127.0.0.1')
run=uuid.uuid4().hex[:8];out=root/'.local'/('create-focus-'+run);out.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(root/'.local/ab3'),AGENT_BROWSER_SESSION='focus-'+run,AGENT_BROWSER_PROFILE=str(out/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad',TMPDIR=str(root/'.local/tmp'))
report={'passed':False,'checks':[]}
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,text=True,capture_output=True,timeout=30);assert not r.returncode,r.stdout+r.stderr
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def check(width,step):
 time.sleep(.15)
 state=js('({tag:document.activeElement.tagName,name:document.activeElement.getAttribute("name"),type:document.activeElement.getAttribute("type"),inside:!!document.activeElement.closest(".modal")})')
 report['checks'].append({'width':width,'step':step,**state});b('screenshot',str(out/f'{width}-{step}.png'))
 if not a.baseline:
  assert state['inside'],state
  if width<600:assert state['tag'] not in ['INPUT','TEXTAREA','SELECT'],state
  elif step=='setup':assert state['name']=='name',state
try:
 b('open',a.url)
 assert js('fetch("/api/register",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps({'name':'焦点验收'+run,'password':'focus-test-only','invite':'page-accept-code'})+')}).then(r=>r.ok)')
 b('reload');b('wait','.lobby-create')
 for width in [390,320,1440]:
  b('set','viewport',str(width),'844');b('click','.lobby-create');b('wait','.choose-games button');check(width,'choose')
  b('click','.choose-games button');b('wait','input[name=name]');check(width,'setup')
  b('click','input[name=name]');b('fill','input[name=name]','手动输入房间名');assert js('document.activeElement.name')=='name'
  b('click','.create-back');check(width,'back')
  b('click','.modal-head button[aria-label=关闭]')
 report['passed']=True
finally:
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));b('close');shutil.rmtree(out/'profile',ignore_errors=True);print(out/'result.json')
