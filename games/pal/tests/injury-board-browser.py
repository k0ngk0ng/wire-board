#!/usr/bin/env python3
"""Test first: original injury dialogue and recovery on the actual React Board.

Failures: a temporary solo party drops readers; an already-read human skips
others; native touch fails; partial reading or the original injured pose changes
on save; absent humans are occupied; restart/re-entry resets the scene; remaining
ordinary reading cannot finish the original recovery and two-role reunion.
Entry is an unedited E2E checkpoint, not an injected story or browser campaign.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
OUT=ROOT/'.local/pal/injury-board-browser'
BASE=(OUT/'url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pal-injury'],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'scope':__doc__,'cases':[],'checkpointSHA256':hashlib.sha256((OUT/'entry-state.json').read_bytes()).hexdigest()}

class Browser:
    def __init__(self,index):
        self.index=index;self.profile=OUT/('profile-'+str(index));self.profile.mkdir(parents=True,exist_ok=True)
        self.env=dict(ENV,AGENT_BROWSER_PROFILE=str(self.profile));self.mobile_process=None
    def b(self,*args):
        r=subprocess.run(['agent-browser','--session',SESSION+'-'+str(self.index),*map(str,args),'--json'],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=45)
        assert r.returncode==0,r.stdout+r.stderr
        data=json.loads(r.stdout);assert data['success'],data
        return data['data']
    def js(self,code):return self.b('eval',code)['result']
    def api(self,path,body=None):
        options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
        d=self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
        assert 200<=d['status']<300,d
        return d['body']
    def room(self):return self.api('/state').get('room')
    def data(self):return self.room()['game']['data']
    def command(self,kind):
        r=self.room();return self.api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
    def tap(self,selector):
        self.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center",behavior:"instant"})')
        x,y=self.js('(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        r=subprocess.run(['node','scripts/browser_touch.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=25)
        assert r.returncode==0,r.stdout+r.stderr
    def mobile(self):
        self.b('set','viewport',320,820)
        self.mobile_process=subprocess.Popen(['node','scripts/browser_device.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,'320','820','--keep'],cwd=ROOT,env=self.env,stdout=subprocess.PIPE,text=True)
        assert self.mobile_process.stdout.readline().strip()=='ready'

def wait(check,label,seconds=55):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if check():return
        time.sleep(.12)
    raise AssertionError(label)

def record(d):
    REPORT['cases'].append(d);print(json.dumps(d,ensure_ascii=False),flush=True)

def read_done(p,actor,prompt):
    d=p.data().get('dialogue')
    return not d or d['id']!=prompt or d['read'][actor]

players=[Browser(i) for i in range(3)]
try:
    for p in players:
        p.b('open',BASE);p.b('set','viewport',1280,900)
        p.api('/register',{'name':'还魂验收'+uuid.uuid4().hex[:7],'password':'pal-injury-test-123','invite':'test-invite'})
    a,b,c=players
    room=a.api('/rooms',{'kind':'pal','name':'原版受伤剧情续档','capacity':3})
    for p in [b,c]:
        r=a.room();p.api('/rooms/'+r['id'],{'type':'join','version':r['version'],'nonce':uuid.uuid4().hex})
    for p in players:p.command('ready')
    a.command('start')
    for p in players:p.b('reload')
    wait(lambda:all(p.room() and not p.room().get('loading') for p in players),'three actual canvases did not finish loading',75)
    for p in players:p.b('snapshot','-i')
    initial=a.data();prompt=initial['dialogue']['id']
    assert initial['scene']==22 and len(initial['party'])==1 and initial['party'][0]['role']==0 and initial['party'][0]['controller']==0
    assert initial['dialogue']['read']==[True,False,False]
    assert any(d['kind']==0 and d['a']==228 for d in initial['draws']),'original injured sprite is absent'
    a.b('screenshot',OUT/'desktop-injury.png','--full')
    a.b('press','Enter')
    assert a.data()['dialogue']==initial['dialogue'],'already-read owner skipped the two unassigned humans'
    b.mobile();b.b('click','[data-fullscreen-enter]');b.b('wait','[data-room-fullscreen]')
    b.b('screenshot',OUT/'phone-injury.png')
    b.tap('[data-pal-action="confirm"]')
    wait(lambda:a.data()['dialogue']['read']==[True,True,False],'unassigned mobile reader could not confirm')
    record({'check':'one original role, all three original readers, already-read owner cannot skip, native phone reading','passed':True})
    a.b('click','[data-save-game]')
    wait(lambda:a.room()['game']['paused'],'first original save did not freeze injury')
    paused=a.data()
    b.tap('[data-room-fullscreen] [data-save-game]');c.b('click','[data-save-game]')
    wait(lambda:all(not p.room() for p in players),'saved injury kept original human seats')
    a.b('wait','[data-resume-save]');a.b('click','[data-resume-save]');a.b('wait','[data-saved-restoring]')
    assert not b.room() and not c.room(),'first confirmation occupied absent original players'
    b.b('wait','[data-resume-save]');b.tap('[data-resume-save]')
    assert not c.room(),'two confirmations occupied the absent third reader'
    c.b('wait','[data-resume-save]');c.b('click','[data-resume-save]')
    wait(lambda:all(p.room() and p.room()['status']=='playing' and not p.room().get('loading') for p in players),'original trio did not restore injury',75)
    resumed=a.data()
    for key in ['scene','map','x','y','cash','party','dialogue','draws','raster','overlay']:
        assert resumed.get(key)==paused.get(key),{'changed':key}
    assert resumed['epoch']!=paused['epoch'] and resumed['dialogue']['read']==[True,True,False]
    record({'check':'original trio saves and continues unchanged injury pose and partial reading while only Li is in the party','passed':True,'read':resumed['dialogue']['read'],'party':resumed['party']})
    c.b('press','Enter')
    wait(lambda:a.data().get('dialogue',{}).get('id')!=prompt,'third reader did not release the original script')
    pages=0
    while pages<40:
        wait(lambda:a.data()['phase'] in ['dialogue','field'],'original recovery next reading/field',100)
        current=a.data()
        if current['phase']=='field':break
        pid=current['dialogue']['id']
        assert len(current['dialogue']['read'])==3,'recovery lost original reader'
        for i,p in enumerate(players):
            if not read_done(p,i,pid):
                if i==1:
                    p.b('wait','[data-pal-action="confirm"]:not(:disabled)');p.tap('[data-pal-action="confirm"]')
                else:p.b('press','Enter')
                wait(lambda p=p,i=i,pid=pid:read_done(p,i,pid),'ordinary recovery reading failed')
        wait(lambda:a.data().get('dialogue',{}).get('id')!=pid,'original recovery page did not advance',100)
        pages+=1
    final=a.data()
    assert final['phase']=='field' and final['scene']==22 and final['cash']==paused['cash']
    assert [(p['role'],p['controller']) for p in final['party']]==[(0,0),(1,1)]
    assert final['party'][0]['hp']==final['party'][0]['maxHP'],'original recovery did not restore Li'
    assert final['party'][0]['mp']==final['party'][0]['maxMP'] and final['field']['actors'][0][2]==2,'original recovery did not restore magic/normal appearance'
    b.b('click','[data-fullscreen-enter]');b.b('wait','[data-room-fullscreen]')
    b.b('screenshot',OUT/'phone-recovered.png')
    record({'check':'remaining original injury/recovery reading and animation finish through native controls; original roles and owners rejoin','passed':True,'pagesAfterResume':pages,'cash':final['cash'],'party':final['party']})
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
