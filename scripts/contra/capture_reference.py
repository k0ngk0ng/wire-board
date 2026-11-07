"""Record the pinned original Contra through real, dual-controller FCEUX input.

Failures considered before use: wrong ROM, escaping output names, stale completion
markers, ignored input, incomplete traces, Lua errors, orphan emulator processes,
and filesystem writes outside the workspace. This is a development oracle only.
"""
from pathlib import Path
import hashlib
import json
import os
import re
import subprocess
import sys
import time
import zipfile

ROOT = Path(__file__).resolve().parents[2]
BASE = Path(os.environ.get('CONTRA_REFERENCE_ROOT', ROOT / '.local/contra')).resolve()
if not BASE.is_relative_to(ROOT):
    raise ValueError('Contra reference output must remain within the workspace')
ROM_SHA256 = '6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
ROM_SOURCE = Path('/Users/jason/Documents/nes/roms/03/魂斗罗日版(完全版).zip')
KEYS = {'A', 'B', 'up', 'down', 'left', 'right', 'start', 'select'}


def prepare():
    for name in ('runs', 'states', 'fceux-home', 'runtime', 'tmp'):
        (BASE / name).mkdir(parents=True, exist_ok=True)
    # The offscreen Qt platform cannot create OpenGL contexts. Pin software
    # rendering and never borrow another game's mutable emulator preferences.
    (BASE / 'fceux-home/fceux.cfg').write_text(
        'SDL.OpenGL = 0\nSDL.VideoDriver = 2\nSDL.NoConfig = 1\nSDL.NewPPU = 0\nSDL.PAL = 0\n')
    with zipfile.ZipFile(ROM_SOURCE) as archive:
        raw = archive.read('Contra (J).nes')
    assert hashlib.sha256(raw).hexdigest() == ROM_SHA256, 'wrong original ROM'
    rom = BASE / 'contra-j.nes'
    if not rom.exists() or rom.read_bytes() != raw:
        rom.write_bytes(raw)
    (BASE / 'workspace.sb').write_text(
        '(version 1)\n(allow default)\n(deny file-write*)\n'
        f'(allow file-write* (subpath {json.dumps(str(ROOT))}) (literal "/dev/null") (literal "/dev/tty"))\n')
    return rom


