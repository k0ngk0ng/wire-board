import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser()
parser.add_argument('mode', choices=['menu', 'battle', 'field'])
parser.add_argument('run', choices=['baseline', 'optimized'])
parser.add_argument('--scope')
parser.add_argument('--continuous', action='store_true')
parser.add_argument('--phone', action='store_true')
parser.add_argument('--delay', type=int, default=0, choices=[0, 100])
parser.add_argument('--keep-open', action='store_true')
args = parser.parse_args()
assert args.scope is None or re.fullmatch(r'[a-z0-9][a-z0-9-]{0,47}', args.scope)
assert args.mode != 'field' or args.continuous
assert not args.continuous or args.mode in ['field', 'battle']
assert not args.keep_open or (args.continuous and args.mode == 'battle' and args.scope)
local = ROOT / '.local/pal'
fixture = local / ('battle-board-browser' if args.mode == 'battle' else 'performance-board-browser')
base = (fixture / 'url').read_text().strip()
out = local / 'performance' / (f'{"continuous" if args.continuous else "selection"}-{args.mode}-{args.run}' + (f'-{args.scope}' if args.scope else ''))
out.mkdir(exist_ok=False)
socket_root = Path(subprocess.check_output(['git','rev-parse','--git-common-dir'],cwd=ROOT,text=True).strip()).resolve().parent/'.local/pal-lf-ab'
environment = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(socket_root), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_PROFILE=str(out / 'profile'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
Path(environment['AGENT_BROWSER_SOCKET_DIR']).mkdir(exist_ok=True)
session = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'ps'], cwd=ROOT, env=environment, text=True).strip() + '-' + args.mode[0]
if args.keep_open:
    session += '-' + hashlib.sha256(args.scope.encode()).hexdigest()[:6]
report = {'mode': args.mode, 'run': args.run, 'passed': False, 'measurementComplete': False, 'cases': [], 'budgets': {'p50': 33, 'p95': 50, 'max': 100}}


def process(*command, **options):
    return subprocess.Popen(command, cwd=ROOT, env=environment, text=True, **options)


def browser(*command):
    result = subprocess.run(['agent-browser', '--session', session, *map(str, command), '--json'], cwd=ROOT, env=environment, text=True, capture_output=True, timeout=45)
    assert result.returncode == 0, result.stderr + result.stdout
    value = json.loads(result.stdout)
    assert value['success'], value
    return value['data']


def js(code):
    return browser('eval', code)['result']


def api(path, body=None):
    options = '' if body is None else ', {method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
    response = js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
    assert 200 <= response['status'] < 300, response
    return response['body']


def room():
    return api('/state')['room']


def command(kind):
    state = room()
    return api('/rooms/' + state['id'], {'type': kind, 'version': state['version'], 'nonce': uuid.uuid4().hex})


def wait(check, label, seconds=30):
    end = time.monotonic() + seconds
    while time.monotonic() < end:
        found = check()
        if found:
            return found
        time.sleep(.05)
    raise AssertionError(label)


def selection():
    r = room()
    if r.get('loading'):
        return None
    view = r['game']['data']
    battle = view.get('battle')
    if args.mode == 'menu' and view['phase'] == 'menu' and view['menu']['kind'] == 1:
        return view['menu']['selected']
    if args.mode == 'battle' and view['phase'] == 'battle' and battle['state'] == 1 and battle['phase'] == battle['menu'] == 0:
        return battle['selected']
    return None


def paint():
    return js('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))')


def trace():
    observer.send_signal(signal.SIGUSR2)
    return json.loads(observer.stdout.readline())


def target_input(target, touch):
    index = targets.index(target)
    if not touch:
        browser('press', codes[index])
        return
    x, y = points[index]
    px, py = js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+' + str(x) + '*r.width/320,r.y+' + str(y) + '*r.height/200]})()')
    result = subprocess.run(['node', 'games/pal/tests/screen-touch.mjs', endpoint, proxied, 'tap', *map(str, [px, py, px, py])], cwd=ROOT, env=environment, text=True, capture_output=True, timeout=25)
    assert result.returncode == 0, result.stderr + result.stdout


def stats(samples):
    values = sorted(samples)
    return {'p50': values[int((len(values)-1)*.50+.5)], 'p95': values[int((len(values)-1)*.95+.5)], 'max': values[-1]}


