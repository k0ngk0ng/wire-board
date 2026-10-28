"""Record original palette load/cycle/commit and snow program phases.

Natural controller timelines and controlled function inputs are distinct. The
controlled probes change only declared inputs; expected palette/queue outputs
always come from original execution, including real PPU palette mirrors.
"""
import argparse,gzip,hashlib,json,os,shutil
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-palette/runtime'))
from capture_reference import BASE,run

HOOK=r'''
local file=assert(io.open(out..'/palette.jsonl','w'))
local pending={};local patched=false;local setup=false;local edits={};local target=TARGET
local function r(a)return memory.readbyte(a)end
local function hex(a,n,read)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function patch(a,v)edits[#edits+1]=string.format('{"address":%d,"before":%d,"value":%d}',a,r(a),v);memory.writebyte(a,v)end
local function snapshot()
 return string.format('{"state":{"desired":"%s","applied":"%s","requestedColors":%d,"fadeTimer":%d,"cycle":%d,"pauseCycle":%d,"flashTimer":%d},"header":"%s","snow":{"visibility":%d,"cursor":%d,"pace":%d},"context":{"gameRoutine":%d,"counter":%d,"level":%d,"sceneType":%d,"alternate":%d,"screen":%d,"scroll":%d},"queueUsed":%d,"queue":"%s","generationBias":%d}',hex(0x7c0,32,r),hex(0x3f00,32,ppu.readbyte),r(0x36),r(0x74),r(0x72),r(0x78),r(0x7d),hex(0x49,15,r),r(0x5a),r(0x5b),r(0x5d),r(0x18),r(0x1a),r(0x30),r(0x40),r(0x48),r(0x64),r(0x65),r(0x21),hex(0x700,r(0x21),r),r(6))
end
memory.registerwrite(6,1,function(a,s,v)
 for _,p in pairs(pending)do p.writes[#p.writes+1]=string.format('{"pcAfter":%d,"value":%d}',memory.getregister('pc'),v)end
end)
local function watch(name,address)
 memory.registerexec(address,1,function()
  if name=='cycle' and target=='load' and not setup then
   patch(0x1a,5);patch(0x36,0);patch(0x78,0);setup=true
  end
  if name==target and not patched then
   PATCHES
   patched=true
  end
  assert(not pending[name]);local s=memory.getregister('s')
  local ret=r(0x100+AND(s+1,255))+256*r(0x100+AND(s+2,255))+1
  pending[name]={frame=emu.framecount(),before=snapshot(),inputA=memory.getregister('a'),sp=AND(s+2,255),writes={}}
  memory.registerexec(ret,1,function()
   local p=pending[name]
   if not p or p.sp~=memory.getregister('s')then return end
   pending[name]=nil
   file:write(string.format('{"routine":"%s","frame":%d,"inputA":%d,"before":%s,"after":%s,"generationWrites":[%s]}\n',name,p.frame,p.inputA,p.before,snapshot(),table.concat(p.writes,',')))
  end)
 end)
end
watch('load',0xcee3);watch('cycle',0xcf6d);watch('commit',0xcc02);watch('snow',0xce59)
function finish_probe()
 assert(next(pending)==nil);if target~=''then assert(patched)end
 file:close();writefile('palette-interventions.json','['..table.concat(edits,',')..']')
end
'''


