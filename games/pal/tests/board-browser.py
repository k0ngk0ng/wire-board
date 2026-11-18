#!/usr/bin/env python3
"""Test-first real platform PAL Board: native browser input + HTTP + SQLite.

Failures: fake loading readiness/byte counts; one reader skips another; held
confirm crosses pages; remapping leaves the old key active; rapid native menu
edges disappear; a nonleader drives menus; original progress changes on save;
phone/fullscreen clips controls or remounts the canvas. No gameplay is injected.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/pal/board-browser';OUT.mkdir(parents=True,exist_ok=True)
BASE=(OUT/'url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pal'],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'scope':'production React shell and PAL Board with authenticated original players, actual original GameMain, native keyboard/touch, shared loading and saved continuation; not full campaign','cases':[]}

class Browser:
    def __init__(self,name):
        self.name=name;self.env=dict(ENV,AGENT_BROWSER_PROFILE=str(OUT/('profile-'+name)))
        self.mobile_process=None
        Path(self.env['AGENT_BROWSER_PROFILE']).mkdir(parents=True,exist_ok=True)
    def b(self,*args):
        r=subprocess.run(['agent-browser','--session',SESSION+'-'+self.name,*map(str,args),'--json'],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=45)
        assert r.returncode==0,r.stdout+r.stderr
        data=json.loads(r.stdout);assert data['success'],data;return data['data']
    def js(self,code):return self.b('eval',code)['result']
    def api(self,path,body=None):
        options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
        r=self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
        assert 200<=r['status']<300,r;return r['body']
    def room(self):return self.api('/state').get('room')
    def data(self):return self.room()['game']['data']
    def command(self,kind,**extra):
        r=self.room();return self.api('/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
    def tap(self,selector):
        self.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center",behavior:"instant"})')
        x,y=self.js('(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const b=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return [b.x+b.width/2,b.y+b.height/2]})()')
        r=subprocess.run(['node','scripts/browser_touch.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=25)
        assert r.returncode==0,r.stdout+r.stderr
    def mobile(self,width,height):
        if self.mobile_process:self.mobile_process.terminate();self.mobile_process.wait(timeout=5)
        self.b('set','viewport',width,height)
        self.mobile_process=subprocess.Popen(['node','scripts/browser_device.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,str(width),str(height),'--keep'],cwd=ROOT,env=self.env,text=True,stdout=subprocess.PIPE)
        assert self.mobile_process.stdout.readline().strip()=='ready'
    def desktop(self,width,height):
        if self.mobile_process:self.mobile_process.terminate();self.mobile_process.wait(timeout=5);self.mobile_process=None
        self.b('set','viewport',width,height)

def wait(check,label='PAL browser state',seconds=35):
    deadline=time.monotonic()+seconds
    while time.monotonic()<deadline:
        if check():return
        time.sleep(.12)
    raise AssertionError(label)

def read_complete(player,seat,prompt):
    dialogue=player.data().get('dialogue')
    return not dialogue or dialogue['id']!=prompt or dialogue['read'][seat]

def record(case):
    REPORT['cases'].append(case)
    print(json.dumps(case,ensure_ascii=False),flush=True)

def position(player):
    current=player.data()
    return [current['x'],current['y']]

def canvas_hash(player):
    return player.js('(async()=>{const c=document.querySelector("[data-pal-board] canvas"),p=c.getContext("2d").getImageData(0,0,320,200).data;return [...new Uint8Array(await crypto.subtle.digest("SHA-256",p))].map(x=>x.toString(16).padStart(2,"0")).join("")})()')

def original_short_step(player,direction,touch=False):
    wait(lambda:player.js('!!document.querySelector("[data-pal-phase=field][data-pal-ready=true] [data-pal-direction='+direction+']:not(:disabled)")'),'displayed field controller is not ready')
    before=position(player);pixels=canvas_hash(player)
    dx,dy,key={'down':(-16,8,'ArrowDown'),'up':(16,-8,'ArrowUp')}[direction]
    if touch:
        player.b('wait','[data-pal-direction="'+direction+'"]:not(:disabled)')
        player.tap('[data-pal-direction="'+direction+'"]')
    else:player.b('press',key)
    wait(lambda:position(player)!=before,'short native direction did not move the original hero')
    expected=[before[0]+dx,before[1]+dy]
    assert position(player)==expected,{'before':before,'expected':expected,'actual':position(player),'touch':touch}
    wait(lambda:canvas_hash(player)!=pixels,'original movement never reached actual canvas pixels')
    # Deliberately cover multiple original 100ms frames after physical release.
    time.sleep(.35)
    assert position(player)==expected,'released native direction kept walking'

players=[Browser('a'),Browser('b')]
try:
    a,b=players
    for p in players:
        p.b('open',BASE)
        p.b('set','viewport',1280,900)
        p.api('/register',{'name':'仙剑浏览器'+uuid.uuid4().hex[:6],'password':'pal-board-test-123','invite':'test-invite'})
    (OUT/'accounts.json').write_text(json.dumps([p.api('/state')['user']['name'] for p in players],ensure_ascii=False)+'\n')
    room=a.api('/rooms',{'kind':'pal','name':'原版仙剑共享旅程','capacity':2})
    b.api('/rooms/'+room['id'],{'type':'join','version':room['version'],'nonce':uuid.uuid4().hex})
    for p in players:p.command('ready')
    (OUT/'asset-fail').touch()
    a.command('start')
    for p in players:p.b('reload')
    wait(lambda:a.js('document.body.textContent.includes("加载未完成")'),'PAL asset failure must keep loading gate closed')
    before=a.data();assert a.room().get('loading')
    (OUT/'asset-fail').unlink()
    a.b('find','role','button','click','--name','重试加载')
    wait(lambda:a.room()['loading']['ready'][0],'first canvas never became ready')
    assert not a.room()['loading']['ready'][1] and a.data()==before,'one ready client advanced original rules'
    a.b('screenshot',OUT/'loading-desktop.png','--full')
    pixels=a.js('(()=>{const c=document.querySelector("[data-pal-board] canvas"),p=c.getContext("2d").getImageData(0,0,320,200).data;return {black:p.every((v,i)=>i%4===3?v===255:v===0),first:[...p.slice(0,12)],nonblack:p.filter((v,i)=>i%4!==3&&v!==0).length}})()')
    assert pixels['black'],{'message':'original initial logical screen must remain black before the first original scene paint','pixels':pixels,'view':a.data()}
    progress=a.js('(()=>{const e=document.querySelector("[data-pal-board]");return {loaded:+e.dataset.loadedBytes,total:+e.dataset.totalBytes}})()')
    assert progress['loaded']==progress['total'] and progress['total']>0,progress
    b.b('find','role','button','click','--name','重试加载')
    wait(lambda:not a.room().get('loading'),'all ready clients did not release loading gate')
    record({'check':'asset failure/retry, real byte counts, original black initial screen, all-human load gate','passed':True,'progress':progress})
    wait(lambda:a.data()['phase']=='dialogue','actual original inn dialogue',80)
    first=a.data();prompt=first['dialogue']['id']
    a.b('click','[data-pal-board] canvas')
    a.b('press','Enter')
    wait(lambda:a.data().get('dialogue',{}).get('read',[False])[0])
    assert a.data()['dialogue']['id']==prompt and not a.data()['dialogue']['read'][1],'one browser skipped original teammate'
    # Save the first partially read original page through the shared UI.
    for p in players:p.b('click','[data-save-game]')
    wait(lambda:not a.room() and not b.room(),'saving did not release both players')
    a.b('wait','[data-resume-save]');a.b('click','[data-resume-save]')
    a.b('wait','[data-saved-restoring]')
    assert not b.room(),'unconfirmed original player was occupied'
    b.b('wait','[data-resume-save]');b.b('click','[data-resume-save]')
    wait(lambda:a.room()['status']=='playing' and not a.room().get('loading'),'resumed PAL Board not ready',60)
    resumed=a.data();assert resumed['dialogue']['id']==prompt and resumed['dialogue']['read']==[True,False] and resumed['epoch']!=first['epoch']
    b.mobile(320,820);b.b('wait','[data-pal-action="confirm"]:not(:disabled)');b.tap('[data-pal-action="confirm"]')
    wait(lambda:a.data().get('dialogue',{}).get('id')!=prompt,'native phone reading did not continue original text')
    record({'check':'real keyboard/phone reading, all-player save/seat release, original-roster continue and preserved partial reading','passed':True})
    # Finish the original 23 reading stops using only the actual Board controls.
    stops=1
    while stops<40:
        wait(lambda:a.data()['phase'] in ['dialogue','field'],'next original reading stop',60)
        d=a.data()
        if d['phase']=='field':break
        pid=d['dialogue']['id']
        for i,p in enumerate(players):
            current=p.data()
            if current.get('dialogue',{}).get('id')==pid and not current['dialogue']['read'][i]:
                if i==0:p.b('press','Enter')
                else:p.b('wait','[data-pal-action="confirm"]:not(:disabled)');p.tap('[data-pal-action="confirm"]')
                wait(lambda p=p,i=i,pid=pid:read_complete(p,i,pid),'original reading control ignored')
        wait(lambda:a.data().get('dialogue',{}).get('id')!=pid,'original page did not advance',60)
        stops+=1
    assert a.data()['phase']=='field' and a.data()['scene']==2 and len(a.data()['party'])==1,'Board did not reach actual original inn/party'
    record({'check':'two humans finish all 23 original reading stops through real keyboard and mobile touch, actual original inn and single-member party','passed':True,'readingStops':stops})
    # Remap the actual platform profile through its shared settings interface.
    a.b('click','button[aria-label="控制器设置"]')
    a.b('wait','button[aria-label="修改 确认 按键"]')
    a.b('click','button[aria-label="修改 确认 按键"]')
    a.b('press','l')
    a.b('find','role','button','click','--name','保存设置')
    wait(lambda:a.api('/controllers?game=pal')['profiles']['game:pal']['keyboard']['confirm']=='KeyL','actual platform remapping was not saved')
    a.b('click','[role="dialog"][aria-label="控制器设置"] button[aria-label="关闭"]')
    a.b('click','[data-pal-board] canvas')
    # Open a real original menu, preserve it across fullscreen and rotate phone.
    a.b('press','Backspace')
    wait(lambda:a.data()['menu']['kind']==1,'native original main menu did not open')
    before_menu=a.data()['menu']
    b.tap('[data-pal-action="confirm"]')
    assert a.data()['menu']==before_menu,'nonleader drove original shared menu'
    a.b('press','Enter');assert a.data()['menu']['kind']==1,'old default confirmation bypassed remapping'
    a.b('press','l');wait(lambda:a.data()['menu']['kind']==2,'real original status menu')
    a.b('screenshot',OUT/'status-desktop.png','--full')
    a.b('press','Backspace');wait(lambda:a.data()['menu']['kind']==0,'status close')
    a.b('wait','[data-pal-panel-action]:not(:disabled)')
    a.b('click','[data-pal-panel-action]')
    a.b('click','[data-pal-menu-panel][aria-label="交接领队"] button:first-child')
    wait(lambda:a.data()['leader']==1,'native field leader was not transferred')
    a.b('press','Backspace');assert a.data()['menu']['kind']==0,'old leader retained menu permission'
    b.tap('[data-pal-action="cancel"]');wait(lambda:a.data()['menu']['kind']==1,'new leader could not open original menu')
    b.tap('[data-pal-action="cancel"]');wait(lambda:a.data()['menu']['kind']==0,'new leader could not close original menu')
    b.b('wait','[data-pal-phase="field"] [data-pal-panel-action]:not(:disabled)')
    b.tap('[data-pal-panel-action]')
    b.b('click','[data-pal-menu-panel][aria-label="交接领队"] button:first-child')
    wait(lambda:a.data()['leader']==0,'leader transfer back failed')
    record({'check':'actual platform remapping, old default key rejected, original status menu and both directions of leader transfer','passed':True})
    a.b('click','[data-pal-board] canvas')
    for _ in range(3):
        original_short_step(a,'down')
        original_short_step(a,'up')
    record({'check':'six real keyboard short directions each move one original step, paint actual changed pixels and stop after release','passed':True})
    # All layout/fullscreen changes must preserve the same Canvas instance.
    a.js('window.palOriginalCanvas=document.querySelector("[data-pal-board] canvas");true')
    a.mobile(320,820)
    a.b('click','button[aria-label="全屏"]');a.b('wait','[data-room-fullscreen]')
    for width,height in [(320,820),(820,320),(1280,900)]:
        if width==1280:a.desktop(width,height)
        else:a.mobile(width,height)
        placement=a.js('(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const c=document.querySelector("[data-pal-board] canvas"),p=document.querySelector("[data-pal-controls]"),b=c.getBoundingClientRect(),q=p.getBoundingClientRect(),controls=!!p.getClientRects().length;return {same:c===window.palOriginalCanvas,ratio:b.width/b.height,controls,visible:b.left>=-1&&b.top>=-1&&b.right<=innerWidth+1&&b.bottom<=innerHeight+1,overlay:!controls||(q.left>=b.left-1&&q.right<=b.right+1&&q.top>=b.top-1&&q.bottom<=b.bottom+1)}})()')
        assert placement['same'] and placement['visible'] and placement['overlay'] and abs(placement['ratio']-1.6)<.01,placement
        assert a.js('Math.abs(document.querySelector("[data-pal-board] canvas").getBoundingClientRect().width-Math.min(innerWidth,innerHeight*1.6))<2'),'fullscreen platform controls reduced original canvas'
        assert placement['controls']==(width!=1280),placement
        if width==320:
            for _ in range(3):
                original_short_step(a,'down',True)
                original_short_step(a,'up',True)
            record({'check':'six native phone direction taps each move one original step with changed canvas pixels and no walking after release','passed':True})
        a.b('screenshot',OUT/f'fullscreen-{width}.png')
    a.b('press','Escape');wait(lambda:not a.js('!!document.querySelector("[data-room-fullscreen]")'),'fullscreen did not exit')
    record({'check':'actual original menus, nonleader rejection, PC/320px/landscape shared fullscreen, same Canvas, mobile overlays and desktop without virtual buttons','passed':True,'readingStops':stops})
    assert not a.js('document.documentElement.scrollWidth>innerWidth+1')
    REPORT['passed']=True
finally:
    if not REPORT['passed']:
        for p in players:
            try:
                p.b('screenshot',OUT/('failure-'+p.name+'.png'),'--full')
                (OUT/('failure-'+p.name+'.json')).write_text(json.dumps({'state':p.api('/state'),'dom':p.js('({focus:document.hasFocus(),active:document.activeElement?.outerHTML,pal:document.querySelector("[data-pal-board]")?.outerHTML})')},ensure_ascii=False,indent=2)+'\n')
            except Exception:pass
    (OUT/'asset-fail').unlink(missing_ok=True)
    for p in players:
        if p.mobile_process:p.mobile_process.terminate();p.mobile_process.wait(timeout=5)
        try:p.b('close')
        except Exception:pass
        shutil.rmtree(p.env['AGENT_BROWSER_PROFILE'],ignore_errors=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'passed':REPORT['passed'],'result':str(OUT/'result.json')},ensure_ascii=False))
