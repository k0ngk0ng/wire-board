"""Execute a recorded controller timeline in a workspace-contained FCEUX process."""
from pathlib import Path
import hashlib
import json
import os
import re
import subprocess
import sys
import time
from PIL import Image

ROOT = Path(__file__).resolve().parents[2]
BASE = ROOT / '.local/nes-basket'


def run(scenario_path):
    spec = json.loads(Path(scenario_path).read_text())
    if not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,64}', spec['name']):
        raise ValueError('Capture names must stay inside the research output directory')
    rom_digest = hashlib.sha256((BASE/'basketball.nes').read_bytes()).hexdigest()
    if rom_digest != '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b':
        raise ValueError('The ROM differs from the version pinned by this reconstruction')
    out = BASE / 'runs' / spec['name']
    out.mkdir(parents=True, exist_ok=True)
    completion = out / 'capture-complete.json'
    completion.unlink(missing_ok=True)
    (out / 'scenario.json').write_text(json.dumps(spec, indent=2)+'\n')
    segments = '\n'.join(
        'if n >= %d and n <= %d then %s end' % (
            s['start'], s['end'], '; '.join('pad[%s]=true' % json.dumps(k) for k in s['keys']))
        for s in spec.get('segments', []))
    captures = '{' + ','.join('[%d]=true' % n for n in spec.get('captures', [])) + '}'
    start = 'emu.poweron()'
    if spec.get('load_slot') is not None:
        state = BASE / 'states' / ('slot-%d.fcs' % spec['load_slot'])
        assert state.is_file(), 'Missing initial savestate'
        start = 'emu.frameadvance(); emu.frameadvance(); savestate.load(savestate.create(%s))' % json.dumps(str(state))
    save = ''
    if spec.get('save_slot') is not None:
        state = BASE / 'states' / ('slot-%d.fcs' % spec['save_slot'])
        state.parent.mkdir(parents=True, exist_ok=True)
        save = 'local s=savestate.create(%s); savestate.save(s); savestate.persist(s)' % json.dumps(str(state))
    hooks = spec.get('hooks_lua', '')
    lua = '''local out = assert(os.getenv("NES_PROBE_OUT"))
local ram = assert(io.open(out.."/ram.bin","wb"))
local events = assert(io.open(out.."/frames.csv","w"))
events:write("frame,emulator_frame,lagged,pc\\n")
local function bytes(first,count,reader)
  local t={}; for i=first,first+count-1 do t[#t+1]=string.char(reader(i)) end
  return table.concat(t)
end
local function writefile(name,data)
  local f=assert(io.open(out.."/"..name,"wb")); f:write(data); f:close()
end
local keys={"A","B","up","down","left","right","start","select"}
local captures=CAPTURES
START
emu.speedmode("turbo")
writefile("initial-ram.bin",bytes(0,2048,memory.readbyte))
writefile("initial-prg.bin",bytes(0x8000,0x8000,memory.readbyte))
HOOKS
for n=1,FRAMES do
  local pad={}; for _,k in ipairs(keys) do pad[k]=false end
  SEGMENTS
  joypad.set(1,pad)
  local empty={}; for _,k in ipairs(keys) do empty[k]=false end
  for player=2,4 do joypad.set(player,empty) end
  emu.frameadvance()
  ram:write(bytes(0,2048,memory.readbyte))
  events:write(string.format("%d,%d,%s,%04x\\n",n,emu.framecount(),tostring(emu.lagged()),memory.getregister("pc")))
  if captures[n] then
    local stem=string.format("frame-%05d",n)
    writefile(stem..".gd",gui.gdscreenshot(true))
    writefile(stem.."-prg.bin",bytes(0x8000,0x8000,memory.readbyte))
    writefile(stem.."-palette.bin",bytes(0x3f00,32,ppu.readbyte))
  end
end
SAVE
ram:close(); events:close()
if finish_probe then finish_probe() end
writefile("capture-complete.tmp",'{"status":"complete"}')
assert(os.rename(out.."/capture-complete.tmp",out.."/capture-complete.json"))
-- Do not return through FCEUX's Qt/Lua shutdown path. No further frame is
-- advanced; the parent observes the closed-file marker and reaps this process.
while true do end
'''
    # Hook-local helpers must not shadow the recorder's RAM/output handles.
    for key, value in [('CAPTURES', captures), ('START', start), ('HOOKS', 'do\n'+hooks+'\nend'),
                       ('FRAMES', str(spec['frames'])), ('SEGMENTS', segments), ('SAVE', save)]:
        lua = lua.replace(key, value)
    script = out / 'record.lua'
    script.write_text(lua)
    env = dict(os.environ, FCEUX_HOME=str(BASE/'fceux-home'), FCEUX_CONFIG_DIR=str(BASE/'fceux-home'),
               QT_QPA_PLATFORM='offscreen', XDG_RUNTIME_DIR=str(BASE/'runtime'), NES_PROBE_OUT=str(out),
               TMPDIR=str(ROOT/'.local/tmp'))
    cmd = ['sandbox-exec','-f',str(BASE/'workspace.sb'),'/opt/homebrew/bin/fceux',
           '--sound','0','--loadlua',str(script),str(BASE/'basketball.nes')]
    with (out/'emulator.log').open('w') as log:
        p = subprocess.Popen(cmd,env=env,stdout=log,stderr=subprocess.STDOUT)
        started=time.monotonic();offset=0
        completed=False
        try:
            while p.poll() is None:
                if completion.is_file():
                    completed=json.loads(completion.read_text())=={'status':'complete'}
                    if not completed:raise RuntimeError('Invalid capture completion marker')
                    break
                with (out/'emulator.log').open('rb') as recorded:
                    recorded.seek(offset);new=recorded.read();offset+=len(new)
                if b'Lua thread bombed out' in new:
                    raise RuntimeError(f'Lua capture failed; see {out}/emulator.log')
                if time.monotonic()-started>30:
                    raise TimeoutError(f'Capture timed out; see {out}/emulator.log')
                time.sleep(.05)
        finally:
            if p.poll() is None:
                if completed:
                    # All outputs are closed and emulation is paused. Avoid Qt
                    # shutdown on its Lua worker, including its SIGTERM path.
                    p.kill();p.wait()
                else:
                    p.terminate()
                    try:p.wait(timeout=3)
                    except subprocess.TimeoutExpired:p.kill();p.wait()
    assert completed, f'FCEUX exited {p.returncode} before capture completion; see {out}/emulator.log'
    assert (out/'ram.bin').stat().st_size==spec['frames']*2048, 'Incomplete RAM trace'
    for gd in sorted(out.glob('*.gd')):
        b=gd.read_bytes(); w=int.from_bytes(b[2:4],'big'); h=int.from_bytes(b[4:6],'big')
        assert len(b)==11+w*h*4
        Image.frombytes('RGBA',(w,h),b[11:],'raw','ARGB').convert('RGB').save(gd.with_suffix('.png'))
    hashes={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(out.iterdir())
            if p.suffix in {'.bin','.csv','.png','.gd','.lua'}}
    (out/'hashes.json').write_text(json.dumps(hashes,indent=2)+'\n')
    (out/'source.json').write_text(json.dumps({'rom_sha256':rom_digest,'frames':spec['frames'],'scenario_sha256':hashlib.sha256((out/'scenario.json').read_bytes()).hexdigest()},indent=2)+'\n')
    print(json.dumps({'run':spec['name'],'frames':spec['frames'],'captures':len(spec.get('captures',[])),
                      'ram_sha256':hashes['ram.bin']}))


if __name__=='__main__':
    run(sys.argv[1])
