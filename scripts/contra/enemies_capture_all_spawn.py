"""Original A97B/AA11 oracle for every Japanese enemy screen stream.

Declare selected level, trigger/cursor, occupied pools and stale base counters
at a real spawn call; restore the surrounding running game after recording.
This is controlled original execution, not a natural campaign completion.
"""
from pathlib import Path
import gzip,hashlib,json,os,sys
from enemies_all_data import ROOT,inventory
BASE=ROOT/'.local/contra-enemies-all'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_capture_initialization import FIELDS

HOOK=r'''
local f=assert(io.open(out..'/spawns.jsonl','w'));local fields=FIELDS;local cases=CASES
local index=0;local pending=nil;local hooks={}
local wf={0x30,0x40,0x41,0x64,0x65,0x82,0x6a,0x86,0x87,0x88,0x89,0x8a}
local function read(a)return memory.readbyte(a)end
local function arr(list,off)local t={};for _,a in ipairs(list)do t[#t+1]=tostring(read(a+off))end;return '['..table.concat(t,',')..']'end
local function state()local t={};for slot=0,15 do t[#t+1]=arr(fields,slot)end;return '{"enemies":['..table.concat(t,',')..'],"world":'..arr(wf,0)..'}'end
memory.registerexec(0xe54f,1,function()
 for p=0,1 do memory.writebyte(0xae+p,255);memory.writebyte(0x38+p,0)end
end)
memory.registerexec(0xa97b,1,function()
 if read(0x8000)~=0x14 then return end
 assert(not pending);index=index+1;local c=cases[index];assert(c,'unexpected extra spawn')
 local restore={};for _,a in ipairs(wf)do restore[a]=read(a)end
 for a=0,15 do restore[a]=read(a)end
 for slot=0,15 do for _,a in ipairs(fields)do restore[a+slot]=read(a+slot)end end
 local regs={};for _,reg in ipairs({'a','x','y','p'})do regs[reg]=memory.getregister(reg)end
 for slot=0,15 do
  for n,a in ipairs(fields)do memory.writebyte(a+slot,AND(slot*17+n*13+index*7,255))end
  memory.writebyte(0x4b8+slot,c.pool==0 and 0 or 2);memory.writebyte(0x528+slot,5)
 end
 if c.pool==1 then memory.writebyte(0x4b8+7,0);memory.writebyte(0x4b8+3,0)end
 if c.pool==2 then memory.writebyte(0x528+9,1);memory.writebyte(0x528+2,1)end
 memory.writebyte(0x30,c.level);memory.writebyte(0x40,c.scene);memory.writebyte(0x41,c.vertical)
 memory.writebyte(0x64,c.screen);memory.writebyte(0x65,c.scroll);memory.writebyte(0x82,c.cursor);memory.writebyte(0x6a,c.cleared)
 for a=0x86,0x8a do memory.writebyte(a,AND(a+index,255))end
 local sp=memory.getregister('s');local ret=read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1
 pending={restore=restore,regs=regs,before=state(),case=index,pc=ret}
 if not hooks[ret]then hooks[ret]=true;memory.registerexec(ret,1,function()
  if not pending or pending.pc~=ret then return end
  local p=pending;pending=nil
  f:write(string.format('{"case":%d,"before":%s,"after":%s}\n',p.case,p.before,state()))
  for a,v in pairs(p.restore)do memory.writebyte(a,v)end
  for reg,v in pairs(p.regs)do memory.setregister(reg,v)end
 end)end
end)
function finish_probe()assert(not pending);f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else 'v1';data=inventory();cases=[]
    for s in data['stages']:
        for stream in s['scripts']:
            common=dict(level=s['level'],scene=1 if s['indoor']else 0,vertical=int(s['level']==2),screen=stream['screen'])
            if s['indoor']:
                for cursor in [0,1,255]:
                    for cleared in [0,1]:cases.append(dict(common,cursor=cursor,scroll=0,pool=3,cleared=cleared))
            else:
                offset=0;script=stream['bytes']
                while True:
                    trigger=script[offset]
                    if trigger==255:
                        cases.append(dict(common,cursor=offset,scroll=255,pool=3,cleared=0));break
                    for scroll in sorted(set([max(0,(trigger&254)-1),trigger&254,min(255,(trigger&254)+3)])):
                        for pool in [0,1,2,3]:cases.append(dict(common,cursor=offset,scroll=scroll,pool=pool,cleared=1))
                    offset+=3+(script[offset+1]>>6)
    hook=HOOK.replace('FIELDS','{'+','.join(hex(a)for a in FIELDS)+'}').replace('CASES','{'+','.join('{'+','.join(f'{k}={v}'for k,v in c.items())+'}'for c in cases)+'}')
    spec={'name':'all-enemy-spawn-'+label,'load_state':'coop-play','frames':len(cases),'segments':[],'captures':[],'hooks_lua':hook,'timeout_seconds':150}
    path=BASE/(spec['name']+'.json')
    if '--reuse' in sys.argv[2:]:
        out=BASE/'runs'/spec['name'];assert json.loads((out/'scenario.json').read_text())==spec
        assert json.loads((out/'capture-complete.json').read_text())=={'status':'complete'}
    else:path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(l)for l in (out/'spawns.jsonl').read_text().splitlines()];assert len(rows)==len(cases),(len(rows),len(cases))
    fixture={'source_is_original_execution':True,'interventions':__doc__,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'records':rows}
    target=ROOT/'games/contra/engine/testdata/original-enemy-all-spawn.json.gz';packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['records']==rows
    else:target.write_bytes(packed)
    print(json.dumps({'cases':len(rows),'scripts':99,'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
