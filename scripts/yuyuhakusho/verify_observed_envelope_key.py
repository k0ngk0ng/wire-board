"""Verify actual envelope stage connection and original key/gate leafs."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_envelope_clock as common

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/score-observed-envelope'
KEY=ROOT/'games/yuyuhakusho/verification/score-clock-key-gate'


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    for folder in[BASE,KEY]:
        r=json.loads((folder/'test-first.json').read_text());assert r['test_first']and not r['production_exists']
        for path,digest in r['files'].items():assert sha(ROOT/path)==digest,('frozen source changed',path)
    assert 'undefined'in(BASE/'missing-api-red.jsonl').read_text()
    assert 'undefined: scoreclock.KeyRequest'in(KEY/'missing-api-red.jsonl').read_text()


def sources():
    r=common.sources()
    for p in[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/score-clock-key-gate-capture.py',KEY/'native-key-gate-full.json.gz',ROOT/'games/yuyuhakusho/audio/scoreclock/KEY_GATE.md',ROOT/'games/yuyuhakusho/audio/scoreclock/OBSERVED_ENVELOPE.md']:
        r[str(p.relative_to(ROOT))]=sha(p)
    return dict(sorted(r.items()))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');args=p.parse_args();frozen()
    if args.audit:
        r=json.loads((BASE/'validation.json').read_text());assert r['pass']
        for k in['sources','artifacts']:
            for path,digest in r[k].items():assert sha(ROOT/path)==digest,('binding changed',path)
        print(json.dumps({'pass':True,'sources':len(r['sources']),'artifacts':len(r['artifacts'])}));return
    assert args.mode and args.out
    out=args.out.resolve();assert out.is_relative_to(ROOT)and out!=BASE;out.mkdir(parents=True,exist_ok=True);record=out/(args.mode+'-command.json');assert not record.exists()
    env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2')
    go=shutil.which('go');assert go;command=[go,'test','-json','-count=1','-timeout=5m']
    if args.mode=='native':command.append('-race')
    elif args.mode=='wasm':
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
    command.append('./games/yuyuhakusho/audio/scoreclock')
    if args.mode=='vet':command=[go,'vet','./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/score']
    log=out/(args.mode+('.txt'if args.mode=='vet'else'.jsonl'));assert not log.exists();before=sources();start=time.monotonic()
    with log.open('w')as stream:result=subprocess.run(command,cwd=ROOT,env=env,stdout=stream,stderr=subprocess.STDOUT)
    assert before==sources(),'source changed during verification';passed=[];skipped=[]
    if args.mode!='vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                item=json.loads(line)
                if'Test'in item and item.get('Action')=='pass':passed.append(item['Test'])
                if'Test'in item and item.get('Action')=='skip':skipped.append(item['Test'])
    r={'schema':1,'mode':args.mode,'pass':result.returncode==0 and not skipped,'exit_code':result.returncode,'command':command,'elapsed_seconds':time.monotonic()-start,'sources':before,'passed_tests':passed,'skipped_tests':skipped,'log_sha256':sha(log),'environment':{k:env[k]for k in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'scope':'Actual580 score-envelope phases, original288keygate and complete scoreclock regressions; full voice/world timeline incomplete'}
    record.write_text(json.dumps(r,indent=2)+'\n');print(json.dumps({k:r[k]for k in['mode','pass','exit_code','elapsed_seconds']}))
    if not r['pass']:print(log.read_text()[-6500:]);raise SystemExit(result.returncode or 1)


if __name__=='__main__':main()
