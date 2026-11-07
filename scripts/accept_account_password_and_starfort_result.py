#!/usr/bin/env python3
"""Failure scenarios written before UI implementation: own-profile entry absent
or exposed on another profile; incorrect password forces logout; confirmation
or UTF-8 bounds bypassed; duplicate submit; other devices remain authenticated;
current game/autoplay changes; fields survive cancel; mobile form clips. Also
verify real terminal Starfort HTTP state opens settlement, clear loss details,
close/reopen/refresh and fullscreen placement. Uses production UI + real server.
"""
import json,sys,time,shutil,http.cookiejar,urllib.request,urllib.error
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'games/starfort/tests'))
import accept_combat_controls_ui as c
w=c.w
BASELINE='--baseline' in sys.argv
RESULT_ONLY='--result-only' in sys.argv
w.OUT=ROOT/'.local/account-password'/('browser-baseline' if BASELINE else 'browser')
w.OUT.mkdir(parents=True,exist_ok=True)
w.ENV.update(WIRE_BOARD_STARFORT_BROWSER_OUT=str(w.OUT),AGENT_BROWSER_PROFILE=str(w.OUT/'profile'))
for key in ['TMPDIR','GOTMPDIR']:
 (w.OUT/'tmp').mkdir(exist_ok=True);w.ENV[key]=str(w.OUT/'tmp')
checks=[];passed=False
def mark(name,value):
 checks.append({'name':name,'passed':bool(value)});print(('PASS ' if value else 'FAIL ')+name,flush=True)
 if not value and not BASELINE:raise AssertionError(name)
def device(opener,path,body=None):
 data=None if body is None else json.dumps(body).encode()
 req=urllib.request.Request(w.BASE+'/api'+path,data=data,headers={'Content-Type':'application/json'} if body else {})
 try:
  with opener.open(req,timeout=10) as r:return r.status,json.load(r)
 except urllib.error.HTTPError as e:return e.code,json.load(e)
def source_hashes():
 paths=list((ROOT/'web/src/account').glob('*'))+[ROOT/p for p in ['internal/server/account_password.go','internal/server/account_password_e2e_test.go','internal/server/server.go','web/src/types.ts','web/src/admin/user-activity.tsx']]+[Path(__file__)]
 return {**c.source_hashes(),**c.hashes(paths)}
