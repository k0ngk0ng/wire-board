#!/usr/bin/env python3
"""Build the real UI, bind production runtime identities, run local E2E."""
import json, os, subprocess, time, urllib.request
from pathlib import Path
root=Path(__file__).resolve().parents[2]
out=root/'.local/platform-admin-acceptance';out.mkdir(parents=True,exist_ok=True)
common=Path(subprocess.check_output(['git','rev-parse','--path-format=absolute','--git-common-dir'],cwd=root,text=True).strip()).parent
cache=common/'.local'
env=dict(os.environ,GOCACHE=str(cache/'go-build'),GOMODCACHE=str(cache/'go-mod'),GOTMPDIR=str(out/'tmp'),TMPDIR=str(out/'tmp'),npm_config_cache=str(out/'npm'),GOTOOLCHAIN='local',WIRE_BOARD_PLATFORM_ADMIN_BROWSER='1')
for key in ('GOCACHE','GOMODCACHE','GOTMPDIR','npm_config_cache'):Path(env[key]).mkdir(parents=True,exist_ok=True)
subprocess.run(['npm','--prefix','web','run','build'],cwd=root,env=env,check=True)
receipt=json.loads((root/'web/dist/contra-delivery.json').read_text())
flags=' '.join('-X github.com/k0ngk0ng/wire-board/games/contra/runtime.Compiled'+key[0].upper()+key[1:]+'='+receipt[key] for key in ('rulesBuildID','runtimeBuildID'))
stop=out/'stop';stop.unlink(missing_ok=True)
with (out/'server.log').open('w') as log:
 process=subprocess.Popen(['go','test','-ldflags='+flags,'./internal/server','-run','^TestPlatformAdminBrowserFixture$','-count=1','-timeout=25m'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
 try:
  deadline=time.monotonic()+120
  while True:
   assert process.poll() is None,'browser fixture exited'
   try:
    with urllib.request.urlopen('http://127.0.0.1:18099/healthz',timeout=1) as r:
     if r.status==200:break
   except OSError:pass
   assert time.monotonic()<deadline,'browser fixture startup timed out';time.sleep(.2)
  subprocess.run(['python3','-B','scripts/platform-admin/check-browser.py'],cwd=root,env=env,check=True)
 finally:
  stop.touch()
  try:process.wait(timeout=15)
  except subprocess.TimeoutExpired:process.terminate();process.wait(timeout=15)
  stop.unlink(missing_ok=True)
 assert process.returncode==0,'browser fixture failed'
