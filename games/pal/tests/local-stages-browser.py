import argparse
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import time
import uuid

ROOT=Path(__file__).resolve().parents[3]
parser=argparse.ArgumentParser()
parser.add_argument('mode',choices=['battle','dialogue'])
args=parser.parse_args()
out=ROOT/'.local/pal/verification/local-first'/(args.mode+'-browser')
out.mkdir(parents=True,exist_ok=True)
base=(out.parent/(args.mode+'-fixture')/'url').read_text().strip()
socket_root=Path(subprocess.check_output(['git','rev-parse','--git-common-dir'],cwd=ROOT,text=True).strip()).resolve().parent/'.local/pal-lf-ab'
env=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(socket_root),AGENT_BROWSER_PROFILE=str(out/'profile'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
session=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','lb' if args.mode=='battle' else 'ld'],cwd=ROOT,env=env,text=True).strip()
def browser(*argv):
    result=subprocess.run(['agent-browser','--session',session,*map(str,argv),'--json'],cwd=ROOT,env=env,text=True,capture_output=True,timeout=45)
    assert result.returncode==0,result.stdout+result.stderr
    value=json.loads(result.stdout)
    assert value['success'],value
    return value['data']
def js(code):
    return browser('eval',code)['result']
def wait(fn,label,seconds=50):
    until=time.monotonic()+seconds
    while time.monotonic()<until:
        value=fn()
        if value:return value
        time.sleep(.05)
    raise AssertionError(label)
def api(path,body=None):
    options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
    value=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
    assert 200<=value['status']<300,value
    return value['body']
def room():return api('/state')['room']
def command(kind):
    value=room()
    return api('/rooms/'+value['id'],{'type':kind,'version':value['version'],'nonce':uuid.uuid4().hex})
def local_ready():
    generation=room()['game']['data']['local']['generation']
    return js('(()=>{const b=document.querySelector("[data-pal-board]");if(!b)return false;const s=JSON.parse(b.dataset.palLocalState||"{}");return b.dataset.palReady==="true"&&s.ready&&s.active&&s.enabled&&s.generation==='+str(generation)+'})()')
proxy=subprocess.Popen(['node','games/pal/tests/performance-proxy.mjs',base],cwd=ROOT,env=env,text=True,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
url=json.loads(proxy.stdout.readline())['url']
report={'passed':False,'mode':args.mode,'budgets':{'p50':33,'p95':50,'max':100},'cases':[]}
observer=device=None
def touch(x,y):
    px,py=js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+'+str(x)+'*r.width/320,r.y+'+str(y)+'*r.height/200]})()')
    subprocess.run(['node','games/pal/tests/screen-touch.mjs',endpoint,url,'tap',*map(str,[px,py,px,py])],cwd=ROOT,env=env,check=True,capture_output=True,text=True,timeout=15)
