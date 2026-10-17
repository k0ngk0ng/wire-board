"""Read-only CPU/PPU IRQ write timing for original configured court captures.
The court selection is the already-declared isolated configuration probe; no
additional original state is changed. Timing is not inferred from image pixels.
"""
from pathlib import Path
import json,csv,argparse
from capture_reference import ROOT,BASE,run

def main(holdout=False):
 records=[]
 for court in [3,4]:
  reference=BASE/f'runs/court-{court}-{"irq-holdout-reference" if holdout else "reference"}';spec=json.loads((reference/'scenario.json').read_text())
  with (reference/'frames.csv').open() as f:frames={int(r['frame']):int(r['emulator_frame']) for r in csv.DictReader(f)}
  windows={frames[n]+delta for n in spec['captures'] for delta in range(-2,3)}
  hook='local selected={'+','.join('[%d]=true'%n for n in sorted(windows))+'}\n'+r'''
local log=assert(io.open(out.."/irq.jsonl","w"))
writefile("initial-clock.json",string.format('{"emulator_frame":%d,"cycles":%d}',emu.framecount(),debugger.getcyclescount()))
local function record(kind,addr,value)
 if selected[emu.framecount()] then log:write(string.format('{"kind":"%s","frame":%d,"cycles":%d,"pc":%d,"address":%d,"value":%d}\n',kind,emu.framecount(),debugger.getcyclescount(),memory.getregister("pc"),addr or 0,value or 0)) end
end
emu.registerbefore(function()record('before')end)
emu.registerafter(function()record('after')end)
for _,addr in ipairs({0x2000,0x2001,0x2005,0x2006,0x8000,0x8001,0xc000,0xc001,0xe000,0xe001})do memory.registerwrite(addr,1,function(a,z,v)record('write',a,v)end)end
for _,pc in ipairs({0xf298,0xf2d4,0xf317,0xf365})do memory.registerexec(pc,1,function()record('exec',pc)end)end
function finish_probe()log:close()end
'''
  spec.update(name=f'court-{court}-{"irq-holdout-timing" if holdout else "irq-clock-observed"}',hooks_lua=spec['hooks_lua']+'\n'+hook);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=BASE/'runs'/spec['name']
  if not (observed/'capture-complete.json').exists() or json.loads((observed/'scenario.json').read_text())!=spec:run(path)
  assert (reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
  for n in spec['captures']:assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
  records.append({'court':court,'original_source':json.loads((reference/'source.json').read_text()),'run':spec['name'],'observed_source':json.loads((observed/'source.json').read_text()),'frames':[{'frame':n,'emulator_frame':frames[n]} for n in spec['captures']],'ram_and_png_equal':True})
 out=ROOT/'.local/basketball/irq-raster';out.mkdir(exist_ok=True);(out/('holdout-timing-capture.json' if holdout else 'clock-capture.json')).write_text(json.dumps({'scope':'original IRQ writes and cycle timestamps; not Go runtime timing','records':records},indent=2)+'\n')
if __name__=='__main__':
 p=argparse.ArgumentParser();p.add_argument('--holdout',action='store_true');main(p.parse_args().holdout)
