SCOPE = "Actual independent single-player PAL rooms and Canvas response with bounded TCP propagation delay."
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
parser=argparse.ArgumentParser(description=SCOPE);parser.add_argument('--baseline',action='store_true');parser.add_argument('--tag',default='baseline');args=parser.parse_args()
assert args.tag.replace('-','').isalnum()
OUT=ROOT/'.local/pal/performance'/args.tag;OUT.mkdir(parents=True,exist_ok=True)
FIXTURE=ROOT/('.local/pal/cdn-performance-board-browser' if os.environ.get('PAL_VISUAL_CDN')=='1' else '.local/pal/performance-board-browser')
BASE=(FIXTURE/'url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pal-perf-ab'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_SOCKET_DIR']).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pp'],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'measurementComplete':False,'fullPerformanceAcceptance':False,'scope':SCOPE+' Current production rule-only bundle and original inn from new game; actual keyboard/touch-to-camera-pixel, rAF, RTT/ACK, game bytes and timer callbacks. Not menu/battle or complete performance acceptance.','budgets':{'responseP50MS':33,'responseP95MS':50,'responseMaxMS':100,'rafP95MS':35,'fieldGameP95Bytes':4096,'ruleP95MS':20},'cases':[],'fixture':json.loads((FIXTURE/'identity.json').read_text())}

def quantile(values,p):
    values=sorted(values)
    return values[min(len(values)-1,int((len(values)-1)*p+.5))] if values else None
def stats(values):return {'count':len(values),'p50':quantile(values,.5),'p95':quantile(values,.95),'max':max(values) if values else None}
def wait(predicate,label,seconds=40):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if predicate():return
        time.sleep(.1)
    raise AssertionError(label)
