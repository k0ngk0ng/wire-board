#!/usr/bin/env python3
"""Real-browser catalogue acceptance with 30 additional registered games.

Failure cases declared before UI changes: squeezed labels/mobile overflow;
unreachable choices with many games; search has wrong/empty results; filtering
loses rooms; switching legacy/module creation submits the wrong game; admin
search hides the wrong publication control. Temporary modules never ship.
Lobby failures: cards displace rooms below the fold; search/category/status
filters conflict; clearing filters loses rooms; creation exposes unpublished
games or selects another game; long names overflow or have low contrast.
Run: . scripts/env.sh && python3 -B scripts/accept_game_filters.py
Evidence: .local/game-filters/browser/result.json and screenshots.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.request
import uuid

root = Path(__file__).resolve().parents[1]
out = root / '.local/game-filters/browser'
out.mkdir(parents=True, exist_ok=True)
env = dict(os.environ)
for key, relative in {'GOCACHE': '.local/go-build', 'GOMODCACHE': '.local/go-mod', 'GOTMPDIR': '.local/tmp', 'TMPDIR': '.local/tmp', 'npm_config_cache': '.local/npm', 'AGENT_BROWSER_SOCKET_DIR': '.local/ab-filters', 'AGENT_BROWSER_PROFILE': '.local/game-filters/browser/profile'}.items():
    path = root / relative
    path.mkdir(parents=True, exist_ok=True)
    env[key] = str(path)
env.update(WIRE_BOARD_MODULE_BROWSER='1', WIRE_BOARD_CATALOG_BROWSER='1', GOTOOLCHAIN='local', AGENT_BROWSER_SESSION='filters-' + uuid.uuid4().hex[:8], AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
report = {'passed': False, 'checks': [], 'command': '. scripts/env.sh && python3 -B scripts/accept_game_filters.py', 'extraGames': 30}
fixtures = []
server = None
opened = False

def run(args, log):
    with (out / log).open('w') as f:
        subprocess.run(args, cwd=root, env=env, stdout=f, stderr=subprocess.STDOUT, check=True)

def b(*args):
    result = subprocess.run(['agent-browser', *args, '--json'], cwd=root, env=env, capture_output=True, text=True, timeout=40)
    if result.returncode: raise RuntimeError(result.stdout + result.stderr)
    value = json.loads(result.stdout)
    assert value['success'], value
    return value['data']

def js(code): return b('eval', code)['result']

def api(path, body=None):
    options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    r = js('fetch(' + json.dumps('/api' + path) + options + ').then(async r=>({status:r.status,body:await r.json()}))')
    assert 200 <= r['status'] < 300, r
    return r['body']

def shot(name):
    b('wait', '--fn', 'document.querySelector(".modal")?.getAnimations().every(a=>a.playState!=="running") ?? true')
    assert not b('errors')['errors'], b('errors')
    assert js('document.documentElement.scrollWidth <= innerWidth + 2'), name
    b('screenshot', str(out / (name + '.png')), *([] if js('!!document.querySelector("[role=dialog]")') else ['--full']))
    colors = js(r"""(() => {
      const rgb=s=>(s.match(/[\d.]+/g)||[]).map(Number);
      const blend=(a,b,o)=>a.slice(0,3).map((v,i)=>v*o+b[i]*(1-o));
      const lum=c=>c.map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((s,v,i)=>s+v*[.2126,.7152,.0722][i],0);
      return [...document.querySelectorAll('.game-search,.game-search input,.room-filters select,.game-choice-summary,.choose-games > button > span,.game-choice-description,.game-choice-players,.room-list h1,.room-list .section-heading p,.room-title h3,.room-title small,.room-row .status,.room-row .join-button,.room-results,.room-filters label,.room-filters input,.lobby-create')].map(e=>{
        const chain=[];for(let n=e;n;n=n.parentElement)chain.unshift(n);
        let bg=[255,255,255];for(const n of chain){const c=rgb(getComputedStyle(n).backgroundColor);bg=blend(c,bg,c[3]??1)}
        const fg=rgb(getComputedStyle(e).color),a=lum(fg),b=lum(bg);
        return {text:e.getAttribute('aria-label')||e.textContent.trim(),ratio:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
      });
    })()""")
    assert all(c['ratio'] >= 4.5 for c in colors), colors
    (out / (name + '-contrast.json')).write_text(json.dumps(colors,ensure_ascii=False,indent=2))
    report['checks'].append(name)
    print('PASS', name, flush=True)

def check_room_viewport(width):
    b('set', 'viewport', str(width), '740')
    js('scrollTo(0,0)')
    layout = js("""(() => {
      const rect=s=>document.querySelector(s)?.getBoundingClientRect().toJSON();
      return {heading:rect('.room-list h1'),create:rect('.lobby-create'),
        firstRoom:rect('.room-row'),join:rect('.room-row .join-button'),
        cards:document.querySelectorAll('.lobby .game-feature,.lobby .choose-games').length,
        width:innerWidth,height:innerHeight};
    })()""")
    assert layout['cards'] == 0, 'game catalogue still displaces rooms'
    for key in ['heading', 'create', 'firstRoom', 'join']:
        r = layout[key]
        assert r and r['top'] >= 0 and r['bottom'] <= layout['height'], layout
        assert r['left'] >= 0 and r['right'] <= layout['width'], layout
    (out / f'room-first-{width}.json').write_text(json.dumps(layout,indent=2))
    shot('room-first-' + str(width))


try:
    source = (root / 'internal/server/testdata/module-ui.tsx').read_text()
    for i in range(31):
        kind = 'acceptance' if i == 0 else f'catalog-{i:02d}'
        folder = root / 'games' / ('_filter_' + kind)
        if folder.exists(): raise RuntimeError('Refusing to overwrite ' + str(folder))
        (folder / 'web').mkdir(parents=True)
        fixtures.append(folder)
        name = '接入验收' if i == 0 else f'目录验收 {i:02d} · 很长的中文游戏名称'
        (folder / 'web/module.tsx').write_text(source.replace('"acceptance"', json.dumps(kind)).replace('接入验收', name))
    run(['npm', '--prefix', 'web', 'run', 'build'], 'build-fixture.log')
    for name in ['url', 'stop']: (out / name).unlink(missing_ok=True)
    with (out / 'server.log').open('w') as log:
        server = subprocess.Popen(['go', 'test', '-count=1', '-timeout=11m', '-run', '^TestModuleBrowserFixture$', './internal/server'], cwd=root, env=env, stdout=log, stderr=subprocess.STDOUT)
    deadline = time.monotonic() + 90
    while not (out / 'url').exists():
        if server.poll() is not None or time.monotonic() > deadline: raise RuntimeError('See server.log')
        time.sleep(.1)
    base = (out / 'url').read_text()
    b('open', base); opened = True
    b('tab', next(t['tabId'] for t in b('tab')['tabs'] if t['active']))
    api('/login', {'name': 'ModuleRoot', 'password': 'module-root-test-123'})
    assert not any(k.startswith('catalog-') for k in api('/state')['availableGames'])
    native = {
        'splendor': {'name': '璀璨宝石', 'players': '2–4 人'},
        'rail': {'name': '铁路环游', 'players': '2–5 人'},
        'catan': {'name': '卡坦岛', 'players': '2–6 人'},
        'carcassonne': {'name': '卡卡颂', 'players': '2–5 人'},
        'sanguosha': {'name': '三国杀', 'players': '4–8 人'},
        # Three/five players are not supported: a range would mislead users.
        'dota': {'name': '兵线争锋', 'players': '2 / 4 / 6 人'},
        'monopoly': {'name': '强手棋', 'players': '2–6 人'},
        'starfort': {'name': '星垒', 'players': '1–4 人'},
        'doudizhu': {'name': '斗地主', 'players': '4 人'},
        'guandan': {'name': '掼蛋', 'players': '4 人'},
        'shengji': {'name': '升级', 'players': '4 人'},
        'snooker': {'name': '斯诺克', 'players': '2 / 4 人'},
        'pairs': {'name': '连连看', 'players': '1–6 人'},
    }
    for kind in native: api('/admin/games/' + kind, {'enabled': True})
    b('reload'); b('wait', '.room-list')
    # Failure inventory (before implementation): room list/CTA below fold;
    # long names overflow; filters conflict; empty results cannot reset;
    # hidden games appear in picker; legacy/module switch creates wrong kind.
    for i in range(1, 31): api(f'/admin/games/catalog-{i:02d}', {'enabled': True})
    for kind in ['splendor', 'rail']:
        peer = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        name = '筛选测试 ' + kind + (' · 很长的房间名称用来检查手机换行' if kind == 'rail' else '')
        for path, body in [('/register', {'name': '目录' + kind, 'password': 'filter-test-only', 'invite': 'test-invite'}), ('/rooms', {'kind': kind, 'name': name, 'capacity': 2})]:
            with peer.open(urllib.request.Request(base + '/api' + path, data=json.dumps(body).encode(), headers={'Content-Type': 'application/json'})) as r: assert 200 <= r.status < 300
    b('reload'); b('wait', '.room-row')
    for width in [1280, 320]: check_room_viewport(width)
    assert js('document.querySelectorAll("select[aria-label=按游戏筛选牌桌] option").length>=37')
    b('select', 'select[aria-label="按游戏筛选牌桌"]', 'splendor')
    b('wait', '--fn', 'document.querySelectorAll(".room-row").length===1 && document.querySelector(".room-row").textContent.includes("splendor")')
    b('fill', 'input[aria-label="搜索房间"]', 'rail')
    b('wait', '--text', '没有符合条件的房间')
    shot('room-empty-filter-mobile')
    b('find', 'role', 'button', 'click', '--name', '清除筛选', '--exact')
    b('wait', '--fn', 'document.querySelectorAll(".room-row").length===2')
    b('fill', 'input[aria-label="搜索房间"]', ' RAIL ')
    b('wait', '--fn', 'document.querySelectorAll(".room-row").length===1 && document.querySelector(".room-row").textContent.includes("rail")')
    room_id = next(r['id'] for r in api('/state')['rooms'] if r['kind'] == 'splendor')
    b('fill', 'input[aria-label="搜索房间"]', room_id.upper())
    b('wait', '--fn', 'document.querySelectorAll(".room-row").length===1 && document.querySelector(".room-row").textContent.includes("splendor")')
    b('fill', 'input[aria-label="搜索房间"]', ' '); b('press', 'Backspace')
    b('select', 'select[aria-label="房间状态"]', 'playing')
    b('wait', '--text', '没有符合条件的房间')
    b('find', 'role', 'button', 'click', '--name', '清除筛选', '--exact')
    b('select', 'select[aria-label="按游戏筛选牌桌"]', 'rail')
    b('select', 'select[aria-label="游戏分类"]', 'realtime')
    assert js('document.querySelector("select[aria-label=按游戏筛选牌桌]").value') == 'all'
    b('wait', '--text', '没有符合条件的房间')
    b('find', 'role', 'button', 'click', '--name', '清除筛选', '--exact')
    b('click', '.lobby-create')
    b('wait', 'input[aria-label="搜索可选游戏"]')
    assert js('document.querySelector(".choose-games").scrollHeight>document.querySelector(".choose-games").clientHeight'), 'choice list must scroll'
    assert js('[...document.querySelectorAll(".choose-games > button")].every(e=>e.getBoundingClientRect().width>=100)'), 'squeezed game cards'
    assert js('document.querySelectorAll(".choose-games > button").length') == len(api('/state')['availableGames'])
    assert not js('document.querySelector(".choose-games").textContent.includes("接入验收")'), 'unpublished game in picker'
    shot('create-many-games-mobile')
    for kind, meta in native.items():
        b('fill', 'input[aria-label="搜索可选游戏"]', meta['name'])
        b('click', '.choose-games button')
        b('wait', '--fn', 'document.querySelector(".choose-games .selected") !== null')
        assert js('document.querySelector(".choose-games .selected > span").textContent.trim()') == meta['name']
        assert js('!!document.querySelector(".choose-games .selected .cover")')
        assert js('document.querySelector(".game-choice-players").textContent.trim()') == meta['players']
        assert js('document.querySelector(".create-selected-game p").textContent.trim().length>0')
        assert js('document.querySelector("[data-create-step=setup]").hidden===false')
        b('find','role','button','click','--name','返回选择游戏','--exact')
    for width in [1280, 320]:
        b('set', 'viewport', str(width), '740')
        shot('create-picker-' + str(width))
    b('fill', 'input[aria-label="搜索可选游戏"]', '没有这款游戏')
    b('wait', '--text', '没有找到匹配的游戏')
    assert js('document.querySelectorAll(".choose-games > button").length===0')
    b('fill', 'input[aria-label="搜索可选游戏"]', ' '); b('press', 'Backspace')
    b('select', 'select[aria-label="可选游戏分类"]', 'realtime')
    assert js('[...document.querySelectorAll(".choose-games > button > span")].map(e=>e.textContent.trim())') == ['星垒']
    b('select', 'select[aria-label="可选游戏分类"]', 'all')
    b('fill', 'input[aria-label="搜索可选游戏"]', '目录验收 30')
    b('press', 'Enter')
    assert api('/state').get('room') is None, 'search Enter accidentally created a room'
    assert js('document.querySelectorAll("[role=dialog]").length===1')
    b('click', '.choose-games button')
    b('wait', 'select[aria-label="座位上限"]')
    assert js('document.querySelector(".create-selected-game").textContent.includes("30")')
    b('find','role','button','click','--name','返回选择游戏','--exact')
    b('fill', 'input[aria-label="搜索可选游戏"]', '璀璨宝石')
    b('click', '.choose-games button')
    b('wait', '--fn', 'document.querySelector(".create-selected-game")?.textContent.includes("璀璨宝石")')
    b('find','role','button','click','--name','返回选择游戏','--exact')
    b('fill', 'input[aria-label="搜索可选游戏"]', '目录验收 30')
    b('click', '.choose-games button')
    b('wait', 'select[aria-label="座位上限"]')
    shot('create-module-selected-mobile')
    b('scrollintoview', '[role=dialog] button[type=submit]')
    b('wait', '--fn', '(()=>{const e=document.querySelector("[role=dialog] button[type=submit]"),r=e.getBoundingClientRect();return e.contains(document.elementFromPoint(r.x+r.width/2,r.y+r.height/2))})()')
    b('click', '[role=dialog] button[type=submit]')
    b('wait', '.waiting-actions')
    assert api('/state')['room']['kind'] == 'catalog-30'
    shot('correct-game-created')
    # The same game search must remain usable in the management catalogue.
    api('/logout', {})
    api('/login', {'name': 'ModuleRoot', 'password': 'module-root-test-123'})
    b('reload'); b('wait', '--text', '管理后台')
    b('find', 'role', 'button', 'click', '--name', '管理后台', '--exact')
    b('wait', 'input[aria-label="搜索管理游戏"]')
    b('fill', 'input[aria-label="搜索管理游戏"]', '目录验收 30')
    assert js('document.querySelectorAll(".admin-game-card").length===1 && document.querySelector(".admin-game-card").textContent.includes("30")')
    assert js('document.querySelector(".admin-game-card").getBoundingClientRect().width>=250'), 'admin game name squeezed on mobile'
    shot('admin-filter-mobile')
    b('fill', 'input[aria-label="搜索管理游戏"]', '不存在')
    b('wait', '--text', '没有找到匹配的游戏')
    assert js('document.querySelectorAll(".admin-game-card").length===0')
    b('fill', 'input[aria-label="搜索管理游戏"]', ' ')
    b('press', 'Backspace')
    assert js('document.querySelectorAll(".admin-game-card").length>=37')
    b('find', 'role', 'button', 'click', '--name', '我的战绩与好友', '--exact')
    b('wait', '.profile-page')
    assert js('document.querySelectorAll(".profile-stats article").length===6'), 'profile lists every game before history'
    shot('profile-mobile')
    b('scrollintoview', '.profile-more-games')
    b('click', '.profile-more-games')
    assert js('document.querySelectorAll(".profile-stats article").length===12')
    # Live unpublication must reset a removed selection and disable creation
    # only when there are no published games; show the preparation empty state.
    current = api('/state')['room']
    api('/rooms/' + current['id'], {'type': 'leave', 'version': current['version'], 'nonce': uuid.uuid4().hex})
    b('open', base); b('wait', '.room-list')
    b('select', 'select[aria-label="按游戏筛选牌桌"]', 'rail')
    for kind in api('/state')['availableGames']: api('/admin/games/' + kind, {'enabled': False})
    b('wait', '--fn', 'document.querySelector(".lobby-create").disabled && document.querySelector("select[aria-label=按游戏筛选牌桌]").value==="all"')
    b('wait', '--text', '游戏正在准备中')
    shot('no-published-games')
    report['passed'] = True
except Exception as e:
    import traceback
    traceback.print_exc()
    report['error'] = repr(e)
    if opened:
        try:
            report['snapshot'] = b('snapshot', '-i')
            report['state'] = api('/state')
            report['form'] = js('(()=>{const e=document.querySelector("[role=dialog] button[type=submit]");return e?{button:e.getBoundingClientRect().toJSON(),valid:e.form.checkValidity(),dialogs:document.querySelectorAll("[role=dialog]").length}:null})()')
            b('screenshot', str(out / 'failure.png'), '--full')
        except Exception: pass
finally:
    if opened:
        try: b('close')
        except Exception: pass
    if server:
        (out / 'stop').write_text('stop')
        try: server.wait(timeout=15)
        except subprocess.TimeoutExpired: server.terminate(); server.wait(timeout=10)
    for folder in fixtures: shutil.rmtree(folder)
    try: run(['npm', '--prefix', 'web', 'run', 'build'], 'build-production.log')
    except Exception as e: report['passed'] = False; report['cleanupError'] = str(e)
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print('Artifact:', out / 'result.json', flush=True)
    if not report['passed']: raise SystemExit(1)
