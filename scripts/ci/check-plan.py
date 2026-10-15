#!/usr/bin/env python3
"""CLI E2E: actual git commits, cancelled predecessor range, rename/deletion,
server fixture changes, invalid baseline, and frontend-only scope. All scratch
repositories and replayable evidence stay inside this repository.
"""
import json, subprocess, tempfile
from pathlib import Path
root=Path(__file__).resolve().parents[2]
out=root/'.local/ci-scope';out.mkdir(parents=True,exist_ok=True)
report={'passed':False,'checks':[]}
with tempfile.TemporaryDirectory(dir=out,prefix='git-') as tmp:
 repo=Path(tmp)
 def cmd(*args,check=True):
  return subprocess.run(args,cwd=repo,text=True,capture_output=True,check=check)
 def write(path,text='test'):
  p=repo/path;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
 def commit(message):
  cmd('git','add','.');cmd('git','commit','-qm',message);return cmd('git','rev-parse','HEAD').stdout.strip()
 def plan(base):
  p=cmd('python3','-B','scripts/ci/scope.py','plan','--base',base)
  return json.loads(p.stdout)
 try:
  cmd('git','init','-q');cmd('git','config','user.name','CI fixture');cmd('git','config','user.email','ci@example.invalid')
  write('.gitignore','.local/\n')
  write('scripts/ci/scope.py',(root/'scripts/ci/scope.py').read_text())
  for game in ['pairs','snooker','catan','rail']:write('games/'+game+'/README.md')
  base=commit('verified baseline')
  write('games/pairs/web/input.ts','first edit');commit('cancelled predecessor')
  write('games/snooker/web/Table.tsx','second edit');commit('latest edit')
  p=plan(base);assert p['games']==[{'game':'pairs','backend':False},{'game':'snooker','backend':False}],p
  report['checks'].append('cancelled-predecessor-web-scope')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  cmd('git','mv','games/pairs/web/input.ts','games/snooker/web/old.ts');commit('cross-game rename')
  p=plan(base);assert [r['game'] for r in p['games']]==['pairs','snooker'],p
  report['checks'].append('rename-both-owners')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('games/catan/server/scenario.json','{}');write('games/snooker/tests/testdata/case.json','{}');commit('backend fixtures')
  p=plan(base);assert all(r['backend'] for r in p['games']),p
  report['checks'].append('server-and-rule-fixtures')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('web/src/main.tsx','shared UI');write('internal/server/server.go','shared API');commit('shared only')
  assert plan(base)['games']==[]
  report['checks'].append('shared-does-not-fan-out')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('internal/server/server_test.go','package server\nfunc TestRailHiddenTasksThroughAPI(t *testing.T) {}\nfunc TestAuthCSRFAndStatic(t *testing.T) {}\n');commit('mixed legacy test file')
  assert plan(base)['games']==[{'game':'rail','backend':True}]
  report['checks'].append('mixed-legacy-test-ownership')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  cmd('git','rm','internal/server/server_test.go');commit('delete mixed legacy test file')
  assert plan(base)['games']==[{'game':'rail','backend':True}]
  report['checks'].append('deleted-test-retains-previous-owner')
  p=cmd('python3','-B','scripts/ci/scope.py','plan','--base','invalid-sha',check=False)
  assert p.returncode!=0
  report['checks'].append('invalid-baseline-fails')
  report['passed']=True
 finally:
  (out/'git-e2e.json').write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report))
