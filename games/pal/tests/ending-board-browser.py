import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[3]
PARSER = argparse.ArgumentParser()
PARSER.add_argument('--credits', action='store_true')
PARSER.add_argument('--cdn', action='store_true')
PARSER.add_argument('--presentation', action='store_true')
PARSER.add_argument('--checkpoint', action='store_true')
PARSER.add_argument('--campaign', action='store_true')
ARGS = PARSER.parse_args()
if ARGS.presentation and not ARGS.credits:
    PARSER.error('presentation regression requires the full additional-credit route')
if ARGS.checkpoint and not ARGS.credits:
    PARSER.error('checkpoint regression requires the full ending route')
OUT = ROOT / ('.local/pal/checkpoint-ending-board-browser' if ARGS.checkpoint else '.local/pal/presentation-ending-board-browser' if ARGS.presentation else '.local/pal/ending-credits-board-browser' if ARGS.credits else '.local/pal/ending-board-browser')
if ARGS.cdn:
    OUT = OUT.with_name('cdn-' + OUT.name)
VISUAL_CDN = os.environ.get('PAL_VISUAL_CDN') == '1'
if VISUAL_CDN:
    OUT = OUT.with_name('visual-' + OUT.name)
if ARGS.campaign:
    OUT = OUT.with_name('campaign-' + OUT.name)
PUBLICATION = json.loads((ROOT / 'games/pal/assets/ending-release.json').read_text())['catalogs']
ENDING_ID = hashlib.sha256((ROOT / '.local/pal/ending-media-e2e/index.json').read_bytes()).hexdigest()
PUBLISHED = PUBLICATION[ENDING_ID]
BASE = ''
for attempt in range(300):
    try:
        candidate = (OUT / 'url').read_text().strip()
        with urllib.request.urlopen(candidate, timeout=1) as response:
            if response.status == 200:
                BASE = candidate
                break
    except (OSError, ValueError):
        pass
    time.sleep(0.1)
assert BASE, 'ending browser fixture is not ready'
if VISUAL_CDN:
    try:
        urllib.request.urlopen(BASE + '/pal/v1/index.json', timeout=5)
    except urllib.error.HTTPError as error:
        assert error.code == 404
    else:
        raise AssertionError('local visual assets are still served')
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/pal-ab'), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
SESSION = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'pal-ending-board'], cwd=ROOT, env=ENV, text=True).strip()

class Browser:
    def __init__(self, index):
        self.session = SESSION + '-' + str(index)
        self.profile = OUT / ('profile-' + str(index))
        self.env = dict(ENV, AGENT_BROWSER_PROFILE=str(self.profile))
        self.device = None

    def run(self, *args):
        result = subprocess.run(['agent-browser', '--session', self.session, '--json', *map(str, args)], cwd=ROOT, env=self.env, capture_output=True, text=True, timeout=50)
        assert result.returncode == 0, result.stdout + result.stderr
        data = json.loads(result.stdout)
        assert data['success'], data
        return data['data']

    def js(self, code):
        return self.run('eval', code)['result']

    def request(self, path, body=None):
        options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
        return self.js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')

    def api(self, path, body=None):
        result = self.request(path, body)
        assert 200 <= result['status'] < 300, result
        return result['body']

    def room(self):
        return self.api('/state').get('room')

    def data(self):
        return self.room()['game']['data']

    def command(self, kind):
        room = self.room()
        return self.api('/rooms/' + room['id'], {'type': kind, 'version': room['version'], 'nonce': uuid.uuid4().hex})

def wait(check, label, seconds=45):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if check():
            return
        time.sleep(0.12)
    raise AssertionError(label)

def ready(player):
    error = player.js('document.querySelector(".pal-board-error")?.textContent')
    assert not error, error
    room = player.room()
    return room and not room.get('loading')

