"""Observe original per-frame CHR selection without altering stage state."""
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys

ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-levels/chr-runtime'))
from capture_reference import BASE,run
from levels_data import original

HOOK=r'''
local changes=assert(io.open(out..'/chr-writes.jsonl','w'))
local frames=assert(io.open(out..'/chr-frames.jsonl','w'))
local animation=assert(io.open(out..'/chr-animation.jsonl','w'))
local applied={};local desired={};local ordinal=0
local pending=nil
for i=0,7 do applied[i]=memory.readbyte(0x7f0+i);desired[i]=applied[i]end
local function vector(v)local t={};for i=0,7 do t[#t+1]=tostring(v[i])end;return '['..table.concat(t,',')..']'end
local function snapshot()
 return string.format('{"stage":%d,"sceneType":%d,"transition":%d,"timer":%d,"phase":%d,"pace":%d,"alternate":%d,"random":%d,"screen":%d,"counter":%d,"desired":%s}',memory.readbyte(0x30),memory.readbyte(0x40),memory.readbyte(0x6a),memory.readbyte(0x5c),memory.readbyte(0x71),memory.readbyte(0x5d),memory.readbyte(0x48),memory.readbyte(0x34),memory.readbyte(0x64),memory.readbyte(0x1a),vector(desired))
end
memory.registerexec(0xc83b,1,function()
 -- CONTROLLED_INPUT
 assert(not pending)
 local s=memory.getregister('s');local ret=memory.readbyte(0x100+AND(s+1,255))+256*memory.readbyte(0x100+AND(s+2,255))+1
 pending={before=snapshot(),sp=AND(s+2,255),frame=emu.framecount()}
 memory.registerexec(ret,1,function()
  if pending and pending.sp==memory.getregister('s')then
   local p=pending;pending=nil
   animation:write(string.format('{"frame":%d,"before":%s,"after":%s}\n',p.frame,p.before,snapshot()))
  end
 end)
end)
local function emit(kind,a,before,v,slot)
 changes:write(string.format('{"frame":%d,"cycles":%d,"pc":%d,"kind":"%s","address":%d,"slot":%d,"before":%d,"after":%d,"stage":%d,"sceneType":%d,"counter":%d,"screen":%d,"scroll":%d}\n',emu.framecount(),debugger.getcyclescount(),memory.getregister('pc'),kind,a,slot,before,v,memory.readbyte(0x30),memory.readbyte(0x40),memory.readbyte(0x1a),memory.readbyte(0x64),memory.readbyte(0x65)))
end
memory.registerwrite(0x7f0,8,function(a,s,v)
 local slot=a-0x7f0;if desired[slot]~=v then emit('desired',a,desired[slot],v,slot)end;desired[slot]=v
end)
memory.registerwrite(0xb000,0x3004,function(a,s,v)
 local bank=math.floor((a-0xb000)/0x1000);local reg=AND(a,0xfff)
 if bank<0 or bank>3 or reg>3 then return end
 local slot=bank*2+math.floor(reg/2);local before=applied[slot]
 local after=AND(reg,1)==0 and (AND(before,0xf0)+AND(v,15))or (AND(before,15)+AND(v,15)*16)
 if after~=before then emit('mapper',a,before,after,slot)end;applied[slot]=after
end)
emu.registerafter(function()
 ordinal=ordinal+1
 frames:write(string.format('{"frame":%d,"emulatorFrame":%d,"desired":%s,"applied":%s}\n',ordinal,emu.framecount(),vector(desired),vector(applied)))
end)
function finish_probe()assert(not pending);changes:close();frames:close();animation:close()end
'''


def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1';rom,_=original();chr=rom[16+131072:]
    (BASE/'states').mkdir(parents=True,exist_ok=True);results=[]
    for stage in range(1,9):
        name=f'chr-stage-{stage}-{label}';state=f'stage-{stage}-v2'
        src=ROOT/'.local/contra-campaign/states'/(state+'.fcs');dst=BASE/'states'/(state+'.fcs')
        if dst.exists():assert dst.read_bytes()==src.read_bytes()
        else:shutil.copyfile(src,dst)
        spec={'name':name,'load_state':state,'frames':256,'captures':[1,64,128,256],
              'segments':[],'hooks_lua':HOOK}
        path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
        writes=[json.loads(s)for s in (observed/'chr-writes.jsonl').read_text().splitlines()]
        frames=[json.loads(s)for s in (observed/'chr-frames.jsonl').read_text().splitlines()]
        animations=[json.loads(s)for s in (observed/'chr-animation.jsonl').read_text().splitlines()]
        samples=[]
        for frame in spec['captures']:
            ppu=(observed/f'frame-{frame:05d}-ppu.bin').read_bytes();mapped=frames[frame-1]['applied'];aliases=[]
            for slot,page in enumerate(mapped):
                assert page<128 and ppu[slot*1024:(slot+1)*1024]==chr[page*1024:(page+1)*1024],(stage,frame,slot,page)
                aliases.append([i for i in range(128)if chr[i*1024:(i+1)*1024]==ppu[slot*1024:(slot+1)*1024]])
            samples.append({'frame':frame,'ppuSHA256':hashlib.sha256(ppu).hexdigest(),'matchingPageAliases':aliases})
        results.append({'stage':stage,'source':json.loads((observed/'source.json').read_text()),
                        'writes':writes,'frames':frames,'samples':samples,'animationCalls':animations})
    result={'passed':True,'scope':'No new interventions; 256 original frames from each declared controlled stage initializer.',
            'stages':results}
    target=BASE/f'chr-{label}-observations.json.gz';target.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'passed':True,'stages':len(results),'desiredWriterPCs':sorted({w['pc']for s in results for w in s['writes']if w['kind']=='desired'}),
                      'mapperWriterPCs':sorted({w['pc']for s in results for w in s['writes']if w['kind']=='mapper'})}))


if __name__=='__main__':main()
