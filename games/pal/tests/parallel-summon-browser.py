import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
from urllib.request import urlopen

ROOT=Path(__file__).resolve().parents[3]
scope=sys.argv[1]
assert len(sys.argv)==2 and re.fullmatch(r'summon-[a-z0-9-]{1,33}',scope)
OUT=ROOT/'.local/pal/performance'/('dense-battle-'+scope)
assert not any((OUT/name).exists() for name in ('desktop','phone','cast-at.json','parallel.json','stop'))
BASE=(OUT/'url').read_text().strip()
assert re.fullmatch(r'http://127\.0\.0\.1:[0-9]+',BASE)
SIDES=('desktop','phone')
REPORT={'passed':False,'scope':scope,'windowRequirementMS':2000,'cases':{}}


def load(path):
    return json.loads(path.read_bytes())


def write(path,row):
    with path.open('x') as stream:
        json.dump(row,stream,ensure_ascii=False,indent=2)
        stream.write('\n')


def q95(values):
    return sorted(values)[round((len(values)-1)*.95)]


def overlap(first,second):
    return max(0,min(first[1],second[1])-max(first[0],second[0]))


def analyze(run,reference,timers):
    wires=run['wire']
    frames=[row for row in wires if row['type']=='game' and row.get('spell')==315 and any(draw[0]==44 for draw in (row.get('effects') or []))]
    first,last=frames[0],frames[-1]
    start,end=first['controlAt'],last['controlAt']
    authority=[row['controlAt']+row['clockOffsetMS'] for row in (first,last)]
    sent=[row for row in wires if row['type']=='sent' and row['action'].get('keys')==2 and row['at']>=run['capture']['wall']+run['capture']['start']]
    assert len(sent)==1
    ack=next(row for row in wires if row['type']=='ack' and row['nonce']==sent[0]['nonce'])
    assert ack['status']==200
    public_frames=[row for row in wires if row['type']=='game']
    assert all(row['roomID']==run['roomID'] for row in public_frames)
    assert len({row['serverInstance'] for row in public_frames})==1
    assert all(row['partyMP']==[726] and row['cash']==0 for row in frames)
    assert any(row.get('cash')==3 and row['partyMP']==[758] for row in wires)
    seen_epochs={row['epoch'] for row in frames}
    rule=[row for row in timers if row.get('epoch') in seen_epochs and start<=row['nowMS']<=end]
    assert all(not row['failed'] and row['players']==1 for row in rule)
    assert {(row['epoch'],row['controlAt']) for row in frames}.issubset({(row['epoch'],row['nowMS']) for row in rule})
    capture=run['capture']
    observed=[row for row in capture['samples'] if row['ready'] and row['focused'] and row.get('phase')=='battle' and row.get('effect',{}).get('id') is not None]
    pixel=[capture['wall']+row['at'] for row in (observed[0],observed[-1])]
    sample=[row for row in capture['samples'] if pixel[0]<=capture['wall']+row['at']<=pixel[1]]
    assert all(row['ready'] and row['focused'] for row in sample)
    gaps=[b['at']-a['at'] for a,b in zip(sample,sample[1:])]
    p95=q95(gaps)
    assert p95<=35
    native=[row['clock'] for row in reference['frames'] if row['effects']]
    assert max(native)-min(native)==2360
    assert abs(authority[1]-authority[0]-2360)<=200
    assert run['observed']['publicFire']==run['observed']['visibleFire']==list(range(16))*3
    assert run['observed']['visibleSprites']==[0,1] and run['reward']==3
    assert run['observed']['wireBytesP95']<=16384
    assert run['geometry'][2:]==[320,200,0]
    if run['paired']=='phone':
        assert run['geometry'][:2]==[320,200] and 150<=run['rtt']['connectionRTTMS']<=300
        assert any(row['trusted'] and row.get('pointer')=='touch' for row in capture['inputs'])
    else:
        assert 0<=run['rtt']['connectionRTTMS']<50
        assert any(row['trusted'] and row.get('code')=='Enter' for row in capture['inputs'])
    case={'roomID':run['roomID'],'matchID':run['matchID'],'instance':first['serverInstance'],'rttMS':run['rtt']['connectionRTTMS'],
          'inputAckMS':ack['ackMS'],'authorityWindowMS':authority,'pixelWindowMS':pixel,'nativeSpanMS':2360,
          'visibleFireChanges':len(run['observed']['visibleFire']),'wireFrames':len(frames),'wireBytesP95':run['observed']['wireBytesP95'],
          'rAFP95MS':p95,'ruleP95MS':q95([row['elapsedMS'] for row in rule]),'ruleSamples':len(rule),'epoch':sorted(seen_epochs),'reward':run['reward']}
    return case,[(row['at'],row['at']+row['elapsedMS']) for row in rule]


