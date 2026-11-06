"""Verify frozen envelope metadata/calculator/PSG and resource compatibility."""
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
BASE=ROOT/'games/yuyuhakusho/verification/score-clock-envelope'


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first=json.loads((BASE/'test-first.json').read_text());assert first['test_first']and not first['production_exists']
    for path,digest in first['files'].items():assert sha(ROOT/path)==digest,('frozen test/oracle changed',path)
    assert 'undefined: score.EnvelopePathWork'in(BASE/'missing-api-red.jsonl').read_text()


def sources():
    result=common.sources()
    for p in[Path(__file__).resolve(),ROOT/'games/yuyuhakusho/audio/scoreclock/ENVELOPE.md',ROOT/'games/yuyuhakusho/data/audio-score-envelope-work.json.gz']:
        result[str(p.relative_to(ROOT))]=sha(p)
    for p in(ROOT/'games/yuyuhakusho/verification/score-envelope-source').glob('*.gz'):result[str(p.relative_to(ROOT))]=sha(p)
    return dict(sorted(result.items()))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');args=p.parse_args();frozen()
    if args.audit:
        r=json.loads((BASE/'validation.json').read_text());assert r['pass']
        for key in['sources','artifacts']:
            for path,digest in r[key].items():assert sha(ROOT/path)==digest,('frozen binding changed',path)
        print(json.dumps({'pass':True,'sources':len(r['sources']),'artifacts':len(r['artifacts'])}));return
    assert args.mode and args.out
    out=args.out.resolve();assert out.is_relative_to(ROOT)and out!=BASE;out.mkdir(parents=True,exist_ok=True);record=out/(args.mode+'-command.json');assert not record.exists()
    env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2')
    go=shutil.which('go');assert go
    flags=[go,'test','-json','-count=1','-timeout=5m']
    if args.mode=='native':flags.append('-race')
    elif args.mode=='wasm':
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();flags+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
    checks=[('clock-resource',flags+['./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/resourcebridge'])]
    if args.mode=='vet':checks=[('vet',[go,'vet']+['./games/yuyuhakusho/audio/'+x for x in['scoreclock','score','resourcebridge']])]
    before=sources();start=time.monotonic();results=[]
    for label,command in checks:
        log=out/(args.mode+'-'+label+('.txt'if args.mode=='vet'else'.jsonl'));assert not log.exists()
        with log.open('w')as stream:r=subprocess.run(command,cwd=ROOT,env=env,stdout=stream,stderr=subprocess.STDOUT)
        passed=[];skipped=[]
        if args.mode!='vet':
            for line in log.read_text().splitlines():
                if line.startswith('{'):
                    item=json.loads(line)
                    if'Test'in item and item.get('Action')=='pass':passed.append({'package':item['Package'],'test':item['Test']})
                    if'Test'in item and item.get('Action')=='skip':skipped.append(item['Test'])
        results.append({'label':label,'pass':r.returncode==0 and not skipped,'exit_code':r.returncode,'command':command,'passed_tests':passed,'skipped_tests':skipped,'log_sha256':sha(log)})
        if not results[-1]['pass']:print(log.read_text()[-6500:]);break
    assert sources()==before,'source changed during verification'
    report={'schema':1,'mode':args.mode,'pass':all(r['pass']for r in results),'elapsed_seconds':time.monotonic()-start,'sources':before,'checks':results,'environment':{k:env[k]for k in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'scope':'All1030 original pitch/volume profiles, logical580work and five-resource binding plus full scoreclock/resourcebridge regression; world/fulltick playback incomplete'}
    record.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'mode':args.mode,'pass':report['pass'],'elapsed_seconds':report['elapsed_seconds'],'checks':[{'label':x['label'],'pass':x['pass'],'records':len(x['passed_tests'])}for x in results]}))
    if not report['pass']:raise SystemExit(1)


if __name__=='__main__':main()
