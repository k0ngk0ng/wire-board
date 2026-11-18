#!/usr/bin/env python3
"""Test first: actual Suzhou continuation in the production React Board.

Failures: checkpoint entry edits original progress; three loading clients lose
an unassigned reader; desktop keys or native mobile touch do not move the party;
phone controls shrink the canvas; resize replaces it; save/resume loses the
original accounts, owners, position or bundle reward. Entry uses an unedited
campaign E2E checkpoint; this is not a full browser campaign or latency test.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/pal/departure-board-browser'
BASE=(OUT/'url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pal-departure'],cwd=ROOT,env=ENV,text=True).strip()
checkpoint=(OUT/'entry-state.json').read_bytes()
REPORT={'passed':False,'scope':__doc__,'cases':[],'checkpointSHA256':hashlib.sha256(checkpoint).hexdigest()}

class Browser:
    def __init__(self,index):
        self.index=index
        self.profile=OUT/('profile-'+str(index))
        self.profile.mkdir(parents=True,exist_ok=True)
        self.env=dict(ENV,AGENT_BROWSER_PROFILE=str(self.profile))
        self.mobile_process=None
        self.name='苏州验收'+uuid.uuid4().hex[:7]
    def b(self,*args):
        r=subprocess.run(['agent-browser','--session',SESSION+'-'+str(self.index),*map(str,args),'--json'],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=45)
        assert r.returncode==0,r.stdout+r.stderr
        data=json.loads(r.stdout);assert data['success'],data
        return data['data']
    def js(self,code):return self.b('eval',code)['result']
    def api(self,path,body=None):
        options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
        data=self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
        assert 200<=data['status']<300,data
        return data['body']
    def room(self):return self.api('/state').get('room')
    def data(self):return self.room()['game']['data']
    def command(self,kind):
        room=self.room()
        return self.api('/rooms/'+room['id'],{'type':kind,'version':room['version'],'nonce':uuid.uuid4().hex})
    def tap(self,selector):
        self.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center",behavior:"instant"})')
        x,y=self.js('(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        r=subprocess.run(['node','scripts/browser_touch.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=25)
        assert r.returncode==0,r.stdout+r.stderr
    def mobile(self,width,height):
        if self.mobile_process:self.mobile_process.terminate();self.mobile_process.wait(timeout=5)
        self.b('set','viewport',width,height)
        self.mobile_process=subprocess.Popen(['node','scripts/browser_device.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,str(width),str(height),'--keep'],cwd=ROOT,env=self.env,stdout=subprocess.PIPE,text=True)
        assert self.mobile_process.stdout.readline().strip()=='ready'

def wait(check,label,seconds=40):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if check():return
        time.sleep(.12)
    raise AssertionError(label)

def position(p):
    d=p.data();return [d['x'],d['y']]

def record(value):
    REPORT['cases'].append(value)
    print(json.dumps(value,ensure_ascii=False),flush=True)

def move(p,touch):
    wait(lambda:p.js('!!document.querySelector("[data-pal-ready=true][data-pal-phase=field]")'),'original field is not painted')
    data=p.data();cells=base64.b64decode(data['field']['cells'])
    # Choose a currently visible, ordinary ground cell; never enter a script
    # trigger or infer an unseen route from the native checkpoint.
    directions=[('down','ArrowDown',(-16,8),(0,1)),('up','ArrowUp',(16,-8),(0,-1)),('left','ArrowLeft',(-16,-8),(-1,0)),('right','ArrowRight',(16,8),(1,0))]
    name,key,(dx,dy),_=next(row for row in directions if cells[(row[3][1]+8)*17+row[3][0]+8]==0)
    before=[data['x'],data['y']]
    if touch:p.tap('[data-pal-direction="'+name+'"]')
    else:p.b('press',key)
    expected=[before[0]+dx,before[1]+dy]
    wait(lambda:position(p)!=before,'native movement never reached the original rules')
    assert position(p)==expected,{'before':before,'expected':expected,'actual':position(p)}
    time.sleep(.35)
    assert position(p)==expected,'released native input kept moving'
    record({'check':'native phone touch' if touch else 'native desktop keyboard','before':before,'after':expected,'originalRoles':data['party'],'passed':True})

players=[Browser(i) for i in range(3)]
try:
    for p in players:
        p.b('open',BASE)
        p.b('set','viewport',1280,900)
        p.api('/register',{'name':p.name,'password':'pal-departure-test-123','invite':'test-invite'})
    a,b,c=players
    room=a.api('/rooms',{'kind':'pal','name':'原版苏州续档验收','capacity':3})
    for p in [b,c]:
        current=a.room()
        p.api('/rooms/'+room['id'],{'type':'join','version':current['version'],'nonce':uuid.uuid4().hex})
    for p in players:p.command('ready')
    a.command('start')
    for p in players:p.b('reload')
    wait(lambda:all(p.room() and not p.room().get('loading') for p in players),'three actual canvases never released loading',70)
    for p in players:p.b('snapshot','-i')
    initial=a.data()
    assert initial['scene']==22 and initial['map']==18 and initial['cash']==1513 and initial['leader']==1,initial
    assert [(p['role'],p['controller']) for p in initial['party']]==[(0,0),(1,1)]
    assert all(p.data()['party']==initial['party'] for p in players)
    assert not b.js('!!document.querySelector("[data-pal-controls]").getClientRects().length'),'desktop shows virtual controls'
    b.b('screenshot',OUT/'desktop.png','--full')
    (OUT/'desktop-snapshot.json').write_text(json.dumps(b.b('snapshot','-i'),ensure_ascii=False,indent=2)+'\n')
    move(b,False)
    before=position(b)
    c.b('press','ArrowUp')
    time.sleep(.25)
    assert position(b)==before,'unassigned third seat moved the party'
    record({'check':'three authenticated accounts, original party/owners, unassigned seat cannot move','passed':True})
    b.js('window.originalPalCanvas=document.querySelector("[data-pal-board] canvas");true')
    b.b('click','[data-fullscreen-enter]');b.b('wait','[data-room-fullscreen]')
    for label,width,height in [('phone',320,820),('landscape',820,320)]:
        b.mobile(width,height)
        geometry=b.js('(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(r))));const c=document.querySelector("[data-pal-board] canvas"),r=c.getBoundingClientRect(),p=document.querySelector("[data-pal-controls]"),buttons=[...p.querySelectorAll("button")].filter(e=>e.getClientRects().length);return {x:r.x,y:r.y,width:r.width,height:r.height,same:c===window.originalPalCanvas,touch:matchMedia("(pointer:coarse)").matches,inside:buttons.every(e=>{const b=e.getBoundingClientRect();return b.left>=r.left-1&&b.right<=r.right+1&&b.top>=r.top-1&&b.bottom<=r.bottom+1}),background:getComputedStyle(buttons[0]).backgroundColor}})()')
        assert geometry['same'] and geometry['touch'] and geometry['inside'] and 'rgba' in geometry['background'],geometry
        assert abs(geometry['width']-min(width,height*1.6))<2 and abs(geometry['width']/geometry['height']-1.6)<.01,geometry
        b.b('screenshot',OUT/(label+'.png'))
        record({'check':label+' maximum proportional canvas with translucent overlay controls','passed':True,'geometry':geometry})
        if label=='phone':move(b,True)
    b.mobile(320,820)
    before=a.data()
    a.b('click','[data-save-game]')
    wait(lambda:a.room()['game']['paused'],'first original save confirmation did not freeze the game')
    paused=a.data()
    b.tap('[data-room-fullscreen] [data-save-game]')
    c.b('click','[data-save-game]')
    wait(lambda:all(not p.room() for p in players),'saved original roster retained active seats')
    a.b('wait','[data-resume-save]');a.b('click','[data-resume-save]')
    a.b('wait','[data-saved-restoring]')
    assert not b.room() and not c.room(),'continuation occupied absent original players'
    b.b('wait','[data-resume-save]');b.tap('[data-resume-save]')
    assert not c.room(),'two confirmations occupied the third original player'
    c.b('wait','[data-resume-save]');c.b('click','[data-resume-save]')
    wait(lambda:all(p.room() and p.room()['status']=='playing' and not p.room().get('loading') for p in players),'original trio did not resume the same scene',70)
    resumed=a.data()
    for key in ['scene','map','x','y','cash','party','leader']:
        assert resumed[key]==paused[key],{'key':key,'before':paused[key],'after':resumed[key]}
    assert resumed['epoch']!=paused['epoch'] and resumed['cash']==1513
    b.b('screenshot',OUT/'phone-resumed.png','--full')
    record({'check':'three original accounts confirm save and continue, including the unassigned seat; same scene, position, owners and 500-cash gift','passed':True,'scene':resumed['scene'],'position':position(a),'cash':resumed['cash'],'party':resumed['party']})
    REPORT['passed']=True
finally:
    for p in players:
        if p.mobile_process:p.mobile_process.terminate();p.mobile_process.wait(timeout=5)
        try:p.b('close')
        except Exception:pass
        shutil.rmtree(p.profile,ignore_errors=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    (OUT/'stop').touch()
    print(json.dumps(REPORT,ensure_ascii=False))