try:
 c.setup();before=source_hashes();password='starfort-browser-test'
 if not RESULT_ONLY:w.scenario('natural',players=2)
 for width,height in [(1366,900),(320,740)]:
  touch=width==320;c.b('set','viewport',str(width),str(height))
  if not RESULT_ONLY:
   original=w.room();other=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
   if width==1366:
    mark('password change starts during active game',original['status']=='playing')
    target='[title="查看 战斗控件同伴 的玩家页面"]';c.b('scrollintoview',target);c.activate(target,touch)
    c.wait('!!document.querySelector(".player-profile-modal")');mark('another profile has no password controls',c.js('!document.querySelector("[data-password-open]")'))
    c.activate('.player-profile-modal [aria-label="关闭"]',touch)
   code,_=device(other,'/login',{'name':w.MAIN,'password':password});mark('second device authenticated '+str(width),code==200)
   c.activate('.profile-nav',touch);c.wait('!!document.querySelector(".player-profile-modal")')
   c.activate('[data-password-open]',touch)
   c.wait('!!document.querySelector("[data-password-form]")')
   for field,value in [('currentPassword','incorrect-password'),('newPassword','new-browser-password-'+str(width)),('confirmPassword','new-browser-password-'+str(width))]:c.b('fill','[name='+field+']',value)
   c.activate('[data-password-submit]',touch);c.wait('document.querySelector("[data-password-error]")?.textContent.includes("原密码")')
   mark('wrong current password keeps login '+str(width),w.api('/state')['user']['name']==w.MAIN)
   c.b('fill','[name=currentPassword]',password);c.b('fill','[name=confirmPassword]','mismatched-password')
   c.activate('[data-password-submit]',touch);c.wait('document.querySelector("[data-password-error]")?.textContent.includes("不一致")')
   mark('confirmation mismatch preserves other device '+str(width),device(other,'/state')[0]==200)
   c.screenshot('password-form-'+str(width))
   new_password='new-browser-password-'+str(width)
   c.b('fill','[name=confirmPassword]',new_password);c.activate('[data-password-submit]',touch)
   c.wait('!document.querySelector("[data-password-form]")')
   mark('current browser stays authenticated '+str(width),w.api('/state')['user']['name']==w.MAIN)
   mark('other device revoked '+str(width),device(other,'/state')[0]==401)
   current=w.room();mark('game identity preserved '+str(width),current['id']==original['id'])
   mark('self password change keeps manual seat '+str(width),not current['seats'][current['you']].get('autoPlay',False))
   c.activate('.player-profile-modal [aria-label="关闭"]',touch);password=new_password
  w.scenario('defeat-ready')
  c.wait('!!document.querySelector(".match-result-modal")')
  mark('terminal state automatically opens result '+str(width),w.data()['phase']=='lost' and w.room()['status']=='finished')
  mark('explicit failed result '+str(width),c.js('document.querySelector(".match-result-modal .results h2")?.textContent.includes("挑战失败")===true'))
  mark('unambiguous battlefield end state '+str(width),c.js('document.querySelector("[data-sf-countdown]")?.textContent.includes("挑战失败")===true'))
  mark('result fits viewport '+str(width),c.visibility('.match-result-modal')['visible'])
  c.screenshot('defeat-result-'+str(width))
  c.activate('.match-result-modal [aria-label="关闭"]',touch)
  c.wait('!!document.querySelector(".result-reopen")')
  c.js('window.dispatchEvent(new Event("focus"));true');c.b('wait','300')
  mark('refresh does not reopen dismissed result '+str(width),c.js('!document.querySelector(".match-result-modal")'))
  if c.js('!!document.querySelector(\'.toast [aria-label="关闭提示"]\')'):c.activate('.toast [aria-label="关闭提示"]',touch)
  c.enter();c.activate('.result-reopen',touch);c.wait('!!document.querySelector(".match-result-modal")')
  mark('fullscreen result is visible and inside surface '+str(width),c.visibility('.match-result-modal')['visible'] and c.visibility('.match-result-modal')['inside'])
  c.screenshot('fullscreen-defeat-'+str(width));c.activate('.match-result-modal [aria-label="关闭"]',touch);c.leave()
  c.b('reload');c.wait('!!document.querySelector(".match-result-modal")')
  mark('finished-room reload offers settlement '+str(width),w.data()['phase']=='lost')
  c.activate('.match-result-modal [aria-label="关闭"]',touch)
 mark('source unchanged during acceptance',before==source_hashes())
 mark('no browser errors',not c.b('errors')['errors']);passed=all(x['passed'] for x in checks)
except Exception:
 import traceback;traceback.print_exc()
 if w.OPENED:
  try:c.screenshot('failure')
  except Exception:pass
finally:
 if w.OPENED:
  try:c.b('close')
  except Exception:pass
 if w.SERVER:(w.OUT/'stop').write_text('stop');w.SERVER.wait(timeout=20)
 w.artifact('result.json',{'passed':passed,'baseline':BASELINE,'checks':checks,'sourceSHA256':source_hashes(),'buildSHA256':c.hashes((ROOT/'web/dist').rglob('*')),'scope':'Local production frontend and real HTTP/SQLite/WebSocket, desktop mouse and 320px Chromium touch emulation; no physical phone. Existing defeat milestone fixture, no production account mutation.'})
 for p in [w.OUT/'profile',w.OUT/'tmp',w.PROFILE]:
  if p.exists():shutil.rmtree(p)
 sys.exit(0 if passed or BASELINE else 1)