class Browser:
    def __init__(self,name,url):
        self.name=name;self.url=url;self.env=dict(ENV,AGENT_BROWSER_PROFILE=str(OUT/('profile-'+name)));Path(self.env['AGENT_BROWSER_PROFILE']).mkdir(exist_ok=True)
        self.observer=None;self.device=None
    def b(self,*args):
        result=subprocess.run(['agent-browser','--session',SESSION+'-'+self.name,*map(str,args),'--json'],cwd=ROOT,env=self.env,text=True,capture_output=True,timeout=45)
        assert result.returncode==0,result.stdout+result.stderr
        value=json.loads(result.stdout);assert value['success'],value;return value['data']
    def js(self,source):return self.b('eval',source)['result']
    def api(self,path,body=None):
        options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
        value=self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()');assert 200<=value['status']<300,value;return value['body']
    def room(self):return self.api('/state')['room']
    def data(self):return self.room()['game']['data']
    def command(self,kind,**extra):
        room=self.room();return self.api('/rooms/'+room['id'],dict(type=kind,version=room['version'],nonce=uuid.uuid4().hex,**extra))
    def trace(self):
        self.observer.send_signal(signal.SIGUSR2)
        return json.loads(self.observer.stdout.readline())
    def observe(self):
        self.observer=subprocess.Popen(['node','games/pal/tests/performance-observer.mjs',self.b('get','cdp-url')['cdpUrl'],self.url],cwd=ROOT,env=self.env,text=True,stdout=subprocess.PIPE)
        assert self.observer.stdout.readline().strip()=='ready'
    def mobile(self):
        self.b('set','viewport',320,820)
        self.device=subprocess.Popen(['node','scripts/browser_device.mjs',self.b('get','cdp-url')['cdpUrl'],self.url,'320','820','--keep'],cwd=ROOT,env=self.env,text=True,stdout=subprocess.PIPE)
        assert self.device.stdout.readline().strip()=='ready'
    def tap(self,direction):
        state=self.data()
        cx,cy=state['x']-state['cameraX'],state['y']-state['cameraY']-14
        px,py=(cx-12,cy+32) if direction=='down' else (cx+12,cy-32)
        self.js('window.palPerformance.touchCentre='+json.dumps({'x':cx,'y':cy}))
        x,y=self.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+'+str(px)+'*r.width/320,r.y+'+str(py)+'*r.height/200]})()')
        result=subprocess.run(['node','games/pal/tests/screen-touch.mjs',self.b('get','cdp-url')['cdpUrl'],self.url,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=self.env,text=True,capture_output=True,timeout=25)
        assert result.returncode==0,result.stdout+result.stderr
    def close(self):
        if self.device:self.device.terminate();self.device.wait(timeout=5)
        if self.observer:
            self.observer.terminate();self.observer.communicate(timeout=5)
        try:self.b('close')
        except Exception:pass
        shutil.rmtree(self.env['AGENT_BROWSER_PROFILE'],ignore_errors=True)

proxy=subprocess.Popen(['node','games/pal/tests/performance-proxy.mjs',BASE],cwd=ROOT,env=ENV,text=True,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
PROXY=json.loads(proxy.stdout.readline())['url']
def configure_delay(delay):
    proxy.stdin.write(json.dumps({'delayMS':delay,'reset':True})+'\n');proxy.stdin.flush();return json.loads(proxy.stdout.readline())
a,b=Browser('a',PROXY),Browser('b',BASE)
players=[a,b]
def trial(player,direction,touch):
    wait(lambda:player.js('!!document.querySelector("[data-pal-phase=field][data-pal-ready=true] canvas")'),'actual original field canvas not ready')
    before=player.data();count=player.js('window.palPerformance.trials.length')
    if touch:player.tap(direction)
    else:player.b('click','[data-pal-board] canvas');player.b('press','ArrowDown' if direction=='down' else 'ArrowUp')
    wait(lambda:player.js('window.palPerformance.trials.length')>count,'native input was not observed')
    wait(lambda:player.js('!!(window.palPerformance.trials.at(-1).responseMS!==undefined||window.palPerformance.trials.at(-1).error)'),'actual canvas response was not sampled')
    result=player.js('(()=>{const {expected,canvas,...result}=window.palPerformance.trials.at(-1);return result})()')
    assert not result.get('error') and result['trusted'] and result['informativePixels']>=128,result
    dx,dy=(-16,8) if direction=='down' else (16,-8)
    def arrived():
        current=player.data()
        return current['x']==before['x']+dx and current['y']==before['y']+dy
    wait(arrived,'native movement disagreed with the sampled camera displacement')
    return result
def run_case(player,label,touch=False,pairs=6):
    network_start=time.time()*1000
    wait(lambda:any(event.get('connectionRTTMS') is not None and event.get('connectionSampledAt',0)>=network_start for event in player.trace()['events']),'no fresh native Ping/Pong sample for '+label,20)
    start=time.time()*1000;player.js('window.palPerformance.mode='+json.dumps(label))
    samples=[]
    for _ in range(pairs):
        samples.append(trial(player,'down',touch));samples.append(trial(player,'up',touch))
    end=time.time()*1000
    raf=player.js('window.palPerformance.raf.filter(frame=>frame.mode==='+json.dumps(label)+').map(frame=>frame.at)')
    all_trace=[event for event in player.trace()['events'] if network_start<=event['at']<=end]
    trace=[event for event in all_trace if start<=event['at']<=end]
    rtt={event['connectionSampledAt']:event['connectionRTTMS'] for event in all_trace if event.get('connectionRTTMS') is not None and event.get('connectionSampledAt',0)>=network_start}
    gaps=[next-current for current,next in zip(raf,raf[1:])]
    case={'name':label,'samples':samples,'responseMS':stats([sample['responseMS'] for sample in samples]),'rafMS':stats(gaps),'gameBytes':stats([event['bytes'] for event in trace if event['type']=='game']),'connectionRTTMS':stats(list(rtt.values())),'inputAckMS':stats([event['ackMS'] for event in trace if event.get('ackMS') is not None]),'network':all_trace,'networkWarmupStart':network_start,'inputMeasurementStart':start}
    timers=player.js('(async()=>await(await fetch("/pal-performance/metrics")).json())()')
    case['ruleSamples']=[sample for sample in timers if start<=sample['at']<=end]
    case['ruleMS']=stats([sample['elapsedMS'] for sample in case['ruleSamples']])
    budgets=REPORT['budgets']
    case['passed']=case['responseMS']['p50']<=budgets['responseP50MS'] and case['responseMS']['p95']<=budgets['responseP95MS'] and case['responseMS']['max']<=budgets['responseMaxMS'] and bool(gaps) and case['rafMS']['p95']<=budgets['rafP95MS'] and bool(case['gameBytes']['count']) and case['gameBytes']['p95']<=budgets['fieldGameP95Bytes'] and case['ruleMS']['count']>0 and case['ruleMS']['p95']<=budgets['ruleP95MS'] and all(sample['players']==1 and not sample['failed'] for sample in case['ruleSamples'])
    REPORT['cases'].append(case)
    print(json.dumps({key:case[key] for key in ['name','passed','responseMS','rafMS','gameBytes','connectionRTTMS','inputAckMS','ruleMS']},ensure_ascii=False),flush=True)
    player.b('screenshot',OUT/(label+'.png'),'--full')
try:
    account_names=[]
    for player in players:
        player.b('open',player.url);player.b('set','viewport',1280,900)
        account_names.append('仙剑性能'+uuid.uuid4().hex[:6])
        player.api('/register',{'name':account_names[-1],'password':'pal-perf-test-123','invite':'test-invite'})
        player.observe()
    (OUT/'accounts.json').write_text(json.dumps(account_names,ensure_ascii=False)+'\n')
    room_ids=[]
    for player in players:
        room=player.api('/rooms',{'kind':'pal','name':'原版单人性能'+player.name,'capacity':1})
        room_ids.append(room['id'])
        player.command('ready')
        started=time.monotonic()
        player.command('start')
        REPORT.setdefault('checkpointSetupMS',[]).append((time.monotonic()-started)*1000)
        player.b('reload')
        wait(lambda:not player.room().get('loading'),'actual solo canvas did not release its loading barrier',90)
        player.b('wait','[data-pal-ready=true]')
        assert player.data()['phase']=='field' and player.data()['scene']==2
        assert player.room()['capacity']==1 and len(player.room()['seats'])==1
        player.b('click','[data-fullscreen-enter]');player.b('wait','[data-room-fullscreen]')
        assert not player.js('document.querySelector("[data-pal-direction],[data-pal-action]")!==null')
    assert room_ids[0]!=room_ids[1]
    REPORT['roomIDs']=room_ids
    wait(lambda:a.trace()['connections']>0 and b.trace()['connections']>0,'live sockets were not ready')
    run_case(a,'normal')
    configure_delay(100)
    wait(lambda:any(event.get('connectionRTTMS',0)>=150 for event in a.trace()['events'][-40:] if event.get('connectionRTTMS') is not None),'the TCP proxy did not produce actual delayed Ping/Pong RTT',30)
    run_case(a,'rtt-200')
    assert b.room()['id']!=a.room()['id'] and b.room()['capacity']==1
    run_case(b,'fast-independent-room-while-a-delayed')
    b.mobile();run_case(b,'phone-320',touch=True,pairs=6)
    REPORT['phoneCanvas']=b.js('(()=>{const c=document.querySelector("[data-pal-board] canvas"),r=c.getBoundingClientRect();return {width:r.width,height:r.height,viewportWidth:innerWidth,viewportHeight:innerHeight}})()')
    assert abs(REPORT['phoneCanvas']['width']-min(320,REPORT['phoneCanvas']['viewportHeight']*1.6))<1
    assert REPORT['fixture']['profile']=='original-98' and REPORT['fixture']['players']==1
    REPORT['measurementComplete']=True
    REPORT['passed']=all(case['passed'] for case in REPORT['cases'])
finally:
    for player in players:
        if not REPORT['measurementComplete']:
            try:
                player.b('screenshot',OUT/('failure-'+player.name+'.png'),'--full')
                (OUT/('failure-'+player.name+'.json')).write_text(json.dumps({'state':player.api('/state'),'network':player.trace(),'trials':player.js('window.palPerformance?.trials.map(({expected,canvas,...trial})=>trial)')},ensure_ascii=False,indent=2)+'\n')
            except Exception:pass
        try:player.close()
        except Exception:pass
    proxy.terminate();proxy.communicate(timeout=5)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps({'passed':REPORT['passed'],'measurementComplete':REPORT['measurementComplete'],'result':str(OUT/'result.json')},ensure_ascii=False),flush=True)
if not args.baseline:assert REPORT['passed'],'frozen performance budgets are not met'