def dialog_style(player):
    value = player.js('''(()=>{const dialog=document.querySelector('[role=dialog]'),body=dialog.querySelector('p'),button=dialog.querySelector('[data-confirm-checkpoint]'),d=getComputedStyle(dialog),p=getComputedStyle(body),b=getComputedStyle(button),rgb=s=>s.match(/[0-9.]+/g).slice(0,3).map(Number),luma=s=>rgb(s).map(v=>v/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0),ratio=(a,c)=>(Math.max(luma(a),luma(c))+.05)/(Math.min(luma(a),luma(c))+.05);return {opacity:Number(d.opacity),foreground:p.color,background:d.backgroundColor,textContrast:ratio(p.color,d.backgroundColor),primaryForeground:b.color,primaryBackground:b.backgroundColor,primaryTextContrast:ratio(b.color,b.backgroundColor),primaryFillContrast:ratio(b.backgroundColor,d.backgroundColor)}})()''')
    assert value['opacity'] == 1 and value['textContrast'] >= 4.5 and value['primaryTextContrast'] >= 4.5 and value['primaryFillContrast'] >= 3, value
    return value

players = [Browser(0), Browser(1)]
report = {'passed': False, 'scope': 'isolated original ending on actual platform Board; native media channel, private desktop/phone saves, direct skip input and maximum canvas; not campaign acceptance or physical-speaker verification', 'cases': [], 'cdnOnlyFixture': ARGS.cdn, 'publicationID': ENDING_ID, 'publishedAudio': {str(clip): PUBLISHED['files'][f'ending/{clip}.flac']['url'] for clip in [4, 5, 6]}}
report['visualCDNOnlyFixture'] = VISUAL_CDN
report['genuineCampaignEntry'] = ARGS.campaign
if ARGS.campaign:
    entry = json.loads((ROOT / 'games/pal/tests/browser-campaign-roots.json').read_text())['entries']['ending-first']
    assert hashlib.sha256((OUT / 'entry-state.json').read_bytes()).hexdigest() == entry['snapshotSHA256']
    report['campaignEntrySHA256'] = entry['snapshotSHA256']
    report['scope'] = 'genuine complete-campaign ending checkpoint on actual platform Board; desktop/phone CDN movie and credits, private saves, manual rollback and original terminal boundary; full campaign route is verified separately'