def cases():
    out=[]
    for count in [16,32]:
        for scene in [0,128]:
            for fade in [0,1,2,3,4,5,6,7,8,9,127,128,255]:
                out.append((f'load-{count}-{scene}-{fade}',1,'load',f'patch(0x40,{scene});patch(0x74,{fade});memory.setregister("a",{count})'))
    out.append(('load16-unused-sprite-indices',1,'load','patch(0x36,32);for i=4,7 do patch(0x50+i,255)end;memory.setregister("a",16)'))
    for count in [0,16,32]:
        for used in [0,70,71,86]:
            code=f'patch(0x36,{count});patch(0x21,{used});for i=0,31 do patch(0x7c0+i,(i*3+7)%64)end'
            out.append((f'commit-{count}-{used}',1,'commit',code))
    for stage in range(1,9):
        for phase in range(4):
            out.append((f'cycle-stage-{stage}-phase-{phase}',stage,'cycle',f'patch(0x36,0);patch(0x1a,5);patch(0x72,{phase});patch(0x78,0);patch(0x7d,0)'))
    for stage in [2,4]:
        for used in [0,64,70,71,86,183,184,246]:
            out.append((f'cycle-boss-{stage}-queue-{used}',stage,'cycle',f'patch(0x40,128);patch(0x36,0);patch(0x1a,5);patch(0x72,0);patch(0x78,0);patch(0x7d,0);patch(0x21,{used})'))
    for flash in range(1,11):
        out.append((f'flash-{flash}',1,'cycle',f'patch(0x36,0);patch(0x1a,6);patch(0x7d,{flash})'))
    for name,code in [
        ('requested-gate','patch(0x36,16);patch(0x1a,5);patch(0x7d,8)'),
        ('pause-cycle','patch(0x36,0);patch(0x1a,5);patch(0x78,1);patch(0x7d,8)'),
        ('cycle-then-flash','patch(0x36,0);patch(0x1a,5);patch(0x78,0);patch(0x7d,9)'),
        ('ending-colors','patch(0x36,0);patch(0x30,8);patch(0x1a,5);patch(0x72,0);patch(0x78,0);patch(0x7d,0)'),
    ]:out.append((name,1,'cycle',code))
    for cursor,screen,scroll in [(0,0,0),(0,20,0),(3,1,255),(3,2,0),(6,3,127),(6,3,128),(9,16,255),(9,17,0),(12,20,0)]:
        out.append((f'snow-{cursor}-{screen}-{scroll}',5,'snow',f'patch(0x5b,{cursor});patch(0x5a,5);patch(0x5d,77);patch(0x64,{screen});patch(0x65,{scroll})'))
    out.append(('snow-other-stage',1,'snow','patch(0x5b,0);patch(0x5a,5);patch(0x5d,77);patch(0x64,20)'))
    return out


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--mode',choices=['natural','controlled'],default='natural');parser.add_argument('--label',default='v1');args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True);specs=[]
    for stage in range(1,9):
        name=f'stage-{stage}-v2';src=ROOT/'.local/contra-campaign/states'/(name+'.fcs');dst=BASE/'states'/(name+'.fcs')
        if dst.exists():assert src.read_bytes()==dst.read_bytes()
        else:shutil.copyfile(src,dst)
    if args.mode=='natural':
        for stage in range(1,9):
            spec={'name':f'palette-stage-{stage}-{args.label}','load_state':f'stage-{stage}-v2','frames':256,
                  'captures':[1,128,256],'segments':[{'start':1,'end':256,'player':p,'keys':['right','B']}for p in [1,2]],
                  'hooks_lua':HOOK.replace('TARGET','""').replace('PATCHES','')}
            specs.append((spec,''))
    else:
        for name,stage,target,patches in cases():
            spec={'name':f'palette-{name}-{args.label}','load_state':f'stage-{stage}-v2','frames':1,
                  'captures':[],'segments':[],'hooks_lua':HOOK.replace('TARGET',json.dumps(target)).replace('PATCHES',patches)}
            specs.append((spec,target))
    observations=[]
    for spec,target in specs:
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
        records=[json.loads(line)for line in (observed/'palette.jsonl').read_text().splitlines()]
        selected=[r for r in records if r['routine']==target]if target else records
        assert selected
        observations.append({'name':spec['name'],'target':target,'source':json.loads((observed/'source.json').read_text()),
                             'scenario':spec,'interventions':json.loads((observed/'palette-interventions.json').read_text()),
                             'records':selected,'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()})
    data={'mode':args.mode,'scope':'Original palette and snow functions; controlled input edits explicitly listed.',
          'traces':observations,'records':sum(len(o['records'])for o in observations)}
    target=BASE/f'palette-{args.mode}-{args.label}.json.gz';target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'traces':len(observations),'records':data['records'],'file':str(target)}))


if __name__=='__main__':main()
