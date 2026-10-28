"""Observe machine-stage AI from declared original entry snapshots.

The saved entries have one previously declared level-index intervention. This
recorder makes no RAM writes: subsequent movement and firing are ordinary input.
Failure cases: wrong stage/table, missed child-slot/global/queue mutations,
sprite-only fire mistaken for harmless visuals, lost audio reset and stale map.
"""
import ast,gzip,hashlib,json,os,shutil,sys
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-machines/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def literal(path,name):
    for node in ast.parse(path.read_text()).body:
        if isinstance(node,ast.Assign)and any(isinstance(t,ast.Name)and t.id==name for t in node.targets):return ast.literal_eval(node.value)
    raise ValueError(name)
ENEMY=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS')
PLAYER=literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS')

HOOK=r'''
local f=assert(io.open(out..'/machines.jsonl','w'));local pending=nil;local returns={};local ordinal=0
local ef=ENEMY;local pf=PLAYER
local function read(a)return memory.readbyte(a)end
local function hx(a,n)local t={};for k=a,a+n-1 do t[#t+1]=string.format('%02x',read(k))end;return table.concat(t)end
local function fields(rows,slot)local t={};for _,a in ipairs(rows)do t[#t+1]=tostring(read(a+slot))end;return '['..table.concat(t,',')..']'end
local function state()
 local entities={};for i=0,15 do entities[#entities+1]=fields(ef,i)end
 return '{"enemies":['..table.concat(entities,',')..'],"players":['..fields(pf,0)..','..fields(pf,1)..'],"globals":"'..hx(0,256)..'","timers":"'..hx(0x190,16)..'","queue":"'..hx(0x700,192)..'","supertiles":"'..hx(0x600,128)..'","collision":"'..hx(0x680,128)..'"}'
end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  if not pending or pending.pc~=pc or pending.sp~=memory.getregister('s')then return end
  local p=pending;pending=nil
  f:write(string.format('{"id":%d,"frame":%d,"slot":%d,"level":%d,"kind":%d,"routine":%d,"before":%s,"after":%s,"audio":[%s]}\n',p.id,p.frame,p.slot,p.level,p.kind,p.routine,p.before,state(),table.concat(p.audio,',')))
 end)
end
memory.registerexec(0xe58a,1,function()
 local slot=memory.getregister('x');local kind=read(0x528+slot)
 if kind<16 then return end
 assert(not pending,'nested enemy dispatch')
 local sp=memory.getregister('s');local pc=read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1
 ordinal=ordinal+1
 pending={id=ordinal,pc=pc,sp=AND(sp+2,255),frame=emu.framecount(),slot=slot,level=read(0x30),kind=kind,routine=read(0x4b8+slot),before=state(),audio={}}
 arm(pc)
end)
memory.registerexec(0xf9bc,1,function()if pending then pending.audio[#pending.audio+1]='{"play":'..memory.getregister('a')..'}'end end)
memory.registerexec(0xf9d4,1,function()if pending then pending.audio[#pending.audio+1]='{"reset":true}'end end)
function finish_probe()assert(not pending,'unfinished machine AI');f:close()end
'''

def main():
    label=sys.argv[1]if len(sys.argv)>1 else'v1'
    metadata=json.loads((ROOT/'docs/research/nes-contra/evidence/campaign-controlled-entries.json').read_text())
    all_records=[];sources=[];counts={}
    for stage in [6,7]:
        src=ROOT/f'.local/contra-campaign/states/stage-{stage}-v2.fcs'
        expected=next(x for x in metadata['entries']if x['stage']==stage)
        assert hashlib.sha256(src.read_bytes()).hexdigest()==expected['snapshotSHA256']
        target=BASE/f'states/stage-{stage}.fcs';target.parent.mkdir(parents=True,exist_ok=True)
        if target.exists():assert target.read_bytes()==src.read_bytes()
        else:shutil.copyfile(src,target)
        spec={'name':f'machines-natural-{label}-{stage}','load_state':f'stage-{stage}','frames':1600,'captures':[1,300,600,1000,1600],
              'segments':[{'start':1,'end':1600,'player':p,'keys':['right']}for p in [1,2]]+
                  [{'start':n,'end':n,'player':p,'keys':['B']}for p in [1,2]for n in range(1,1601,4)]+
                  [{'start':n,'end':n+1,'player':p,'keys':['A']}for p in [1,2]for n in range(40,1600,80)],
              'hooks_lua':HOOK.replace('ENEMY','{'+','.join(hex(a)for _,a in ENEMY)+'}').replace('PLAYER','{'+','.join(hex(a)for _,a in PLAYER)+'}'),'timeout_seconds':120}
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
        out=BASE/'runs'/spec['name']
        if out.exists():
            assert json.loads((out/'capture-complete.json').read_text())=={'status':'complete'}
            assert json.loads((out/'scenario.json').read_text())==spec
        else:out=run(path)
        records=[json.loads(line)for line in(out/'machines.jsonl').read_text().splitlines()]
        assert all(r['level']==stage-1 for r in records)
        for r in records:
            key=f'{stage}:{r["kind"]:02x}:{r["routine"]}';counts[key]=counts.get(key,0)+1
        all_records.extend(records);sources.append({'stage':stage,'entry':expected,'source':json.loads((out/'source.json').read_text()),'scenario':spec,'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()})
    data={'source_is_original_execution':True,'scope':'Controller-only continuation of declared controlled stage-entry snapshots; not natural campaign traversal.',
          'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ENEMY],'player_fields':[{'name':n,'address':hex(a)}for n,a in PLAYER],
          'sources':sources,'records':all_records,'counts':counts}
    target=ROOT/'games/contra/engine/testdata/original-enemy-machines-natural.json.gz'
    packed=gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0)
    if target.exists():
        old=json.loads(gzip.decompress(target.read_bytes()));assert old['records']==all_records,'machine replay differs'
    else:target.write_bytes(packed)
    print(json.dumps({'records':len(all_records),'counts':counts,'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
