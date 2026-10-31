#!/usr/bin/env python3
"""Real local admin UI: user statistics, full history and ordinary-user privacy.
Failure cases: no actionable name; wrong target; missing closed histories;
finished/closed records counted alike; narrow-screen overflow; refresh closes
the inspected profile; ordinary users receive privileged history.
"""
import argparse,http.cookiejar,json,os,shutil,subprocess,time,uuid,urllib.request
from pathlib import Path
from urllib.parse import urlsplit
root=Path(__file__).resolve().parents[1];p=argparse.ArgumentParser();p.add_argument('--url',default='http://127.0.0.1:18099');p.add_argument('--admin-name',default='DialogAdmin');p.add_argument('--admin-password',default='dialog-local-password');p.add_argument('--invite',default='page-accept-code');p.add_argument('--existing-name');a=p.parse_args();assert urlsplit(a.url).hostname in ('127.0.0.1','localhost')
run=uuid.uuid4().hex[:8];out=root/'.local'/('admin-user-history-'+run);out.mkdir(parents=True)
env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(root/'.local/ab3'),AGENT_BROWSER_SESSION='auh-'+run,AGENT_BROWSER_PROFILE=str(out/'profile'),AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad',TMPDIR=str(root/'.local/tmp'))
report={'passed':False,'widths':[]}
def b(*args):
 r=subprocess.run(['agent-browser',*args,'--json'],env=env,text=True,capture_output=True,timeout=35);assert not r.returncode,r.stdout+r.stderr
 v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(code):return b('eval',code)['result']
def client():return urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def api(c,path,body=None):
 with c.open(urllib.request.Request(a.url+'/api'+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'}),timeout=20) as r:return json.load(r)
try:
 owner,peer=client(),client();name=a.existing_name or '历史验收'+run;password='history-test-only'
 api(peer,'/register',dict(name='对手'+run,password=password,invite=a.invite))
 if a.existing_name:
  api(owner,'/login',dict(name=name,password=password));uid=api(owner,'/state')['user']['id']
  saved=api(owner,'/players/'+uid);assert saved['total']==2 and saved['stats']['splendor']['played']==1
  expected_wins=saved['stats']['splendor']['wins'];report['reusedNaturalHistory']=True
 else:
  api(owner,'/register',dict(name=name,password=password,invite=a.invite))
  uid=api(owner,'/state')['user']['id']
  def command(c,kind,**extra):
   r=api(c,'/state')['room'];return api(c,'/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
  api(owner,'/rooms',dict(kind='splendor',capacity=2,name='双人托管自然结算'))
  r=api(owner,'/state')['room'];api(peer,'/rooms/'+r['id'],dict(type='join',version=r['version'],nonce=uuid.uuid4().hex));command(owner,'ready');command(peer,'ready');command(owner,'start');command(owner,'autoplay',enabled=True);command(peer,'autoplay',enabled=True)
  end=time.monotonic()+240
  while True:
   finished=api(owner,'/state')['room']
   if finished['game']['finished']:break
   assert time.monotonic()<end,'Natural delegated match did not finish'
   time.sleep(1)
  expected_wins=int(finished['you'] in finished['game']['winners']);report['naturalMatch']={'round':finished['game']['round'],'won':bool(expected_wins)}
  command(owner,'leave');command(peer,'leave')
  api(owner,'/rooms',dict(kind='splendor',capacity=2,name='真实中止记录'));command(owner,'add_bot');command(owner,'ready');command(owner,'start');command(owner,'close');command(owner,'leave')
 public=api(peer,'/players/'+uid);assert public['total']==1 and public['stats']['splendor']=={'played':1,'wins':expected_wins},public
 report['ordinaryHistoryCount']=public['total']
 b('open',a.url)
 assert js('fetch("/api/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps({'name':a.admin_name,'password':a.admin_password})+')}).then(r=>r.ok)')
 b('reload');b('wait','.admin-nav-button');b('click','.admin-nav-button');b('wait','.admin-tabs');b('find','role','button','click','--name','用户与在线','--exact')
 for width in [1440,320]:
  b('set','viewport',str(width),'900');b('fill','.admin-search input',name);b('click','.admin-user-name .player-link');b('wait','.profile-history article')
  data=js('fetch("/api/players/'+uid+'").then(r=>r.json())');assert data['user']['id']==uid and data['total']==2 and data['stats']['splendor']=={'played':1,'wins':expected_wins},data
  assert js('document.querySelectorAll(".profile-history article").length')==2
  assert not js('document.documentElement.scrollWidth>innerWidth')
  b('screenshot',str(out/f'{width}.png'))
  js("[...document.querySelectorAll('.profile-stats article')].find(e=>e.textContent.includes('璀璨宝石')).dataset.acceptStats='yes'")
  b('scrollintoview','[data-accept-stats=yes]');b('screenshot',str(out/f'{width}-stats.png'))
  b('scrollintoview','.profile-history');b('screenshot',str(out/f'{width}-history.png'))
  assert js('(()=>{const e=document.querySelector(".profile-history article"),r=e.getBoundingClientRect();return r.bottom>0&&r.top<innerHeight})()'), 'History cannot be brought into view'

  time.sleep(5.2);assert js('!!document.querySelector(".profile-history")'),'Admin refresh closed the profile'
  report['widths'].append({'width':width,'history':data['total'],'stats':data['stats']['splendor']})
  b('click','.modal[aria-label=玩家页面] button[aria-label=关闭]');b('wait','--fn','!document.querySelector(".modal[aria-label=玩家页面]")')
 report['passed']=True;assert not b('errors')['errors']
finally:
 (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2));b('close');shutil.rmtree(out/'profile',ignore_errors=True);print(out/'result.json')
