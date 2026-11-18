SCOPE = 'Actual accepted campaign earth-beast arrival on platform Board: full-frame physical shift pixels, palette-zero edges, private mid-shake save/resume and maximum desktop/phone canvas.'
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pal/prayer-shake-board-browser'
BASE = (OUT / 'url').read_text().strip()
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/ps-ab'), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR', 'TMPDIR']:
    Path(ENV[key]).mkdir(parents=True, exist_ok=True)
SESSION = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', 'ps'], cwd=ROOT, env=ENV, text=True).strip()
REPORT = {'passed': False, 'scope': SCOPE, 'entrySHA256': hashlib.sha256((OUT / 'entry-state.json').read_bytes()).hexdigest(), 'cases': []}

def wait(check, label, seconds=60):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        result = check()
        if result:
            return result
        time.sleep(.08)
    raise AssertionError(label)

def shift(view):
    graph = view.get('raster')
    if not graph:
        return 0
    fill = graph['nodes'][graph['root'] - 1]
    if fill[0] != 4 or fill[9] != 0:
        return 0
    copy = graph['nodes'][fill[1] - 1]
    return copy[10] - copy[6] if copy[0] == 8 else 0

class Browser:
    def __init__(self, label):
        self.label = label
        self.profile = OUT / ('profile-' + label)
        self.profile.mkdir(parents=True, exist_ok=True)
        self.env = dict(ENV, AGENT_BROWSER_PROFILE=str(self.profile))
        self.device = None

    def b(self, *values):
        result = subprocess.run(['agent-browser', '--session', SESSION + '-' + self.label, *map(str, values), '--json'], cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=45)
        assert result.returncode == 0, result.stdout + result.stderr
        data = json.loads(result.stdout)
        assert data['success'], data
        return data['data']

    def js(self, source):
        return self.b('eval', source)['result']

    def api(self, path, body=None):
        options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
        value = self.js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
        assert 200 <= value['status'] < 300, value
        return value['body']

    def room(self):
        return self.api('/state').get('room')

    def data(self):
        return self.room()['game']['data']

    def command(self, kind):
        room = self.room()
        return self.api('/rooms/' + room['id'], {'type': kind, 'version': room['version'], 'nonce': uuid.uuid4().hex})

    def touch(self, selector):
        x, y = self.js('(()=>{const r=document.querySelector(' + json.dumps(selector) + ').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        run = subprocess.run(['node', 'games/pal/tests/screen-touch.mjs', self.b('get', 'cdp-url')['cdpUrl'], BASE, 'tap', *map(str, [x, y, x, y])], cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=25)
        assert run.returncode == 0, run.stdout + run.stderr

    def close(self):
        if self.device:
            self.device.terminate()
            self.device.wait(timeout=5)
        try:
            self.b('close')
        finally:
            shutil.rmtree(self.profile, ignore_errors=True)

players = [Browser('desktop'), Browser('phone')]
try:
    for player in players:
        phone = player.label == 'phone'
        player.b('open', BASE)
        player.b('set', 'viewport', 320 if phone else 1280, 820 if phone else 900)
        if phone:
            player.device = subprocess.Popen(['node', 'scripts/browser_device.mjs', player.b('get', 'cdp-url')['cdpUrl'], BASE, '320', '820', '--keep'], cwd=ROOT, env=player.env, text=True, stdout=subprocess.PIPE)
            assert player.device.stdout.readline().strip() == 'ready'
        assert player.js('(async()=>(await fetch("/pal/v1/index.json")).status)()') == 404
        player.api('/register', {'name': '真实震屏' + uuid.uuid4().hex[:8], 'password': 'pal-story-shake-test', 'invite': 'test-invite'})
        player.api('/rooms', {'kind': 'pal', 'name': '原版地魔兽登场', 'capacity': 1})
        player.command('ready')
        player.command('start')
        player.b('reload')
        wait(lambda: player.room() and not player.room().get('loading'), 'actual original scene was not ready', 100)
        player.b('click', '[data-fullscreen-enter]')
        player.b('wait', '[data-room-fullscreen]')
        player.js('(async()=>{window.palStoryShake=await import("/pal-shake-probe.js");await palStoryShake.prime();palStoryShake.start(' + json.dumps(player.label) + ');return true})()')
        assert not player.js('!!document.querySelector("[data-pal-direction],[data-pal-action]")')
        saved = False
        pages = 0
        until = time.monotonic() + 150
        while time.monotonic() < until:
            current = player.data()
            samples = player.js('palStoryShake.results.length')
            if samples and shift(current) and not saved:
                selector = '[data-room-fullscreen] [data-save-game]'
                if phone:
                    player.touch(selector)
                else:
                    player.b('click', selector)
                wait(lambda: not player.room(), 'actual mid-shake private save did not release owner')
                frozen = json.loads((OUT / 'frozen-view.json').read_text())
                assert shift(frozen), 'save missed the real physical-shake window'
                player.b('wait', '[data-resume-save]')
                if phone:
                    player.touch('[data-resume-save]')
                else:
                    player.b('click', '[data-resume-save]')
                wait(lambda: player.room() and not player.room().get('loading'), 'actual mid-shake personal continuation did not finish', 100)
                restored = json.loads((OUT / 'restored-view.json').read_text())
                normalize = lambda value: {key: item for key, item in value.items() if key not in ['epoch', 'seq', 'appliedSeq', 'discardedSeq', 'controlAt']}
                assert normalize(frozen) == normalize(restored), 'saved physical-shake public frame changed'
                for name, value in [('frozen', frozen), ('restored', restored)]:
                    (OUT / (player.label + '-' + name + '.json')).write_text(json.dumps(value, ensure_ascii=False) + '\n')
                player.b('click', '[data-fullscreen-enter]')
                player.b('wait', '[data-room-fullscreen]')
                saved = True
                continue
            if current.get('dialogue'):
                text = current['dialogue']['text']
                if '黑苗人竟然' in text:
                    break
                wait(lambda: player.js('document.querySelector(".pal-sr-only")?.textContent===' + json.dumps(text)), 'original dialogue did not reach the actual Board')
                if phone:
                    player.touch('[data-pal-board] canvas')
                else:
                    player.js('document.querySelector("[data-pal-board] canvas").focus();true')
                    player.b('press', 'Enter')
                prompt = current['dialogue']['id']
                wait(lambda: player.data().get('dialogue', {}).get('id') != prompt, 'original visible dialogue did not advance')
                pages += 1
            time.sleep(.05)
        else:
            raise AssertionError('actual earth-beast arrival did not finish')
        player.js('palStoryShake.stop();true')
        samples = player.js('palStoryShake.results')
        diagnostics = player.js('palStoryShake.diagnostics')
        assert saved and not diagnostics['errors'], diagnostics
        assert sum(row['shift'] < 0 for row in samples) >= 4 and sum(row['shift'] > 0 for row in samples) >= 4, {'samples': samples, 'diagnostics': diagnostics}
        expected = [320, 200] if phone else [1280, 800]
        for index, row in enumerate(samples):
            assert all(abs(a-b) < 1 for a, b in zip(row['canvas'], expected)), row['canvas']
            (OUT / f'{player.label}-physical-{index}.png').write_bytes(base64.b64decode(row.pop('canvasPNG').split(',', 1)[1]))
        player.b('screenshot', OUT / (player.label + '-arrival.png'))
        case = {'name': player.label, 'passed': True, 'wholeFramePixelsMatched': True, 'paletteZeroEdgesMatched': True, 'savedDuringPhysicalShake': True, 'pages': pages, 'samples': samples, 'diagnostics': diagnostics}
        REPORT['cases'].append(case)
        print(json.dumps({'name': player.label, 'passed': True, 'pixelComparisons': len(samples), 'savedDuringPhysicalShake': saved}), flush=True)
    REPORT['passed'] = True
finally:
    for player in players:
        if not REPORT['passed']:
            try:
                player.b('screenshot', OUT / (player.label + '-failure.png'))
                (OUT / (player.label + '-failure.json')).write_text(json.dumps({'state': player.api('/state'), 'probe': player.js('window.palStoryShake?{results:palStoryShake.results,diagnostics:palStoryShake.diagnostics,view:palStoryShake.currentView()}:null')}, ensure_ascii=False) + '\n')
            except Exception:
                pass
        try:
            player.close()
        except Exception:
            pass
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'passed': REPORT['passed'], 'pixelComparisons': sum(len(case['samples']) for case in REPORT['cases'])}), flush=True)
