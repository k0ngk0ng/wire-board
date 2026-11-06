"""Verify outer sound IRQ/catchup and joint canonical audio state."""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import verify_driver_control_clock as common

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/sound-irq-runtime'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def frozen():
    first=json.loads((BASE/'test-first.json').read_text());fix=json.loads((BASE/'test-adapter-correction.json').read_text());assert first['test_first']and not first['production_exists']
    for path,digest in first['files'].items():
        p=ROOT/fix['original_test_archive']if path==fix['current_test']else ROOT/path
        assert sha(p)==digest,('frozen input changed',path)
    assert sha(ROOT/fix['current_test'])==fix['current_test_sha256']
    red=(BASE/'missing-api-red.jsonl').read_text();assert 'no required module provides package'in red and 'undefined: SoundSchedulerHooks'in red
def sources():
    result=common.sources()
    paths=[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/score-clock-sound-irq-capture.py',ROOT/'scripts/yuyuhakusho/sound-irq-observer.c',ROOT/'scripts/yuyuhakusho/sound_irq_cases.py',ROOT/'games/yuyuhakusho/audio/scoreclock/SOUND_IRQ.md',ROOT/'games/yuyuhakusho/audio/session/README.md',ROOT/'docs/research/md-yuyuhakusho/static/animation-records-tail.txt',ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-boundaries.json.gz',ROOT/'games/yuyuhakusho/engine/testdata/audio-timing-boundaries-manifest.json']
    paths+=[BASE/'native-sound-irq-full.json.gz']+list((ROOT/'games/yuyuhakusho/audio/session').glob('*.go'))
    for p in paths:result[str(p.relative_to(ROOT))]=sha(p)
    return dict(sorted(result.items()))
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native-clock','wasm-clock','native-audio','wasm-audio','native-engine','wasm-engine','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');a=p.parse_args();frozen()
    if a.audit:
        r=json.loads((BASE/'validation.json').read_text());assert r['pass']and sources()==r['sources']
        for key in ['sources','artifacts']:
            for path,digest in r[key].items():assert sha(ROOT/path)==digest,('binding changed',path)
        for path,meta in r['compressed_logs'].items():
            raw=gzip.decompress((ROOT/path).read_bytes());assert len(raw)==meta['raw_bytes']and hashlib.sha256(raw).hexdigest()==meta['raw_sha256']
        print(json.dumps({'pass':True,'sources':len(r['sources']),'artifacts':len(r['artifacts'])}));return
    assert a.mode and a.out;out=a.out.resolve();assert out.is_relative_to(ROOT)and out!=BASE;out.mkdir(parents=True,exist_ok=True);record=out/(a.mode+'-command.json');log=out/(a.mode+('.txt'if a.mode=='vet'else'.jsonl'));assert not record.exists()and not log.exists()
    env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2');go=shutil.which('go');assert go;cmd=[go,'test','-json','-count=1','-timeout=12m']
    if a.mode.startswith('native'):cmd+=['-race']
    elif a.mode.startswith('wasm'):
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();cmd+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
    if a.mode.endswith('clock'):cmd+=['./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/rootcontrol']
    elif a.mode.endswith('audio'):cmd+=['-p=1','./games/yuyuhakusho/audio/pcmcommands','./games/yuyuhakusho/audio/chips']
    elif a.mode.endswith('engine'):cmd+=['-run','^Test(SoundScheduler|SoundRequestLive)','./games/yuyuhakusho/engine']
    else:cmd=[go,'vet']+['./games/yuyuhakusho/audio/'+x for x in ['scoreclock','session','rootcontrol','pcmcommands','mixed']]+['./games/yuyuhakusho/engine']
    before,start=sources(),time.monotonic()
    with log.open('w')as f:r=subprocess.run(cmd,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
    assert before==sources(),'source changed during verification';passed=[];skipped=[]
    if a.mode!='vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                x=json.loads(line)
                if'Test'in x and x.get('Action')=='pass':passed.append({'package':x['Package'],'test':x['Test']})
                if'Test'in x and x.get('Action')=='skip':skipped.append(x['Test'])
    report={'schema':1,'mode':a.mode,'pass':r.returncode==0 and not skipped,'exit_code':r.returncode,'command':cmd,'elapsed_seconds':time.monotonic()-start,'sources':before,'passed_tests':passed,'skipped_tests':skipped,'log_sha256':sha(log),'environment':{k:env[k]for k in ['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'scope':'122 original complete sound interrupt/catchup calls, joint canonical state JSON restore and atomic failure; full clock/Root and PCM/chip consumers, engine scheduler only. Given entry clocks, NOT world IRQ/request production or platform full-match audio.'};record.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in ['mode','pass','exit_code','elapsed_seconds']}))
    if not report['pass']:print(log.read_text()[-6500:]);raise SystemExit(r.returncode or 1)
if __name__=='__main__':main()
