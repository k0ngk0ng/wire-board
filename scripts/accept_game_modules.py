#!/usr/bin/env python3
"""Build and exercise an independently registered game plus all six migrated UIs.

Uses only test accounts, normal HTTP commands and real browser interactions.
The temporary game is never shipped. All caches/profiles/results stay in .local.
Failure cases: discovery/creation failures, lost custom action fields, stale UI,
missing privacy projection, broken asset paths, JS errors, mobile overflow, or
result changes after reload. Run from the repository root with agent-browser.
"""
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

root = Path(__file__).resolve().parents[1]
out = root / '.local/module-refactor/browser'
out.mkdir(parents=True, exist_ok=True)
fixture = root / 'games/_module_acceptance'
if fixture.exists():
    raise SystemExit(f'Refusing to overwrite {fixture}')
env = dict(os.environ)
for key, value in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/abmodules', 'AGENT_BROWSER_PROFILE': '.local/module-refactor/browser/profile'}.items():
    location = root / value
    location.mkdir(parents=True, exist_ok=True)
    env[key] = str(location)
env['AGENT_BROWSER_PROFILE']=str(out/'profiles'/uuid.uuid4().hex[:8])
Path(env['AGENT_BROWSER_PROFILE']).mkdir(parents=True,exist_ok=True)
env.update(GOTOOLCHAIN='local', WIRE_BOARD_MODULE_BROWSER='1', AGENT_BROWSER_SESSION='module-' + uuid.uuid4().hex[:8], AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'checks': [], 'command': 'python3 -B scripts/accept_game_modules.py'}
server = None
browser_open = False
log_handles = []


def run(command, log):
    with (out / log).open('w') as handle:
        result = subprocess.run(command, cwd=root, env=env, stdout=handle, stderr=subprocess.STDOUT)
    if result.returncode:
        raise RuntimeError(f'{command} failed; see {out / log}')


def b(*command):
    result = subprocess.run(['agent-browser', *command, '--json'], cwd=root, env=env, capture_output=True, text=True, timeout=35)
    if result.returncode:
        raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    if not value['success']:
        raise RuntimeError(str(value))
    return value['data']


def js(code):
    return b('eval', code)['result']


def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    value = js('fetch(' + json.dumps(path) + options + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200 <= value['status'] < 300, value
    return value['body']


def room():
    return api('/api/state')['room']


def command(kind, **extra):
    for _ in range(10):
        r = room()
        body = dict(type=kind, version=r['version'], nonce=uuid.uuid4().hex, **extra)
        response = js('fetch(' + json.dumps('/api/rooms/' + r['id']) + ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}).then(async r=>({status:r.status,body:await r.json()}))')
        if response['status'] == 409:
            continue
        assert response['status'] == 200, response
        return response['body']
    raise AssertionError('version conflict did not settle')


def checkpoint(name):
    assert not b('errors')['errors'], b('errors')
    assert js('document.documentElement.scrollWidth <= innerWidth + 2'), 'page overflow: ' + name
    assert js('[...document.images].every(i=>!i.complete || i.naturalWidth>0)'), 'broken image: ' + name
    b('screenshot', str(out / (name + '.png')), '--full')
    (out / (name + '.snapshot.json')).write_text(json.dumps(b('snapshot', '-i'), ensure_ascii=False, indent=2))
    report['checks'].append(name)
    print('PASS', name, flush=True)

