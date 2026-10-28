#!/usr/bin/env python3
"""CLI E2E: actual git commits, cancelled predecessor range, rename/deletion,
server fixture changes, invalid baseline, and frontend-only scope. All scratch
repositories and replayable evidence stay inside this repository.
"""
import json, os, subprocess, tempfile
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
  for game in ['pairs','snooker','catan','rail','contra']:write('games/'+game+'/README.md')
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
  write('games/contra/engine/leveldata/stage-8.json','{}');commit('cancelled rule data predecessor')
  write('games/contra/engine/testdata/visible.json.gz','fixture');commit('latest AI fixture')
  selected=plan(base)
  assert selected['games']==[{'game':'contra','backend':True}],selected
  assert not selected['has_contra_campaigns'] and selected['contra_campaign_matrix']=={'include':[]},selected
  report['checks'].append('contra-rule-data-and-ai-fixture-backend')
  outputs=repo/'.local/github-backend-output';summary=repo/'.local/github-backend-summary'
  subprocess.run(['python3','-B','scripts/ci/scope.py','plan','--base',base],cwd=repo,env=dict(os.environ,GITHUB_OUTPUT=str(outputs),GITHUB_STEP_SUMMARY=str(summary)),capture_output=True,text=True,check=True)
  values=dict(line.split('=',1)for line in outputs.read_text().splitlines())
  assert values['has_contra_campaigns']=='false' and json.loads(values['contra_campaign_matrix'])=={'include':[]},values
  report['checks'].append('contra-backend-default-skips-computer-research')
  research=json.loads(cmd('python3','-B','scripts/ci/scope.py','plan','--base',base,'--contra-campaigns').stdout)
  expected={'include':[{'mode':mode,'players':players}for mode in ['standard-terminal','assisted-clear']for players in [1,2]]}
  assert research['contra_campaign_requested'] and research['contra_campaign_matrix']==expected,research
  report['checks'].append('explicit-contra-research-selects-four-mode-seat-jobs')
  for game in ['contra','all']:
   selected=json.loads(cmd('python3','-B','scripts/ci/scope.py','plan','--base',base,'--manual',game).stdout)
   assert not selected['contra_campaign_requested'] and not selected['has_contra_campaigns'],selected
  report['checks'].append('ordinary-manual-contra-and-all-keep-research-disabled')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('games/contra/web/Board.tsx','render');write('games/contra/assets/runtime-stages.json','{}');commit('contra frontend only')
  selected=plan(base)
  assert selected['games']==[{'game':'contra','backend':False}],selected
  assert not selected['has_contra_campaigns'] and selected['contra_campaign_matrix']=={'include':[]},selected
  report['checks'].append('contra-frontend-does-not-run-campaign')
  outputs=repo/'.local/github-frontend-output'
  subprocess.run(['python3','-B','scripts/ci/scope.py','plan','--base',base],cwd=repo,env=dict(os.environ,GITHUB_OUTPUT=str(outputs)),capture_output=True,text=True,check=True)
  values=dict(line.split('=',1)for line in outputs.read_text().splitlines())
  assert values['has_contra_campaigns']=='false' and json.loads(values['contra_campaign_matrix'])=={'include':[]},values
  report['checks'].append('contra-frontend-emits-explicit-campaign-skip')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('games/contra/runtime/bridge.go','package runtime');write('games/contra/server/production_native.go','package contra');commit('runtime and initialization only')
  selected=plan(base)
  assert selected['games']==[{'game':'contra','backend':True}] and not selected['contra_campaign_requested'],selected
  report['checks'].append('contra-runtime-keeps-go-contracts-without-campaign')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('games/contra/server/input.go','package contra');commit('server input rule')
  selected=plan(base)
  assert selected['games']==[{'game':'contra','backend':True}] and not selected['contra_campaign_requested'],selected
  report['checks'].append('contra-server-action-keeps-rules-without-computer-campaign')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('games/registry.go','package games');write('games/registry_test.go','package games\nfunc TestAllRegistryHooks(t *testing.T) {}\n');commit('public registration contract')
  assert plan(base)['games']==[],plan(base)
  selected=cmd('python3','-B','-c','import sys;from pathlib import Path;sys.path.insert(0,"scripts/ci");import scope;print(scope.go_commands(Path.cwd(),"shared",race=True))')
  assert 'TestAllRegistryHooks'in selected.stdout and './games'in selected.stdout,selected.stdout
  report['checks'].append('shared-registry-without-unrelated-games')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('web/src/main.tsx','shared UI');write('internal/server/server.go','shared API');commit('shared only')
  assert plan(base)['games']==[]
  report['checks'].append('shared-does-not-fan-out')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('internal/server/server_test.go','package server\nfunc TestRailHiddenTasksThroughAPI(t *testing.T) {}\nfunc TestAuthCSRFAndStatic(t *testing.T) {}\n');commit('mixed legacy test file')
  assert plan(base)['games']==[{'game':'rail','backend':True}]
  report['checks'].append('mixed-legacy-test-ownership')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('internal/server/server_test.go','package server\nfunc TestRailHiddenTasksThroughAPI(t *testing.T) {}\nfunc TestAuthCSRFAndStatic(t *testing.T) { t.Log("public CSP { }") }\n');commit('change only public CSP test in mixed file')
  assert plan(base)['games']==[],plan(base)
  report['checks'].append('mixed-file-public-test-does-not-run-rail')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('internal/server/server_test.go','package server\nfunc TestRailHiddenTasksThroughAPI(t *testing.T) { t.Log("rail") }\nfunc TestAuthCSRFAndStatic(t *testing.T) { t.Log("public CSP { }") }\n');commit('change actual rail test')
  assert plan(base)['games']==[{'game':'rail','backend':True}],plan(base)
  report['checks'].append('mixed-file-game-test-retains-owner')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('internal/server/server_test.go','package server\nfunc TestRailHiddenTasksThroughAPI(t *testing.T) { t.Log("rail") }\n');commit('delete only public CSP test')
  assert plan(base)['games']==[],plan(base)
  report['checks'].append('mixed-file-public-test-deletion-does-not-run-rail')
  base=cmd('git','rev-parse','HEAD').stdout.strip()
  write('internal/server/server_test.go','package server\nfunc fixtureNumber() int { return 2 }\nfunc TestRailHiddenTasksThroughAPI(t *testing.T) { t.Log("rail") }\n');commit('change shared test helper')
  assert plan(base)['games']==[{'game':'rail','backend':True}],plan(base)
  report['checks'].append('mixed-file-helper-conservatively-retains-game-owner')
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
