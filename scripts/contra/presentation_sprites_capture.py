"""Observe complete original scene composition before independent Go code.

Natural timelines and explicitly controlled rendering inputs are kept separate.
Only presentation-owned fields are changed in the controlled cases. No expected
OAM, snow movement or generation-bias value is patched into the output.
"""
import argparse,gzip,hashlib,json,os,shutil
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-presentation-sprites/runtime'))
from capture_reference import BASE,run

HOOK=r'''
local file=assert(io.open(out..'/composition.jsonl','w'))
local pending=nil;local changed=false;local interventions={};local draining=false
local function r(a)return memory.readbyte(a)end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',r(i))end;return table.concat(t)end
local function patch(a,v)interventions[#interventions+1]=string.format('{"address":%d,"before":%d,"value":%d}',a,r(a),v);memory.writebyte(a,v)end
local function snapshot()
 local objects={};for i=0,25 do objects[#objects+1]=string.format('{"code":%d,"x":%d,"y":%d,"attributes":%d}',r(0x300+i),r(0x334+i),r(0x31a+i),r(0x34e+i))end
 local snow={};for i=0,15 do snow[#snow+1]=string.format('{"x":%d,"y":%d,"flags":%d}',r(0x1b0+i),r(0x1a0+i),r(0x7b0+i))end
 return string.format('{"input":{"objects":[%s],"flicker":%s,"hud":%s,"twoPlayers":%s,"demo":%s,"level":%d,"frameScroll":%d,"snowControl":%d,"lives":[%d,%d],"gameOver":[%d,%d]},"state":{"oam":"%s","rotation":%d,"snow":[%s]},"generationBias":%d,"context":{"gameRoutine":%d,"levelRoutine":%d,"graphicsQueueUsed":%d}}',table.concat(objects,','),tostring(AND(r(0x7eb),1)~=0),tostring(r(0x37)~=0),tostring(r(0x22)~=0),tostring(r(0x1c)~=0),r(0x30),r(0x68),r(0x5a),r(0x32),r(0x33),r(0x38),r(0x39),hex(0x200,256),r(0x35),table.concat(snow,','),r(6),r(0x18),r(0x2c),r(0x21))
end
memory.registerwrite(6,1,function(a,s,v)
 if pending then pending.writes[#pending.writes+1]=string.format('{"pcAfter":%d,"value":%d}',memory.getregister('pc'),v)end
end)
memory.registerexec(0x919f,1,function()
 if draining then return end
 if r(0x8000)~=0x14 then return end
 assert(not pending)
 if not changed then
  CONTROLLED
  changed=true
 end
 local sp=memory.getregister('s');local ret=r(0x100+AND(sp+1,255))+256*r(0x100+AND(sp+2,255))+1
 pending={frame=emu.framecount(),before=snapshot(),writes={},sp=AND(sp+2,255)}
 memory.registerexec(ret,1,function()
  if not pending or pending.sp~=memory.getregister('s')then return end
  local p=pending;pending=nil
  file:write(string.format('{"frame":%d,"endFrame":%d,"before":%s,"after":%s,"generationWrites":[%s]}\n',p.frame,emu.framecount(),p.before,snapshot(),table.concat(p.writes,',')))
 end)
end)
function finish_probe()
 draining=true;local extra=0
 while pending do extra=extra+1;assert(extra<=10);emu.frameadvance()end
 file:close();writefile('composition-drain.json',string.format('{"additionalVideoFrames":%d,"reason":"complete an already-entered original composition; no new calls recorded"}',extra))
 writefile('composition-interventions.json','['..table.concat(interventions,',')..']')
end
'''


