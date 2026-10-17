"""Record the original pre-collision movement block without changing game state.

The declared failure cases in games/basketball/VERIFICATION.md precede the Go
implementation. Captures are the independent input/output oracle for its tests.
"""
from pathlib import Path
import argparse,csv,hashlib,json
from capture_reference import BASE,run

p=argparse.ArgumentParser();p.add_argument('--output',type=Path,default=Path('games/basketball/engine/testdata/motion.csv'));args=p.parse_args()
hook='''
local trace=assert(io.open(out.."/motion.csv","w"))
trace:write("frame,object,x,y,z,vx,vy,vz,gravity,next_x,next_y,next_z,next_vz\\n")
local pending=nil
local function word(lo,hi,x) return memory.readbyte(lo+x)+256*memory.readbyte(hi+x) end
local function wide(lo,mid,hi,x) return word(lo,mid,x)+65536*memory.readbyte(hi+x) end
memory.registerexec(0xef5f,1,function()
 local x=memory.getregister("x")
 pending={emu.framecount(),x,wide(0x68,0x75,0x82,x),word(0x8f,0x9c,x),wide(0xa9,0xb6,0xc3,x),word(0x478,0x485,x),word(0x492,0x49f,x),word(0x4ac,0x4b9,x),word(0x4c6,0x4d3,x)}
end)
memory.registerexec(0xefd5,1,function()
 assert(pending,"missing movement entry")
 local x=memory.getregister("x");assert(x==pending[2])
 pending[#pending+1]=wide(0x58e,0x59b,0x5a8,x)
 pending[#pending+1]=word(0x5b5,0x5c2,x)
 pending[#pending+1]=wide(0xa9,0xb6,0xc3,x)
 pending[#pending+1]=word(0x4ac,0x4b9,x)
 trace:write(table.concat(pending,",").."\\n");pending=nil
end)
function finish_probe() trace:close() end
'''
rows=[]
for name in ['idle','right','left','jump','a','b']:
    spec=json.loads((BASE/f'control-{name}.json').read_text());spec['name']='motion-'+name;spec['hooks_lua']=hook
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    capture=BASE/'runs'/spec['name'];reference=BASE/'runs'/('control-'+name)
    assert (capture/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes(),'Instrumentation changed original state'
    for row in csv.DictReader((capture/'motion.csv').open()):rows.append({'scenario':name,**row})
args.output.parent.mkdir(parents=True,exist_ok=True)
with args.output.open('w') as file:
    writer=csv.DictWriter(file,fieldnames=list(rows[0]));writer.writeheader();writer.writerows(rows)
manifest={'rom_sha256':'0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b','original_full_nes_sha256':'4f1372eb9c2d2d7dcfdfb75827c291f9970716a5fe6ed58d33872851e1ef9968','source_bank_8k':15,'entry':'0xef5f','exit_before_collision':'0xefd5','rows':len(rows),'scenarios':sorted({r['scenario'] for r in rows}),'sha256':hashlib.sha256(args.output.read_bytes()).hexdigest(),'command':'python3 -B scripts/basketball/capture_physics.py','source_is_original_execution':True}
(args.output.parent/'motion-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps({'observations':len(rows),'scenarios':6,'fixture':str(args.output)}))
