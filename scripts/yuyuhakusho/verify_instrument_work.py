"""Verify canonical instrument sound/work references against original trajectories."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/audio-instrument-work'


def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()


def frozen():
    first=json.loads((BASE/'test-first.json').read_text());assert first['test_first']and not first['production_exists']
    for name,digest in first['files'].items():
        p=ROOT/name
        if p.name in['catalog.go','driver.go','state.go']:p=BASE/(p.name+'.before-instrument-binding.txt')
        assert sha(p)==digest,('pre-implementation input changed',name)
    bridge=json.loads((BASE/'resourcebridge-test-first.json').read_text());assert bridge['test_first']
    for name,digest in bridge['files'].items():
        p=ROOT/name
        if p.name=='library.go':p=BASE/'resourcebridge-library.before-instrument-binding.txt'
        assert sha(p)==digest,('resource bridge test-first changed',name)
    assert 'undefined: score.BindInstrumentWork'in(BASE/'missing-api-red.jsonl').read_text()
    assert 'undefined: resourcebridge.DecodeWithInstrumentWork'in(BASE/'resourcebridge-missing-api-red.jsonl').read_text()


def sources():
    paths=[]
    for package in['score','score/param','resourcebridge','pcmassets','pcmcommands','phaseclock']:
        paths.extend((ROOT/'games/yuyuhakusho/audio'/package).glob('*.go'))
    paths += [Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/export_instrument_work.py',ROOT/'games/yuyuhakusho/audio/score/INSTRUMENT_WORK.md']
    paths += [ROOT/'games/yuyuhakusho/data'/name for name in['audio-score.json.gz','audio-score-instrument-work.json.gz','audio-score-parser-work.json.gz','audio-pcm-payloads.json.gz']]
    return{str(p.relative_to(ROOT)):sha(p)for p in sorted(set(paths))}


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');a=p.parse_args();frozen()
    if a.audit:
        d=json.loads((BASE/'validation.json').read_text());assert d['pass']
        for key in['sources','artifacts']:
            for name,digest in d[key].items():assert sha(ROOT/name)==digest,('frozen binding changed',name)
        print(json.dumps({'pass':True,'sources':len(d['sources']),'artifacts':len(d['artifacts'])}));return
    assert a.mode and a.out;out=a.out.resolve();assert out.is_relative_to(ROOT)and out!=BASE;out.mkdir(parents=True,exist_ok=True)
    log=out/(a.mode+('.txt'if a.mode=='vet'else'.jsonl'));record=out/(a.mode+'-command.json');assert not log.exists()and not record.exists()
    before=sources();env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2');go=shutil.which('go');assert go
    command=[go,'test','-json','-count=1','-timeout=20m','-run','^(TestInstrument.*|TestOriginalContinuousDriver|TestOwnedSnapshotsAndAtomicRejection)$']
    if a.mode=='native':command.append('-race')
    elif a.mode=='wasm':
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
    else:command=[go,'vet']
    command+=['./games/yuyuhakusho/audio/score','./games/yuyuhakusho/audio/resourcebridge'];started=time.monotonic()
    with log.open('w')as stream:result=subprocess.run(command,cwd=ROOT,env=env,stdout=stream,stderr=subprocess.STDOUT)
    assert before==sources(),'source changed during verification';passed=[]
    if a.mode!='vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                row=json.loads(line)
                if row.get('Action')=='pass'and'Test'in row:passed.append(row['Package']+':'+row['Test'])
    report={'schema':1,'mode':a.mode,'pass':result.returncode==0,'exit_code':result.returncode,'command':command,'elapsed_seconds':time.monotonic()-started,'environment':{k:env[k]for k in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'sources':before,'passed_tests':passed,'log_sha256':sha(log),'scope':'Instrument binding plus full legacy42 and selected bound original trajectories, canonical snapshot/identity tests; no whole audio timing claim'}
    record.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in['mode','pass','exit_code','elapsed_seconds','log_sha256']}))
    if result.returncode:print(log.read_text()[-7000:]);raise SystemExit(result.returncode)


if __name__=='__main__':main()
