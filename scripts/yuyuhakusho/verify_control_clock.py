"""Verify normal control timing, actual Root queue effects and compatibility."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_observed_pcm as common

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/score-clock-control'


def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first=json.loads((BASE/'test-first.json').read_text());fix=json.loads((BASE/'api-name-correction.json').read_text())
    assert first['test_first']and not first['production_exists']
    for path,digest in first['files'].items():
        if path==fix['current_test']:
            before=ROOT/fix['original_test_archive'];after=ROOT/path
            assert sha(before)==digest==fix['before_sha256']and sha(after)==fix['after_sha256']
            assert before.read_text().replace(fix['old'],fix['new'])==after.read_text(),'API rename changed assertions'
        else:assert sha(ROOT/path)==digest,('frozen source changed',path)
    assert 'undefined: scoreclock.PriceControlWork'in(BASE/'missing-api-red.jsonl').read_text()


def sources():
    result=common.sources()
    paths=[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/score-clock-control-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-psg-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-psg-observer.c',ROOT/'games/yuyuhakusho/audio/scoreclock/CONTROL.md',BASE/'native-control-full.json.gz']
    for directory in['games/yuyuhakusho/audio/rootcontrol/testdata','games/yuyuhakusho/verification/root-control/native/canonical-v3','games/yuyuhakusho/verification/root-control/native/actions-v2']:
        paths+=list((ROOT/directory).rglob('*.json.gz'))
    result.update({str(p.relative_to(ROOT)):sha(p)for p in paths if p.is_file()});return dict(sorted(result.items()))


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
    checks=[('clock-root',flags+['./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/rootcontrol']),
            ('score-observed',flags+['-run','^Test(Observed(TempoWorkComesFromCurrentState|FeedbackReadsAreStaged|PriorityWritesAreAtOriginalPositions|ErrorsDoNotCommitScoreOrRepeatCallbacks|RecordsHaveNoMutableOutputAlias)|TickTransaction.*|MusicResetHeaderEffectsAreExplicitAndOrdered)$','./games/yuyuhakusho/audio/score']),
            ('score-original',flags+['-run','^TestObservedOriginalTrajectories$/^('+'|'.join(common.ORIGINAL_CASES)+')$','./games/yuyuhakusho/audio/score'])]
    if args.mode=='vet':checks=[('vet',[go,'vet']+['./games/yuyuhakusho/audio/'+x for x in['scoreclock','score','rootcontrol']])]
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
    report={'schema':1,'mode':args.mode,'pass':all(r['pass']for r in results),'elapsed_seconds':time.monotonic()-start,'sources':before,'checks':results,'original_cases':common.ORIGINAL_CASES,'original_frames':546,'environment':{k:env[k]for k in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'scope':'Normal control body/continuation, actual PSG and single canonical Root queue effect, complete scoreclock+Root regressions and bounded score original compatibility; world/fulltick audio remains incomplete'}
    record.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'mode':args.mode,'pass':report['pass'],'elapsed_seconds':report['elapsed_seconds'],'checks':[{'label':x['label'],'pass':x['pass'],'records':len(x['passed_tests'])}for x in results]}))
    if not report['pass']:raise SystemExit(1)


if __name__=='__main__':main()
