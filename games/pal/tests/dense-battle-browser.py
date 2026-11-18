import argparse
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

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser()
parser.add_argument('scope')
parser.add_argument('--phone', action='store_true')
parser.add_argument('--delay', type=int, choices=[0,100], default=0)
parser.add_argument('--asset-delay', type=int, choices=[0,600], default=0)
parser.add_argument('--require-prepared', action='store_true')
args = parser.parse_args()
assert re.fullmatch(r'[a-z][a-z0-9-]{0,39}',args.scope)
OUT = ROOT/'.local/pal/performance'/('dense-battle-'+args.scope)
BASE = (OUT/'url').read_text().strip()
REPORT = {'passed':False,'measured':False,'scope':args.scope,'phone':args.phone,'proxyDelayMS':args.delay,
          'assetDelayMS':args.asset_delay,
          'budgets':{'visibleRate':24,'pixelGapP95':80,'rafGapP95':35,'wireBytesP95':16384,'ruleMSP95':20}}
PROFILE = OUT/'profile'
assert not (OUT/'result.json').exists()
PROFILE.mkdir()
ENV = dict(os.environ,AGENT_BROWSER_PROFILE=str(PROFILE),AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pdb'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(ENV['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
SESSION = subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pdb'],cwd=ROOT,env=ENV,text=True).strip()+'-'+uuid.uuid4().hex[:6]


def process(*argv, **options):
    return subprocess.Popen(argv,cwd=ROOT,env=ENV,text=True,**options)


def browser(*argv):
    ret = subprocess.run(['agent-browser','--session',SESSION,*map(str,argv),'--json'],cwd=ROOT,env=ENV,text=True,capture_output=True,timeout=35)
    assert ret.returncode==0, ret.stdout+ret.stderr
    value = json.loads(ret.stdout)
    assert value['success'], value
    return value['data']


def js(source):
    return browser('eval',source)['result']


def api(path, body=None):
    init = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
    result = js('(async()=>{const r=await fetch("/api'+path+'"'+init+');return {status:r.status,body:await r.json()}})()')
    assert 200<=result['status']<300, result
    return result['body']


def room():
    return api('/state')['room']


def command(kind):
    s = room()
    return api('/rooms/'+s['id'],{'type':kind,'version':s['version'],'nonce':uuid.uuid4().hex})


def wait(check, message, seconds=45):
    end = time.monotonic()+seconds
    while time.monotonic()<end:
        if value:=check():
            return value
        time.sleep(.1)
    raise AssertionError(message)


def select(cooperative):
    snapshot = room()
    b = snapshot['game']['data'].get('battle',{})
    return not snapshot.get('loading') and b.get('state')==1 and b.get('current')==0 and b.get('selected')==(2 if cooperative else 0)


def trace():
    observer.send_signal(signal.SIGUSR2)
    return json.loads(observer.stdout.readline())['events']


def native_click():
    if not args.phone:
        browser('press','Enter')
        return
    rect = js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.left,r.top,r.width,r.height]})()')
    x,y,w,h = rect
    assert (w,h)==(320,200), rect
    x+=66
    y+=162
    subprocess.run(['node','games/pal/tests/screen-touch.mjs',endpoint,URL,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=ENV,check=True,timeout=15)


def percentile(rows, q):
    return sorted(rows)[int((len(rows)-1)*q+.5)] if rows else None


def diffs(rows):
    return [round(b-a,3) for a,b in zip(rows,rows[1:])]


def collapse(rows):
    return [row for i,row in enumerate(rows) if i==0 or rows[i-1]['hash']!=row['hash']]


def summarize(sample, events, timer):
    first_input = next(row for row in sample['controls'] if row['type'] in ('keydown','pointerup'))
    assert first_input['trusted'] and (first_input.get('pointer')=='touch' if args.phone else first_input.get('code')=='Enter')
    sent = next(row for row in events if row['type']=='sent' and row['at']>=sample['wall']+sample['start'] and row['action'].get('keys')==2)
    ack = next(row for row in events if row['type']=='ack' and row['nonce']==sent['nonce'])
    assert ack['status']==200
    packets = [row for row in events if row['type']=='game' and row['at']>=sent['at'] and row.get('battlePhase')==1]
    first = packets[0]
    assert first['cash']==0 and first['enemies']==[399]
    original = [row for row in ORIGINAL['frames'] if row['phase']==1 and row['kind']==4 and row['waitMS']==40]
    assert [row['i'] for row in original]==list(range(2,71))
    reference = collapse(original)
    duration = (original[-1]['clock']-original[0]['clock'])/1000
    hashes = set(row['hash'] for row in reference)
    eligible = [row for row in sample['samples'] if row['hash'] in hashes and sample['wall']+row['at']>=sent['at']]
    drawn = collapse(eligible)
    index = {row['hash']:i for i,row in enumerate(reference)}
    order = [index[row['hash']] for row in drawn]
    fx = [row['effects'][0][1] for row in reference if row['effects']]
    pix = [r for r in sample['samples'] if first['at']-30<=r['at']+sample['wall']<=first['at']+duration*1000+120]
    assert pix and all(r['focused'] for r in pix), 'measurement requires actual foreground frames'
    shown = [r for r in pix if r['at']+sample['wall']>=first['at']+20]
    actual = collapse(shown)
    observed = [row['effect'] for row in pix if row['effect'] is not None]
    observed = [value for i,value in enumerate(observed) if i==0 or value!=observed[i-1]]
    visual_gaps = diffs([r['at'] for r in actual])
    ticks = [r for r in timer if first['controlAt']<=r['nowMS']<=first['controlAt']+duration*1000]
    contiguous = [r for r in packets if first['controlAt']<=r['controlAt']<=first['controlAt']+duration*1000]
    present_effects = [r['effects'] for r in contiguous if r.get('effects')]
    reference_effects = [r['effects'] for r in original if r['effects']]
    values = dict(nativeFrames=len(original), nativePixelFrames=len(reference),
                  fullCanvasCMatches=len(set(order)),
                  displayedChanges=len(actual)-1, visibleRate=round((len(actual)-1)/duration,3),
                  pixelGapP95=percentile(visual_gaps,.95), longestPixelGapMS=max(visual_gaps),
                  rafGapP95=percentile(diffs([r['at'] for r in pix]),.95),
                  captureMSP95=percentile([r['captureMS'] for r in pix],.95),
                  wireBytesP95=percentile([r['bytes'] for r in contiguous],.95),
                  wireFrames=len(contiguous), ruleFrames=len(ticks),
                  ruleMSP95=percentile([r['elapsedMS'] for r in ticks],.95),
                  ackMS=ack['ackMS'], originalEffectPixelFrames=len(fx),
                  observedEffectPixelFrames=len(set(observed)), originalEffects=fx, observedEffects=observed,
                  originalEffectPackets=reference_effects, observedEffectPackets=present_effects,
                  firstAuthorityAt=first['at'], durationSeconds=duration)
    b = REPORT['budgets']
    REPORT.update(values=values,measured=True)
    REPORT['passed'] = bool(values['visibleRate']>=b['visibleRate'] and values['pixelGapP95']<=b['pixelGapP95']
                            and values['rafGapP95']<=b['rafGapP95'] and values['wireBytesP95']<=b['wireBytesP95']
                            and values['ruleMSP95']<=b['ruleMSP95']
                            and observed==fx
                            and present_effects==reference_effects)


proxy = observer = device = None
try:
    proxy = process('node','games/pal/tests/performance-proxy.mjs',BASE,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
    URL = json.loads(proxy.stdout.readline())['url']
    browser('open',URL)
    browser('set','viewport',1280,810)
    api('/register',{'name':'原版法术采样'+uuid.uuid4().hex[:7],'password':'dense-battle-case-123','invite':'test-invite'})
    api('/rooms',{'kind':'pal','name':'原版合击性能','capacity':1})
    command('ready')
    publication = json.loads((ROOT/'games/pal/assets/visual-release.json').read_bytes())['catalogs']['997f43b90cbcb6432137d46dfa29ad5c5cfcfbf8a2d151aff933a3a9cfae2c1c']['files']
    effect_urls = [publication['atlases/fire/50.'+ext]['url'] for ext in ('json','png')]
    REPORT['effectURLs'] = effect_urls
    endpoint = browser('get','cdp-url')['cdpUrl']
    asset_delay = [str(args.asset_delay),effect_urls[0]] if args.asset_delay else []
    observer = process('node','games/pal/tests/performance-observer.mjs',endpoint,URL,'dense-battle-page.js',*asset_delay,stdout=subprocess.PIPE)
    assert observer.stdout.readline().strip()=='ready'
    command('start')
    subprocess.run(['node','games/pal/tests/dense-battle-original.mjs',args.scope],env=dict(ENV,PAL_ORIGINAL_PIXELS='1',PAL_NATIVE_WASM='.local/pal/screen-input/battle-current/rules.wasm'),cwd=ROOT,check=True,timeout=20)
    ORIGINAL = json.loads((OUT/'original.json').read_bytes())
    browser('reload')
    wait(lambda:select(False),'original solo battle did not reach first command')
    browser('wait','[data-pal-ready=true]')
    if args.phone:
        browser('set','viewport',320,820)
        device = process('node','scripts/browser_device.mjs',endpoint,URL,'320','820','--keep',stdout=subprocess.PIPE)
        assert device.stdout.readline().strip()=='ready'
    browser('click','[data-fullscreen-enter]')
    browser('wait','[data-room-fullscreen]')
    geometry_probe = '(()=>{const c=document.querySelector("[data-pal-board] canvas"),r=c.getBoundingClientRect();return [r.width,r.height,c.width,c.height,document.querySelectorAll("[data-pal-controls] button").length]})()'
    if args.phone:
        browser('wait','--fn',geometry_probe+'[0]===320 && '+geometry_probe+'[1]===200')
    geometry = js(geometry_probe)
    assert geometry[2:]==[320,200,0] and geometry[0]/geometry[1]==1.6 and (not args.phone or geometry[:2]==[320,200]), geometry
    REPORT['geometry']=geometry
    manifest = json.loads((OUT/'web-dist/game-load-manifest.json').read_bytes())
    entries = [manifest['home']['files'][0]['url'],manifest['games']['pal'][0]['url']]
    seen = js('performance.getEntriesByType("resource").map(r=>new URL(r.name).pathname)')
    assert all(url in seen for url in entries)
    REPORT['bundles'] = [{'url':url,'sha256':hashlib.sha256((OUT/'web-dist'/url.lstrip('/')).read_bytes()).hexdigest()} for url in entries]
    original_effect = list(dict.fromkeys(tuple(row['effects'][0]) for row in ORIGINAL['frames'] if row['effects']))
    REPORT['witnesses']=js('palDenseBattle.prepareEffect('+json.dumps(original_effect)+')')
    if args.require_prepared:
        data = room()['game']['data']
        REPORT['plan']=data['battleAssets']
        assert [8,50] in REPORT['plan'] and [8,54] not in REPORT['plan'] and len(REPORT['plan'])<=19
        REPORT['beforeInputEffects']=js('performance.getEntriesByType("resource").filter(r=>'+json.dumps(effect_urls)+'.includes(r.name)).map(r=>r.name)')
        assert sorted(REPORT['beforeInputEffects'])==sorted(effect_urls), REPORT['beforeInputEffects']
    browser('press','ArrowRight')
    wait(lambda:select(True),'original cooperative attack icon not selected')
    proxy.stdin.write(json.dumps({'delayMS':args.delay})+'\n')
    proxy.stdin.flush()
    assert json.loads(proxy.stdout.readline())['delayMS']==args.delay
    clock = time.time_ns()//1000000
    rtt = wait(lambda:next((e for e in reversed(trace()) if e.get('connectionSampledAt',0)>=clock and (150<=e.get('connectionRTTMS',0)<=300 if args.delay else e.get('connectionRTTMS',1000)<50)),None),'fresh WebSocket ping/pong sample not available')
    REPORT['rtt']=rtt
    browser('screenshot',OUT/'before-cast.png')
    js('palDenseBattle.begin(document.querySelector("[data-pal-board] canvas"));true')
    native_click()
    time.sleep(3.8)
    capture = js('palDenseBattle.end()')
    REPORT['capture']=capture
    REPORT['wire']=trace()
    if args.asset_delay:
        holds = [row for row in REPORT['wire'] if row['type'].startswith('effect-response-')]
        assert [row['type'] for row in holds]==['effect-response-held','effect-response-released'] and holds[0]['status']==200, holds
        assert holds[1]['at']-holds[0]['at']>=args.asset_delay-1, holds
    REPORT['effectRequests'] = js('performance.getEntriesByType("resource").filter(r=>'+json.dumps(effect_urls)+'.includes(r.name)).map(r=>({url:r.name,start:r.startTime,end:r.responseEnd,duration:r.duration}))')
    if args.require_prepared:
        assert sorted(row['url'] for row in REPORT['effectRequests'])==sorted(effect_urls)
        assert all(row['end']<=capture['start'] for row in REPORT['effectRequests'])
    with urlopen(BASE+'/pal-performance/metrics',timeout=10) as r:
        REPORT['timer']=json.load(r)
    wait(lambda:room()['game']['data']['cash']==3,'original cooperative battle did not finish with three cash')
    REPORT['cash']=3
    browser('screenshot',OUT/'after-cast.png')
    summarize(capture,REPORT['wire'],REPORT['timer'])
    print(json.dumps(REPORT['values'],ensure_ascii=False),flush=True)
except Exception as exc:
    REPORT['error']=str(exc)
    raise
finally:
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    if device:
        device.terminate()
        device.wait(timeout=6)
    if observer:
        observer.terminate()
        observer.communicate(timeout=6)
    if proxy:
        proxy.terminate()
        proxy.communicate(timeout=6)
    try:
        browser('close')
    finally:
        shutil.rmtree(PROFILE)
        (OUT/'stop').touch()
raise SystemExit(int(not REPORT['passed']))
