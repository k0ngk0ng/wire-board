"""Trace the original AI entry/return and retain state changes for reconstruction.

No replacement AI or injected decisions are used. A bank guard distinguishes
the same CPU address in different MMC3 mappings. Original replay must stay equal.
"""
from pathlib import Path
import argparse,csv,json,struct
from capture_reference import BASE,run

p=argparse.ArgumentParser();p.add_argument('--scenario',type=Path,default=BASE/'shot-observation.json');args=p.parse_args()
spec=json.loads(args.scenario.read_text());reference=BASE/'runs'/spec['name'];spec['name']='ai-'+spec['name']
spec['hooks_lua']='''
local trace=assert(io.open(out.."/ai.csv","w"))
local raw=assert(io.open(out.."/ai-ram.bin","wb"))
trace:write("frame,actor,action,request_before,mode_before,next_before,holder,request_after,mode_after,next_after\\n")
local pending=nil
local function ram() local t={};for i=0,2047 do t[#t+1]=string.char(memory.readbyte(i)) end;return table.concat(t) end
memory.registerexec(0xa05d,1,function()
 if memory.readbyte(0xbfff)~=5 then return end
 assert(not pending,"nested AI entry needs investigation")
 local x=memory.getregister("x")
 pending={emu.framecount(),x,memory.readbyte(0x512+x),memory.readbyte(0x51f+x),memory.readbyte(0x672+x),memory.readbyte(0x676+x),memory.readbyte(0x53f)}
 raw:write(ram())
end)
memory.registerexec(0xfd8e,1,function()
 assert(pending,"AI return without matching entry")
 local x=memory.getregister("x");assert(x==pending[2],"AI changed actor index")
 pending[#pending+1]=memory.readbyte(0x51f+x)
 pending[#pending+1]=memory.readbyte(0x672+x)
 pending[#pending+1]=memory.readbyte(0x676+x)
 trace:write(table.concat(pending,",").."\\n");raw:write(ram());pending=nil
end)
function finish_probe() assert(not pending);trace:close();raw:close() end
'''
path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
capture=BASE/'runs'/spec['name']
assert (capture/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes(),'AI tracing changed original game state'
screenshots=list(reference.glob('frame-*.png'))
for image in screenshots:
    assert image.read_bytes()==(capture/image.name).read_bytes(),'AI tracing changed original rendering'
rows=list(csv.DictReader((capture/'ai.csv').open()));assert rows
assert (capture/'ai-ram.bin').stat().st_size==len(rows)*4096
rom=(BASE/'basketball.nes').read_bytes()[16:16+131072];bank=rom[5*16384:6*16384]
handlers=struct.unpack('<42H',bank[0x209c:0x20f0])
out=Path('.local/basketball/ai');out.mkdir(parents=True,exist_ok=True)
summary={'calls':len(rows),'actors':sorted({int(r['actor']) for r in rows}),'observed_modes':sorted({int(r['mode_before'])&0x7f for r in rows}),'dispatch_table':[{'mode':i,'address':f'0x{a:04x}'} for i,a in enumerate(handlers)],'state_changed_by_observation':False,'identical_screenshots':len(screenshots),'source':'PRG 16 KiB bank 5, CPU $A05D; return at fixed-bank $FD8E','status':'captured, not implemented'}
(out/'capture.json').write_text(json.dumps(summary,indent=2)+'\n')
(out/'bank5-visible.bin').write_bytes(bank+rom[-16384:])
seeds='0xa05d\tfunction\tOriginalAIDispatch\n'+''.join(f'0x{a:04x}\tfunction\tOriginalAIHandler_{a:04x}\n' for a in sorted(set(handlers)))
(out/'seeds.tsv').write_text(seeds)
print(json.dumps(summary))
