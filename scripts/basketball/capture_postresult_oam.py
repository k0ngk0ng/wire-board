"""Read-only original CE09 input/output and selected DMA lineage.
Complete controller timeline and first-BA40 overrides are inherited verbatim
from the declared controlled postresult reference. No new writes are added.
"""
import argparse,csv,hashlib,json
from capture_reference import ROOT,BASE,run

HOOK=r'''
local oamlog=assert(io.open(out..'/oam-source.jsonl','w'))
local oamraw=assert(io.open(out..'/oam-source.ram','wb'))
local dmalog=assert(io.open(out..'/dma-source.jsonl','w'))
local ordinal=0;local pending=nil;local latest=-1;local handoff=false
local selected=SELECTED
local function scope()
 local scene=memory.readbyte(0x588);local phase=memory.readbyte(0x589)%128
 return scene==1 and (phase==6 or phase==7) or scene==7 and phase<=10
end
memory.registerexec(0xce09,1,function()
 if not scope() then return end
 assert(not pending)
 pending={before=memory.readbyterange(0,2048),frame=emu.framecount(),cycles=debugger.getcyclescount()}
end)
memory.registerexec(0xd078,1,function()
 if not pending then return end
 oamraw:write(pending.before);oamraw:write(memory.readbyterange(0,2048))
 oamlog:write(string.format('{"index":%d,"startFrame":%d,"endFrame":%d,"startCycles":%d,"endCycles":%d}\n',ordinal,pending.frame,emu.framecount(),pending.cycles,debugger.getcyclescount()))
 latest=ordinal;ordinal=ordinal+1;pending=nil
end)
memory.registerexec(0xbca0,1,function()
 if handoff or memory.readbyte(0xbfff)~=4 then return end
 local scene=memory.readbyte(0x588);local phase=memory.readbyte(0x589)
 if not(scene==1 and phase==7 or scene==7 and phase==0) then return end
 handoff=true;writefile('handoff.ram',memory.readbyterange(0,2048))
 local parts={};for a=0,0x3fff do parts[#parts+1]=string.char(ppu.readbyte(a))end
 writefile('handoff-ppu.bin',table.concat(parts))
 writefile('handoff.json',string.format('{"frame":%d,"cycles":%d,"pc":48288}',emu.framecount(),debugger.getcyclescount()))
end)
memory.registerwrite(0x4014,1,function(a,s,v)
 if not selected[emu.framecount()] then return end
 dmalog:write(string.format('{"frame":%d,"cycles":%d,"submittedIndex":%d,"partialDraw":%s}\n',emu.framecount(),debugger.getcyclescount(),latest,tostring(pending~=nil)))
 writefile(string.format('dma-%05d.bin',emu.framecount()),memory.readbyterange(v*256,256))
end)
function finish_probe()assert(not pending);oamlog:close();oamraw:close();dmalog:close()end
'''

def main():
 p=argparse.ArgumentParser();p.add_argument('kind',choices=['reward','ending']);args=p.parse_args()
 reference=BASE/f'runs/postresult-{args.kind}-reference';spec=json.loads((reference/'scenario.json').read_text())
 with (reference/'frames.csv').open() as f:mapping={int(r['frame']):int(r['emulator_frame']) for r in csv.DictReader(f)}
 selected={mapping[n] for n in spec['captures']};table='{'+','.join(f'[{n}]=true' for n in sorted(selected))+'}'
 spec['name']=f'postresult-{args.kind}-oam';spec['hooks_lua']+='\n'+HOOK.replace('SELECTED',table)
 target=ROOT/'.local/postresult-surface-source'/f'{args.kind}-capture.json';target.write_text(json.dumps(spec,indent=2)+'\n');run(target)
 observed=BASE/'runs'/spec['name']
 files=['ram.bin','injection.json']+[f'frame-{n:05d}.png' for n in spec['captures']]
 assert all((observed/name).read_bytes()==(reference/name).read_bytes() for name in files)
 rows=[json.loads(l) for l in (observed/'oam-source.jsonl').read_text().splitlines()];assert (observed/'oam-source.ram').stat().st_size==len(rows)*4096
 report=dict(kind=args.kind,fullCalls=len(rows),crossVideoCalls=sum(r['startFrame']!=r['endFrame'] for r in rows),matchesReference=True,hashes={name:hashlib.sha256((observed/name).read_bytes()).hexdigest() for name in ['oam-source.ram','oam-source.jsonl','dma-source.jsonl','ram.bin']})
 (observed/'oam-report.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report))
if __name__=='__main__':main()
