"""Verify actual Root track-end restoration and existing audio rule consumers."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_note_voice_clock as common

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/score-clock-track-end'
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def frozen():
    r=json.loads((BASE/'test-first.json').read_text());fix=json.loads((BASE/'test-projection-correction.json').read_text());assert r['test_first']and not r['production_exists']
    for path,digest in r['files'].items():
        p=(BASE/'track_end_e2e_test.before-dac-projection-fix.txt')if path.endswith('track_end_e2e_test.go')else ROOT/path
        assert sha(p)==digest,('frozen input changed',path)
    assert sha(ROOT/'games/yuyuhakusho/audio/scoreclock/track_end_e2e_test.go')==fix['current_test_sha256']
    assert 'undefined:'in(BASE/'missing-api-red.jsonl').read_text()
def sources():
    r=common.sources()
    for p in[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/score-clock-track-end-capture.py',ROOT/'games/yuyuhakusho/audio/scoreclock/VOICE.md']:
        r[str(p.relative_to(ROOT))]=sha(p)
    return dict(sorted(r.items()))
def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');a=p.parse_args();frozen()
    if a.audit:
        r=json.loads((BASE/'validation.json').read_text());assert r['pass']
        for key in['sources','artifacts']:
            for path,digest in r[key].items():assert sha(ROOT/path)==digest,('binding changed',path)
        print(json.dumps({'pass':True,'sources':len(r['sources']),'artifacts':len(r['artifacts'])}));return
    assert a.mode and a.out;out=a.out.resolve();assert out.is_relative_to(ROOT)and out!=BASE;out.mkdir(parents=True,exist_ok=True);log=out/(a.mode+('.txt'if a.mode=='vet'else'.jsonl'));record=out/(a.mode+'-command.json');assert not log.exists()and not record.exists()
    env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2');go=shutil.which('go');assert go;cmd=[go,'test','-json','-count=1','-timeout=8m']
    if a.mode=='native':cmd.append('-race')
    elif a.mode=='wasm':
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();cmd+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
    cmd+=['./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/rootcontrol']
    if a.mode=='vet':cmd=[go,'vet','./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/rootcontrol','./games/yuyuhakusho/audio/score']
    before,start=sources(),time.monotonic()
    with log.open('w')as f:r=subprocess.run(cmd,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
    assert before==sources(),'source changed during verification';passed=[];skipped=[]
    if a.mode!='vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                x=json.loads(line)
                if'Test'in x and x.get('Action')=='pass':passed.append({'package':x['Package'],'test':x['Test']})
                if'Test'in x and x.get('Action')=='skip':skipped.append(x['Test'])
    report={'schema':1,'mode':a.mode,'pass':r.returncode==0 and not skipped,'exit_code':r.returncode,'command':cmd,'elapsed_seconds':time.monotonic()-start,'sources':before,'passed_tests':passed,'skipped_tests':skipped,'log_sha256':sha(log),'environment':{k:env[k]for k in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'scope':'119 whole finite-track calls with actual Root working state/effect leases plus complete scoreclock/rootcontrol regression; full driver/world/platform audio incomplete.'};record.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in['mode','pass','exit_code','elapsed_seconds']}))
    if not report['pass']:print(log.read_text()[-6500:]);raise SystemExit(r.returncode or 1)
if __name__=='__main__':main()
