"""Capture original music 76 twice in private evidence directories; no catalog writes."""
from pathlib import Path
import csv, fcntl, hashlib, json, os, subprocess, time, wave
from extract_audio import LUA, SOURCE_SHA, carrier

ROOT = Path(__file__).resolve().parents[2]
ASSETS = ROOT / '.local/music76-source/assets'
OLD = ROOT / '.local/basketball/audio/export'
BASE = ROOT / '.local/nes-basket'
NAMES = ['full.wav', 'driver-ram.bin', 'sample-cursors.csv', 'sequence-jumps.csv', 'apu.csv', 'calls.csv']

def sha(p):
    h = hashlib.sha256()
    with p.open('rb') as f:
        while b := f.read(1048576): h.update(b)
    return h.hexdigest()

def save(p, v): p.write_text(json.dumps(v, indent=2) + '\n')

def main():
    ASSETS.mkdir(parents=True, exist_ok=True)
    locks = []
    for p in [OLD/'court-capture.lock', ASSETS/'capture.lock']:
        lock = p.open('a'); fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB); locks.append(lock)
    rom = (BASE/'basketball.nes').read_bytes()
    assert hashlib.sha256(rom).hexdigest() == SOURCE_SHA
    nsf, bridge = OLD/'sound-076.nsf', OLD/'wave_bridge.dylib'
    assert nsf.read_bytes() == carrier(rom, 76)
    tool = {'romSHA256': SOURCE_SHA, 'nsfSHA256': sha(nsf), 'bridgeSHA256': sha(bridge),
            'fceuxSHA256': sha(Path('/opt/homebrew/Cellar/fceux/2.6.6_12/libexec/fceux'))}
    settings = dict(tool, frames=45000, volume=192, sampleRate=44100, channels=1, sampleWidth=2)
    for repeat in (1, 2):
        run = ASSETS/f'capture-45000-run-{repeat}'; info = run/'capture-info.json'
        if info.exists():
            old = json.loads(info.read_text()); assert old['settings'] == settings
            assert all(sha(run/k) == v for k, v in old['files'].items())
            print('reused', run.name, flush=True); continue
        run.mkdir(exist_ok=True)
        assert not (run/'complete').exists(), 'unverified completed capture requires inspection'
        home = run/'fceux-home'; home.mkdir(exist_ok=True)
        (home/'fceux.cfg').write_text('SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.VideoVsync = 0\n')
        script = run/'record.lua'; script.write_text(LUA)
        env = dict(os.environ, QT_QPA_PLATFORM='offscreen', SDL_AUDIODRIVER='dummy',
                   FCEUX_HOME=str(home), FCEUX_CONFIG_DIR=str(home),
                   XDG_RUNTIME_DIR=str(BASE/'runtime'), TMPDIR=str(ROOT/'.local/tmp'),
                   NES_AUDIO_OUT=str(run), NES_AUDIO_WAV=str(run/'full.wav'), NES_AUDIO_BRIDGE=str(bridge),
                   NES_AUDIO_FRAMES='45000', NES_AUDIO_FRAME_COUNTS=str(run/'sample-cursors.csv'))
        cmd = ['sandbox-exec', '-f', str(BASE/'workspace.sb'), '/opt/homebrew/bin/fceux',
               '--pal', '0', '--sound', '1', '--soundrate', '44100', '--soundq', '2',
               '--volume', '192', '--loadlua', str(script), str(nsf)]
        print('capturing', run.name, flush=True)
        started = time.monotonic()
        with (run/'emulator.log').open('w') as log:
            process = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT)
            try:
                while process.poll() is None and not (run/'complete').exists():
                    assert time.monotonic()-started < 90, 'capture deadline exceeded'
                    assert 'Lua thread bombed out' not in (run/'emulator.log').read_text(), 'Lua error'
                    time.sleep(.05)
                assert (run/'complete').read_text() == 'complete'
            finally:
                if process.poll() is None: process.kill()
                process.wait()
        assert (run/'driver-ram.bin').stat().st_size == 45000*231
        cursors = list(csv.DictReader((run/'sample-cursors.csv').open()))
        assert len(cursors) == 45001
        calls = list(csv.DictReader((run/'calls.csv').open()))
        assert [int(r['value']) for r in calls if r['kind'] == 'init'] == [0, 76]
        with wave.open(str(run/'full.wav')) as f:
            assert (f.getnchannels(), f.getsampwidth(), f.getframerate()) == (1,2,44100)
            count = f.getnframes(); assert count == int(cursors[-1]['samples'])
        save(info, {'settings': settings, 'files': {k: sha(run/k) for k in NAMES},
                    'command': cmd, 'captureWallSeconds': time.monotonic()-started, 'samples': count})
        print('completed', run.name, count, 'samples', flush=True)
    first, second = [ASSETS/f'capture-45000-run-{n}' for n in (1,2)]
    assert all(sha(first/k) == sha(second/k) for k in NAMES)
    prefix = OLD/'court-music-076-20000-run-1'
    for k in NAMES[1:]: assert (first/k).read_bytes().startswith((prefix/k).read_bytes()), k
    with wave.open(str(first/'full.wav')) as a, wave.open(str(prefix/'full.wav')) as b:
        n = b.getnframes(); assert a.readframes(n) == b.readframes(n)
    cursors = list(csv.DictReader((first/'sample-cursors.csv').open()))
    start, end = [int(cursors[i]['samples']) for i in (4304,41168)]
    assert start == 3157480
    states = (first/'driver-ram.bin').read_bytes()
    assert states[4303*231:4304*231] == states[41167*231:41168*231]
    result = {'status': 'passed', 'settings': settings, 'independentCapturesIdentical': True,
              'old20000FramePrefixIdentical': True, 'sourceFrames': [4304,41168],
              'loopStartSample': start, 'loopEndSample': end, 'loopSamples': end-start,
              'sampleRate': 44100, 'prefixSeconds': start/44100, 'loopSeconds': (end-start)/44100,
              'seconds': end/44100, 'files': {k: sha(first/k) for k in NAMES}}
    save(ASSETS/'capture-verification.json', result); print(json.dumps(result), flush=True)

if __name__ == '__main__': main()
