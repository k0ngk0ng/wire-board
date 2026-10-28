#!/usr/bin/env python3
"""Verify the real application entry with linked Contra identities on loopback.

Checks startup, health, default registration and room creation. This does not
claim browser runtime delivery, campaign completion, publication or deployment.
"""
import argparse
import http.cookiejar
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
import traceback
import urllib.request

ROOT = Path(__file__).resolve().parents[2]
sys.dont_write_bytecode = True
sys.path.insert(0, str(ROOT/'games/contra/build'))
from identity import go_env, sha, verify_runtime_file


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--runtime', required=True, type=Path)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--browser-label')
    args = parser.parse_args()
    out = args.out.resolve()
    if not out.is_relative_to(ROOT):
        parser.error('evidence must stay inside workspace')
    manifest, runtime_id, _ = verify_runtime_file(ROOT, args.runtime)
    if args.browser_label and not re.fullmatch('[a-z0-9-]+',args.browser_label):
        parser.error('invalid browser evidence label')
    out.mkdir(parents=True, exist_ok=False)
    temporary = out/'temporary'
    temporary.mkdir()
    binary = out/'wire-board'
    report = {'passed': False, 'scope': __doc__, 'runtimeBuildID': runtime_id,
              'rulesBuildID': manifest['rulesBuildID']}
    process = None
    try:
        with (out/'build.log').open('w') as log:
            subprocess.run([sys.executable, '-B', 'games/contra/build/native.py',
                            '--runtime', str(args.runtime), '--out', str(binary)],
                           cwd=ROOT, env=go_env(ROOT), stdout=log,
                           stderr=subprocess.STDOUT, check=True, timeout=180)
        env = go_env(ROOT)
        for name in list(env):
            if name.startswith(('OSS_', 'ADMIN_', 'WISH_')) or name in (
                    'PUBLIC_ORIGIN', 'ASSETS_BASE_URL', 'COOKIE_SECURE'):
                del env[name]
        invite='contra-browser' if args.browser_label else 'contra-main-accept-only'
        env.update(ADDR='127.0.0.1:0', DATA_DIR=str(temporary/'db'),
                   INVITE_CODE=invite, WIRE_BOARD_VERSION='local-contra-accept')
        with (out/'server.log').open('w') as log:
            process = subprocess.Popen([str(binary)], cwd=ROOT, env=env,
                                       stdout=log, stderr=subprocess.STDOUT)
            deadline = time.monotonic()+40
            base = None
            while time.monotonic() < deadline:
                if process.poll() is not None:
                    raise RuntimeError('application exited before listening')
                match = re.search(r'Wire Board is listening on (127\.0\.0\.1:\d+)',
                                  (out/'server.log').read_text())
                if match:
                    base = 'http://'+match[1]
                    break
                time.sleep(.05)
            if base is None:
                raise RuntimeError('application did not start within deadline')
            client = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
            def request(path, body=None):
                data = None if body is None else json.dumps(body).encode()
                req = urllib.request.Request(base+path, data=data,
                                             headers={'Content-Type':'application/json'})
                with client.open(req, timeout=15) as response:
                    return json.load(response)
            report['health'] = request('/healthz')
            assert report['health'] == {'ok': True}
            request('/api/register', {'name':'ContraMainCheck','password':'contra-main-test-123',
                                      'invite':invite})
            state = request('/api/state')
            assert state['availableGames'].count('contra') == 1, state['availableGames']
            room = request('/api/rooms', {'kind':'contra','capacity':2,
                                          'name':'魂斗罗主程序启动验收','options':{}})
            state = request('/api/state')
            assert state['room']['id'] == room['id']
            report.update(availableGames=state['availableGames'], createdRoomID=room['id'],
                          nativeBuild=json.loads(binary.with_suffix('.build.json').read_text()), passed=True)
            if args.browser_label:
                fixture={'url':base,'supportsBots':False,'runtimeBuildID':runtime_id,'rulesBuildID':manifest['rulesBuildID'],
                         'visualsID':manifest['visualsID'],'webRoot':str(ROOT/'web/dist'),
                         'publicRoot':str(args.runtime.resolve().parent),
                         'webManifestSHA256':sha((ROOT/'web/dist/game-load-manifest.json').read_bytes()),
                         'users':[{'name':'main-browser-'+str(i),'password':'main-browser-test-123'} for i in range(3)]}
                fixture_path=temporary/'browser-fixture.json'
                fixture_path.write_text(json.dumps(fixture))
                with (out/'browser.log').open('w') as browser_log:
                    subprocess.run([sys.executable,'-B','scripts/contra/platform_accept.py','--fixture',str(fixture_path),
                                    '--label',args.browser_label,'--presentation','--network-boundaries'],
                                   cwd=ROOT,env=env,stdout=browser_log,stderr=subprocess.STDOUT,check=True,timeout=300)
                evidence=ROOT/'.local/contra-browser'/('platform-'+args.browser_label)/'result.json'
                assert json.loads(evidence.read_text())['passed']
                report['realApplicationBrowserEvidence']=str(evidence.relative_to(ROOT))
    except BaseException:
        report['passed'] = False
        report['error'] = traceback.format_exc()
    finally:
        if process is not None:
            process.terminate()
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
                report['passed'] = False
            report['serverExitCode'] = process.returncode
        shutil.rmtree(temporary)
        report['temporaryDatabaseRemoved'] = True
        report['scriptSHA256'] = sha(Path(__file__).read_bytes())
        (out/'result.json').write_text(json.dumps(report, indent=2, ensure_ascii=False)+'\n')
        print(json.dumps({'passed':report['passed'],'out':str(out),'error':report.get('error')}))
    return 0 if report['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
