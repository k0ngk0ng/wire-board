#!/usr/bin/env python3
"""Verify/replay frozen Root rules evidence without starting a native core."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

BASE=Path('games/yuyuhakusho/verification/root-control-orchestration')
PACKAGE='./games/yuyuhakusho/audio/rootcontrol'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read_json(path):
    return json.loads(path.read_bytes())


def verify_bindings(root,rows):
    for row in rows:
        assert sha(root/row['path'])==row['sha256'],row['path']


def verify_test_first(root):
    first=read_json(root/BASE/'test-first-manifest.json')
    retained={
        'games/yuyuhakusho/audio/rootcontrol/orchestrator_e2e_test.go':BASE/'orchestrator-native-v2-test.go.txt',
        'games/yuyuhakusho/audio/rootcontrol/orchestrator_test.go':BASE/'orchestrator-unit-test-first.go.txt',
        'docs/research/md-yuyuhakusho/AUDIO_ROOT_CONTROL_ORCHESTRATION_FAILURES.md':BASE/'failures-test-first.md.txt',
        'docs/research/md-yuyuhakusho/AUDIO_ROOT_CONTROL_ORCHESTRATION_API.md':BASE/'api-test-first.md.txt',
    }
    for row in first['files']:
        assert sha(root/retained.get(row['path'],Path(row['path'])))==row['sha256'],row['path']
    red=(root/BASE/'root-missing-api-red.jsonl').read_text()
    assert 'undefined: rootcontrol.RuleOwners' in red
    for name,needle in [('root-reentrancy-red.jsonl','reentrant Root ran before acquiring unique score lease'),('missing-score-consumer-red.jsonl','missing effect consumer silently succeeded')]:
        assert needle in (root/BASE/name).read_text()
    return len(first['files'])


def verify_fixtures(root):
    scratch=root/'.local/root-control-orchestration-verify'
    scratch.mkdir(parents=True,exist_ok=True)
    rebuilt=[]
    for suffix,profile,count,frames,capture in [
        ('native-v3','callable-stress',5,785,'.local/md-yuyuhakusho/root-control-canonical-v3/capture.json'),
        ('legal-v3','fight',3,330,'.local/md-yuyuhakusho/root-control-legal-v3/capture.json')]:
        manifest=read_json(root/f'games/yuyuhakusho/audio/rootcontrol/testdata/orchestrator-{suffix}-manifest.json')
        verify_bindings(root,manifest['sources'])
        verify_bindings(root,[manifest['producer'],manifest['artifact']])
        with tempfile.TemporaryDirectory(dir=scratch) as temporary:
            temporary=Path(temporary)
            target=temporary/'fixture.json.gz'
            args=['python3','-B','scripts/root-control-orchestration-fixtures.py','--capture',capture,
                  '--out',str(target.relative_to(root)),'--manifest',str((temporary/'manifest.json').relative_to(root)),
                  '--expected-cases',str(count),'--expected-frames',str(frames),'--profile',profile]
            result=subprocess.run(args,cwd=root,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            assert result.returncode==0,result.stderr
            assert sha(target)==manifest['artifact']['sha256'],suffix
        rebuilt.append({'fixture':suffix,'source_frames':frames,'byte_identical':True})
    return rebuilt


def environment(root):
    env=os.environ.copy()
    env.update(TEST_TELEMETRY_DIR=str(root/'.local/go-telemetry'),GOCACHE=str(root/'.local/go-cache'),
               GOMODCACHE=str(root/'.local/go-mod'),GOTMPDIR=str(root/'.local/go-tmp'),
               TMPDIR=str(root/'.local/tmp'),GOTOOLCHAIN='local')
    return env


def run_tests(root):
    env=environment(root)
    goroot=subprocess.check_output(['go','env','GOROOT'],cwd=root,env=env,text=True).strip()
    stages=[('native-race',['go','test','-race','-json','-count=1',PACKAGE],{}),
            ('wasm',['go','test','-json','-count=1','-exec',str(Path(goroot)/'lib/wasm/go_js_wasm_exec'),PACKAGE],{'GOOS':'js','GOARCH':'wasm'}),
            ('vet',['go','vet',PACKAGE],{})]
    for name,args,extra in stages:
        current=env.copy();current.update(extra)
        args=['nice','-n','10',*args]
        target=root/BASE/('replay-'+name+('.txt' if name=='vet' else '.jsonl'))
        with target.open('w') as stream:
            result=subprocess.run(args,cwd=root,env=current,stdout=stream,stderr=subprocess.STDOUT)
        (root/BASE/('replay-'+name+'-command.json')).write_text(json.dumps({'args':args,'exit_code':result.returncode,'cwd':str(root)},indent=2)+'\n')
        assert result.returncode==0,name
        print(json.dumps({'test_stage':name,'pass':True}),flush=True)


def verify_results(root,prefix):
    results=[]
    required={
        'TestOrchestratorRetainedNative785FramesV3':'785 retained native frames; 7199 ordered',
        'TestOrchestratorFightNative180FramesV3':'180 fight native frames; 1246 ordered',
        'TestOrchestratorFightActivePCM04TypedCheckpointV3':'148 rule frames from typed pre04 checkpoint, 35 strict',
    }
    for name in ['native-race','wasm']:
        rows=[json.loads(line) for line in (root/BASE/f'{prefix}-{name}.jsonl').read_text().splitlines()]
        assert not any(row.get('Action')=='fail' for row in rows),name
        passed={row.get('Test') for row in rows if row.get('Action')=='pass' and row.get('Test')}
        for test,needle in required.items():
            assert test in passed,(name,test)
            assert any(needle in row.get('Output','') for row in rows if row.get('Test')==test),(name,test)
        for test in ['TestOrchestratorLeasePrecedesEveryRootAndScoreCallback','TestOrchestratorRequiresBothOperationConsumersBeforeCallbacks']:
            assert test in passed,(name,test)
        command=read_json(root/BASE/f'{prefix}-{name}-command.json')
        assert command['exit_code']==0
        results.append({'stage':name,'pass':True,'passed_cases':len(passed)})
    assert read_json(root/BASE/f'{prefix}-vet-command.json')['exit_code']==0
    results.append({'stage':'vet','pass':True})
    return results


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--run-tests',action='store_true')
    parser.add_argument('--out',default=str(BASE/'replay-verification.json'))
    args=parser.parse_args()
    root=Path.cwd()
    assert (root/'games/yuyuhakusho/audio/rootcontrol/orchestrator.go').is_file(),'run from the authorized worktree root'
    frozen=read_json(root/BASE/'frozen-sources.json')
    verify_bindings(root,frozen['sources'])
    verify_bindings(root,frozen['artifacts'])
    initial=verify_test_first(root)
    fixtures=verify_fixtures(root)
    if args.run_tests:
        run_tests(root)
    results=verify_results(root,'replay' if args.run_tests else 'final')
    output={'version':1,'pass':True,'frozen_sources':len(frozen['sources']),'frozen_artifacts':len(frozen['artifacts']),
            'initial_test_first_bindings':initial,'fixtures':fixtures,'tests':results,
            'rule_frames':1113,'ordered_effective_ym_psg_requests':8480,
            'native_source_frames':1115,'excluded_ordinary1f_acceptance_frames':2,
            'full_master_cycle_cost':False,'per_long_interrupt_visibility':False,
            'world_IRQ_schedule':False,'ordinary_PCM_acceptance_lifecycle':False,'whole_goal_complete':False}
    (root/args.out).write_text(json.dumps(output,indent=2,sort_keys=True)+'\n')
    print(json.dumps(output,sort_keys=True))


if __name__=='__main__':
    main()
