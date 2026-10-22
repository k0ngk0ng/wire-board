#!/usr/bin/env python3
"""Two actual browsers; a failed resource holds the server gate until retry succeeds."""
import json,os,subprocess,time,uuid,shutil
from pathlib import Path
root=Path(__file__).resolve().parents[2];run='peer'+uuid.uuid4().hex[:5];out=root/'.local/game-loading'/run;temp=out/'temporary';temp.mkdir(parents=True);base=os.environ.get('LOADING_BASE','http://127.0.0.1:18189')
report={'passed':False};opened=[];sockets=[]
def browser(who,*args):
 socket=root/'.s'/(run+who);socket.mkdir(parents=True,exist_ok=True)
 if socket not in sockets:sockets.append(socket)
 env=dict(os.environ,AGENT_BROWSER_SESSION=run+who,AGENT_BROWSER_SOCKET_DIR=str(socket),AGENT_BROWSER_PROFILE=str(temp/who),TMPDIR=str(temp),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
 p=subprocess.run(['agent-browser',*args,'--json'],env=env,cwd=root,capture_output=True,text=True,timeout=60);assert p.returncode==0,p.stdout+p.stderr
 j=json.loads(p.stdout);assert j['success'],j;return j['data']
def js(who,s):return browser(who,'eval',s)['result']
def api(who,p,b=None):
 opts='' if b is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(b)+')}'
 r=js(who,'(async()=>{const r=await fetch('+json.dumps('/api'+p)+opts+');return {status:r.status,body:await r.json()}})()');assert r['status'] in (200,201),r;return r['body']
def room(who):return api(who,'/state')['room']
def cmd(who,t,**extra):
 r=room(who);return api(who,'/rooms/'+r['id'],{'type':t,'version':r['version'],'nonce':uuid.uuid4().hex,**extra})
def wait(fn,timeout=30):
 end=time.monotonic()+timeout
 while time.monotonic()<end:
  r=fn()
  if r:return r
  time.sleep(.15)
 raise AssertionError('timed out')
def shot(who,name):
 assert js(who,'document.documentElement.scrollWidth<=innerWidth+2')
 browser(who,'screenshot',str(out/(name+'.png')),'--full')
try:
 for who in ['a','b']:
  browser(who,'open',base);opened.append(who)
  wait(lambda:js(who,'!!document.querySelector("input[type=password]")'))
  api(who,'/register',{'name':run+who,'password':'loading-test-123','invite':'test-invite'})
  browser(who,'reload');wait(lambda:js(who,'document.body.innerText.includes("创建房间")'))
 browser('a','find','role','button','click','--name','创建房间','--exact')
 wait(lambda:js('a','document.querySelectorAll(".game-choice").length>0 || !!document.querySelector("[aria-modal=true]")'))
 report['catalog']=js('a','document.querySelector("[aria-modal=true]").innerText')
 shot('a','catalog-desktop')
 browser('a','press','Escape')
 r=api('a','/rooms',{'kind':'bubbleparty','name':'双人慢加载','capacity':2})
 api('b','/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex})
 cmd('a','ready');cmd('b','ready')
 browser('b','network','route','**/bubbleparty/v4/characters/*','--abort')
 cmd('a','start')
 wait(lambda:room('a').get('loading',{}).get('ready',[False])[0])
 wait(lambda:js('b','document.body.innerText.includes("重试加载")'))
 first=room('a');time.sleep(3);second=room('a')
 assert first['loading']['token']==second['loading']['token']
 assert first['game']['data']['serverAt']==second['game']['data']['serverAt']
 assert second['loading']['ready']==[True,False]
 assert second['turnDeadline']==0
 report['waiting']={'first':first,'second':second}
 shot('a','waiting-desktop');shot('b','failure-desktop')
 for who in ['a','b']:
  browser(who,'set','viewport','320','740');shot(who,'mobile-'+who)
  assert js(who,'(()=>{const e=document.querySelector("progress");const r=e.getBoundingClientRect();return r.width>200&&r.left>=0&&r.right<=innerWidth})()')
 browser('b','network','unroute')
 browser('b','find','role','button','click','--name','重试加载')
 wait(lambda:not room('a').get('loading'))
 wait(lambda:not js('a','!!document.querySelector("[data-game-loading]")'))
 resumed=room('a');assert resumed['clockOffsetMs']>3000
 assert resumed['game']['data']['selectUntil']-resumed['game']['data']['serverAt']>15000
 report['resumed']=resumed
 browser('a','reload')
 wait(lambda:not js('a','!!document.querySelector("[data-game-loading]")') and js('a','!!document.querySelector(".bubble-board")'))
 assert room('b')['loading'] is None
 assert room('a')['matchId']==resumed['matchId']
 report['reconnected']=room('a')
 shot('a','reconnected-mobile')
 cmd('a','close');cmd('a','leave');cmd('b','leave')
 report['passed']=True
finally:
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
 for who in opened:
  try:browser(who,'close')
  except:pass
 shutil.rmtree(temp,ignore_errors=True)
 for socket in sockets:shutil.rmtree(socket,ignore_errors=True)
