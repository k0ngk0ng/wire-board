#!/usr/bin/env python3
"""Late-animation regression: two real humans and native placement/spawning."""
import json
import base64
import os
from pathlib import Path
import shutil
import subprocess
import time
import uuid

ROOT = Path(__file__).resolve().parents[3]
OUT = ROOT / '.local/pvz/frontend-animation'
OUT.mkdir(parents=True, exist_ok=True)
BASE = os.environ.get('PVZ_BROWSER_URL', '').rstrip('/')
REPORT = {'passed': False, 'scope': 'production room, two real browser clients, normal resources; late pole vault native pose sequence', 'command': 'PVZ_BROWSER_URL=<local URL> python3 -B games/pvz/tests/frontend-animation-browser.py'}
clients = []

class Browser:
    def __init__(self, suffix):
        self.env = dict(os.environ, AGENT_BROWSER_SESSION=suffix + uuid.uuid4().hex[:3], AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/ab'), AGENT_BROWSER_PROFILE=str(OUT / ('profile-' + suffix)), TMPDIR=str(OUT / 'tmp'), AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
        for key in ['AGENT_BROWSER_SOCKET_DIR', 'AGENT_BROWSER_PROFILE', 'TMPDIR']: Path(self.env[key]).mkdir(parents=True, exist_ok=True)
        self.opened = False
        clients.append(self)
        self.opened = True; self.b('open', BASE)
        self.api('/register', {'name': '动作时钟' + suffix + uuid.uuid4().hex[:5], 'password': 'pvz-browser-local', 'invite': 'pvz-browser'})
        self.b('reload'); self.b('wait', '--text', '创建房间')
        self.b('set', 'viewport', 1440, 960)
    def b(self, *args):
        result = subprocess.run(['agent-browser', *map(str, args), '--json'], cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=60)
        assert result.returncode == 0, result.stdout + result.stderr
        record = json.loads(result.stdout); assert record['success'], record
        return record['data']
    def js(self, code): return self.b('eval', code)['result']
    def js_stdin(self, code):
        result = subprocess.run(['agent-browser', 'eval', '--stdin', '--json'], input=code, cwd=ROOT, env=self.env, text=True, capture_output=True, timeout=60)
        assert result.returncode == 0, result.stdout + result.stderr
        record = json.loads(result.stdout); assert record['success'], record
        return record['data']['result']
    def api(self, path, body=None):
        options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
        result = self.js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
        assert 200 <= result['status'] < 300, result
        return result['body']
    def room(self): return self.api('/state')['room']
    def command(self, kind, **extra):
        state = self.room()
        return self.api('/rooms/' + state['id'], dict(type=kind, version=state['version'], nonce=uuid.uuid4().hex, **extra))
    def select(self, kind):
        selector = '[data-pvz-card=' + json.dumps(kind) + ']'
        self.b('click', selector)
        selected = 'document.querySelector(' + json.dumps(selector) + ').getAttribute("aria-pressed")==="true"'
        if not self.js(selected): self.b('click', selector)
        assert self.js(selected), {'kind': kind, 'frame': self.js('document.querySelector("[data-pvz-canvas]").__pvzFrame')}
    def click(self, x, y):
        self.b('scrollintoview', '[data-pvz-canvas]')
        rect = self.js('document.querySelector("[data-pvz-canvas]").getBoundingClientRect().toJSON()')
        REPORT.setdefault('inputSetup', []).append(self.js('({frame:document.querySelector("[data-pvz-canvas]").__pvzFrame,focused:document.hasFocus(),hit:document.elementFromPoint(' + str(round(rect['left'] + rect['width'] * x / 800)) + ',' + str(round(rect['top'] + rect['height'] * y / 600)) + ')?.outerHTML.slice(0,160)})'))
        self.b('mouse', 'move', round(rect['left'] + rect['width'] * x / 800), round(rect['top'] + rect['height'] * y / 600))
        self.b('mouse', 'down'); self.b('mouse', 'up')
    def close(self):
        if self.opened:
            try:
                self.b('close')
                for suffix in ['config', 'target']: (Path(self.env['AGENT_BROWSER_SOCKET_DIR']) / (self.env['AGENT_BROWSER_SESSION'] + '.' + suffix)).unlink(missing_ok=True)
            except Exception: pass
        shutil.rmtree(self.env['AGENT_BROWSER_PROFILE'], ignore_errors=True)

def wait(fn, seconds=60):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if fn(): return
        time.sleep(.1)
    raise AssertionError('condition timed out')

try:
    assert BASE, 'PVZ_BROWSER_URL must point to a production-module fixture'
    host, peer = Browser('a'), Browser('b')
    created = host.api('/rooms', {'kind': 'pvz', 'name': '晚触发动作时钟', 'capacity': 2, 'options': {'mode': 'duel', 'teams': [0, 1], 'scene': 'day', 'roundSeconds': 180, 'prepSeconds': 10}})
    peer.api('/rooms/' + created['id'], {'type': 'join', 'version': host.room()['version'], 'nonce': uuid.uuid4().hex})
    host.command('ready'); peer.command('ready'); host.command('start')
    host.b('wait', '[data-pvz-board][data-ready=true]'); peer.b('wait', '[data-pvz-board][data-ready=true]')
    wait(lambda: not host.room().get('loading'))
    host.select('wallnut'); host.click(480, 130)
    wait(lambda: any(plant['kind'] == 'wallnut' for plant in host.room()['game']['data']['plants']))
    host.select('peashooter'); host.click(80, 230)
    wait(lambda: any(plant['kind'] == 'peashooter' for plant in host.room()['game']['data']['plants']))
    wait(lambda: peer.room()['game']['data']['phase'] == 'battle')
    host.js('window.__pvzLateSamples=[];window.__pvzFxOverlap=[];window.__pvzLateSampling=true;(function sample(){if(!window.__pvzLateSampling)return;const f=document.querySelector("[data-pvz-canvas]").__pvzFrame;for(const p of f.poses||[])if(["pole","polevaulter"].includes(p.kind)&&p.clip==="vaulting")window.__pvzLateSamples.push({...p,tick:f.tick});const tracks=[...document.querySelectorAll("audio[data-platform-track^=pvz-fx-]")].filter(m=>!m.paused&&!m.ended&&m.readyState>=2).map(m=>({key:m.dataset.platformTrack,src:m.currentSrc,time:m.currentTime}));if(new Set(tracks.map(m=>m.src)).size>=2&&window.__pvzFxOverlap.length<30)window.__pvzFxOverlap.push({tick:f.tick,tracks});requestAnimationFrame(sample)})();true')
    peer.select('polevaulter'); peer.click(780, 130)
    peer.select('zombie'); peer.click(780, 230)
    wait(lambda: host.js('window.__pvzLateSamples.length>0'), 60)
    # Local deterministic prediction can enter the jump before the next public
    # authoritative snapshot. Verify the authoritative phase separately.
    wait(lambda: any(zombie['kind'] == 'polevaulter' and zombie['state'] == 'vaulting' for zombie in host.room()['game']['data']['zombies']), 10)
    state = host.room()['game']['data']
    pole = next(zombie for zombie in state['zombies'] if zombie['kind'] == 'polevaulter')
    REPORT['pole'] = pole
    REPORT['samplesAtEntry'] = host.js('window.__pvzLateSamples')
    assert pole['flags']['phaseAt'] - pole['bornAt'] > 60, 'vault must occur long after birth'
    time.sleep(2)
    samples = host.js('window.__pvzLateSampling=false;window.__pvzLateSamples')
    indexes = [sample['index'] for sample in samples]
    assert min(indexes) <= 2 and max(indexes) >= 25, samples
    assert any(right > left for left, right in zip(indexes, indexes[1:])), samples
    overlap = host.js('window.__pvzFxOverlap')
    assert overlap, 'different native one-shot effects did not overlap'
    host.b('screenshot', OUT / 'late-vault.png')
    host.b('set', 'viewport', 320, 760); host.b('scrollintoview', '[data-pvz-canvas]')
    wait(lambda: host.js('document.querySelector("[data-pvz-canvas]").__pvzFrame.poses.some(p=>p.kind==="peashooter"&&p.clip==="attack")'), 10)
    host.b('screenshot', OUT / 'peashooter-attack-mobile.png')
    rectangle = host.js('document.querySelector("[data-pvz-canvas]").getBoundingClientRect().toJSON()')
    crop = (round(rectangle['left'] + rectangle['width'] * 30 / 800), round(rectangle['top'] + rectangle['height'] * 175 / 600), round(rectangle['left'] + rectangle['width'] * 130 / 800), round(rectangle['top'] + rectangle['height'] * 285 / 600))
    image_url = 'data:image/png;base64,' + base64.b64encode((OUT / 'peashooter-attack-mobile.png').read_bytes()).decode()
    code = '(async()=>{const image=new Image();image.src=' + json.dumps(image_url) + ';await image.decode();const [x1,y1,x2,y2]=' + json.dumps(crop) + ';const canvas=document.createElement("canvas");canvas.width=(x2-x1)*5;canvas.height=(y2-y1)*5;const context=canvas.getContext("2d");context.imageSmoothingEnabled=false;context.drawImage(image,x1,y1,x2-x1,y2-y1,0,0,canvas.width,canvas.height);return canvas.toDataURL("image/png")})()'
    detail = host.js_stdin(code)
    (OUT / 'peashooter-body-detail.png').write_bytes(base64.b64decode(detail.split(',', 1)[1]))
    REPORT.update(passed=True, pole=pole, samples=samples, nativeEffectsOverlap=overlap)
    host.command('close'); host.command('leave'); peer.command('leave')
finally:
    if not REPORT['passed'] and clients:
        try:
            REPORT['failure'] = {'page': clients[0].js('document.body.innerText'), 'room': clients[0].room(), 'frame': clients[0].js('document.querySelector("[data-pvz-canvas]")?.__pvzFrame')}
            clients[0].b('screenshot', OUT / 'failure.png', '--full')
        except Exception: pass
        try:
            if clients[0].room()['status'] == 'playing': clients[0].command('close')
            for client in clients:
                if client.api('/state').get('room'): client.command('leave')
        except Exception: pass
    for client in clients: client.close()
    (OUT / 'result.json').write_text(json.dumps(REPORT, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'passed': REPORT['passed']}))
