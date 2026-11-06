"""Repeat original ordinary publication with actual resources/Root and scoreclock."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_music_clock as common

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/score-clock-ordinary'


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first=json.loads((BASE/'test-first.json').read_text())
    assert first['test_first']and not first['production_exists']
    for path,digest in first['files'].items():assert sha(ROOT/path)==digest,('frozen test/oracle changed',path)
    assert 'undefined: pcmcommands.ArbitrationField'in(BASE/'missing-api-red.jsonl').read_text()


def sources():
    result=common.sources()
    for p in[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/score-clock-ordinary-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-ordinary-observer.c',ROOT/'games/yuyuhakusho/audio/scoreclock/ORDINARY.md',ROOT/'games/yuyuhakusho/engine/testdata/pcm-arbitration-private.json.gz']:
        result[str(p.relative_to(ROOT))]=sha(p)
    return dict(sorted(result.items()))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');args=p.parse_args();frozen()
    if args.audit:
        r=json.loads((BASE/'validation.json').read_text());assert r['pass']
        for key in['sources','artifacts']:
            for path,digest in r[key].items():assert sha(ROOT/path)==digest,('frozen binding changed',path)
        print(json.dumps({'pass':True,'sources':len(r['sources']),'artifacts':len(r['artifacts'])}));return
    assert args.mode and args.out
    out=args.out.resolve();assert out.is_relative_to(ROOT)and out!=BASE;out.mkdir(parents=True,exist_ok=True)
    log=out/(args.mode+('.txt'if args.mode=='vet'else'.jsonl'));record=out/(args.mode+'-command.json');assert not log.exists()and not record.exists(),'preserve previous checks'
    env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2')
    go=shutil.which('go');assert go
    command=[go,'test','-json','-count=1','-timeout=5m']
    if args.mode=='native':command.append('-race')
    elif args.mode=='wasm':
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
    command.append('./games/yuyuhakusho/audio/scoreclock')
    if args.mode=='vet':command=[go,'vet']+['./games/yuyuhakusho/audio/'+x for x in['scoreclock','pcmcommands','mixed','rootcontrol','resourcebridge']]
    before=sources();start=time.monotonic()
    with log.open('w')as stream:result=subprocess.run(command,cwd=ROOT,env=env,stdout=stream,stderr=subprocess.STDOUT)
    assert before==sources(),'source changed during verification'
    passed=[];skipped=[]
    if args.mode!='vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                r=json.loads(line)
                if'Test'in r and r.get('Action')=='pass':passed.append(r['Test'])
                if'Test'in r and r.get('Action')=='skip':skipped.append(r['Test'])
    r={'schema':1,'mode':args.mode,'pass':result.returncode==0 and not skipped,'exit_code':result.returncode,'command':command,'elapsed_seconds':time.monotonic()-start,'sources':before,'environment':{k:env[k]for k in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'passed_tests':passed,'skipped_tests':skipped,'log_sha256':sha(log),'scope':'All122 ordinary resources/22normal arbitration shapes with actual PCM/Root dispatch plus scoreclock regressions; full world audio remains incomplete'}
    record.write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({k:r[k]for k in['mode','pass','exit_code','elapsed_seconds','log_sha256']}))
    if not r['pass']:print(log.read_text()[-7000:]);raise SystemExit(result.returncode or 1)


if __name__=='__main__':main()