def run(scenario_path):
    from PIL import Image
    spec = json.loads(Path(scenario_path).read_text())
    assert re.fullmatch(r'[a-z0-9][a-z0-9-]{0,64}', spec['name']), 'invalid run name'
    assert isinstance(spec['frames'], int) and 0 < spec['frames'] <= 100000
    for segment in spec.get('segments', []):
        assert 1 <= segment['start'] <= segment['end'] <= spec['frames']
        assert segment.get('player', 1) in (1, 2)
        assert set(segment['keys']) <= KEYS
    rom = prepare()
    out = BASE / 'runs' / spec['name']
    out.mkdir(parents=True, exist_ok=True)
    assert not any(out.iterdir()), 'use a new run name; never overwrite evidence'
    (out / 'scenario.json').write_text(json.dumps(spec, indent=2) + '\n')
    segments = '\n'.join(
        'if n >= %d and n <= %d then %s end' % (
            s['start'], s['end'], '; '.join('pads[%d][%s]=true' % (s.get('player', 1), json.dumps(k)) for k in s['keys']))
        for s in spec.get('segments', []))
    captures = '{' + ','.join('[%d]=true' % n for n in spec.get('captures', [])) + '}'
    start = 'emu.poweron()'
    initial_state = None
    if spec.get('load_state'):
        assert re.fullmatch(r'[a-z0-9-]+', spec['load_state'])
        state = BASE / 'states' / (spec['load_state'] + '.fcs')
        assert state.is_file(), 'missing source savestate'
        initial_state = {'name': state.name, 'sha256': hashlib.sha256(state.read_bytes()).hexdigest()}
        start = 'emu.frameadvance(); emu.frameadvance(); savestate.load(savestate.create(%s))' % json.dumps(str(state))
    saves = []
    for frame, name in spec.get('save_states', {}).items():
        assert re.fullmatch(r'[a-z0-9-]+', name)
        assert 1 <= int(frame) <= spec['frames']
        state = BASE / 'states' / (name + '.fcs')
        assert not state.exists(), 'do not overwrite initial-state evidence'
        saves.append('if n==%d then local s=savestate.create(%s); savestate.save(s); savestate.persist(s) end' % (int(frame), json.dumps(str(state))))
    lua = r'''
local out=assert(os.getenv("NES_PROBE_OUT"))
local ram=assert(io.open(out.."/ram.bin","wb"))
local frames=assert(io.open(out.."/frames.csv","w"))
local inputs=assert(io.open(out.."/inputs.csv","w"))
frames:write("frame,emulator_frame,lagged,pc\n")
inputs:write("frame,p1,p2\n")
local function bytes(first,count,reader)
 local t={};for i=first,first+count-1 do t[#t+1]=string.char(reader(i)) end;return table.concat(t)
end
local function writefile(name,data)
 local f=assert(io.open(out.."/"..name,"wb"));f:write(data);f:close()
end
local keys={"A","B","up","down","left","right","start","select"}
local bits={A=128,B=64,select=32,start=16,up=8,down=4,left=2,right=1}
local function packed_pad(pad)
 local result=0
 for key,value in pairs(pad)do
  assert(bits[key] and type(value)=='boolean','invalid controller callback value')
  if value then result=result+bits[key] end
 end
 return result
end
local captures=CAPTURES
START
emu.speedmode("turbo")
writefile("initial-ram.bin",bytes(0,2048,memory.readbyte))
writefile("initial-prg.bin",bytes(0x8000,0x8000,memory.readbyte))
do
HOOKS
end
for n=1,FRAMES do
 local pads={{},{}};for p=1,2 do for _,k in ipairs(keys) do pads[p][k]=false end end
 SEGMENTS
 if control_frame then control_frame(n,pads) end
 inputs:write(string.format("%d,%d,%d\n",n,packed_pad(pads[1]),packed_pad(pads[2])))
 for p=1,2 do joypad.set(p,pads[p]) end
 emu.frameadvance()
 ram:write(bytes(0,2048,memory.readbyte))
 frames:write(string.format("%d,%d,%s,%04x\n",n,emu.framecount(),tostring(emu.lagged()),memory.getregister("pc")))
 if captures[n] then
  local stem=string.format("frame-%05d",n)
  writefile(stem..".gd",gui.gdscreenshot(true))
  writefile(stem.."-prg.bin",bytes(0x8000,0x8000,memory.readbyte))
  writefile(stem.."-ppu.bin",bytes(0,0x4000,ppu.readbyte))
 end
 SAVES
end
ram:close();frames:close();inputs:close()
if finish_probe then finish_probe() end
writefile("capture-complete.tmp",'{"status":"complete"}')
assert(os.rename(out.."/capture-complete.tmp",out.."/capture-complete.json"))
while true do end
'''
    for key, value in [('CAPTURES', captures), ('START', start), ('HOOKS', spec.get('hooks_lua', '')),
                       ('FRAMES', str(spec['frames'])), ('SEGMENTS', segments), ('SAVES', '\n'.join(saves))]:
        lua = lua.replace(key, value)
    script = out / 'record.lua'
    script.write_text(lua)
    env = dict(os.environ, FCEUX_HOME=str(BASE/'fceux-home'), FCEUX_CONFIG_DIR=str(BASE/'fceux-home'),
               QT_QPA_PLATFORM='offscreen', XDG_RUNTIME_DIR=str(BASE/'runtime'), NES_PROBE_OUT=str(out),
               TMPDIR=str(BASE/'tmp'))
    emulator = '/opt/homebrew/bin/fceux'
    launcher = Path(emulator).resolve()
    executable = launcher.parent.parent/'libexec/fceux'
    macho = {b'\xcf\xfa\xed\xfe', b'\xfe\xed\xfa\xcf', b'\xca\xfe\xba\xbe', b'\xbe\xba\xfe\xca'}
    if launcher.read_bytes()[:4] in macho:
        executable = launcher
    assert executable.is_file() and executable.read_bytes()[:4] in macho, 'cannot identify actual FCEUX executable'
    cmd = ['sandbox-exec', '-f', str(BASE/'workspace.sb'), emulator,
           '--sound', '0', '--loadlua', str(script), str(rom)]
    completion = out / 'capture-complete.json'
    completed = False
    with (out/'emulator.log').open('w') as log:
        process = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT)
        started = time.monotonic()
        try:
            while process.poll() is None:
                if completion.exists():
                    assert json.loads(completion.read_text()) == {'status': 'complete'}
                    completed = True
                    break
                diagnostics = (out/'emulator.log').read_text(errors='replace')
                if any(marker in diagnostics for marker in ('Lua thread bombed out', 'Failed to compile file:')):
                    raise RuntimeError(f'Lua capture failed: {out}')
                if time.monotonic() - started > spec.get('timeout_seconds', 45):
                    raise TimeoutError(f'Original capture timed out: {out}')
                time.sleep(.05)
        finally:
            if process.poll() is None:
                process.kill()
            process.wait()
    assert completed, f'FCEUX ended before capture completed: {out}'
    assert (out/'ram.bin').stat().st_size == spec['frames'] * 2048
    for gd in sorted(out.glob('*.gd')):
        b = gd.read_bytes()
        width, height = int.from_bytes(b[2:4], 'big'), int.from_bytes(b[4:6], 'big')
        assert len(b) == 11 + width * height * 4
        Image.frombytes('RGBA', (width, height), b[11:], 'raw', 'ARGB').convert('RGB').save(gd.with_suffix('.png'))
    hashes = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(out.iterdir())
              if p.suffix in {'.bin', '.csv', '.png', '.lua', '.json'} and p.name not in {'source.json', 'hashes.json'}}
    (out/'hashes.json').write_text(json.dumps(hashes, indent=2) + '\n')
    source = {'rom_sha256': ROM_SHA256, 'source_archive': str(ROM_SOURCE), 'initial_state': initial_state,
              'emulator': emulator, 'emulator_sha256': hashlib.sha256(Path(emulator).read_bytes()).hexdigest(),
              'emulator_launcher_sha256': hashlib.sha256(launcher.read_bytes()).hexdigest(),
              'emulator_executable': str(executable), 'emulator_executable_sha256': hashlib.sha256(executable.read_bytes()).hexdigest(),
              'frames': spec['frames'], 'scenario_sha256': hashes['scenario.json'],
              'has_custom_hooks': bool(spec.get('hooks_lua')), 'completed': completed}
    (out/'source.json').write_text(json.dumps(source, indent=2) + '\n')
    print(json.dumps({'run': spec['name'], 'frames': spec['frames'], 'ram_sha256': hashes['ram.bin']}))
    return out


if __name__ == '__main__':
    run(sys.argv[1])
