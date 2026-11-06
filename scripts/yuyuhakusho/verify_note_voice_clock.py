"""Verify whole new-note calls, documented setup correction and clock regression."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time

import verify_held_voice_clock as common

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'games/yuyuhakusho/verification/score-clock-note-voice'


def sha(path): return hashlib.sha256(path.read_bytes()).hexdigest()


def frozen():
    first=json.loads((BASE/'test-first.json').read_text())
    correction=json.loads((BASE/'test-setup-correction.json').read_text())
    assert first['test_first'] and not first['production_exists']
    test='games/yuyuhakusho/audio/scoreclock/note_voice_e2e_test.go'
    capture='scripts/yuyuhakusho/score-clock-note-voice-capture.py'
    for path,digest in first['files'].items():
        archive=(BASE/'note_voice_e2e_test.before-setup-correction.txt')if path==test else(BASE/'capture.before-aux-baseline-correction.py.txt')if path==capture else ROOT/path
        assert sha(archive)==digest,('original frozen source changed',path)
    assert sha(ROOT/test)==correction['current_test_sha256']
    assert sha(ROOT/capture)==correction['current_capture_sha256']
    fixed=correction['corrected_oracle'];assert sha(ROOT/fixed['path'])==fixed['sha256']
    assert 'undefined: scoreclock.NoteVoiceTransfer'in(BASE/'missing-api-red.jsonl').read_text()


def sources():
    result=common.sources()
    for path in[Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/score-clock-note-voice-capture.py',ROOT/'scripts/yuyuhakusho/score-clock-note-voice-observer.c',ROOT/'scripts/yuyuhakusho/pcm-command-capture.py',ROOT/'games/yuyuhakusho/audio/scoreclock/NOTE_VOICE.md',BASE/'native-note-voice-canonical-aux.json.gz']:
        result[str(path.relative_to(ROOT))]=sha(path)
    return dict(sorted(result.items()))


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--mode',choices=['native','wasm','vet']);p.add_argument('--out',type=Path);p.add_argument('--audit',action='store_true');args=p.parse_args();frozen()
    if args.audit:
        report=json.loads((BASE/'validation.json').read_text());assert report['pass']
        for key in['sources','artifacts']:
            for path,digest in report[key].items():assert sha(ROOT/path)==digest,('binding changed',path)
        print(json.dumps({'pass':True,'sources':len(report['sources']),'artifacts':len(report['artifacts'])}));return
    assert args.mode and args.out
    out=args.out.resolve();assert out.is_relative_to(ROOT)and out!=BASE;out.mkdir(parents=True,exist_ok=True)
    log=out/(args.mode+('.txt'if args.mode=='vet'else'.jsonl'));record=out/(args.mode+'-command.json');assert not log.exists()and not record.exists(),'preserve prior checks'
    env=os.environ.copy();env.update(TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),GOMAXPROCS='2')
    go=shutil.which('go');assert go
    command=[go,'test','-json','-count=1','-timeout=8m']
    if args.mode=='native':command.append('-race')
    elif args.mode=='wasm':
        goroot=subprocess.check_output([go,'env','GOROOT'],cwd=ROOT,env=env,text=True).strip();command+=['-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec')];env.update(GOOS='js',GOARCH='wasm',GOMAXPROCS='1')
    command.append('./games/yuyuhakusho/audio/scoreclock')
    if args.mode=='vet':command=[go,'vet','./games/yuyuhakusho/audio/scoreclock','./games/yuyuhakusho/audio/score']
    before,start=sources(),time.monotonic()
    with log.open('w')as stream:r=subprocess.run(command,cwd=ROOT,env=env,stdout=stream,stderr=subprocess.STDOUT)
    assert before==sources(),'source changed during verification'
    passed=[];skipped=[]
    if args.mode!='vet':
        for line in log.read_text().splitlines():
            if line.startswith('{'):
                item=json.loads(line)
                if'Test'in item and item.get('Action')=='pass':passed.append(item['Test'])
                if'Test'in item and item.get('Action')=='skip':skipped.append(item['Test'])
    report={'schema':1,'mode':args.mode,'pass':r.returncode==0 and not skipped,'exit_code':r.returncode,'command':command,'elapsed_seconds':time.monotonic()-start,'sources':before,'passed_tests':passed,'skipped_tests':skipped,'log_sha256':sha(log),'environment':{k:env[k]for k in['TEST_TELEMETRY_DIR','GOCACHE','GOMODCACHE','GOTMPDIR','TMPDIR','GOMAXPROCS','GOOS','GOARCH']if k in env},'scope':'1095 complete new-note calls including real PCM/bus and 721 held calls plus all scoreclock regressions. Track-end/ownership/full world/platform audio remain incomplete.'}
    record.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({k:report[k]for k in['mode','pass','exit_code','elapsed_seconds']}))
    if not report['pass']:print(log.read_text()[-6500:]);raise SystemExit(r.returncode or 1)


if __name__=='__main__':main()