try:
    browser('open',url)
    browser('set','viewport',1280,900)
    api('/register',{'name':'仙剑原版'+args.mode+uuid.uuid4().hex[:5],'password':'pal-local-stage-123','invite':'test-invite'})
    endpoint=browser('get','cdp-url')['cdpUrl']
    observer=subprocess.Popen(['node','games/pal/tests/performance-observer.mjs',endpoint,url,'local-performance-page.js'],cwd=ROOT,env=env,text=True,stdout=subprocess.PIPE)
    assert observer.stdout.readline().strip()=='ready'
    api('/rooms',{'kind':'pal','name':'本地原版阶段','capacity':1})
    command('ready')
    command('start')
    browser('reload')
    wait(lambda:not room().get('loading'),'original loading gate did not release',90)
    browser('wait','[data-pal-ready=true]')
    if args.mode=='dialogue':
        wait(lambda:room()['game']['data']['phase']=='dialogue','original dialogue entry missing')
        saved_id=room()['id']
        command('save')
        api('/saves/'+saved_id+'/resume',{})
        wait(lambda:room() and not room().get('loading'),'original saved page did not resume',90)
    browser('click','[data-fullscreen-enter]')
    browser('wait','[data-room-fullscreen]')
    js('document.querySelector("[data-pal-board]").focus()')
    wait(local_ready,'original local replica not controllable')
    before=room()['game']['data']
    for delay,phone in [(100,False),(100,True),(0,True)]:
        proxy.stdin.write(json.dumps({'delayMS':delay})+'\n')
        proxy.stdin.flush()
        assert json.loads(proxy.stdout.readline())['delayMS']==delay
        if phone and device is None:
            browser('set','viewport',320,820)
            device=subprocess.Popen(['node','scripts/browser_device.mjs',endpoint,url,'320','820','--keep'],cwd=ROOT,env=env,text=True,stdout=subprocess.PIPE)
            assert device.stdout.readline().strip()=='ready'
        samples=[]
        for repeat in range(4):
            if args.mode=='battle':
                steps=[('ArrowDown',{'battleMenu':0,'battleState':1,'battleSelected':3,'area':[27,170,24,15]},(39,177)),('Enter',{'battleMenu':4,'battleState':1,'area':[0,25,110,100]},(39,177)),('Backspace',{'battleMenu':0,'battleState':1,'area':[0,25,110,100]},None),('ArrowUp',{'battleMenu':0,'battleState':1,'battleSelected':0,'area':[27,140,24,15]},(39,147))]
            else:
                page=js('document.querySelector("[data-pal-board]").dataset.palDialogId')
                steps=[('Enter',{'dialogueFrom':page,'area':[0,0,320,200]},(160,120))]
            for key,extra,point in steps:
                spec={'code':key,'phase':'*' if args.mode=='dialogue' else args.mode,'kind':0,**extra}
                if phone and point:spec['trigger']='pointerup'
                js('window.palLocalPerformance.arm('+json.dumps(spec)+')')
                if phone and point:touch(*point)
                else:browser('press',key)
                sample=wait(lambda:js('window.palLocalPerformance.done'),'original local stage did not change pixels',6)
                assert sample['trusted']
                samples.append(sample)
                def converged():
                    data=room()['game']['data']
                    if args.mode=='dialogue':return data['phase']=='dialogue' and data['dialogue']['id']!=before['dialogue']['id']
                    return all(data['battle'][name]==value for name,value in [('menu',extra['battleMenu']),('state',extra['battleState'])]+([('selected',extra['battleSelected'])] if 'battleSelected' in extra else []))
                wait(converged,'authority did not converge to original selection')
                if args.mode=='dialogue':
                    command('load_checkpoint')
                    wait(lambda:not room().get('loading') and local_ready(),'original dialogue checkpoint did not restore')
                    assert room()['game']['data']['dialogue']['text']==before['dialogue']['text']
                time.sleep(.1)
        values=sorted(sample['responseMS'] for sample in samples)
        stats={'p50':values[round((len(values)-1)*.5)],'p95':values[round((len(values)-1)*.95)],'max':max(values)}
        case={'delayMS':delay,'phone':phone,'responseMS':stats,'samples':samples,'passed':all(stats[name]<=limit for name,limit in report['budgets'].items())}
        report['cases'].append(case)
        browser('screenshot',out/(str(delay)+'-'+str(phone)+'.png'))
        print(json.dumps({key:case[key] for key in ['delayMS','phone','responseMS','passed']}),flush=True)
    after=room()['game']['data']
    assert after['cash']==before['cash'] and after['party']==before['party']
    report['originalStatsPreserved']=True
    if args.mode=='battle':
        for player in range(3):
            wait(lambda:local_ready() and room()['game']['data']['battle']['current']==player,'next original party member did not receive control')
            browser('press','g')
            if player<2:
                wait(lambda:room()['game']['data']['battle']['current']==player+1,'original defensive command did not advance party slot')
                assert room()['game']['data']['battle']['players'][player]['action']==1
        wait(lambda:room()['game']['data']['battle']['phase']==0 and room()['game']['data']['battle']['state']==1 and room()['game']['data']['battle']['current']==0,'original enemy turn did not complete')
        report['allThreePartyCommandsAndEnemyTurn']=True
    report['canvas']=js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')
    assert report['canvas']==[320,200]
    report['passed']=all(case['passed'] for case in report['cases'])
    command('close')
except BaseException:
    report['failure']={'room':room(),'dom':js('document.querySelector("[data-pal-board]")?.outerHTML'),'probe':js('window.palLocalPerformance?.debug()')}
    browser('screenshot',out/'failure.png')
    raise
finally:
    if observer:
        observer.send_signal(signal.SIGUSR2)
        report['network']=json.loads(observer.stdout.readline())
        observer.terminate()
        observer.communicate(timeout=5)
    if device:
        device.terminate()
        device.wait(timeout=5)
    (out/'result.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    try:browser('close')
    finally:
        proxy.terminate()
        proxy.communicate(timeout=5)
        shutil.rmtree(out/'profile',ignore_errors=True)
assert report['passed'],'original local stage response exceeds frozen budget'
