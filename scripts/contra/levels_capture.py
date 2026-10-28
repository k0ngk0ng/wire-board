"""Execute real original map loaders and resource transitions at declared inputs.

The initial states are the parent's controlled stage initializers. Loader cases
set streaming cursors once, then replace only the first original E09C input A;
boss-map cases additionally select the original six-byte Boss pointer tuple.
Transition cases set the exact scroll boundary once and let original code select
its pointers, CHR and palette/collision values. No campaign-clear claim is made.
"""
import argparse
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil

ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-levels/runtime'))
from capture_reference import BASE,run
from levels_data import build

HOOK=r'''
local mode=MODE;local index=INDEX;local stage=STAGE
local injected=false;local entered=false;local completed=false;local pending=nil
local changes={};local events={};local watchpending={}
local function read(a)return memory.readbyte(a)end
local function word(a)return read(a)+256*read(a+1)end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function snapshot()
 return string.format('{"frame":%d,"sceneType":%d,"screen":%d,"phase":%d,"header":"%s","chr":"%s","palette":"%s"}',emu.framecount(),read(0x40),read(0x64),read(0x65),hex(0x40,32),hex(0x7f0,8),hex(0x7c0,32))
end
local function patch(a,v)
 changes[#changes+1]=string.format('{"address":%d,"before":%d,"after":%d}',a,read(a),v);memory.writebyte(a,v)
end
local function watch(name,address)
 memory.registerexec(address,1,function()
  assert(not watchpending[name]);local s=memory.getregister('s')
  local ret=read(0x100+AND(s+1,255))+256*read(0x100+AND(s+2,255))+1
  watchpending[name]={before=snapshot(),inputA=memory.getregister('a'),sp=AND(s+2,255)}
  memory.registerexec(ret,1,function()
   local p=watchpending[name]
   if p and p.sp==memory.getregister('s')then
    watchpending[name]=nil
    events[#events+1]=string.format('{"routine":"%s","inputA":%d,"before":%s,"after":%s}',name,p.inputA,p.before,snapshot())
   end
  end)
 end)
end
watch('chr',0xc778);watch('alternate',0xcec6)
memory.registerexec(0xfa3a,1,function()
 if injected then return end
 assert(read(0x30)==stage and read(0x18)==5 and read(0x2c)==4)
 injected=true
 patch(0x75,0);patch(0x76,0);patch(0x77,0)
 if mode=='transition' then
  if read(0x40)==1 then
   patch(0x6a,0x80);patch(0x61,0);patch(0x73,1);patch(0x64,read(0x48));patch(0x65,3)
  else
   patch(0x64,AND(read(0x48)-1,255));patch(0x65,read(0x41)==0 and 255 or 239);patch(0x68,1)
  end
 elseif read(0x40)==1 then
  patch(0x6a,0x80);patch(0x61,0);patch(0x73,1);patch(0x64,0);patch(0x65,0)
 elseif read(0x41)~=0 then
  patch(0x60,0);patch(0x65,7);patch(0x68,1)
 else
  patch(0x60,31);patch(0x65,7);patch(0x68,1)
 end
 writefile('intervention-before.json',snapshot())
end)
memory.registerexec(0xe09c,1,function()
 if entered then return end
 assert(injected);entered=true
 local prior=memory.getregister('a')
 if mode~='transition'then
  if mode=='boss-loader'then for i=0,5 do patch(0x42+i,read(0xdd45+i))end end
  memory.setregister('a',index)
 end
 local s=memory.getregister('s');local ret=read(0x100+AND(s+1,255))+256*read(0x100+AND(s+2,255))+1
 pending={offset=read(0x69),input=memory.getregister('a'),beforeA=prior,header=hex(0x40,32),before=hex(0x600,128),sp=AND(s+2,255),returnPC=ret}
 memory.registerexec(ret,1,function()
  if not pending or pending.sp~=memory.getregister('s')then return end
  local p=pending;pending=nil;completed=true
  writefile('loader.json',string.format('{"frame":%d,"beforeA":%d,"input":%d,"offset":%d,"pointer":%d,"mapBank":%d,"header":"%s","before":"%s","after":"%s","returnPC":%d}',emu.framecount(),p.beforeA,p.input,p.offset,p.pointer,p.bank,p.header,p.before,hex(0x600,128),p.returnPC))
 end)
end)
memory.registerexec(0xe0ae,1,function()
 if pending then pending.pointer=word(0);pending.bank=read(0x8000)end
end)
function finish_probe()
 assert(injected and not pending and next(watchpending)==nil)
 if mode~='transition'then assert(entered and completed)end
 writefile('interventions.json','['..table.concat(changes,',')..']')
 writefile('events.json','['..table.concat(events,',')..']')
 writefile('final-state.json',snapshot())
end
'''


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--mode',choices=['loader','transitions'],default='loader')
    parser.add_argument('--label',default='v1');parser.add_argument('--stage',type=int)
    parser.add_argument('--index',type=int);args=parser.parse_args()
    data=build();(BASE/'states').mkdir(parents=True,exist_ok=True)
    cases=[]
    for stage in data['stages']:
        n=stage['stage']
        if args.stage and n!=args.stage:continue
        source=ROOT/'.local/contra-campaign/states'/f'stage-{n}-v2.fcs'
        target=BASE/'states'/f'stage-{n}-v2.fcs'
        if target.exists():assert target.read_bytes()==source.read_bytes()
        else:shutil.copyfile(source,target)
        if args.mode=='transitions':cases.append((stage,'transition',0,None))
        else:
            for entry in stage['maps']['entries']:
                if args.index is None or args.index==entry['index']:cases.append((stage,'loader',entry['index'],entry))
            if 'baseBoss'in stage and args.index is None:
                boss=stage['baseBoss']['mapIndex'];cases.append((stage,'boss-loader',boss,data['extraMapSets']['base-boss']['entries'][boss]))
    observations=[]
    for stage,mode,index,expected in cases:
        n=stage['stage'];name=f'{mode}-stage-{n}-map-{index}-{args.label}'
        hook=HOOK.replace('MODE',json.dumps(mode)).replace('INDEX',str(index)).replace('STAGE',str(n-1))
        frames=50 if mode=='transition'else 2
        spec={'name':name,'load_state':f'stage-{n}-v2','frames':frames,
              'captures':[1,2,34,50]if mode=='transition'else [],'hooks_lua':hook,'segments':[]}
        path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
        record={'stage':n,'mode':mode,'requestedIndex':index,'run':name,
                'source':json.loads((observed/'source.json').read_text()),
                'interventions':json.loads((observed/'interventions.json').read_text()),
                'events':json.loads((observed/'events.json').read_text()),
                'final':json.loads((observed/'final-state.json').read_text())}
        if (observed/'loader.json').exists():record['loader']=json.loads((observed/'loader.json').read_text())
        if mode!='transition':
            loader=record['loader'];actual=bytes.fromhex(loader['after']);offset=loader['offset']
            assert loader['mapBank']==0x14 and loader['pointer']==expected['pointer']
            assert actual[offset:offset+len(expected['supertiles'])]==bytes(expected['supertiles']),(n,index,loader)
            assert loader['input']==index
        observations.append(record)
    result={'romSHA256':data['romSHA256'],'scope':'Controlled original streaming/loader inputs; not natural map traversal or campaign completion.',
            'passed':True,'mode':args.mode,'observations':observations}
    path=BASE/f'{args.mode}-{args.label}-observations.json.gz'
    path.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'passed':True,'observations':len(observations),'path':str(path)}))


if __name__=='__main__':main()
