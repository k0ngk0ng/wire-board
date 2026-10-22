#!/usr/bin/env python3
"""Bubbleparty classic map previews E2E; real app and original image resources.

Failure inventory is recorded before implementation in
 games/bubbleparty/tests/MAP_PREVIEW_SCENARIOS.md. This acceptance changes
 real form controls, checks all source previews and injects only a network
 image failure on the local fixture. It never injects game state.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import traceback
import uuid

parser = argparse.ArgumentParser()
parser.add_argument('--out', required=True)
args = parser.parse_args()
root = Path(__file__).resolve().parents[2]
out = (root / args.out).resolve()
assert out.is_relative_to(root) and not out.exists()
out.mkdir(parents=True)
run = 'mp' + uuid.uuid4().hex[:6]
temp = out / 'temporary'
sockets = root / '.s' / run
env = dict(os.environ)
for key, path in {'TMPDIR': temp / 'tmp', 'GOTMPDIR': temp / 'tmp',
                  'GOCACHE': root / '.local/go-build', 'GOMODCACHE': root / '.local/go-mod',
                  'npm_config_cache': root / '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': sockets}.items():
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(WIRE_BOARD_BUBBLEPARTY_BROWSER='1', WIRE_BOARD_BUBBLEPARTY_BROWSER_OUT=str(out),
           GOTOOLCHAIN='local', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'checks': [], 'screenshots': [],
          'distSHA256': hashlib.sha256((root / 'web/dist/index.html').read_bytes()).hexdigest()}
opened = set()
server = log = None


def browser(who, *args):
    profile = temp / who
    profile.mkdir(parents=True, exist_ok=True)
    e = dict(env, AGENT_BROWSER_SESSION=run + who, AGENT_BROWSER_PROFILE=str(profile))
    p = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=e,
                       text=True, capture_output=True, timeout=45)
    assert p.returncode == 0, p.stdout + p.stderr
    result = json.loads(p.stdout)
    assert result['success'], result
    return result['data']


def js(who, code):
    return browser(who, 'eval', code)['result']


def api(who, path, body=None):
    opts = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    result = js(who, '(async()=>{const r=await fetch(' + json.dumps('/api' + path) + opts + ');return {status:r.status,data:await r.json()}})()')
    assert result['status'] in (200, 201), result
    return result['data']


def room(who='host'):
    return api(who, '/state')['room']


def cmd(who, kind, **extra):
    r = room(who)
    return api(who, '/rooms/' + r['id'], dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra))


def wait(fn, seconds=20):
    deadline = time.monotonic() + seconds
    while time.monotonic() < deadline:
        if server is not None and server.poll() is not None:
            raise AssertionError('fixture exited; inspect server.log')
        if value := fn():
            return value
        time.sleep(.1)
    raise AssertionError('condition timed out')


def click(who, name):
    browser(who, 'find', 'role', 'button', 'click', '--name', name, '--exact')


def view(who):
    return js(who, '''(()=>{const map=document.querySelector('select[aria-label="水泡战场地图"]');return {map:map?.value,seats:[...document.querySelectorAll('.waiting-seat.filled')].map(e=>({bot:!!e.querySelector('.bot-badge'),ready:e.textContent.includes('已准备'),text:e.textContent})),buttons:[...document.querySelectorAll('.waiting-actions button')].map(e=>({text:e.textContent.trim(),disabled:e.disabled})),status:document.querySelector('.waiting-seats [role=status]')?.textContent}})()''')


def record(name, who='host'):
    data = preview(who)
    report['checks'].append({'name': name, 'view': data})
    print('PASS', name, flush=True)
    return data


def ready(who, flags):
    return [s['ready'] for s in view(who)['seats']] == flags


def shot(name, who='host'):
    browser(who, 'screenshot', str(out / (name + '.png')), '--full')
    report['screenshots'].append(name + '.png')


def preview(who='host'):
    return js(who, """(()=>{const root=document.querySelector('.bubble-map-preview'),img=root?.querySelector('img'),r=root?.getBoundingClientRect();return {map:root?.dataset.map,state:root?.dataset.previewState,text:root?.textContent,src:img?.getAttribute('src'),complete:img?.complete,width:img?.naturalWidth,height:img?.naturalHeight,left:r?.left,right:r?.right,top:r?.top,bottom:r?.bottom,viewport:innerWidth,scrollWidth:document.documentElement.scrollWidth}})()""")


def select(map_id):
    browser('host','select','select[aria-label="水泡战场地图"]',map_id)

try:
    log=(out/'server.log').open('w')
    server=subprocess.Popen(['go','test','-count=1','-timeout=12m','-run','^TestBubblepartyBrowserFixture$','./games/bubbleparty/tests'],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
    wait(lambda:(out/'url').exists(),120)
    base=(out/'url').read_text()
    browser('host','open',base);opened.add('host');browser('host','set','viewport','1280','1000')
    api('host','/login',{'name':'泡泡验收管理员','password':'bubbleparty-browser-test-only'})
    browser('host','reload');browser('host','wait','.room-list')
    browser('host','click','.lobby-create');browser('host','wait','[role=dialog]')
    browser('host','fill','input[aria-label="搜索可选游戏"]','泡泡堂');browser('host','click','.choose-games button')
    browser('host','wait','select[aria-label="水泡战场地图"]')
    choices=js('host','[...document.querySelectorAll("select[aria-label=水泡战场地图] option")].map(e=>e.value)')
    groups=js('host','[...document.querySelectorAll("select[aria-label=水泡战场地图] optgroup")].map(e=>e.label)')
    originals=json.loads((root/'games/bubbleparty/data/classic-2002.json').read_text())['maps']
    assert len(choices)==117 and len(set(choices))==117,choices
    assert len(groups)==9 and all('2002' not in g and '现有' not in g for g in groups),groups
    placement=js('host', '[...document.querySelectorAll("select[aria-label=水泡战场地图] option")].map(e=>({id:e.value,group:e.parentElement.label,text:e.textContent}))')
    for map_id, family in [('community10','小区'),('forest','森林'),('desert','沙漠'),('waterpark','海洋')]:
        assert next(m for m in placement if m['id']==map_id)['group']==family,placement
    assert set(choices)=={'community10','forest','desert','waterpark','random'}|{m['id'] for m in originals}
    for map_id in ['community10','forest','desert','waterpark']:
        select(map_id)
        wait(lambda:preview()['state']=='loaded')
        assert preview()['map']==map_id,preview()
        canvas=js('host', '(()=>{const c=document.querySelector(".bubble-map-preview canvas");const a=c?.getContext("2d").getImageData(0,0,c.width,c.height).data;let colors=new Set();if(a)for(let i=0;i<a.length;i+=4)colors.add(a.slice(i,i+3).join());return {width:c?.width,height:c?.height,colors:colors.size}})()')
        assert canvas['colors']>100 and canvas['width']==600 and canvas['height']==520,canvas
        assert all(text not in preview()['text'] for text in ['2002','重建','国服']),preview()
        record('actual game sprite preview '+map_id)
        shot(map_id+'-desktop')
        browser('host','set','viewport','320','844')
        p=preview();assert p['left']>=0 and p['right']<=320 and p['scrollWidth']<=322,p
        shot(map_id+'-320')
        browser('host','set','viewport','1280','1000')
        assert not preview().get('src'),preview()
    select('random');assert preview()['state']=='random' and not preview().get('src'),preview()
    record('original four maps and random remain truthful without classic thumbnails')
    results=[]
    for original in [m for m in originals if m["id"] != "classic2002-village09"]:
        select(original['id'])
        wait(lambda:preview()['state']=='loaded')
        p=preview();assert p['map']==original['id'] and p['complete'],p
        assert p['width']==original['preview']['width'] and p['height']==original['preview']['height'],p
        assert p['src'].endswith('/'+original['preview']['file']),p
        assert all(text not in p['text'] for text in ['2002','留存','重建','国服']),p
        results.append({'id':p['map'],'width':p['width'],'height':p['height'],'src':p['src']})
    (out/'all-previews.json').write_text(json.dumps(results,ensure_ascii=False,indent=2)+'\n')
    record('first 111 verified maps render their own original-resource previews')
    select('classic2002-village08');wait(lambda:preview()['state']=='loaded');shot('classic-create-desktop')
    browser('host','set','viewport','320','844')
    p=preview();assert p['left']>=0 and p['right']<=320 and p['scrollWidth']<=322,p
    shot('classic-create-320');record('320px preview keeps original aspect and fits dialog')
    browser('host','network','route',base+'/bubbleparty/v4/classic/previews/classic2002-village09.png','--abort')
    # Reload also verifies the app can recover its form and remain usable.
    browser('host','reload');browser('host','wait','.room-list')
    browser('host','click','.lobby-create');browser('host','wait','[role=dialog]')
    browser('host','fill','input[aria-label="搜索可选游戏"]','泡泡堂');browser('host','click','.choose-games button')
    browser('host','wait','select[aria-label="水泡战场地图"]');select('classic2002-village09')
    wait(lambda:preview()['state']=='error');assert '暂时无法显示' in preview()['text'],preview()
    shot('classic-failed-320');record('failed preview gives explicit readable fallback, no stale map')
    browser('host','network','unroute',base+'/bubbleparty/v4/classic/previews/classic2002-village09.png')
    select('classic2002-village08');wait(lambda:preview()['state']=='loaded')
    select('classic2002-village09');wait(lambda:preview()['state']=='loaded')
    record('switching maps after failure loads new and previously failed image')
    original=next(m for m in originals if m['id']=='classic2002-village09');p=preview()
    assert p['width']==original['preview']['width'] and p['height']==original['preview']['height'],p
    results.append({'id':p['map'],'width':p['width'],'height':p['height'],'src':p['src']})
    assert len(results)==112
    (out/'all-previews.json').write_text(json.dumps(results,ensure_ascii=False,indent=2)+'\n')
    record('all 112 verified map previews loaded, including recovery after failure')
    browser('host','fill','input[name=name]','经典地图预览验收')
    browser('host','click','[role=dialog] button[type=submit]');browser('host','wait','.waiting-layout')
    wait(lambda:preview()['state']=='loaded');assert room()['options']['map']=='classic2002-village09'
    assert preview()['map']=='classic2002-village09';shot('classic-waiting-320')
    record('real create persists selected classic map and waiting preview')
    select('classic2002-ocean00');wait(lambda:preview()['state']=='loaded')
    click('host','保存并准备');wait(lambda:room()['options']['map']=='classic2002-ocean00')
    assert room()['seats'][0]['ready'];shot('classic-switched-320')
    record('waiting map switch saves new classic ID and matching preview')
    report['passed']=True
except BaseException:
    report['error']=traceback.format_exc();print(report['error'],flush=True)
finally:
    for who in opened:
        try:browser(who,'close')
        except BaseException:report['passed']=False
    if server:
        (out/'stop').write_text('stop')
        try:server.wait(timeout=20)
        except subprocess.TimeoutExpired:server.kill();server.wait();report['passed']=False
        if server.returncode:report['passed']=False
    if log:log.close()
    for path in [temp,sockets,out/'data']:
        if path.exists():shutil.rmtree(path)
    for name in ['url','stop']:(out/name).unlink(missing_ok=True)
    (out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(out/'report.json',flush=True)
    if not report['passed']:raise SystemExit(1)
