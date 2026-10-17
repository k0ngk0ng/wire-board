"""Capture original player action phases related to hoop pickup/carry/throw."""
import hashlib
import json
import argparse
from capture_reference import ROOT,BASE,run

HOOK=r'''
local f=assert(io.open(out.."/hoop-player.jsonl","w"));local pending=nil;local actor=nil
local function word(a,b) return memory.readbyte(a)+256*memory.readbyte(b) end
local function wide(a,b,c) return word(a,b)+65536*memory.readbyte(c) end
local function selected(a) a=AND(a,127);return (a>=0x30 and a<=0x3c) or a==0x57 end
local function state(i)
 local fields={action=0x512,request=0x51f,pose=0x5b,facing=0x46b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,heading=0x4ec,shotCycle=0x572,aimWindow=0x54c,control=0x539,runGrade=0x615,jumpGrade=0x619,character=0x3e3,boost=0x576,passage=0x6d7}
 local parts={};for name,a in pairs(fields) do parts[#parts+1]=string.format('"%s":%d',name,memory.readbyte(a+i)) end
 parts[#parts+1]=string.format('"ballAction":%d,"ballRequest":%d',memory.readbyte(0x518),memory.readbyte(0x525))
 parts[#parts+1]=string.format('"x":%d,"depth":%d,"height":%d,"vx":%d,"vy":%d,"vz":%d,"gravity":%d,"holder":%d',wide(0x68+i,0x75+i,0x82+i),word(0x8f+i,0x9c+i),wide(0xa9+i,0xb6+i,0xc3+i),word(0x478+i,0x485+i),word(0x492+i,0x49f+i),word(0x4ac+i,0x4b9+i),word(0x4c6+i,0x4d3+i),memory.readbyte(0x53f))
 local owners,actions,facings={},{},{}
 for k=0,5 do owners[#owners+1]=tostring(memory.readbyte(0x540+k));actions[#actions+1]=tostring(memory.readbyte(0x519+k));facings[#facings+1]=tostring(memory.readbyte(0x472+k)) end
 parts[#parts+1]=string.format('"owners":[%s],"hoopActions":[%s],"hoopFacings":[%s]',table.concat(owners,','),table.concat(actions,','),table.concat(facings,','))
 return '{'..table.concat(parts,',')..'}'
end
memory.registerexec(0xe165,1,function()
 local i=memory.getregister("x")
 if i>3 or not (selected(memory.readbyte(0x512+i)) or selected(memory.readbyte(0x51f+i))) then return end
 assert(not pending);actor=i;pending=string.format('{"frame":%d,"actor":%d,"before":%s,',emu.framecount(),i,state(i))
end)
memory.registerexec(0xe168,1,function()
 if not pending then return end
 f:write(pending..'"after":'..state(actor)..'}\n');pending=nil
end)
function finish_probe()
 if pending then writefile('incomplete-hoop-player.json',pending..'"incomplete":true}') end
 f:close()
end
'''

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--ball',action='store_true');args=parser.parse_args()
    kind='ball-player' if args.ball else 'hoop-player'
    hook=HOOK
    if args.ball:
        hook=hook.replace('return (a>=0x30 and a<=0x3c) or a==0x57','return a==0x12 or a==0x13 or a==0x15 or a==0x1d or a==0x1e or a==0x1f or a==0x57').replace('hoop-player',kind)
        hook=hook.replace('selected(memory.readbyte(0x51f+i)))','selected(memory.readbyte(0x51f+i)) or memory.readbyte(0x53f)==i)')
    references=['hoop-control-cpu-long']
    if args.ball:
        references.extend(['human-possession-baseline','ball-control-pass'])
        cases={
            'charge':[dict(start=1,end=6,keys=['left']),dict(start=13,end=165,keys=['left'])],
            'charge-evade':[dict(start=1,end=6,keys=['left']),dict(start=13,end=165,keys=['left']),dict(start=70,end=110,keys=['up'])],
            'brake':[dict(start=1,end=6,keys=['left']),dict(start=13,end=60,keys=['left']),dict(start=61,end=90,keys=['right'])],
            'running-jump':[dict(start=1,end=6,keys=['right']),dict(start=13,end=70,keys=['right']),dict(start=25,end=27,keys=['A','B'])],
        }
        for name,segments in cases.items():
            spec=dict(name='ball-carry-'+name,load_slot=7,frames=180,segments=segments,captures=[180])
            path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
            references.append(spec['name'])
    records=[];sources=[]
    for name in references:
        reference=BASE/'runs'/name
        spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name=kind+'-'+name,hooks_lua=hook)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name']
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/(kind+'.jsonl')).read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    fixture=ROOT/f'games/basketball/engine/testdata/{kind}.json';fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
    manifest=dict(source_is_original_execution=True,entry='fixed:e165',exit='fixed:e168',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources)
    fixture.with_name(kind+'-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps(dict(records=len(records))))

if __name__=='__main__':main()
