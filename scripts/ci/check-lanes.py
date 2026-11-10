#!/usr/bin/env python3
"""Replay CI lanes against real Git histories and execute the selected Go tests.

Failures declared before implementation: three changed games create dozens of
shards or unrelated jobs; leaf edits run all public races; unknown shared edits
silently use the fast lane; docs/CI-only edits build release assets; a cancelled
predecessor is missed; a failed required lane still certifies the aggregate;
Contra research/negative startup tests run on every application package.
"""
import json, os, shutil, subprocess, tempfile
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'.local/ci-lanes';OUT.mkdir(parents=True,exist_ok=True)
(ROOT/'.local/tmp').mkdir(parents=True,exist_ok=True)
report={'passed':False,'checks':[]}
env=dict(os.environ,GOCACHE=str(ROOT/'.local/go-build'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/tmp'),TMPDIR=str(ROOT/'.local/tmp'),GOTELEMETRY='off',GOTOOLCHAIN='local')
with tempfile.TemporaryDirectory(dir=OUT,prefix='replay-') as tmp:
    repo=Path(tmp)
    def run(*args,ok=True):
        r=subprocess.run(args,cwd=repo,env=env,text=True,capture_output=True)
        if ok and r.returncode:raise AssertionError((args,r.stdout,r.stderr))
        return r
    def write(name,body):
        p=repo/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(body)
    def commit(message):
        run('git','add','.');run('git','commit','-qm',message)
        return run('git','rev-parse','HEAD').stdout.strip()
    def plan(base,*extra,ok=True):
        r=run('python3','-B','scripts/ci/scope.py','plan','--base',base,*extra,ok=ok)
        return json.loads(r.stdout) if ok else r
    def check(name,condition):
        assert condition,name
        report['checks'].append(name)
    try:
        run('git','init','-q');run('git','config','user.name','CI lane fixture');run('git','config','user.email','ci@example.invalid')
        write('.gitignore','.local/\n')
        write('scripts/ci/scope.py',(ROOT/'scripts/ci/scope.py').read_text())
        write('go.mod','module ci.example/fixture\n\ngo 1.23.0\n')
        for game in ['catan','rail','yuyuhakusho']:write('games/'+game+'/README.md',game)
        write('internal/server/wishes.go','package server\nconst limit = 2000\n')
        write('internal/server/wish_test.go','package server\nimport "testing"\nfunc TestWishBoundary(t *testing.T) {}\nfunc TestOSSSignature(t *testing.T) {}\n')
        write('internal/server/other_test.go','package server\nimport "testing"\nfunc TestUnrelatedMustNotRun(t *testing.T) { t.Fatal("unrelated suite ran") }\n')
        base=commit('verified baseline')
        write('internal/server/wishes.go','package server\nconst limit = 10000\n');commit('small field edit')
        p=plan(base)
        check('wish-field-uses-fast-lane',p['quick_components']==['wishes'] and not p['has_shared'] and not p['has_games'])
        check('application-edit-still-builds',p['build_required'])
        fast=plan(base,'--validation','quick')
        check('explicit-quick-skips-packaging',not fast['build_required'] and fast['has_quick'])
        p=plan(base)
        r=run('python3','-B','scripts/ci/scope.py','run','--game','quick','--plan','.local/ci/plan.json')
        receipt=json.loads((repo/'.local/ci/quick/result-0.json').read_text())
        check('quick-runner-executes-only-owned-tests',receipt['passed'] and len(receipt['commands'])==1 and '-race' not in receipt['commands'][0])
        log=(repo/'.local/ci/quick/0-0.log').read_text()
        check('test-file-ownership-includes-oss-test', 'TestOSSSignature' in log and 'TestUnrelatedMustNotRun' not in log)
        base=run('git','rev-parse','HEAD').stdout.strip()
        write('scripts/ci/scope.py',(ROOT/'scripts/ci/scope.py').read_text()+'\n# CI-only fixture edit\n');commit('CI selection only')
        p=plan(base)
        check('ci-only-skips-app-build-and-shared-races',p['quick_components']==['ci'] and not p['build_required'] and not p['has_shared'])
        base=run('git','rev-parse','HEAD').stdout.strip()
        write('docs/notes.md','documentation');commit('docs only');p=plan(base)
        check('docs-only-has-no-heavy-lanes',not p['build_required'] and not p['has_shared'] and not p['has_quick'] and not p['has_games'])
        base=run('git','rev-parse','HEAD').stdout.strip()
        write('games/catan/server/rules.go','package catan');commit('cancelled predecessor')
        write('games/rail/server/rules.go','package rail');write('games/yuyuhakusho/server/rules.go','package yuyuhakusho');commit('two more games')
        p=plan(base)
        check('three-games-create-exactly-three-parallel-rows',len(p['matrix']['include'])==3 and {r['game'] for r in p['matrix']['include']}=={'catan','rail','yuyuhakusho'})
        check('game-edits-skip-public-races',not p['has_shared'])
        check('cancelled-predecessor-remains-covered',any(r['game']=='catan' for r in p['matrix']['include']))
        base=run('git','rev-parse','HEAD').stdout.strip()
        write('web/src/main.tsx','public UI');commit('public frontend only');p=plan(base)
        check('public-frontend-does-not-run-server-races',p['frontend_checks'] and not p['has_shared'])
        base=run('git','rev-parse','HEAD').stdout.strip()
        write('internal/server/server.go','package server');commit('unknown shared core');p=plan(base)
        check('shared-core-keeps-public-suite',p['has_shared'] and not p['has_games'])
        check('unsafe-manual-quick-is-rejected',plan(base,'--validation','quick',ok=False).returncode!=0)
        p=plan(base,'--validation','full')
        check('manual-full-is-explicit',p['has_shared'] and len(p['games'])==3)
        workflow=(ROOT/'.github/workflows/build.yml').read_text()
        verify=workflow.split('  verify:',1)[1].split('  complete:',1)[0]
        game=workflow.split('  games:',1)[1].split('  verify:',1)[0]
        check('contra-negative-acceptance-is-game-owned','accept_production.py' not in verify and 'accept_production.py' in game)
        check('research-is-not-on-main-pipeline','  contra_campaigns:' not in workflow and (ROOT/'.github/workflows/contra-research.yml').is_file())
        check('runtime-go-cache-uses-real-build-paths','.local/go-build' in workflow and '.local/go-mod' in workflow)
        check('quick-result-does-not-wait-for-frontend','    needs: plan\n' in workflow.split('  quick:',1)[1].split('  frontend:',1)[0])
        check('aggregate-waits-for-required-lanes','needs: [plan, quick, frontend, shared, games, verify]' in workflow)
        block=workflow.split('      - name: Require every selected lane to pass\n',1)[1].split('        run: |\n',1)[1]
        shell='\n'.join(line[10:] if line.startswith('          ') else line for line in block.splitlines())+'\n'
        flags={'has_quick':'true','build_required':'false','has_shared':'false','has_games':'false'}
        for failure in [None,'plan','quick','missing-quick','games','shared','frontend','verify']:
            states={name:{'result':'skipped'} for name in ['quick','frontend','shared','games','verify']}
            states.update(plan={'result':'success','outputs':dict(flags)},quick={'result':'success'})
            if failure=='missing-quick':states['quick']['result']='skipped'
            elif failure:
                states[failure]['result']='failure'
                if failure in ('games','shared'):states['plan']['outputs']['has_'+failure]='true'
                if failure in ('frontend','verify'):states['plan']['outputs']['build_required']='true'
            result=subprocess.run(['bash','-s'],input=shell,cwd=repo,env=dict(env,RESULTS=json.dumps(states)),text=True,capture_output=True)
            check('aggregate-'+(failure or 'success'),(result.returncode==0)==(failure is None))
        report['passed']=True
    finally:
        (OUT/'result.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