try:
    owner, stranger = players
    for player in players:
        player.run('open', BASE)
        player.run('set', 'viewport', 1280, 900)
        player.api('/register', {'name': '结局验收' + uuid.uuid4().hex[:6], 'password': 'ending-test-123', 'invite': 'test-invite'})
    room = owner.api('/rooms', {'kind': 'pal', 'name': '原版结局续档', 'capacity': 1})
    owner.command('ready')
    owner.command('start')
    owner.run('reload')
    wait(lambda: ready(owner), 'ending canvas did not become ready', 80)
    match_id = owner.room()['matchId']
    owner.run('click', '[data-fullscreen-enter]')
    owner.run('wait', '[data-room-fullscreen]')
    wait(lambda: owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].some(m=>!m.paused&&m.readyState>=2)'), 'ending did not use a playing platform media track')
    actual_source = owner.js('document.querySelector("audio[data-platform-track=pal-ending]").currentSrc')
    assert actual_source == PUBLISHED['files']['ending/4.flac']['url'], actual_source
    initial = owner.data()
    (OUT / 'initial-view.json').write_text(json.dumps(initial, ensure_ascii=False) + '\n')
    assert initial['phase'] == 'ending' and initial['ending']['clip'] == 4, initial.get('ending')
    if ARGS.checkpoint:
        assert owner.js('document.querySelector("[data-room-fullscreen] [data-load-checkpoint]")?.disabled===true'), 'checkpoint control did not reflect the missing manual save'
    geometry = owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')
    assert geometry == [1280, 800] and not owner.js('!!document.querySelector("[data-pal-controls] button")')
    owner.run('screenshot', OUT / 'desktop.png')
    owner.run('press', 'Enter')
    wait(lambda: owner.data()['ending']['clip'] == 5, 'desktop keyboard did not skip the original clip')
    owner.run('click', '[data-room-fullscreen] [data-save-game]')
    wait(lambda: not owner.room(), 'desktop ending save did not release the seat')
    assert stranger.api('/state')['saves'] == []
    assert stranger.request('/saves/' + room['id'] + '/resume', {})['status'] in [403, 404]
    frozen = json.loads((OUT / 'frozen-view.json').read_text())
    owner.run('click', '[data-resume-save]')
    wait(lambda: owner.room() and not owner.room().get('loading'), 'desktop ending did not resume', 80)
    restored = json.loads((OUT / 'restored-view.json').read_text())
    assert {k: v for k, v in frozen.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']} == {k: v for k, v in restored.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']}
    assert owner.room()['matchId'] == match_id
    wait(lambda: owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].some(m=>!m.paused&&m.readyState>=2&&m.currentSrc===' + json.dumps(PUBLISHED['files']['ending/5.flac']['url']) + ')'), 'saved second movie did not use its published CDN audio')
    report['cases'].append({'device': 'desktop', 'canvas': geometry, 'nativeAudio': True, 'privateSave': True, 'keyboardSkip': True})
    if ARGS.checkpoint:
        owner.run('click', '[data-fullscreen-enter]')
        owner.run('wait', '[data-room-fullscreen]')
        owner.run('click', '[data-room-fullscreen] [data-load-checkpoint]')
        owner.run('wait', '[data-cancel-checkpoint]')
        owner.run('click', '[data-cancel-checkpoint]')
        time.sleep(0.7)
        assert owner.data()['ending']['clip'] == 5
        owner.run('click', '[data-pal-board] canvas')
        wait(lambda: owner.data()['ending']['clip'] == 6, 'desktop did not advance beyond its manual checkpoint')
        active_epoch = owner.data()['epoch']
        owner.run('click', '[data-room-fullscreen] [data-load-checkpoint]')
        owner.run('wait', '[data-confirm-checkpoint]')
        time.sleep(0.2)
        desktop_dialog = dialog_style(owner)
        owner.run('screenshot', OUT / 'desktop-load-dialog.png')
        (OUT / 'checkpoint-restored-view.json').unlink(missing_ok=True)
        owner.run('click', '[data-confirm-checkpoint]')
        wait(lambda: owner.room() and not owner.room().get('loading') and owner.data().get('ending', {}).get('clip') == 5, 'desktop did not restore the prior original clip', 80)
        restored_point = json.loads((OUT / 'checkpoint-restored-view.json').read_text())
        assert {k: v for k, v in frozen.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']} == {k: v for k, v in restored_point.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']}
        assert restored_point['epoch'] > active_epoch
        assert owner.js('!!document.querySelector("[data-room-fullscreen]")'), 'desktop checkpoint loading exited the existing fullscreen surface'
        assert owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()') == [1280, 800]
        wait(lambda: owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].some(m=>!m.paused&&m.readyState>=2&&m.currentSrc===' + json.dumps(PUBLISHED['files']['ending/5.flac']['url']) + ')'), 'desktop checkpoint did not recover its original movie audio')
        report['cases'].append({'device': 'desktop', 'manualCheckpointLoad': True, 'cancelLoad': True, 'priorClipRestored': True, 'inputEpochIncreased': True, 'dialogStyle': desktop_dialog})
        owner.run('click', '[data-room-fullscreen] [data-fullscreen-toggle]')
    owner.run('set', 'viewport', 320, 820)
    owner.device = subprocess.Popen(['node', 'scripts/browser_device.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, '320', '820', '--keep'], cwd=ROOT, env=owner.env, text=True, stdout=subprocess.PIPE)
    assert owner.device.stdout.readline().strip() == 'ready'
    owner.run('click', '[data-fullscreen-enter]')
    owner.run('wait', '[data-room-fullscreen]')
    geometry = owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')
    assert geometry == [320, 200]
    wait(lambda: owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].some(m=>!m.paused&&m.readyState>=2)'), 'resumed phone ending lost its audio')
    owner.run('screenshot', OUT / 'phone.png')
    point = owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
    subprocess.run(['node', 'scripts/browser_touch.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, 'tap', *map(str, point + point)], cwd=ROOT, env=owner.env, check=True)
    wait(lambda: owner.data()['ending']['clip'] == 6, 'phone canvas tap did not skip the original clip')
    wait(lambda: owner.data()['ending']['frame'] >= 15 and owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].some(m=>!m.paused&&m.readyState>=2&&m.currentSrc===' + json.dumps(PUBLISHED['files']['ending/6.flac']['url']) + ')'), 'phone final clip did not show ongoing native audio and frames')
    for gesture in ['cancel', 'pinch']:
        subprocess.run(['node', 'games/pal/tests/screen-touch.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, gesture, *map(str, point + point)], cwd=ROOT, env=owner.env, check=True)
        time.sleep(0.7)
        assert owner.data()['phase'] == 'ending' and owner.data()['ending']['clip'] == 6, 'cancelled touch or pinch skipped the next clip'
    owner.run('click', '[data-room-fullscreen] [data-save-game]')
    wait(lambda: not owner.room(), 'phone ending save did not release the seat')
    frozen = json.loads((OUT / 'frozen-view.json').read_text())
    owner.run('click', '[data-resume-save]')
    wait(lambda: owner.room() and not owner.room().get('loading'), 'phone ending did not resume', 80)
    restored = json.loads((OUT / 'restored-view.json').read_text())
    assert {k: v for k, v in frozen.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']} == {k: v for k, v in restored.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']}
    wait(lambda: owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].some(m=>!m.paused&&m.readyState>=2&&m.currentSrc===' + json.dumps(PUBLISHED['files']['ending/6.flac']['url']) + ')'), 'final clip audio did not resume from the private save')
    playback = owner.js('(()=>{const m=document.querySelector("audio[data-platform-track=pal-ending]");return {src:m.currentSrc,time:m.currentTime}})()')
    assert abs(playback['time'] - owner.data()['ending']['elapsedMS'] / 1000) < 2, playback
    report['cases'].append({'device': 'phone', 'canvas': geometry, 'nativeAudio': True, 'privateSave': True, 'canvasSkip': True, 'cancelAndPinchInert': True, 'frozenEnding': frozen['ending'], 'restoredAudio': playback})
    if ARGS.checkpoint:
        owner.run('click', '[data-fullscreen-enter]')
        owner.run('wait', '[data-room-fullscreen]')
        wait(lambda: owner.data()['ending']['frame'] >= frozen['ending']['frame'] + 12, 'phone did not progress after its manual save')
        owner.run('click', '[data-room-fullscreen] [data-load-checkpoint]')
        owner.run('wait', '[data-confirm-checkpoint]')
        time.sleep(0.2)
        phone_dialog = dialog_style(owner)
        owner.run('screenshot', OUT / 'phone-load-dialog.png')
        assert owner.js('!!document.querySelector("[data-room-fullscreen] [role=dialog]")'), 'checkpoint confirmation escaped fullscreen'
        (OUT / 'checkpoint-restored-view.json').unlink(missing_ok=True)
        confirm_point = owner.js('(()=>{const r=document.querySelector("[data-confirm-checkpoint]").getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        subprocess.run(['node', 'scripts/browser_touch.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, 'tap', *map(str, confirm_point + confirm_point)], cwd=ROOT, env=owner.env, check=True)
        wait(lambda: owner.room() and not owner.room().get('loading') and (OUT / 'checkpoint-restored-view.json').is_file(), 'phone checkpoint load did not complete', 80)
        restored_point = json.loads((OUT / 'checkpoint-restored-view.json').read_text())
        assert {k: v for k, v in frozen.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']} == {k: v for k, v in restored_point.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']}
        assert owner.js('!!document.querySelector("[data-room-fullscreen]")'), 'phone checkpoint loading exited fullscreen'
        assert owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()') == [320, 200]
        wait(lambda: owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].some(m=>!m.paused&&m.readyState>=2&&m.currentSrc===' + json.dumps(PUBLISHED['files']['ending/6.flac']['url']) + ')'), 'phone manual checkpoint did not recover its audio')
        checkpoint_audio = owner.js('document.querySelector("audio[data-platform-track=pal-ending]").currentTime')
        assert abs(checkpoint_audio - owner.data()['ending']['elapsedMS'] / 1000) < 2
        report['cases'].append({'device': 'phone', 'manualCheckpointLoad': True, 'fullBoundaryRestored': True, 'canvas': [320, 200], 'dialogInsideFullscreen': True, 'nativeTouchConfirm': True, 'dialogStyle': phone_dialog})
    if ARGS.credits:
        if not owner.js('!!document.querySelector("[data-room-fullscreen]")'):
            owner.run('click', '[data-fullscreen-enter]')
        owner.run('wait', '[data-room-fullscreen]')
        point = owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        subprocess.run(['node', 'scripts/browser_touch.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, 'tap', *map(str, point + point)], cwd=ROOT, env=owner.env, check=True)
        wait(lambda: owner.data()['phase'] == 'menu' and owner.data()['menu']['kind'] == 13, 'last original movie did not reach the actual SDLPAL additional credit page')
        time.sleep(0.8)
        assert not owner.room()['game'].get('finished') and owner.data()['menu']['kind'] == 13, 'old movie skip crossed the additional credit page'
        wait(lambda: owner.js('[...document.querySelectorAll("audio[data-platform-track=pal-ending]")].every(m=>m.paused)'), 'movie audio continued over the additional credit page')
        assert not owner.js('!!document.querySelector("[data-pal-controls] button")')
        owner.run('screenshot', OUT / 'phone-credits.png')
        owner.run('click', '[data-room-fullscreen] [data-save-game]')
        wait(lambda: not owner.room(), 'credit-page save did not release the owner seat')
        frozen = json.loads((OUT / 'frozen-view.json').read_text())
        assert frozen['menu']['kind'] == 13 and frozen.get('ending') is None
        assert stranger.api('/state')['saves'] == []
        owner.run('click', '[data-resume-save]')
        wait(lambda: owner.room() and not owner.room().get('loading'), 'saved credit page did not restore', 80)
        restored = json.loads((OUT / 'restored-view.json').read_text())
        assert {k: v for k, v in frozen.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']} == {k: v for k, v in restored.items() if k not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']}
        owner.run('click', '[data-fullscreen-enter]')
        owner.run('wait', '[data-room-fullscreen]')
        point = owner.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        for gesture in ['cancel', 'pinch']:
            subprocess.run(['node', 'games/pal/tests/screen-touch.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, gesture, *map(str, point + point)], cwd=ROOT, env=owner.env, check=True)
            time.sleep(0.7)
            assert owner.data()['phase'] == 'menu' and owner.data()['menu']['kind'] == 13
        subprocess.run(['node', 'scripts/browser_touch.mjs', owner.run('get', 'cdp-url')['cdpUrl'], BASE, 'tap', *map(str, point + point)], cwd=ROOT, env=owner.env, check=True)
        wait(lambda: owner.room()['game'].get('finished') and owner.data()['phase'] == 'finished', 'phone canvas did not confirm the actual SDLPAL credit wait and native shutdown')
        assert owner.room()['matchId'] == match_id
        owner.run('screenshot', OUT / 'phone-finished.png')
        report['cases'].append({'device': 'phone', 'additionalCredits': True, 'creditPrivateSave': True, 'oldMovieSkipInert': True, 'cancelAndPinchInert': True, 'canvasConfirmNativeShutdown': True})
        stranger.api('/rooms', {'kind': 'pal', 'name': '桌面署名页确认', 'capacity': 1})
        stranger.command('ready')
        stranger.command('start')
        stranger.run('reload')
        wait(lambda: stranger.room() and not stranger.room().get('loading'), 'desktop credits canvas did not load', 80)
        stranger.run('click', '[data-fullscreen-enter]')
        stranger.run('wait', '[data-room-fullscreen]')
        for clip in [4, 5, 6]:
            wait(lambda: stranger.data().get('ending', {}).get('clip') == clip, 'desktop original ending order changed')
            stranger.run('press', 'Enter')
            if clip < 6:
                wait(lambda: stranger.data().get('ending', {}).get('clip') == clip + 1, 'desktop skip did not finish its original wait')
        wait(lambda: stranger.data()['phase'] == 'menu' and stranger.data()['menu']['kind'] == 13, 'desktop did not reach the SDLPAL additional credit page')
        time.sleep(0.8)
        assert not stranger.room()['game'].get('finished')
        assert stranger.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()') == [1280, 800]
        stranger.run('screenshot', OUT / 'desktop-credits.png')
        stranger.run('click', '[data-pal-board] canvas')
        wait(lambda: stranger.room()['game'].get('finished') and stranger.data()['phase'] == 'finished', 'desktop canvas did not confirm the actual native credit wait')
        stranger.run('screenshot', OUT / 'desktop-finished.png')
        report['cases'].append({'device': 'desktop', 'additionalCredits': True, 'canvas': [1280, 800], 'keyboardMovieSkips': True, 'oldMovieSkipInert': True, 'canvasConfirmNativeShutdown': True})
    report['passed'] = True
finally:
    for player in players:
        if player.device:
            player.device.terminate()
            player.device.wait(timeout=5)
        try:
            player.run('close')
        except Exception:
            pass
        shutil.rmtree(player.profile, ignore_errors=True)
    (OUT / 'stop').write_text('done\n')
    (OUT / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
print(json.dumps(report, ensure_ascii=False))