def controlled_cases():
    clear='for i=0,25 do patch(0x300+i,0)end\n'
    common=clear+'patch(0x7eb,1);patch(0x37,0);patch(0x1c,0);patch(0x35,0);patch(6,171)\n'
    result=[
        ('empty-retains-generation',1,common),
        ('shared-same-generation',1,common+'patch(0x300,15);patch(0x334,80);patch(0x31a,100);patch(0x34e,0);patch(6,12)'),
        ('all26-small',1,common+'for i=0,25 do patch(0x300+i,30);patch(0x334+i,8+i*8);patch(0x31a+i,80);patch(0x34e+i,i%4)end'),
        ('capacity-overrun',1,common+'patch(0x37,1);patch(0x22,1);patch(0x32,4);patch(0x33,4);patch(0x38,0);patch(0x39,0);for i=0,25 do patch(0x300+i,222);patch(0x334+i,128);patch(0x31a+i,100);patch(0x34e+i,0)end'),
        ('linear-empty',1,common+'patch(0x7eb,0)'),
        ('linear-all26',1,common+'patch(0x7eb,0);patch(0x35,252);patch(0x37,1);for i=0,25 do patch(0x300+i,30);patch(0x334+i,8+i*8);patch(0x31a+i,80);patch(0x34e+i,i%4)end'),
        ('linear-wrap',1,common+'patch(0x7eb,0);for i=0,25 do patch(0x300+i,222);patch(0x334+i,128);patch(0x31a+i,100);patch(0x34e+i,0)end'),
        ('flip-clipping',1,common+'for i=0,25 do patch(0x300+i,15);patch(0x334+i,(i%2)*255);patch(0x31a+i,i*9);patch(0x34e+i,(i%4)*64+13)end'),
    ]
    for name,mode,demo,lives,over in [
        ('hud-solo-zero',0,0,[0,99],[0,0]),('hud-solo-four',0,0,[4,0],[0,0]),
        ('hud-coop-many',1,0,[5,99],[0,0]),('hud-game-over',1,0,[2,3],[1,0]),
        ('hud-both-over',1,0,[2,3],[1,1]),('hud-demo',0,1,[0,0],[0,0]),
        ('hud-even-status',1,0,[1,2],[2,2]),
    ]:
        code=common+f'patch(0x37,1);patch(0x22,{mode});patch(0x1c,{demo});'
        code+=''.join(f'patch({0x32+i},{v});'for i,v in enumerate(lives))
        code+=''.join(f'patch({0x38+i},{v});'for i,v in enumerate(over))
        result.append((name,1,code))
    for name,visibility,scroll in [('snow-visible',1,0),('snow-hidden',0,0),('snow-fast-scroll',1,250)]:
        code=common+f'patch(0x5a,{visibility});patch(0x68,{scroll});'
        code+='for i=0,15 do patch(0x1b0+i,i%3);patch(0x1a0+i,236+i%4);patch(0x7b0+i,(i%2)*2+1)end'
        result.append((name,5,code))
    result.append(('snow-capacity',5,common+'patch(0x5a,1);for i=17,25 do patch(0x300+i,15);patch(0x334+i,128);patch(0x31a+i,100);patch(0x34e+i,0)end'))
    result.append(('linear-snow-ignored',5,common+'patch(0x7eb,0);patch(0x35,252);patch(0x5a,1);patch(0x37,1)'))
    return result


def main():
    parser=argparse.ArgumentParser();parser.add_argument('--mode',choices=['natural','controlled'],default='natural');parser.add_argument('--label',default='v1');args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True);specs=[]
    if args.mode=='natural':
        for two in [False,True]:
            name=f'composition-boot-{int(two)}-{args.label}'
            segments=[{'start':f,'end':f+2,'keys':['start']}for f in [120,720,1080]]
            if two:segments.insert(0,{'start':100,'end':102,'keys':['select']})
            specs.append({'name':name,'frames':1420,'segments':segments,'captures':[119,840,1400],
                          'hooks_lua':HOOK.replace('CONTROLLED','')})
        for stage in range(1,9):
            state=f'stage-{stage}-v2';src=ROOT/'.local/contra-campaign/states'/(state+'.fcs');dst=BASE/'states'/(state+'.fcs')
            if dst.exists():assert src.read_bytes()==dst.read_bytes()
            else:shutil.copyfile(src,dst)
            specs.append({'name':f'composition-stage-{stage}-{args.label}','load_state':state,'frames':256,
                          'segments':[{'start':1,'end':256,'player':p,'keys':['right','B']}for p in [1,2]]+
                                     [{'start':f,'end':f+1,'player':p,'keys':['A']}for p in [1,2]for f in [20,110,200]],
                          'captures':[1,64,128,256],'hooks_lua':HOOK.replace('CONTROLLED','')})
    else:
        for name,stage,patches in controlled_cases():
            state=f'stage-{stage}-v2';src=ROOT/'.local/contra-campaign/states'/(state+'.fcs');dst=BASE/'states'/(state+'.fcs')
            if not dst.exists():shutil.copyfile(src,dst)
            specs.append({'name':f'composition-case-{name}-{args.label}','load_state':state,'frames':2,
                          'segments':[],'captures':[1,2],'hooks_lua':HOOK.replace('CONTROLLED',patches)})
    traces=[]
    for spec in specs:
        p=BASE/(spec['name']+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');observed=run(p)
        records=[json.loads(s)for s in (observed/'composition.jsonl').read_text().splitlines()]
        assert records and all(r['before']['input']['level']<8 for r in records)
        traces.append({'name':spec['name'],'source':json.loads((observed/'source.json').read_text()),
                       'scenario':spec,'interventions':json.loads((observed/'composition-interventions.json').read_text()),
                       'drain':json.loads((observed/'composition-drain.json').read_text()),
                       'records':records,'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()})
    result={'mode':args.mode,'scope':'Complete original scene composition; controlled rendering inputs are explicitly listed.',
            'traces':traces,'records':sum(len(t['records'])for t in traces)}
    target=BASE/f'composition-{args.mode}-{args.label}.json.gz';target.write_bytes(gzip.compress(json.dumps(result,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'records':result['records'],'traces':len(traces),'file':str(target)}))


if __name__=='__main__':main()