def continuous():
    clock = time.time() * 1000
    def fresh_rtt():
        return [row for row in trace()['events'] if row.get('connectionSampledAt', 0) >= clock and row.get('connectionRTTMS') is not None and (150 <= row['connectionRTTMS'] <= 300 if args.delay else row['connectionRTTMS'] < 50)]
    proxy.stdin.write(json.dumps({'delayMS': args.delay})+'\n')
    proxy.stdin.flush()
    assert json.loads(proxy.stdout.readline())['delayMS'] == args.delay
    report['rtt'] = wait(fresh_rtt, 'original cadence needs a live ping measurement')
    report['phone'] = args.phone
    before = room()['game']['data']
    assert before['phase'] == args.mode, before['phase']
    if args.mode == 'battle':
        assert before['battle']['current'] == 0 and before['battle']['state'] == 1 and before['battle']['menu'] == 0 and len(before['party']) == 3
        direction = None
        report['originalArrowMaskPixels']=js('window.palContinuousPerformance.prepareArrow()')
        driver=process('node','games/pal/tests/lifecycle-driver.mjs',endpoint,proxied,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
        assert driver.stdout.readline().strip()=='ready'
        driver.stdin.write('{"type":"foreground"}\n')
        driver.stdin.flush()
        assert json.loads(driver.stdout.readline())=={'hidden':False,'focused':True}
        driver.communicate(timeout=5)
        assert driver.returncode==0
        report['displayWarmup']=js('''(async()=>{
            const gaps=[],times=[];
            for(let i=0;i<120;i++){
                times.push(await new Promise(r=>requestAnimationFrame(r)));
                if (times.length>1) gaps.push(times.at(-1)-times.at(-2));
                if(gaps.length>=8 && gaps.slice(-8).every(ms=>ms<=35)) return {ready:true,gaps};
            }
            return {ready:false,gaps};
        })()''')
        assert report['displayWarmup']['ready'], report['displayWarmup']
        js('window.palContinuousPerformance.begin("battle",null)')
        time.sleep(6)
        capture = js('window.palContinuousPerformance.end()')
        start, end = capture['started'] + 200, capture['started'] + 5700
    else:
        state = before['field']
        radius = state['radius']
        cells = base64.b64decode(state['cells'])
        def span(u, v):
            for n in range(1,radius+1):
                if cells[(v*n+radius)*(2*radius+1)+u*n+radius]:
                    return n-1
            return radius
        opts = [('ArrowUp', 0, -1, 16, -8), ('ArrowDown', 0, 1, -16, 8), ('ArrowRight', 1, 0, 16, 8), ('ArrowLeft', -1, 0, -16, -8)]
        key,u,v,dx,dy = max(opts,key=lambda row:span(row[1],row[2]))
        safe = span(u,v)
        report['route']={'safeCells':safe,'anchor':state['anchor'],'step':[dx,dy],'key':key}
        assert safe >= 5, report['route']
        direction = [dx,dy]
        js('window.palContinuousPerformance.begin("field",'+json.dumps(direction)+')')
        assert not args.phone
        driver=process('node','games/pal/tests/lifecycle-driver.mjs',endpoint,proxied,stdin=subprocess.PIPE,stdout=subprocess.PIPE)
        assert driver.stdout.readline().strip()=='ready'
        try:
            for down in [True,False]:
                driver.stdin.write(json.dumps({'type':'key','code':key,'down':down})+'\n')
                driver.stdin.flush()
                assert json.loads(driver.stdout.readline())['at']>0
                if down:
                    time.sleep(min(0.73,(safe-0.75)/10))
        finally:
            driver.communicate(timeout=5)
            assert driver.returncode==0
        time.sleep(.25+args.delay/1000)
        capture = js('window.palContinuousPerformance.end()')
        controls = [row for row in capture['controls'] if row['code'] == key]
        assert len(controls) == 2 and all(row['trusted'] for row in controls), controls
        assert [row['type'] for row in controls] == ['keydown','keyup'], controls
        start,end = controls[0]['at'],controls[-1]['at']
    after = room()['game']['data']
    samples = [row for row in capture['samples'] if start<=row['at']<=end]
    assert end<=capture['stopped'] and len(samples)>10 and all(row['focused'] and row['phase']==args.mode for row in samples)
    network = [row for row in trace()['events'] if capture['wall']+start<=row['at']<=capture['wall']+end]
    game = [row for row in network if row['type']=='game']
    assert game
    visible = [row['at'] for row in samples if row['changed']]
    render_gap = [b['at']-a['at'] for a,b in zip(samples,samples[1:])]
    visible_gap = [b-a for a,b in zip(visible,visible[1:])]
    rate = len(visible)*1000/(end-start)
    if args.mode=='battle':
        hashes=sorted({row['hash'] for row in samples})
        assert len(hashes)==2 and set(f for row in game for f in row.get('arrowFrames',[]))=={68,69}, hashes
        assert after['party']==before['party'] and after['battle']['menu']==0 and after['battle']['selected']==before['battle']['selected']
    else:
        delta=[after['x']-before['x'],after['y']-before['y']]
        steps=delta[0]//direction[0]
        assert 0<steps<=safe and delta==[steps*axis for axis in direction],delta
        time.sleep(.35)
        final=room()['game']['data']
        assert [after['x'],after['y']]==[final['x'],final['y']]
        report['route'].update({'authoritySteps':steps,'releasedAndStopped':True,'fieldFrameAfter':after['field']['frame'],'fieldFrameBefore':before['field']['frame']})
    budgets={'visibleRate':24 if args.mode=='battle' else 9,'visibleGapP95':80 if args.mode=='battle' else 150,'rafP95':35,'gameP95Bytes':16384 if args.mode=='battle' else 4096}
    values={'visibleRate':rate,'visibleGapP95':stats(visible_gap)['p95'],'rafP95':stats(render_gap)['p95'],'gameP95Bytes':stats([row['bytes'] for row in game])['p95']}
    passed=rate>=budgets['visibleRate'] and all(values[k]<=budgets[k] for k in ['visibleGapP95','rafP95','gameP95Bytes'])
    report.update({'measurementComplete':True,'passed':passed,'budgets':budgets,'values':values,'window':[start,end],'visibleFrames':visible,'nativeGameFrames':len(game),'capture':capture,'network':network})
    if args.run == 'optimized':
        timers=js('(async()=>{const r=await fetch("/pal-performance/metrics");return r.headers.get("content-type")?.includes("application/json")?await r.json():null})()')
        assert isinstance(timers,list), 'candidate needs fixture timer observations'
        report['timerSamples']=[row for row in timers if capture['wall']+start<=row['at']<=capture['wall']+end]
        assert report['timerSamples'] and all(not row['failed'] and row['players']==1 for row in report['timerSamples'])
    browser('screenshot',out/'canvas.png')
    print(json.dumps({'mode':args.mode,'run':args.run,'passed':passed,'visibleFrames':len(visible),'nativeGameFrames':len(game),**values}),flush=True)


proxy = process('node', 'games/pal/tests/performance-proxy.mjs', base, stdin=subprocess.PIPE, stdout=subprocess.PIPE)
proxied = json.loads(proxy.stdout.readline())['url']
observer = device = None
try:
    browser('open', proxied)
    browser('set', 'viewport', 1280, 900)
    api('/register', {'name': '仙剑选项响应' + uuid.uuid4().hex[:7], 'password': 'pal-selection-fixture-123', 'invite': 'test-invite'})
    endpoint = browser('get', 'cdp-url')['cdpUrl']
    observer = process('node', 'games/pal/tests/performance-observer.mjs', endpoint, proxied, 'continuous-performance-page.js' if args.continuous else 'selection-performance-page.js', stdout=subprocess.PIPE)
    assert observer.stdout.readline().strip() == 'ready'
    created = api('/rooms', {'kind': 'pal', 'name': '原版选项响应', 'capacity': 1})
    report['roomID'] = created['id']
    command('ready')
    command('start')
    browser('reload')
    browser('wait', '[data-pal-ready=true]')
    wait(lambda: not room().get('loading'), 'original single player surface blocked', 90)
    report['clientBundle'] = js('''(async()=>{
        const manifest=await (await fetch("/game-load-manifest.json")).json();
        const name=manifest.games.pal[0].url;
        const response=await fetch(name);
        if(!response.ok) throw Error("missing loaded PAL bundle");
        const bytes=await response.arrayBuffer();
        const digest=await crypto.subtle.digest("SHA-256",bytes);
        return {name,sha256:Array.from(new Uint8Array(digest),b=>b.toString(16).padStart(2,"0")).join(""),
                prediction:(new TextDecoder().decode(bytes).includes(":main-menu") || new TextDecoder().decode(bytes).includes("PALNET01")),
                bytes:bytes.byteLength};
    })()''')
    assert report['clientBundle']['prediction'] == (args.continuous or args.run == 'optimized'), report['clientBundle']
    browser('click', '[data-fullscreen-enter]')
    browser('wait', '[data-room-fullscreen]')
    browser('click', '[data-pal-board] canvas')
    if args.continuous:
        if args.mode == 'battle':
            wait(lambda: selection() == 0, 'original arrow actor not ready')
        if args.phone:
            browser('set','viewport',320,820)
            device=process('node','scripts/browser_device.mjs',endpoint,proxied,'320','820','--keep',stdout=subprocess.PIPE)
            assert device.stdout.readline().strip()=='ready'
            assert js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')==[320,200]
            assert not js('!!document.querySelector("[data-pal-controls] button")')
        continuous()
        if args.keep_open:
            ready = time.time()*1000
            (out/'peer-ready.json').write_text(json.dumps({'roomID':report['roomID'],'at':ready})+'\n')
            wait(lambda:(out/'peer-stop').is_file(),'second client did not end the shared measurement',110)
            peer_trace=trace()
            report['peerConnection']={'readyAt':ready,'endedAt':time.time()*1000,'openConnections':peer_trace['connections'],'events':[row for row in peer_trace['events'] if row['at']>=ready]}
        raise SystemExit(int(args.run=='optimized' and not report['passed']))
    if args.mode == 'menu':
        targets, areas, points, codes = [1, 2], [[16, 50, 48, 16], [16, 68, 48, 16]], [[24, 57], [24, 75]], ['ArrowUp', 'ArrowDown']
    else:
        targets, areas, points, codes = [0, 3], [[27, 140, 24, 15], [27, 170, 24, 15]], [[39, 147], [39, 177]], ['ArrowUp', 'ArrowDown']
    wait(lambda: selection() is not None, 'original expected menu did not open')
    view = room()['game']['data']
    report['projection'] = {'raster': bool(view.get('raster')), 'menu': view['menu']}
    colors = {}
    if args.mode == 'menu':
        effect = view['paletteEffect'] or {}
        assert not effect.get('factors') and not effect.get('redStep'), effect
        palette = json.loads((local / 'visual/pal/v1/palettes.json').read_bytes())[effect.get('palette', view['palette'])][int(effect.get('night', view['night']))]
        rgb = lambda index: int.from_bytes(bytes(palette[index]+[255]), 'little')
        colors = {'normal': rgb(79), 'highlights': [rgb(index) for index in range(249, 255)]}
    if args.mode == 'battle':
        for target in reversed(targets):
            if selection() != target:
                target_input(target, False)
                wait(lambda: selection() == target, 'calibration input did not reach native choice')
            paint()
            js('window.palSelectionPerformance.capture(' + json.dumps(target) + ',' + json.dumps(areas) + ')')
    for label, delay, touch in [('desktop', 0, False), ('desktop-rtt', 100, False), ('phone-rtt', 100, True), ('phone', 0, True)]:
        if touch and device is None:
            browser('set', 'viewport', 320, 820)
            device = process('node', 'scripts/browser_device.mjs', endpoint, proxied, '320', '820', '--keep', stdout=subprocess.PIPE)
            assert device.stdout.readline().strip() == 'ready'
            assert js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()') == [320, 200]
            assert not js('!!document.querySelector("[data-pal-controls] button")')
        proxy.stdin.write(json.dumps({'delayMS': delay}) + '\n')
        proxy.stdin.flush()
        assert json.loads(proxy.stdout.readline())['delayMS'] == delay
        began = time.time() * 1000
        def fresh_rtt():
            return [event for event in trace()['events'] if event.get('connectionSampledAt', 0) >= began and event.get('connectionRTTMS') is not None and (150 <= event['connectionRTTMS'] <= 300 if delay else event['connectionRTTMS'] < 50)]
        rtt = wait(fresh_rtt, 'no fresh Ping/Pong RTT for ' + label)
        samples = []
        for step in range(12):
            old = selection()
            assert old in targets, old
            old_index = targets.index(old)
            target = targets[1-old_index]
            spec = {'mode': args.mode, 'kind': 'touch' if touch else 'key', 'code': codes[1-old_index], 'areas': areas, 'oldIndex': old_index, 'target': target, **colors}
            if args.mode == 'menu':
                def menu_pixels_ready():
                    state = js('window.palSelectionPerformance.menuColors(' + json.dumps(areas) + ',' + json.dumps(colors) + ')')
                    return state if state[old_index]['highlight'] >= 12 and state[1-old_index]['normal'] >= 12 else None
                report['menuPixelDiagnostic'] = js('window.palSelectionPerformance.menuColors(' + json.dumps(areas) + ',' + json.dumps(colors) + ')')
                wait(menu_pixels_ready, 'original highlighted row does not match authority: ' + str(report['menuPixelDiagnostic']), 4)
            js('window.palSelectionPerformance.arm(' + json.dumps(spec) + ')')
            target_input(target, touch)
            sample = wait(lambda: js('window.palSelectionPerformance.done'), 'trusted input was not measured')
            assert sample['trusted'] and sample['responseMS'] is not None, {**sample, 'frames': sample['frames'][:12]}
            try:
                wait(lambda: selection() == target, 'original server selection did not converge')
            except AssertionError:
                report['failedSelection'] = {'sample': sample, 'current': selection(), 'network': [row for row in trace()['events'] if int(sample['epochAt']) <= row['at'] <= sample['epochAt']+2000]}
                raise
            paint()
            field = 'menuSelected' if args.mode == 'menu' else 'battleSelected'
            def authority_trace():
                trace_rows = [row for row in trace()['events'] if row['at'] >= int(sample['epochAt'])]
                sent = next((row for row in trace_rows if row['type'] == 'sent' and row['action']['keys'] == (4 if old_index == 0 else 16)), None)
                if not sent:
                    return None
                changed = next((row for row in trace_rows if row.get(field) == target and row.get('appliedSeq', -1) >= sent['seq']), None)
                ack = next((row for row in trace_rows if row['type'] == 'ack' and row.get('nonce') == sent['nonce']), None)
                return {'sent': sent, 'authority': changed, 'ack': ack} if changed and ack else None
            network = wait(authority_trace, 'native input and authority were not traced')
            assert network['ack']['status'] == 200, network
            sample['network'] = network
            sample['beforeAuthority'] = sample['pixelEpochAt'] < network['authority']['at']
            samples.append(sample)
        measured = stats([row['responseMS'] for row in samples])
        passed = all(measured[key] <= limit for key, limit in report['budgets'].items()) and (not delay or all(row['beforeAuthority'] for row in samples))
        case = {'name': label, 'rtt': rtt, 'samples': samples, 'responseMS': measured, 'passed': passed}
        report['cases'].append(case)
        print(json.dumps({'case': label, 'responseMS': measured, 'beforeAuthority': sum(row['beforeAuthority'] for row in samples), 'passed': passed}), flush=True)
        browser('screenshot', out / (label + '.png'))
    if args.run == 'optimized':
        before = room()['game']['data']
        assert selection() == targets[0] and not room()['game'].get('paused')
        if args.mode == 'battle':
            browser('press', 'ArrowDown')
            wait(lambda: selection() == 3, 'original miscellaneous icon was not selected')
            browser('press', 'Enter')
            def target_ready():
                view = room()['game']['data']
                return view if view['phase'] == 'battle' and view['battle']['state'] == 1 and view['battle']['menu'] == 4 else None
            result = wait(target_ready, 'confirmed icon did not open original miscellaneous menu')
            assert result['party'] == before['party'] and result['cash'] == before['cash']
        else:
            browser('press', 'Enter')
            def status_ready():
                view = room()['game']['data']
                return view if view['phase'] == 'menu' and view['menu']['kind'] == 2 else None
            result = wait(status_ready, 'confirmed status did not open native status menu')
            assert result['party'] == before['party'] and result['cash'] == before['cash']
        report['confirmation'] = {'passed': True, 'onlyExplicitConfirmChangedBoundary': True, 'cashAndPartyUnchanged': True, 'mode': args.mode}
    report['measurementComplete'] = True
    report['passed'] = all(case['passed'] for case in report['cases'])
finally:
    if observer and not report['measurementComplete']:
        report['failureNetwork'] = trace()['events'][-40:]
    (out / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
    if device:
        device.terminate()
        device.wait(timeout=5)
    if observer:
        observer.terminate()
        observer.communicate(timeout=5)
    try:
        browser('close')
    finally:
        proxy.terminate()
        proxy.communicate(timeout=5)
        shutil.rmtree(out / 'profile', ignore_errors=True)
if args.run == 'optimized':
    assert report['passed'], 'original selection latency budgets failed'
