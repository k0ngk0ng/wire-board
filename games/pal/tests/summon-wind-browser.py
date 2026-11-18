import argparse
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import time
import uuid
from urllib.request import urlopen

ROOT=Path(__file__).resolve().parents[3]
parser=argparse.ArgumentParser()
parser.add_argument('scope')
parser.add_argument('--phone-save',action='store_true')
parser.add_argument('--paired',choices=['desktop','phone'])
args=parser.parse_args()
assert re.fullmatch(r'summon-[a-z0-9-]{1,33}',args.scope)
assert not (args.phone_save and args.paired)
phone=args.phone_save or args.paired=='phone'
FIXTURE=ROOT/'.local/pal/performance'/('dense-battle-'+args.scope)
OUT=FIXTURE/args.paired if args.paired else FIXTURE
if args.paired:
    OUT.mkdir()
BASE=(FIXTURE/'url').read_text().strip()
PROFILE=OUT/'profile'
assert not (OUT/'result.json').exists()
PROFILE.mkdir()
ENV=dict(os.environ,AGENT_BROWSER_PROFILE=str(PROFILE),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pws'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pws'],cwd=ROOT,env=ENV,text=True).strip()+'-'+uuid.uuid4().hex[:6]
REPORT={'scope':args.scope,'phoneSave':args.phone_save,'paired':args.paired,'passed':False}


def process(*argv,**kwargs):
    return subprocess.Popen(argv,cwd=ROOT,env=ENV,text=True,**kwargs)


def browser(*argv):
    result=subprocess.run(['agent-browser','--session',SESSION,*map(str,argv),'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=35)
    assert result.returncode==0,result.stdout+result.stderr
    raw=json.loads(result.stdout)
    assert raw['success'],raw
    return raw['data']


def js(source):
    return browser('eval',source)['result']


def api(path,data=None):
    options='' if data is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(data)+')}'
    response=js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
    assert 200<=response['status']<300,response
    return response['body']


def room():
    return api('/state').get('room')


def view():
    return room()['game']['data']


def command(kind):
    current=room()
    return api('/rooms/'+current['id'],{'type':kind,'version':current['version'],'nonce':uuid.uuid4().hex})


def trace():
    observer.send_signal(signal.SIGUSR2)
    return json.loads(observer.stdout.readline())['events']


def wait(check,label,seconds=55):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if result:=check():
            return result
        time.sleep(.06)
    raise AssertionError(label)


def touch(x,y):
    subprocess.run(['node','games/pal/tests/screen-touch.mjs',endpoint,URL,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=ENV,check=True,timeout=10)


def tap_canvas(x,y):
    left,top,w,h=js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x,r.y,r.width,r.height]})()')
    assert (w,h)==(320,200),[w,h]
    touch(left+x,top+y)


