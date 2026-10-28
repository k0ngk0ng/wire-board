#!/usr/bin/env python3
"""Observe original $34 accesses and idle ADC timing without changing World.

Failure inventory before any semantic implementation: ignored incoming Carry;
byte wrap confused with saturation; frame counter changed after LDA but before
ADC; NMI/lag counted as a game tick; slot readers assumed immutable; startup
clears omitted; CPU-time budget invented; or captured after values used as code.
Natural runs contain observation hooks only. Controlled ADC cases explicitly
replace A/$34/Carry at the real F882 instruction and are labeled separately.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT = Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT', str(ROOT / '.local/contra-random/oracle'))
from capture_reference import BASE, run

HOOK = r'''
local events=assert(io.open(out..'/random-events.jsonl','w'))
local frames=assert(io.open(out..'/random-frames.jsonl','w'))
local samples=assert(io.open(out..'/random-adds.jsonl','w'))
local controlled=CONTROLLED
local frameIndex=0;local idle=0;local reads=0;local writes=0;local calls=0;local nmis=0
local total=0;local mismatch=0;local stale=0;local pending=nil;local inhook=false
local readPCs={};local writePCs={};local case=0
local frameValues={0,1,2,127,128,254,255}
local function rb(a)return memory.readbyte(a)end
local function emit(kind,value)
 local pc=memory.getregister('pc')
 events:write(string.format('{"kind":"%s","frame":%d,"cycles":%d,"pc":%d,"value":%d,"counter":%d,"random":%d,"carry":%d,"a":%d,"busy":%d,"bank":%d,"stage":%d}\n',kind,emu.framecount(),debugger.getcyclescount(),pc,value or -1,rb(0x1a),rb(0x34),AND(memory.getregister('p'),1),memory.getregister('a'),rb(0x1b),rb(0x8000),rb(0x30)))
end
local function finish_idle(value)
 if not pending then return end
 local p=pending;pending=nil;local carry=AND(memory.getregister('p'),1);local sum=p.a+p.value+p.carry
 total=total+1;idle=idle+1
 if p.a~=p.counter then stale=stale+1 end
 if value~=sum%256 or carry~=math.floor(sum/256)then mismatch=mismatch+1 end
 if controlled and total<=3584 or not controlled and (idle<=2 or p.a~=p.counter or value~=sum%256 or carry~=math.floor(sum/256))then
  samples:write(string.format('{"case":%d,"frame":%d,"cyclesBefore":%d,"cyclesAfter":%d,"loadedFrame":%d,"counterAtADC":%d,"before":%d,"carryIn":%d,"after":%d,"carryOut":%d}\n',p.case,emu.framecount(),p.cycles,debugger.getcyclescount(),p.a,p.counter,p.value,p.carry,value,carry))
 end
end
memory.registerread(0x34,1,function(a,s,v)
 if inhook then return end;inhook=true
 local pc=memory.getregister('pc');readPCs[pc]=(readPCs[pc]or 0)+1;reads=reads+1
 if pc~=0xf884 then emit('read',v)end
 inhook=false
end)
memory.registerwrite(0x34,1,function(a,s,v)
 if inhook then return end;inhook=true
 local pc=memory.getregister('pc');writePCs[pc]=(writePCs[pc]or 0)+1;writes=writes+1
 if pc==0xf886 then finish_idle(v) else emit('write',v)end
 inhook=false
end)
memory.registerexec(0xf882,1,function()
 if controlled and case<3584 then
  local n=case;local carry=n%2;n=math.floor(n/2);local value=n%256;n=math.floor(n/256)
  memory.setregister('a',frameValues[n+1]);memory.writebyte(0x34,value)
  memory.setregister('p',AND(memory.getregister('p'),254)+carry);case=case+1
 end
 pending={a=memory.getregister('a'),value=rb(0x34),carry=AND(memory.getregister('p'),1),counter=rb(0x1a),case=case,cycles=debugger.getcyclescount()}
end)
for pc,kind in pairs({[0xf889]='nmi',[0xf8e1]='rti',[0xc001]='tick-start',[0xf8cc]='tick-end'})do
 memory.registerexec(pc,1,function()
  if kind=='nmi'then nmis=nmis+1 elseif kind=='tick-start'then calls=calls+1 end
  emit(kind,-1)
 end)
end
emu.registerafter(function()
 frameIndex=frameIndex+1
 frames:write(string.format('{"frame":%d,"emulatorFrame":%d,"cycles":%d,"lagged":%s,"counter":%d,"random":%d,"pc":%d,"busy":%d,"idleAdds":%d,"reads":%d,"writes":%d,"ticks":%d,"nmis":%d}\n',frameIndex,emu.framecount(),debugger.getcyclescount(),tostring(emu.lagged()),rb(0x1a),rb(0x34),memory.getregister('pc'),rb(0x1b),idle,reads,writes,calls,nmis))
 idle=0;reads=0;writes=0;calls=0;nmis=0
end)
function finish_probe()
 local function counts(t)local parts={};for pc,n in pairs(t)do parts[#parts+1]=string.format('"%04x":%d',pc,n)end;return '{'..table.concat(parts,',')..'}'end
 local f=assert(io.open(out..'/random-summary.json','w'))
 f:write(string.format('{"idleAdds":%d,"mismatches":%d,"loadedCounterDiffers":%d,"controlledCases":%d,"readPCs":%s,"writePCs":%s}',total,mismatch,stale,case,counts(readPCs),counts(writePCs)));f:close()
 events:close();frames:close();samples:close()
end
'''


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--label',default='v1')
    parser.add_argument('--mode',choices=['pilot','natural','controlled','boot'],default='pilot')
    args=parser.parse_args()
    scenarios=[]
    if args.mode=='controlled':
        scenarios=[('adc','solo-play',12,[],True)]
    elif args.mode=='boot':
        scenarios=[('boot-play',None,1500,[{'start':120,'end':122,'keys':['start']},{'start':720,'end':722,'keys':['start']},{'start':1241,'end':1243,'keys':['start']}],False)]
    else:
        scenarios=[('solo','solo-play',120 if args.mode=='pilot' else 600,[{'start':1,'end':600 if args.mode!='pilot' else 120,'keys':['right','B']}],False)]
        if args.mode=='natural':
            scenarios += [('boot',None,360,[{'start':260,'end':261,'keys':['start']}],False),('coop','coop-play',600,[{'start':1,'end':600,'player':p,'keys':['right','B']}for p in [1,2]],False)]
            for stage in range(1,9):scenarios.append((f'stage-{stage}',f'stage-{stage}-v2',256,[],False))
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    results=[]
    for name,state,frames,segments,controlled in scenarios:
        if state:
            source=ROOT/('.local/contra-campaign/states' if state.startswith('stage-') else '.local/contra/states')/(state+'.fcs')
            destination=BASE/'states'/(state+'.fcs')
            if destination.exists():assert destination.read_bytes()==source.read_bytes()
            else:shutil.copyfile(source,destination)
        spec={'name':f'random-{args.label}-{name}','frames':frames,'segments':segments,'captures':[],
              'hooks_lua':HOOK.replace('CONTROLLED','true'if controlled else'false'),'timeout_seconds':120}
        if state:spec['load_state']=state
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
        observed=run(path)
        baseline_sha=None
        if args.mode in ('natural','boot'):
            baseline=dict(spec,name=spec['name']+'-unhooked',hooks_lua='')
            baseline_path=BASE/(baseline['name']+'.json');baseline_path.write_text(json.dumps(baseline,indent=2)+'\n')
            plain=run(baseline_path)
            assert (plain/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes(),'observation changed original execution'
            baseline_sha=hashlib.sha256((plain/'ram.bin').read_bytes()).hexdigest()
        def lines(name):return [json.loads(s)for s in (observed/name).read_text().splitlines()]
        summary=json.loads((observed/'random-summary.json').read_text())
        assert summary['mismatches']==0,summary
        if controlled:assert summary['controlledCases']==3584
        results.append({'name':name,'controlled':controlled,'source':json.loads((observed/'source.json').read_text()),'scenario':spec,'summary':summary,
                        'events':lines('random-events.jsonl'),'frames':lines('random-frames.jsonl'),'adds':lines('random-adds.jsonl'),
                        'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest(),'unhookedRamSHA256':baseline_sha})
        print(json.dumps({'scenario':name,'summary':summary}))
    target=ROOT/f'docs/research/nes-contra/evidence/random-{args.mode}-{args.label}.json.gz'
    assert not target.exists()
    target.write_bytes(gzip.compress(json.dumps({'scope':'Original Japanese Contra $34 observation; natural runs are read-only. Controlled runs inject ADC operands/Carry and do not establish game pacing.','scenarios':results},separators=(',',':')).encode(),mtime=0))
    print(target)


if __name__=='__main__':main()
