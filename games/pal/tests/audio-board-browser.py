import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import time
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[3]
VISUAL_CDN = os.environ.get('PAL_VISUAL_CDN') == '1'
OUT = ROOT / ('.local/pal/cdn-visual-board-browser' if VISUAL_CDN else '.local/pal/audio-board-browser')
if scope := os.environ.get('PAL_AUDIO_BROWSER_SCOPE'):
    assert re.fullmatch(r'[a-z][a-z0-9-]{0,39}',scope)
    OUT = ROOT / '.local/pal/performance' / ('battle-recovery-'+scope)
ENV = dict(os.environ, AGENT_BROWSER_SOCKET_DIR=str(ROOT / '.local/pal-ab'), TMPDIR=str(ROOT / '.local/tmp'), AGENT_BROWSER_NO_VERSION_CHECK='1', AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')


def wait(check, label, seconds=45):
    until = time.monotonic() + seconds
    while time.monotonic() < until:
        if check():
            return
        time.sleep(0.12)
    raise AssertionError(label)


class Browser:
    def __init__(self, session, index):
        self.session = session + '-' + str(index)
        self.profile = OUT / ('profile-' + str(index))
        self.env = dict(ENV, AGENT_BROWSER_PROFILE=str(self.profile))
        self.device = None
        self.lifecycle = None

    def run(self, *args):
        result = subprocess.run(['agent-browser', '--session', self.session, '--json', *map(str, args)], cwd=ROOT, env=self.env, capture_output=True, text=True, timeout=50)
        assert result.returncode == 0, result.stdout + result.stderr
        data = json.loads(result.stdout)
        assert data['success'], data
        return data['data']

    def js(self, source):
        return self.run('eval', source)['result']

    def api(self, path, body=None):
        options = '' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(' + json.dumps(body) + ')}'
        result = self.js('(async()=>{const r=await fetch(' + json.dumps('/api' + path) + options + ');return {status:r.status,body:await r.json()}})()')
        assert 200 <= result['status'] < 300, result
        return result['body']

    def room(self):
        return self.api('/state').get('room')

    def command(self, kind):
        room = self.room()
        return self.api('/rooms/' + room['id'], {'type': kind, 'version': room['version'], 'nonce': uuid.uuid4().hex})

    def audio(self):
        return self.js('''(()=>{const m=document.querySelector('audio[data-platform-track="pal-music"]');return m?{time:m.currentTime,duration:m.duration,paused:m.paused,ready:m.readyState,source:m.currentSrc,connected:m.isConnected}:null})()''')

    def observe(self):
        self.js('''(()=>{window.palNativeAudio=[];const watch=m=>{if(m.tagName!=='AUDIO'||m.dataset.palObserved)return;m.dataset.palObserved='true';for(const event of ['loadstart','loadedmetadata','loadeddata','canplay','canplaythrough','playing','error','abort','emptied','suspend','stalled'])m.addEventListener(event,()=>window.palNativeAudio.push({event,at:performance.now(),time:m.currentTime,source:m.currentSrc||m.src,ready:m.readyState,network:m.networkState,duration:m.duration,preparation:m.dataset.platformPreparation,track:m.dataset.platformTrack,error:m.error?.message}))};document.querySelectorAll('audio').forEach(watch);new MutationObserver(records=>records.forEach(record=>record.addedNodes.forEach(watch))).observe(document.body,{childList:true,subtree:true})})()''')

    def tap(self, selector, base):
        if not self.device:
            self.run('click', selector)
            return
        point = self.js('(async()=>{const e=document.querySelector(' + json.dumps(selector) + ');e.scrollIntoView({block:"center",behavior:"instant"});await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const b=e.getBoundingClientRect();return [b.x+b.width/2,b.y+b.height/2]})()')
        result = subprocess.run(['node', 'scripts/browser_touch.mjs', self.run('get', 'cdp-url')['cdpUrl'], base, 'tap', *map(str, [*point, *point])], cwd=ROOT, env=self.env, capture_output=True, text=True, timeout=25)
        assert result.returncode == 0, result.stdout + result.stderr

    def drive(self, kind, base):
        if not self.lifecycle:
            self.lifecycle = subprocess.Popen(['node', 'games/pal/tests/audio-lifecycle-driver.mjs', self.run('get', 'cdp-url')['cdpUrl'], base], cwd=ROOT, env=self.env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
            assert self.lifecycle.stdout.readline().strip() == 'ready'
        self.lifecycle.stdin.write(json.dumps({'type': kind, 'touch': bool(self.device)}) + '\n')
        self.lifecycle.stdin.flush()
        return json.loads(self.lifecycle.stdout.readline())

    def close_helpers(self):
        if self.lifecycle:
            self.lifecycle.stdin.close()
            self.lifecycle.wait(timeout=5)
            self.lifecycle = None
        if self.device:
            self.device.terminate()
            self.device.wait(timeout=5)
            self.device = None


def main():
    base = (OUT / 'url').read_text().strip()
    with urllib.request.urlopen(base, timeout=5) as response:
        assert response.status == 200
    if VISUAL_CDN:
        try:
            urllib.request.urlopen(base + '/pal/v1/index.json', timeout=5)
        except urllib.error.HTTPError as error:
            assert error.code == 404
        else:
            raise AssertionError('local visual assets are still served')
    prefix = 'pr-audio' if os.environ.get('PAL_AUDIO_BROWSER_SCOPE') else 'pal-audio-board'
    session = subprocess.check_output(['agent-browser', 'session', 'id', '--scope', 'worktree', '--prefix', prefix], cwd=ROOT, env=ENV, text=True).strip()
    if os.environ.get('PAL_AUDIO_BROWSER_SCOPE'):
        session += '-' + uuid.uuid4().hex[:6]
    report = {'passed': False, 'scope': 'actual new-game original Board, native platform music, repeated snapshots, private save/resume and logical offset on desktop/emulated phone with maximum canvas; not full campaign, native phone or physical-speaker acceptance', 'cases': []}
    players = []
    try:
        for index, (width, height) in enumerate([(1280, 800), (320, 820)]):
            player = Browser(session, index)
            players.append(player)
            player.run('open', base)
            player.run('set', 'viewport', width, height)
            if index:
                player.device = subprocess.Popen(['node', 'scripts/browser_device.mjs', player.run('get', 'cdp-url')['cdpUrl'], base, str(width), str(height), '--keep'], cwd=ROOT, env=player.env, stdout=subprocess.PIPE, text=True)
                assert player.device.stdout.readline().strip() == 'ready'
            player.api('/register', {'name': '音乐验收' + uuid.uuid4().hex[:6], 'password': 'pal-audio-test-123', 'invite': 'test-invite'})
            player.api('/rooms', {'kind': 'pal', 'name': '原版音乐个人存档', 'capacity': 1})
            player.command('ready')
            player.command('start')
            player.run('reload')
            player.observe()
            wait(lambda: player.room() and not player.room().get('loading'), 'original audio Board did not become ready', 80)
            player.run('snapshot', '-i')
            player.tap('[data-fullscreen-enter]', base)
            player.run('wait', '[data-room-fullscreen]')
            wait(lambda: player.room()['game']['data'].get('dialogue'), 'original first dialogue did not arrive')
            wait(lambda: player.audio() and not player.audio()['paused'] and player.audio()['ready'] >= 2, 'original music did not play through platform channel')
            first = player.audio()
            publication = json.loads((ROOT / 'games/pal/assets/audio-release.json').read_text())
            current_audio = player.room()['game']['data']['audio']
            published = publication['catalogs'][current_audio['assetID']]
            music_path = 'music/' + str(current_audio['music']['id']) + '.flac'
            expected_source = published.get('files', {}).get(music_path, {}).get('url', published['assetsBaseURL'] + '/' + music_path)
            assert first['source'] == expected_source, first
            player.js('window.palMusicElement=document.querySelector(\'audio[data-platform-track="pal-music"]\')')
            time.sleep(0.5)
            assert player.js('window.palMusicElement===document.querySelector(\'audio[data-platform-track="pal-music"]\')')
            assert player.audio()['time'] > first['time'] + 0.3
            bounds = player.js('''(()=>{const b=document.querySelector('canvas').getBoundingClientRect();return [b.width,b.height]})()''')
            if VISUAL_CDN:
                resources = player.js('performance.getEntriesByType("resource").map(value=>value.name).filter(name=>name.includes("/visual-v1/")||name.includes("/pal/v1/"))')
                assert resources and all(name.startswith('https://cdn.ichenj.com/bga/') for name in resources), resources
            assert abs(bounds[0] - min(width, height * 1.6)) < 1 and abs(bounds[0] / bounds[1] - 1.6) < 0.001, bounds
            before_save = player.room()['game']['data']
            player.command('save')
            wait(lambda: player.room() is None, 'save did not release account')
            time.sleep(0.4)
            assert player.js('document.querySelector(\'audio[data-platform-track="pal-music"]\')===null')
            saves = player.api('/state')['saves']
            assert len(saves) == 1
            player.api('/saves/' + saves[0]['id'] + '/resume', {})
            player.run('reload')
            player.observe()
            wait(lambda: player.room() and not player.room().get('loading'), 'resumed audio Board did not become ready', 80)
            player.run('snapshot', '-i')
            player.tap('[data-fullscreen-enter]', base)
            player.run('wait', '[data-room-fullscreen]')
            player.tap('canvas', base)
            wait(lambda: player.audio() and not player.audio()['paused'], 'saved music did not resume')
            state = player.api('/state')
            room = state['room']
            music = room['game']['data']['audio']['music']
            observed = player.audio()
            expected = ((state['serverNow'] - room.get('clockOffsetMs', 0) - music['startedAt']) / 1000) % observed['duration']
            assert abs(observed['time'] - expected) < 0.4, (observed, expected)
            assert music['seq'] == before_save['audio']['music']['seq'] and music['startedAt'] == before_save['audio']['music']['startedAt']
            assert observed['source'] == expected_source
            assert player.js('document.querySelectorAll("[data-pal-virtual-controls], [data-pal-read-button]").length===0')
            player.js("""(()=>{window.palSoundCapture=undefined;const watch=event=>{const m=event.target;if(!(m instanceof HTMLAudioElement)||!m.dataset.platformTrack?.startsWith('pal-sfx-'))return;document.removeEventListener('playing',watch,true);const capture=window.palSoundCapture={first:{at:performance.now(),time:m.currentTime,source:m.currentSrc}};void(async()=>{const state=await(await fetch('/api/state')).json(),room=state.room;capture.state=state;capture.at=performance.now();if(!room?.game?.data?.audio?.sounds?.some(sound=>sound.id===98))throw Error('Original sound was no longer live at save');const response=await fetch('/api/rooms/'+room.id,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({type:'save',version:room.version,nonce:crypto.randomUUID()})});capture.status=response.status;capture.response=await response.json();if(!response.ok)throw Error('Original mid-sound save failed');capture.saved=true})().catch(error=>capture.error=String(error))};document.addEventListener('playing',watch,true)})()""")
            pages = 0
            until = time.monotonic() + 95
            while not player.js('!!window.palSoundCapture?.saved'):
                capture = player.js('window.palSoundCapture')
                assert not capture or not capture.get('error'), capture
                room = player.room()
                if not room:
                    wait(lambda: player.js('!!window.palSoundCapture?.saved'), 'mid-sound save did not complete')
                    break
                value = room['game']['data']
                if value.get('dialogue'):
                    prompt = value['dialogue']['id']
                    text = value['dialogue']['text']
                    wait(lambda: player.js('document.querySelector("[data-pal-board] .pal-sr-only")?.textContent===' + json.dumps(text)), 'visible original dialogue did not match authority')
                    player.js('new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)))')
                    player.tap('canvas', base)
                    wait(lambda: (lambda current: not current or not current['game']['data'].get('dialogue') or current['game']['data']['dialogue']['id'] != prompt)(player.room()), 'actual screen input did not advance original dialogue')
                    pages += 1
                else:
                    time.sleep(0.03)
                assert pages <= 24 and time.monotonic() < until, 'opening did not reach its original sound'
            capture = player.js('window.palSoundCapture')
            sound = next(sound for sound in capture['state']['room']['game']['data']['audio']['sounds'] if sound['id'] == 98)
            assert capture['saved'] and 0 < capture['first']['time'] < 0.8756463, capture
            wait(lambda: player.room() is None, 'mid-sound save did not release owner')
            saved_cutoff = player.js('performance.now()')
            assert player.drive('arm-visibility', base)
            saves = player.api('/state')['saves']
            player.api('/saves/' + saves[0]['id'] + '/resume', {})
            lifecycle = player.drive('result', base)
            assert 'result' in lifecycle, lifecycle
            visibility = lifecycle['result']
            assert visibility['quiet']['hidden'] and not visibility['returned']['hidden'], visibility
            resumed_state = visibility['before']['state']
            logical_now = resumed_state['serverNow'] - resumed_state['room'].get('clockOffsetMs', 0)
            expected_sound = (logical_now - sound['startedAt'] - (visibility['before']['at'] - visibility['first']['at'])) / 1000
            assert 0 < visibility['first']['time'] < 0.8756463 and abs(visibility['first']['time'] - expected_sound) < 0.2, (visibility, expected_sound)
            returned_state = visibility['returned']['state']
            active_after_visibility = returned_state['room']['game']['data']['audio']['sounds']
            returned_position = (returned_state['serverNow'] - returned_state['room'].get('clockOffsetMs', 0) - sound['startedAt']) / 1000
            assert any(value['seq'] == sound['seq'] for value in active_after_visibility) and returned_position < 0.8756463, 'visibility test missed the live original sound boundary'
            wait(lambda: player.audio() and not player.audio()['paused'], 'foreground did not restore current music')
            time.sleep(1)
            assert player.js('window.palNativeAudio.filter(e=>e.at>' + str(saved_cutoff) + '&&e.event==="playing"&&e.track?.startsWith("pal-sfx-")).length') == 1, 'foreground replayed an existing finite sound'
            player.run('find', 'role', 'button', 'click', '--name', '退出全屏') if player.js('!!document.querySelector("[data-room-fullscreen]")') else None
            player.tap('[data-sound-trigger]', base)
            player.run('snapshot', '-i')
            muted_cutoff = player.js('performance.now()')
            assert player.drive('arm-mute', base)
            player.command('load_checkpoint')
            lifecycle = player.drive('result', base)
            assert 'result' in lifecycle, lifecycle
            muted = lifecycle['result']
            assert not any(not media['paused'] for media in muted['quiet']['media']), muted
            returned_state = muted['returned']['state']
            mute_returned_position = (returned_state['serverNow'] - returned_state['room'].get('clockOffsetMs', 0) - sound['startedAt']) / 1000
            assert mute_returned_position < 0.8756463, 'mute test missed the live original sound boundary'
            wait(lambda: player.audio() and not player.audio()['paused'], 'unmute did not restore current music')
            time.sleep(1)
            assert player.js('window.palNativeAudio.filter(e=>e.at>' + str(muted_cutoff) + '&&e.event==="playing"&&e.track?.startsWith("pal-sfx-")).length') == 1, 'unmute replayed an existing finite sound'
            player.tap('[data-sound-trigger]', base)
            player.tap('[data-fullscreen-enter]', base)
            player.run('wait', '[data-room-fullscreen]')
            player.run('screenshot', OUT / f'audio-board-{width}.png')
            report['cases'].append({'viewport': [width, height], 'canvas': bounds, 'first': first, 'resumed': observed, 'expectedPosition': expected, 'music': music, 'sameNativeElementAcrossSnapshots': True, 'offsetError': abs(observed['time'] - expected), 'screenPages': pages, 'sound': sound, 'originalMidSoundSave': capture, 'restoredSound': visibility['first'], 'expectedSoundOffset': expected_sound, 'visibility': visibility, 'mute': muted, 'globalMuteAndCurrentMusicRestore': True, 'staleSoundNotReplayed': True, 'activeSoundAfterVisibility': active_after_visibility})
            player.close_helpers()
            player.run('close')
        report['passed'] = True
    except Exception as error:
        report['error'] = str(error)
        report['diagnostics'] = []
        for player in players:
            try:
                report['diagnostics'].append({'session': player.session, 'state': player.api('/state'), 'page': player.js('''(()=>{const board=document.querySelector('[data-pal-board]');let fiber=board&&board[Object.keys(board).find(k=>k.startsWith('__reactFiber'))],props,controller,master,input;for(let f=fiber;f;f=f.return){if(f.memoizedProps?.clock&&f.memoizedProps?.audio)props={sound:f.memoizedProps.sound,audioEnabled:f.memoizedProps.audioEnabled,clock:f.memoizedProps.clock};for(let h=f.memoizedState;h&&h.next!==undefined;h=h.next){const v=h.memoizedState;if(v?.current?.clockSeenAt!==undefined&&v.current.current){const i=v.current,p=i.current();input={enabled:i.enabled(),fresh:i.fresh(performance.now()),now:performance.now(),clockSeenAt:i.clockSeenAt,controlAt:i.controlAt,epoch:i.epoch,decided:[...i.decided],inFlight:i.inFlight,queue:i.queue,props:{busy:p.busy,controlAt:p.data.controlAt,epoch:p.data.epoch,dialogue:p.data.dialogue,phase:p.data.phase},focused:document.hasFocus()}}if(v?.current?.bank)controller={blocked:v.current.blocked,music:v.current.music,entries:[...v.current.bank.entries].map(([path,e])=>({path,url:e.url,bytes:e.bytes})),active:v.current.bank.active,jobs:v.current.bank.jobs.length};if(v?.tracks instanceof Map&&v?.pausedOwners instanceof Set)master={allowed:v.allowed,level:v.level,pausedOwners:[...v.pausedOwners],tracks:[...v.tracks].map(([k,t])=>({key:k,stopped:t.stopped,paused:t.paused,task:!!t.audio.task})),preparations:[...v.preparation.tasks].map(([k,t])=>[k,t.size])}}}return {events:window.palNativeAudio,hidden:document.hidden,board:board?Object.fromEntries([...board.attributes].map(a=>[a.name,a.value])):null,props,controller,master,input,errors:[...document.querySelectorAll('.pal-board-error,[role=alert]')].map(e=>e.textContent),media:[...document.querySelectorAll('audio')].map(m=>({dataset:Object.fromEntries(Object.entries(m.dataset)),src:m.src,paused:m.paused,ready:m.readyState,duration:m.duration,error:m.error?.message,muted:m.muted})),resources:performance.getEntriesByType('resource').filter(r=>r.name.includes('/audio-v1/')).map(r=>({name:r.name,bytes:r.decodedBodySize,duration:r.duration}))}})()''')} )
                player.run('screenshot', OUT / ('failure-' + str(len(report['diagnostics'])) + '.png'))
            except Exception:
                pass
        raise
    finally:
        for player in players:
            player.close_helpers()
            try:
                player.run('close')
            except Exception:
                pass
            shutil.rmtree(player.profile, ignore_errors=True)
        (OUT / 'result.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
        (OUT / 'stop').write_text('done\n')
    print(json.dumps({'passed': report['passed'], 'cases': len(report['cases'])}))


if __name__ == '__main__':
    main()
