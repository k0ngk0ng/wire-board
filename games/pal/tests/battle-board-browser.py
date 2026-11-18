#!/usr/bin/env python3
"""Test-first three-human original combat through the actual PAL Board.

Failures: quick directions vanish before original battle icon selection;
another owner submits a role; cooperative approval/cancel bypasses teammates;
shared save loses partial approvals or resources; resumed Board cannot submit
the actual native action and receive the original lantern reward.
This explicitly selects PAL_StartBattle verification, not campaign progress.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3];OUT=ROOT/'.local/pal/battle-board-browser';OUT.mkdir(parents=True,exist_ok=True)
BASE=(OUT/'url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pb'],cwd=ROOT,env=ENV,text=True).strip()
profiles=[OUT/('profile-'+str(i)) for i in range(3)]
for p in profiles:p.mkdir(exist_ok=True)
def b(i,*args):
    env=dict(ENV,AGENT_BROWSER_PROFILE=str(profiles[i]))
    r=subprocess.run(['agent-browser','--session',SESSION+'-'+str(i),*map(str,args),'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=45)
    assert r.returncode==0,r.stdout+r.stderr
    v=json.loads(r.stdout);assert v['success'],v;return v['data']
def js(i,code):return b(i,'eval',code)['result']
def api(i,path,body=None):
    options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
    v=js(i,'(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()');assert 200<=v['status']<300,v;return v['body']
def room(i=0):return api(i,'/state').get('room')
def data(i=0):return room(i)['game']['data']
def command(i,kind,**extra):
    r=room(i);return api(i,'/rooms/'+r['id'],dict(type=kind,version=r['version'],nonce=uuid.uuid4().hex,**extra))
def wait(check,label,seconds=40):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if check():return
        time.sleep(.15)
    raise AssertionError(label)
def continued():
    current=room()
    return current and current['status']=='playing' and not current.get('loading')
def commands(battle):
    # Original idle sprite animation keeps advancing while players select.
    # Keep every command/resource field, excluding only presentation frames.
    return {key:[{k:v for k,v in member.items() if k!='frame'} for member in value] if key in ['players','enemies'] else value for key,value in battle.items()}
REPORT={'passed':False,'scope':'real React platform Board and three human accounts, explicit original PAL_StartBattle fixture, actual role controls/cooperative consent/save/reward; not full campaign','cases':[]}
try:
    for i in range(3):
        b(i,'open',BASE);b(i,'set','viewport',1280,900)
        api(i,'/register',{'name':'合击浏览器'+uuid.uuid4().hex[:6],'password':'pal-battle-test-123','invite':'test-invite'})
    r=api(0,'/rooms',{'kind':'pal','name':'原版三人合击','capacity':3})
    for i in [1,2]:api(i,'/rooms/'+r['id'],{'type':'join','version':room()['version'],'nonce':uuid.uuid4().hex})
    for i in range(3):command(i,'ready')
    command(0,'start')
    for i in range(3):b(i,'reload')
    wait(lambda:not room().get('loading'),'all three canvases failed to load',60)
    wait(lambda:data().get('battle',{}).get('state')==1,'actual first original battle role unavailable')
    assert [p['controller'] for p in data()['party']]==[0,1,2]
    before=data()['battle']
    b(1,'click','[data-pal-board] canvas');b(1,'press','Enter')
    time.sleep(.2)
    after=data()['battle']
    assert commands(after)==commands(before),{'message':'another browser controlled the first role','before':commands(before),'after':commands(after)}
    b(0,'click','[data-pal-board] canvas')
    for _ in range(8):
        b(0,'press','ArrowRight')
        wait(lambda:data()['battle']['selected']==2,'quick native key did not select original cooperative icon')
        b(0,'press','ArrowUp')
        wait(lambda:data()['battle']['selected']==0,'quick native key did not return to original attack icon')
    b(0,'press','ArrowRight')
    wait(lambda:data()['battle']['selected']==2,'final quick native cooperative selection')
    b(0,'press','Enter')
    for _ in range(6):
        if data()['phase']=='cooperation':break
        b(0,'press','Enter');time.sleep(.15)
    wait(lambda:data()['phase']=='cooperation','original cooperative commit did not wait for all owners')
    request=data()['cooperation'];assert request['approved']==[True,False,False] and request['contributors']==[0,2]
    b(1,'find','role','button','click','--name','同意合击')
    wait(lambda:data()['cooperation']['approved']==[True,True,False],'second owner did not approve actual cooperative request')
    before=data();b(0,'screenshot',OUT/'pending-desktop.png','--full')
    for i in range(3):b(i,'click','[data-save-game]')
    wait(lambda:all(not room(i) for i in range(3)),'original three-person save did not release seats')
    for i in range(3):b(i,'wait','[data-resume-save]');b(i,'click','[data-resume-save]')
    wait(continued,'saved combat Board did not resume',60)
    after=data();assert after['cooperation']==before['cooperation'] and after['battle']==before['battle'] and after['epoch']!=before['epoch']
    for i in range(3):b(i,'wait','[data-pal-ready="true"]')
    b(2,'find','role','button','click','--name','同意合击')
    wait(lambda:data()['cash']==3,'actual original cooperative victory/reward missing',60)
    assert not data().get('cooperation')
    b(0,'screenshot',OUT/'victory-desktop.png','--full')
    REPORT['cases'].append({'check':'three real owners, quick original direction/confirm, cooperative partial consent saved and continued by original roster, actual original three-cash victory','passed':True,'request':request})
    REPORT['passed']=True
finally:
    if not REPORT['passed']:
        for i in range(3):
            try:
                b(i,'screenshot',OUT/f'failure-{i}.png','--full');(OUT/f'failure-{i}.json').write_text(json.dumps({'state':api(i,'/state'),'dom':js(i,'({focus:document.hasFocus(),active:document.activeElement?.outerHTML,pal:document.querySelector("[data-pal-board]")?.outerHTML})')},ensure_ascii=False,indent=2)+'\n')
            except Exception:pass
    for i in range(3):
        try:b(i,'close')
        except Exception:pass
        shutil.rmtree(profiles[i],ignore_errors=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n');print(json.dumps({'passed':REPORT['passed'],'result':str(OUT/'result.json')},ensure_ascii=False))
