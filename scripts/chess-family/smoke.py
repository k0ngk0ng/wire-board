#!/usr/bin/env python3
"""Integrated embedded-binary smoke check, not a game-rule matrix.
Run after . scripts/env.sh and npm --prefix web run build. All files and the
throwaway database stay in this worktree; no remote game settings are touched.
"""
import hashlib
import http.cookiejar
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import urllib.request

root=Path(__file__).resolve().parents[2]
out=root/'.local/chess-family/final';out.mkdir(parents=True,exist_ok=True)
subprocess.run(['go','build','-o',str(out/'wire-board'),'.'],cwd=root,check=True)
report={'passed':False,'scope':'embedded binary health, new-game visibility and exact fallback media','media':[]}
with tempfile.TemporaryDirectory(prefix='smoke-',dir=out) as data:
 env=dict(os.environ,ADDR='127.0.0.1:0',DATA_DIR=data,ADMIN_USERNAME='发布整合验收',ADMIN_PASSWORD='local-smoke-only-password',ADMIN_EXISTING_USERNAME='',INVITE_CODE='local-smoke-only',PUBLIC_ORIGIN='',COOKIE_SECURE='false')
 log=out/'binary.log'
 with log.open('w') as f:server=subprocess.Popen([str(out/'wire-board')],cwd=root,env=env,stdout=f,stderr=subprocess.STDOUT)
 try:
  base=None
  for _ in range(200):
   match=re.search(r'listening on (127\.0\.0\.1:\d+)',log.read_text())
   if match:base='http://'+match[1];break
   if server.poll() is not None:raise RuntimeError('Binary exited; see binary.log')
   time.sleep(.05)
  assert base,'binary did not listen'
  client=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
  def api(path,body=None):
   with client.open(urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'}),timeout=10) as r:return json.load(r)
  assert api('/healthz')['ok'] is True
  api('/api/login',{'name':env['ADMIN_USERNAME'],'password':env['ADMIN_PASSWORD']})
  games={g['kind']:g for g in api('/api/admin')['games']};family=['gomoku','chess','xiangqi','junqi','weiqi'];new=[*family,'racing']
  assert all(g in games and games[g]['enabled'] is False for g in new)
  assert not set(new)&set(api('/api/state')['availableGames'])
  report['newGames']=[games[g] for g in new]
  for game in family:
   manifest=json.loads((root/'games'/game/'assets/sources.json').read_text())
   for item in manifest['files']:
    path='/'+manifest['version']+'/'+item['file']
    with client.open(base+path,timeout=10) as r:
     raw=r.read();assert hashlib.sha256(raw).hexdigest()==item['sha256'],path
     mime=r.headers.get_content_type();assert mime in (['image/webp'] if path.endswith('.webp') else ['audio/wav','audio/x-wav','audio/wave','audio/vnd.wave']),(path,mime)
     report.setdefault('mediaTypes',{})[path]=mime
    report['media'].append(path)
  for name in ['cars.webp','scenery.webp','atlas.json','cover.webp','engine.mp3','tires.mp3','collision-light.mp3','collision-heavy.mp3','countdown.mp3','start.mp3','lap.mp3','finish.mp3','attribution.json']:
   path='/racing/v1/'+name
   expected=(root/'games/racing/assets/public'/path.lstrip('/')).read_bytes()
   with client.open(base+path,timeout=10) as r:assert r.read()==expected,path
   report['media'].append(path)
  with client.open(urllib.request.Request(base+'/weiqi/v1/audio/place.wav',headers={'Range':'bytes=0-15'}),timeout=10) as r:assert r.status==206 and len(r.read())==16
  report['rangeRequest']=True;report['passed']=True
 finally:
  server.terminate();server.wait(timeout=15)
  (out/'binary-smoke.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print('PASS: integrated binary, six unpublished new games,',len(report['media']),'media objects and audio range requests')