def tap_control(selector):
    xy=js('(()=>{const el=document.querySelector('+json.dumps(selector)+');el.scrollIntoView({block:"center",behavior:"instant"});const r=el.getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
    touch(*xy)


def glyph_selected():
    d=view()
    return any(n[0]==2 and n[5]==ord('风') and n[6:8]==[209,72] and 249<=n[8]<=254 for n in d['raster']['nodes'])


def collapse(values):
    return [value for i,value in enumerate(values) if not i or value!=values[i-1]]


def load(path):
    return json.loads(path.read_bytes())


def q95(rows):
    return sorted(rows)[round((len(rows)-1)*.95)] if rows else None


def validate():
    sample,wire=REPORT['capture'],REPORT['wire']
    live=[r for r in wire if r['type'] in ('game','state') and r.get('spell')==315]
    winds=[(row,draw[1]) for row in live for draw in (row.get('effects') or []) if draw[0]==44]
    assert winds,'original wind fire phase never arrived'
    first,last=winds[0][0],winds[-1][0]
    sprites=collapse([draw[1] for row in live for draw in (row.get('sprites') or []) if draw[0]==14])
    pixels=[row for row in sample['samples'] if row['ready'] and row['focused'] and row.get('phase')=='battle']
    visible_sprites=collapse([r['sprite']['id'] for r in pixels if r['sprite']['id'] is not None])
    visible=collapse([r['effect']['id'] for r in pixels if r['effect']['id'] is not None])
    packets=collapse([value for row,value in winds])
    expected=collapse([row['effects'][0][1] for row in ORIGINAL['frames'] if row['effects']])
    native_fire=[row for row in ORIGINAL['frames'] if row['effects']]
    assert len(expected)==48 and expected==list(range(16))*3
    held=[r for r in wire if r['type']=='effect-response-held']
    released=[r for r in wire if r['type']=='effect-response-released']
    began=sample['wall']+sample['start']
    if args.phone_save:
        assert len(held)>=2 and len(held)==len(released)
    else:
        assert len(held)==len(released)==2
    for row in held:
        assert row['status']==200
        assert any(r['url']==row['url'] and r['at']>=row['at']+599 for r in released)
    assert all(row['at']<began for row in held[:2]+released[:2])
    REPORT['observed']={'nativeFireFrames':len(native_fire),'nativeFireSpanMS':native_fire[-1]['clock']-native_fire[0]['clock'],
                        'publicFire':packets,'visibleFire':visible,'publicSprites':sprites,'visibleSprites':visible_sprites,
                        'wireBytesP95':q95([row['bytes'] for row,value in winds if row['type']=='game']),
                        'fireSpanMS':last['at']-first['at']}
    assert all(x['partyMP']==[726] for x,value in winds)
    assert first['enemies']==[399] and last['enemies']==[399]
    assert any(row['enemies']==[0] and row['at']>last['at'] for row in live)
    first_sprite=next(row for row in live if row.get('sprites'))
    assert first_sprite['at'] < first['at']
    first_visible_fire=next((row['at'] for row in pixels if row['effect']['id'] is not None),None)
    first_visible_sprite=next((row['at'] for row in pixels if row['sprite']['id']==0),None)
    if args.phone_save:
        saved,resumed=[load(OUT/('summon-'+label+'.json')) for label in ('saved','resumed')]
        assert saved['paused'] and saved['deadline']==0 and 0<=saved['tickLeftMS']<=40
        assert not resumed['paused'] and resumed['deadline']-resumed['now']==saved['tickLeftMS']
        assert resumed['wall']-saved['wall']>=3000
        a,b=saved['public'],resumed['public']
        for key in ('raster','battle','battleAssets','cash','party','paletteEffect'):
            assert a[key]==b[key],key
        assert a['epoch']<b['epoch'] and a['party'][0]['mp']==726 and a['cash']==0
        assert saved['remainingMS']==resumed['remainingMS'] and 0<saved['remainingMS']<=40
        frozen=[n[7] for n in a['raster']['nodes'] if n[0]==1 and n[5:7]==[8,44]]
        assert frozen and 6<=frozen[0]<=15,frozen
        assert [row['type'] for row in wire if row['type'].startswith('save-')]==['save-trigger','save-dispatched']
        assert any(row['save'] and row['trusted'] and row['pointer']=='touch' for row in sample['inputs'])
        REPORT['save']={'frame':frozen[0],'tickLeftMS':saved['tickLeftMS'],'offlineMS':resumed['wall']-saved['wall'],'exactRaster':True}
    else:
        assert abs(last['at']-first['at']-REPORT['observed']['nativeFireSpanMS'])<=200
    assert packets==expected,REPORT['observed']
    assert visible==expected and first_visible_fire is not None and first_visible_sprite is not None and first_visible_sprite<first_visible_fire,REPORT['observed']
    assert REPORT['observed']['wireBytesP95']<=16384


proxy=observer=device=None
try:
    proxy=process('node','games/pal/tests/performance-proxy.mjs',BASE,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
    URL=json.loads(proxy.stdout.readline())['url']
    browser('open',URL)
    browser('set','viewport',1280,810)
    api('/register',{'name':'风神验收'+uuid.uuid4().hex[:7],'password':'summon-wind-stage-123','invite':'test-invite'})
    api('/rooms',{'kind':'pal','name':'原版风神演出','capacity':1})
    command('ready')
    endpoint=browser('get','cdp-url')['cdpUrl']
    files=load(ROOT/'games/pal/assets/visual-release.json')['catalogs']['997f43b90cbcb6432137d46dfa29ad5c5cfcfbf8a2d151aff933a3a9cfae2c1c']['files']
    names=['atlases/f/14','atlases/fire/44']
    cdn=[files[name+'.json']['url'] for name in names]
    effects=[files[name+'.'+suffix]['url'] for name in names for suffix in ('json','png')]
    REPORT['publishedEffects']=effects
    if args.phone_save:
        ENV['PAL_OBSERVE_SUMMON_SAVE']='1'
    observer=process('node','games/pal/tests/performance-observer.mjs',endpoint,URL,'summon-battle-page.js','600',*cdn,stdout=subprocess.PIPE)
    assert observer.stdout.readline().strip()=='ready'
    if args.paired:
        with (FIXTURE/'prepare-start.lock').open('a+') as lock:
            fcntl.flock(lock,fcntl.LOCK_EX)
            command('start')
            shutil.copyfile(FIXTURE/'start.json',OUT/'start.json')
    else:
        command('start')
    side=[args.paired] if args.paired else []
    subprocess.run(['node','games/pal/tests/summon-wind-original.mjs',args.scope,*side],cwd=ROOT,env=dict(ENV,PAL_ORIGINAL_PIXELS='1',PAL_NATIVE_WASM='.local/pal/summon-wind/native/rules.wasm'),check=True,timeout=15)
    ORIGINAL=load(OUT/'original.json')
    browser('reload')
    browser('wait','[data-pal-board][data-pal-ready=true]')
    original_room=wait(lambda: (r if not r.get('loading') and r['game']['data'].get('battle',{}).get('state')==1 else None) if (r:=room()) else None,'first genuine original command menu')
    original_party=original_room['game']['data']['party']
    assert original_party[0]['role']==3 and original_party[0]['mp']==790
    REPORT['roomID']=original_room['id']
    REPORT['matchID']=original_room['matchId']
    REPORT['plan']=original_room['game']['data']['battleAssets']
    assert [6,14] in REPORT['plan'] and [8,44] in REPORT['plan']
    seen=js('performance.getEntriesByType("resource").map(row=>row.name)')
    assert all(seen.count(url)==1 for url in effects),effects
    REPORT['preloaded']=effects
    if phone:
        browser('set','viewport',320,820)
        device=process('node','scripts/browser_device.mjs',endpoint,URL,'320','820','--keep',stdout=subprocess.PIPE)
        assert device.stdout.readline().strip()=='ready'
    browser('click','[data-fullscreen-enter]')
    browser('wait','[data-room-fullscreen]')
    if phone:
        browser('wait','--fn','document.querySelector("[data-pal-board] canvas")?.getBoundingClientRect().width===320')
    geometry=js('(()=>{const c=document.querySelector("[data-pal-board] canvas"),r=c.getBoundingClientRect();return [r.width,r.height,c.width,c.height,document.querySelectorAll("[data-pal-controls] button").length]})()')
    assert geometry[2:]==[320,200,0] and geometry[0]/geometry[1]==1.6
    if phone:
        assert geometry[:2]==[320,200]
    REPORT['geometry']=geometry
    manifest=load(FIXTURE/'web-dist/game-load-manifest.json')
    paths=[manifest['home']['files'][0]['url'],manifest['games']['pal'][0]['url']]
    loaded=js('performance.getEntriesByType("resource").map(r=>new URL(r.name).pathname)')
    assert all(path in loaded for path in paths)
    REPORT['bundles']=[{'path':name,'sha256':hashlib.sha256((FIXTURE/'web-dist'/name.lstrip('/')).read_bytes()).hexdigest()} for name in paths]
    samples=lambda key:list(dict.fromkeys(tuple(draw) for row in ORIGINAL['frames'] for draw in row[key]))
    REPORT['witnesses']=js('palSummonBattle.prepare('+json.dumps(samples('sprites'))+','+json.dumps(samples('effects'))+')')
    switch=time.time_ns()//1000000
    if phone:
        proxy.stdin.write('{"delayMS":100}\n')
        proxy.stdin.flush()
        assert json.loads(proxy.stdout.readline())['delayMS']==100
        REPORT['rtt']=wait(lambda:next((row for row in reversed(trace()) if (row.get('connectionSampledAt') or 0)>=switch and 150<=row.get('connectionRTTMS',0)<=300),None),'fresh delayed WebSocket RTT')
    elif args.paired:
        REPORT['rtt']=wait(lambda:next((row for row in reversed(trace()) if (row.get('connectionSampledAt') or 0)>=switch and 0<=row.get('connectionRTTMS',1000)<50),None),'fresh desktop WebSocket RTT')
    if phone:
        tap_canvas(12,162)
        wait(lambda:view()['battle']['selected']==1,'original magic icon selection')
        tap_canvas(12,162)
        wait(lambda:view()['battle']['menu']==1,'original mobile spell menu')
        tap_canvas(215,79)
        wait(glyph_selected,'mobile wind name selection')
    else:
        browser('press','ArrowLeft')
        wait(lambda:view()['battle']['selected']==1,'original magic icon selection')
        browser('press','Enter')
        wait(lambda:view()['battle']['menu']==1,'original desktop spell menu')
        for key in ('ArrowDown','ArrowRight','ArrowRight'):
            browser('press',key)
            time.sleep(.08)
        wait(glyph_selected,'desktop wind name selection')
    browser('screenshot',OUT/'spell-selected.png')
    if args.paired:
        ready=OUT/'ready.tmp'
        ready.write_text(json.dumps({'roomID':REPORT['roomID'],'matchID':REPORT['matchID'],'rttMS':REPORT['rtt']['connectionRTTMS']}))
        ready.rename(OUT/'ready.json')
        wait(lambda:(FIXTURE/'cast-at.json').exists(),'peer did not reach original Wind God command',160)
        target=load(FIXTURE/'cast-at.json')['wallMS']/1000-(.1 if phone else 0)
        js('palSummonBattle.start();true')
        time.sleep(max(0,target-time.time()))
    else:
        js('palSummonBattle.start();true')
    if args.phone_save:
        tap_canvas(215,79)
        browser('wait','[data-resume-save]')
        state=api('/state')
        assert state.get('room') is None and any(row['id']==original_room['id'] for row in state['saves'])
        time.sleep(3.25)
        tap_control('[data-resume-save]')
        wait(lambda:room() and room().get('matchId')==original_room['matchId'],'private same-match continuation')
    elif phone:
        tap_canvas(215,79)
    else:
        browser('press','Enter')
    finishing=wait(lambda:(d if d['cash']==3 else None) if (d:=view()) else None,'normal original wind victory',40)
    assert finishing['party'][0]['mp']==ORIGINAL['frames'][-1]['mp']==758
    REPORT['capture']=js('palSummonBattle.stop()')
    REPORT['wire']=trace()
    with urlopen(BASE+'/pal-performance/metrics',timeout=5) as stream:
        REPORT['timer']=json.load(stream)
    REPORT['reward']=finishing['cash']
    browser('screenshot',OUT/'reward.png')
    validate()
    REPORT['passed']=True
    print(json.dumps(REPORT['observed'],ensure_ascii=False),flush=True)
except Exception as error:
    REPORT['error']=str(error)
    if observer and observer.poll() is None:
        REPORT.setdefault('wire',trace())
    if 'capture' not in REPORT:
        try:
            REPORT['capture']=js('palSummonBattle.stop()')
            browser('screenshot',OUT/'failure.png')
        except Exception:
            pass
    raise
finally:
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    for proc in (device,observer,proxy):
        if proc:
            proc.terminate()
            proc.communicate(timeout=6)
    try:
        browser('close')
    finally:
        shutil.rmtree(PROFILE)
        if not args.paired:
            (OUT/'stop').touch()