processes={}
try:
    for side in SIDES:
        with (OUT/(side+'.log')).open('x') as log:
            processes[side]=subprocess.Popen([sys.executable,'-B','games/pal/tests/summon-wind-browser.py',scope,'--paired',side],cwd=ROOT,env=os.environ,stdout=log,stderr=subprocess.STDOUT)
    until=time.monotonic()+180
    while not all((OUT/side/'ready.json').is_file() for side in SIDES):
        assert all(proc.poll() is None for proc in processes.values()),'a participant stopped before the real cast'
        assert time.monotonic()<until,'two live participants did not reach their original menus'
        time.sleep(.1)
    ready={side:load(OUT/side/'ready.json') for side in SIDES}
    assert ready['desktop']['roomID']!=ready['phone']['roomID'] and ready['desktop']['matchID']!=ready['phone']['matchID']
    write(OUT/'cast-at.tmp',{'wallMS':time.time_ns()//1000000+1600})
    (OUT/'cast-at.tmp').rename(OUT/'cast-at.json')
    until=time.monotonic()+100
    while any(proc.poll() is None for proc in processes.values()):
        assert time.monotonic()<until,'wind battle clients did not finish'
        time.sleep(.1)
    REPORT['clientExitCodes']={side:processes[side].returncode for side in SIDES}
    with urlopen(BASE+'/pal-performance/metrics',timeout=5) as stream:
        timers=json.load(stream)
    write(OUT/'paired-timer.json',timers)
    clocks={}
    bundle=None
    for side in SIDES:
        run=load(OUT/side/'result.json')
        reference=load(OUT/side/'original.json')
        assert run['roomID']==ready[side]['roomID']
        assert reference['nativeBuild']==hashlib.sha256((ROOT/'.local/pal/summon-wind/native/rules.wasm').read_bytes()).hexdigest()
        if bundle is None:
            bundle=run['bundles']
        assert run['bundles']==bundle
        REPORT['cases'][side],clocks[side]=analyze(run,reference,timers)
        assert run['passed']
    desk,phone=[REPORT['cases'][side] for side in SIDES]
    assert desk['instance'] and desk['instance']==phone['instance']
    assert not set(desk['epoch']).intersection(phone['epoch'])
    REPORT['authorityOverlapMS']=overlap(desk['authorityWindowMS'],phone['authorityWindowMS'])
    REPORT['visibleOverlapMS']=overlap(desk['pixelWindowMS'],phone['pixelWindowMS'])
    REPORT['parallelCallbackPairs']=sum(overlap(a,b)>0 for a in clocks['desktop'] for b in clocks['phone'])
    assert REPORT['authorityOverlapMS']>=2000 and REPORT['visibleOverlapMS']>=2000
    assert REPORT['parallelCallbackPairs']>0 and all(code==0 for code in REPORT['clientExitCodes'].values())
    REPORT['passed']=True
    print(json.dumps(REPORT,ensure_ascii=False),flush=True)
except Exception as err:
    REPORT['error']=str(err)
    raise
finally:
    for proc in processes.values():
        if proc.poll() is None:
            proc.send_signal(signal.SIGINT)
            try:
                proc.wait(timeout=15)
            except subprocess.TimeoutExpired:
                proc.terminate()
                proc.wait(timeout=7)
    write(OUT/'parallel.json',REPORT)
    (OUT/'stop').touch()
