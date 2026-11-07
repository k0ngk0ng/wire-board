"""Original tank/carrier continuations after one declared initial setup.

No later entity, graphics-budget, palette, timer or outcome patch is applied.
The initial stage entry and explicit defeated/carrier scenes are controlled
research starts, not a naturally won campaign.
"""
import gzip,hashlib,json,sys
from pathlib import Path
from enemies_snow_capture import ROOT,BASE,FIELDS,WORLD,HOOK
from capture_reference import run

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1';sequences=[]
    for mode in [0,1,2,3]:
        # 0: moving tank with joints; 1: destroyed tank; 2: defeated carrier
        # plus boxes; 3 starts at the explosion step with a nonzero offset so
        # the boxes' first-frame queue contention exercises position rollback.
        control=r'''
local controlSetup=nil;local setup=false;local mode=MODE
memory.registerexec(0xe54f,1,function()
 if setup then return end;setup=true
 for slot=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+slot,0)end end
 for i=0,1 do memory.writebyte(0xb0+i,255)end
 local slot=mode>=2 and 0 or 15
 local values={1,mode>=2 and 80 or 144,mode==0 and 240 or mode==1 and 200 or 128,0,mode==0 and 1 or mode==1 and 5 or mode==2 and 10 or 11,0,0,0,0,0,0,mode>=2 and 20 or 18,0,0,0,0,mode==0 and 0xf0 or 0,0x79,mode==3 and 0x85 or 4,0,mode==3 and 3 or 0,0,0,0}
 for i,a in ipairs(fields)do memory.writebyte(a+slot,values[i])end
 memory.writebyte(0x3b,0);memory.writebyte(0x2a,255);memory.writebyte(0x2b,0)
 memory.writebyte(0x77,0);memory.writebyte(0x78,0);memory.writebyte(0x7f,0)
 if mode<2 then
  for i=13,14 do memory.writebyte(0x528+i,19);memory.writebyte(0x4b8+i,1);memory.writebyte(0x30a+i,1);memory.writebyte(0x33e+i,180+(i-13)*32);memory.writebyte(0x324+i,120);memory.writebyte(0x598+i,0x81)end
 else
  for box=13,15 do
   memory.writebyte(0x528+box,2);memory.writebyte(0x4b8+box,3);memory.writebyte(0x30a+box,1);memory.writebyte(0x578+box,0xf0);memory.writebyte(0x598+box,0x0f);memory.writebyte(0x538+box,1);memory.writebyte(0x568+box,0);memory.writebyte(0x5c8+box,0)
   memory.writebyte(0x33e+box,80+(box-13)*32);memory.writebyte(0x324+box,160)
  end
 end
 writefile('snow-setup.json',string.format('{"mode":%d,"slot":%d,"frame":%d,"intervention":"one snow entity/joint/box setup and player B shield; all subsequent frames unmodified"}',mode,slot,emu.framecount()))
end)
'''.replace('MODE',str(mode))
        wf=WORLD+[('requestedColors',0x36),('work06',6)]
        hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in wf)+'}').replace('CONTROLLED',control)
        frames=1800 if mode==0 else 900 if mode==1 else 500
        name=f'snow-sequence-{label}-{mode}';spec={'name':name,'load_state':'stage-5','frames':frames,'segments':[],'captures':[1,60,180,frames],'hooks_lua':hook,'timeout_seconds':90}
        path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
        rows=[json.loads(line)for line in (out/'snow.jsonl').read_text().splitlines()];assert rows
        target=0 if mode>=2 else 15;kind=20 if mode>=2 else 18
        focused=[r for r in rows if r['slot']==target and r['before']['enemies'][target][11]==kind]
        assert focused and focused[-1]['after']['enemies'][target][4]==0,(mode,'target did not finish')
        if mode>=2:
            assert focused[-1]['after']['world'][5]==1 and focused[-1]['after']['world'][24:26]==[48,0]
        sequences.append({'mode':mode,'slot':target,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'setup':json.loads((out/'snow-setup.json').read_text()),'records':rows,'target_calls':len(focused)})
    data={'source_is_original_execution':True,'interventions':'One initial tank or defeated-carrier setup with joints/boxes and player B shield. No later patches. Source stage entry was controlled. Not a natural Boss victory.',
          'world_fields':[{'name':n,'address':hex(a)}for n,a in wf],'enemy_fields':FIELDS,'sequences':sequences}
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0);target=ROOT/'games/contra/engine/testdata/original-enemies-snow-sequences-v2.json.gz'
    if target.exists():assert [s['records']for s in json.loads(gzip.decompress(target.read_bytes()))['sequences']]==[s['records']for s in sequences],'repeat differs'
    else:target.write_bytes(packed)
    print(json.dumps({'sequences':[(s['mode'],len(s['records']),s['target_calls'])for s in sequences],'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