def check_map_navigation(kind, width):
    viewport='.car-viewport' if kind=='carcassonne' else ('.catan-map-scroll' if kind=='catan' else '.rail-board .map-scroll')
    selector=viewport+' svg'
    if js('[...document.querySelectorAll("button")].some(e=>e.textContent.trim()==="收起看地图")'):
        b('find','role','button','click','--name','收起看地图','--exact')
    b('scrollintoview',viewport)
    r=js('document.querySelector('+json.dumps(viewport)+').getBoundingClientRect().toJSON()')
    x,y=round(r['x']+r['width']/2),round(r['y']+r['height']/2)
    metric='document.querySelector('+json.dumps(selector)+').viewBox.baseVal.width' if kind=='carcassonne' else 'document.querySelector('+json.dumps(selector)+').getBoundingClientRect().width'
    before=js(metric)
    subprocess.run(['node','scripts/browser_wheel.mjs',b('get','cdp-url')['cdpUrl'],base,str(x),str(y),'-300'],cwd=root,env=env,check=True,timeout=15)
    b('wait','--fn',metric+('<' if kind=='carcassonne' else '>')+str(before))
    pan='document.querySelector('+json.dumps(selector)+').getAttribute("viewBox")' if kind=='carcassonne' else 'JSON.stringify([document.querySelector('+json.dumps(viewport)+').scrollLeft,document.querySelector('+json.dumps(viewport)+').scrollTop])'
    origin=js(pan)
    b('mouse','move',str(x),str(y));b('mouse','down');b('mouse','move',str(x-55),str(y-20));b('mouse','up')
    assert js(pan)!=origin, ('map did not pan',kind,width)
    checkpoint(kind+'-wheel-drag-'+str(width))

def check_splendor_contrast():
    # Actual computed foreground/background after all game styles load.
    contrast = js(r"""(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number);
      const blend=(a,b,o)=>a.slice(0,3).map((v,i)=>v*o+b[i]*(1-o));
      const lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return ['.reserved > span','.reserved > p','.bank-actions .subtle','.bank-actions .primary','.pass-button','.wallet-caption > span','.wallet-caption small','.wallet-caption > b'].flatMap(selector=>[...document.querySelectorAll('.splendor-board '+selector)].filter(e=>e.getClientRects().length).map(e=>{
        const chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);
        let bg=[255,255,255],opacity=1;for(const n of chain){const c=getComputedStyle(n),a=rgb(c.backgroundColor);bg=blend(a,bg,a[3]??1);opacity*=Number(c.opacity)}
        const fg=blend(rgb(getComputedStyle(e).color),bg,opacity),a=lum(fg),b=lum(bg);
        return {selector,text:e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05),fg,bg};
      }));
    })()""")
    (out / ('splendor-contrast-' + str(js('innerWidth')) + '.json')).write_text(json.dumps(contrast,ensure_ascii=False,indent=2))
    assert len(contrast)>=3 and all(c['ratio']>=4.5 for c in contrast), contrast


try:
    for name in ('url', 'stop'):
        (out / name).unlink(missing_ok=True)
    (fixture / 'web').mkdir(parents=True)
    shutil.copyfile(root / 'internal/server/testdata/module-ui.tsx', fixture / 'web/module.tsx')
    run(['npm', '--prefix', 'web', 'run', 'build'], 'build-fixture.log')
    handle = (out / 'server.log').open('w'); log_handles.append(handle)
    server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=11m', '-run', '^TestModuleBrowserFixture$', './internal/server'], cwd=root, env=env, stdout=handle, stderr=subprocess.STDOUT)
    started = time.monotonic()
    while not (out / 'url').exists():
        if server.poll() is not None or time.monotonic() - started > 90:
            raise RuntimeError('fixture failed to start; see server.log')
        time.sleep(.1)
    base = (out / 'url').read_text()
    b('open', base); browser_open = True
    main_tab = next(t['tabId'] for t in b('tab')['tabs'] if t['active'])
    b('tab', main_tab)
    api('/api/login', {'name': 'ModuleRoot', 'password': 'module-root-test-123'})
    api('/api/admin/games/acceptance', {'enabled': True})
    api('/api/logout', {})
    api('/api/register', {'name': '模块浏览器验收', 'password': 'module-browser-test-only', 'invite': 'test-invite'})
    b('reload')
    b('wait', '.room-list')
    b('wait', '.sound-controls')
    js('window.__platformPlayed=[];window.__platformFailures=[];const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return play.call(this).then(()=>{if(src.length>100||!src.startsWith("data:"))window.__platformPlayed.push({src,platform:this.hasAttribute("data-platform-audio")})},e=>{window.__platformFailures.push({src:src.slice(0,160),name:e.name,message:e.message,hidden:document.hidden});throw e})}')
    b('find','role','button','click','--name','试听音效','--exact')
    b('wait','--fn','window.__platformPlayed.length>0')
    assert js('window.__platformPlayed.every(p=>p.platform)'), 'preview bypassed the platform media player'
    b('find','role','button','click','--name','静音','--exact')
    assert js('document.querySelector("audio[data-platform-audio]").paused && document.querySelector("audio[data-platform-audio]").muted')
    assert js('document.querySelector(".sound-preview").disabled && localStorage.getItem("wb_sound")==="off"')
    b('reload'); b('wait','.sound-controls')
    assert js('document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="true" && document.querySelector(".sound-preview").disabled'), 'mute preference lost after reload'
    b('set','viewport','320','740'); checkpoint('platform-mute-mobile')
    b('tab','new',base); b('wait','.sound-controls')
    other_tab = next(t['tabId'] for t in b('tab')['tabs'] if t['active'])
    b('find','role','button','click','--name','取消静音','--exact')
    b('tab',main_tab); b('wait','--fn','document.querySelector(".sound-toggle").getAttribute("aria-pressed")==="false"')
    assert js('document.querySelector("audio[data-platform-audio]").paused'), 'cross-tab unmute replayed old audio'
    b('tab','close',other_tab)
    checkpoint('platform-mute-cross-tab')
    b('set', 'viewport', '1280', '900')
    assert js('[...document.querySelectorAll(".room-filters option")].some(b=>b.textContent==="接入验收")'), 'module missing from lobby filters'
    checkpoint('lobby-desktop')
    b('click', '.lobby-create')
    b('wait', '[role=dialog]')
    b('wait', '--fn', 'document.querySelector(".modal").getAnimations().every(a=>a.playState!=="running")')
    b('fill', 'input[aria-label=搜索可选游戏]', '璀璨宝石')
    b('click', '.choose-games button')
    b('wait', '.choose-games')
    b('wait', '--fn', 'document.querySelector(".modal").getAnimations().every(a=>a.playState!=="running")')
    b('fill', 'input[aria-label=搜索可选游戏]', '接入验收')
    b('click', '.choose-games button')
    b('wait', 'select[aria-label=座位上限]')
    b('fill', 'input[name=name]', '模块浏览器牌桌')
    b('wait', '--fn', 'document.querySelector(".modal").getAnimations().every(a=>a.playState!=="running")')
    b('scrollintoview', '[role=dialog] button[type=submit]')
    b('click', '[role=dialog] button[type=submit]')
    b('wait', '.waiting-actions')
    b('find', 'role', 'button', 'click', '--name', '添加电脑玩家', '--exact')
    b('wait', '--fn', '!document.querySelector(".waiting-actions button")?.disabled')
    b('click', '.waiting-actions > button:first-child')
    b('wait', '--fn', '[...document.querySelectorAll(".waiting-actions button")].some(b=>b.textContent==="开始游戏" && !b.disabled)')
    b('click', '.waiting-actions > button:last-child')
    b('wait', '.acceptance-board')
    assert js('document.querySelector("[data-testid=private-card]").textContent') == 'private-test-card'
    assert js('Number(document.querySelector(".turn-card time").getAttribute("aria-label").match(/[0-9]+/)[0])') > 0, 'deadline display uses the wrong clock'
    checkpoint('module-board-desktop')
    b('set', 'viewport', '390', '844')
    checkpoint('module-board-mobile')
    b('find', 'role', 'button', 'click', '--name', '对局日志')
    b('wait', '.journal-dialog')
    assert js('document.querySelector(".journal-dialog").textContent.includes("牌已洗好")'), 'empty module log is not renderable'
    b('click', '[role=dialog] button[aria-label=关闭]')
    js('window.__actionAudio=[];const play=HTMLMediaElement.prototype.play;HTMLMediaElement.prototype.play=function(){const src=this.src;return play.call(this).then(()=>{if(src.length>100||!src.startsWith("data:"))window.__actionAudio.push(src)})}')
    b('find', 'text', '增加一次', 'click', '--exact')
    b('wait', '--fn', 'document.querySelector(".acceptance-board h2")?.textContent.includes("2")')
    b('wait','--fn','window.__actionAudio.length>0')
    b('find','role','button','click','--name','静音','--exact')
    before_sounds=js('window.__actionAudio.length')
    b('find', 'text', '增加一次', 'click', '--exact')
    b('wait', '.results')
    assert js('window.__actionAudio.length')==before_sounds, 'muted action or finish played sound'
    b('find','role','button','click','--name','取消静音','--exact')
    result = room()['result']
    b('reload'); b('wait', '.results')
    assert room()['result'] == result
    checkpoint('module-finished-reloaded')
    command('leave')
    for kind, players, selector in [('splendor', 2, '.splendor-board'), ('rail', 2, '.rail-board'), ('catan', 3, '.catan-board'), ('carcassonne', 2, '.car-board'), ('sanguosha', 4, '.sg-board'), ('dota', 2, '.dota-table')]:
        api('/api/rooms', {'kind': kind, 'name': kind + ' 模块验收', 'capacity': players})
        for _ in range(players - 1):
            command('add_bot')
        command('ready'); command('start')
        b('wait', selector)
        if kind == 'splendor': b('wait', '.bank-actions')
        b('set', 'viewport', '1280', '900'); checkpoint(kind + '-desktop')
        if kind == 'splendor': check_splendor_contrast()
        if kind in ['rail','catan','carcassonne']: check_map_navigation(kind,1280)
        b('set', 'viewport', '390', '844'); checkpoint(kind + '-mobile')
        if kind == 'splendor':
            check_splendor_contrast()
            b('click', '.token-button[data-bank-color="1"]')
            b('wait', '.token-button.chosen')
            check_splendor_contrast()
            assert js('[...document.querySelectorAll(".market-row .dev-card")].every(e=>{const r=e.getBoundingClientRect(),x=r.x+r.width/2,y=r.y+r.height/2;return y<0||y>innerHeight||e.contains(document.elementFromPoint(x,y))})'), 'mobile chat covers the card market'
            checkpoint('splendor-mobile-selected')
        if kind in ['rail','catan','carcassonne']:
            b('set','viewport','320','740');check_map_navigation(kind,320)
        assert room()['game']['kind'] == kind
        command('close'); command('leave')
    report['passed'] = True
except Exception as error:
    import traceback
    report['error'] = repr(error)
    traceback.print_exc()
    if browser_open:
        try:
            report['audioFailures'] = js('window.__platformFailures || []')
            report['mediaState'] = js('({hidden:document.hidden,visibility:document.visibilityState,media:[...document.querySelectorAll("audio")].map(a=>({src:a.src,paused:a.paused,muted:a.muted}))})')
            report['tabs'] = b('tab')
            report['browserErrors'] = b('errors')
            report['failureSnapshot'] = b('snapshot', '-i')
            report['failureState'] = api('/api/state')
            b('screenshot', str(out / 'failure.png'), '--full')
        except Exception as diagnostic_error:
            report['diagnosticError'] = str(diagnostic_error)
finally:
    if browser_open:
        try: b('close')
        except Exception as error: report['browserCloseError'] = str(error)
    if server:
        (out / 'stop').write_text('stop')
        try: server.wait(timeout=15)
        except subprocess.TimeoutExpired: server.terminate(); server.wait(timeout=10)
    for handle in log_handles: handle.close()
    shutil.rmtree(fixture)
    # Always restore a production build without the acceptance-only module.
    try: run(['npm', '--prefix', 'web', 'run', 'build'], 'build-production.log')
    except Exception as error:
        report['passed'] = False; report['productionBuildError'] = str(error)
    report['commit'] = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2))
    print('Artifact:', out / 'result.json', flush=True)
    if not report['passed']: raise SystemExit(1)
