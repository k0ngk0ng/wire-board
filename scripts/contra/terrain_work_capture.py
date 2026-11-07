"""Observe full C001 frames and named spatial operations before owner integration.

Never infer a generic CPU scratch interface from these traces. Raw writes are
source evidence; independent projections use named query/scan/stream inputs.
"""
from pathlib import Path
import argparse,gzip,hashlib,json,os,shutil
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-terrain-work'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

HOOK=r'''
local frames=assert(io.open(out..'/terrain-work.jsonl','w'))
local active=nil;local serial=0;local calls={};local hooks={};local installed={};local gaps={};local staged=false;local guardianSaved=false;local patching=false;local controlledCases=CONTROLLED_CASES;local fullFrames=FULL_FRAMES
local function read(a)return memory.readbyte(a)end
local function word(a)return read(a)+256*read(a+1)end
local function hx(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function state()
 return string.format('{"work":%d,"game":%d,"routine":%d,"level":%d,"counter":%d,"screen":%d,"progress":%d,"vertical":%d,"scene":%d,"scrollX":%d,"scrollY":%d,"control":%d,"frameScroll":%d,"axis":%d,"tileAddress":%d,"queueUsed":%d,"players":"%s","collision":"%s","enemies":"%s","weapons":"%s","ram":"%s"}',word(0x12),read(0x18),read(0x2c),read(0x30),read(0x1a),read(0x64),read(0x65),read(0x41),read(0x40),read(0xfd),read(0xfc),read(0xff),read(0x68),read(0x60),word(0x62),read(0x21),hx(0x90,80),hx(0x680,128),hx(0x4b8,320),hx(0x368,336),(controlledCases or fullFrames)and hx(0,2048)or '')
end
local function ret()local sp=memory.getregister('s');return read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1,AND(sp+2,255)end
local function operation_input(name)
 return string.format('{"a":%d,"x":%d,"y":%d,"carry":%d,"work":%d,"scrollX":%d,"scrollY":%d,"control":%d,"pageToggle":%d,"vertical":%d,"scene":%d,"axis":%d,"tileAddress":%d,"queueUsed":%d,"code":%d,"playerX":%d,"playerY":%d,"sourceX":%d,"graphicX":%d,"projectileX":%d,"projectileY":%d,"entityX":%d,"entityY":%d,"solidFlags":%d,"screenCleared":%d,"frameScroll":%d,"ram":"%s","collision":"%s"}',memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),AND(memory.getregister('p'),1),word(0x12),read(0xfd),read(0xfc),read(0xff),read(0x10),read(0x41),read(0x40),read(0x60),word(0x62),read(0x21),read(0x10),read(0x334+memory.getregister('x')),read(0x31a+memory.getregister('x')),read(0x0c),read(0x11),read(0x3c8+memory.getregister('x')),read(0x3b8+memory.getregister('x')),read(0x33e+memory.getregister('x')),read(0x324+memory.getregister('x')),read(0x59),read(0x6a),read(0x68),(name=='players'or name=='weapons')and hx(0,2048)or '',hx(0x680,128))
end
local function phase()
 for i=#calls,1,-1 do if calls[i].group then return calls[i].name end end
 return 'frame'
end
local function install(pc)
 if installed[pc]then return end;installed[pc]=true
 memory.registerexec(pc,1,function()
  if active then
   local sp=memory.getregister('s')
   for i=#calls,1,-1 do
    local p=calls[i]
    if p.pc==pc and p.sp==sp then
     p.last=active.sequence;active.sequence=active.sequence+1;p.after=word(0x12);p.a=memory.getregister('a');p.flags=memory.getregister('p');p.ram=(p.name=='weapons')and hx(0,2048)or ''
     table.remove(calls,i)
    end
   end
  end
  if hooks[pc]then hooks[pc]()end
 end)
end
local function watch(address,name,bank,group)
 hooks[address]=function()
  if not active or bank and read(0x8000)~=bank then return end
  local pc,sp=ret();local parent=0;if #calls>0 then parent=calls[#calls].id end
  local p={id=#active.operations+1,name=name,address=address,bank=read(0x8000),pc=pc,sp=sp,parent=parent,group=group,phase=phase(),first=active.sequence,input=operation_input(name)}
  active.sequence=active.sequence+1;active.operations[#active.operations+1]=p;calls[#calls+1]=p;install(pc)
 end
 install(address)
end
hooks[0xc001]=function()
 if read(0x18)~=5 then return end
 assert(not active and #calls==0,'unfinished frame')
 local restore=nil
 if controlledCases then
  local c=assert(controlledCases[serial+1])
  patching=true;for a,v in pairs(c)do memory.writebyte(a,v)end;patching=false
 elseif not staged then
  staged=true
  INITIAL_SETUP
 end
 staged=true;serial=serial+1;active={id=serial,frame=emu.framecount(),before=state(),sequence=0,writes={},operations={},checkpoints={},gaps=gaps,restore=restore,prefixWrites={}};gaps={}
end
hooks[0xf8db]=function()
 if not active then return end
 assert(#calls==0,'operation did not return by F8DB')
 local p=active;active=nil;local ops={}
 for _,o in ipairs(p.operations)do
  assert(o.after,'unfinished named operation')
  ops[#ops+1]=string.format('{"id":%d,"name":"%s","address":%d,"bank":%d,"parent":%d,"group":%s,"phase":"%s","first":%d,"last":%d,"input":%s,"after":%d,"a":%d,"flags":%d,"afterRAM":"%s"}',o.id,o.name,o.address,o.bank,o.parent,tostring(o.group or false),o.phase,o.first,o.last,o.input,o.after,o.a,o.flags,o.ram)
 end
 frames:write(string.format('{"id":%d,"frame":%d,"before":%s,"after":%s,"writes":[%s],"operations":[%s],"checkpoints":[%s],"priorGap":[%s],"prefixWrites":[%s]}\n',p.id,p.frame,p.before,state(),table.concat(p.writes,','),table.concat(ops,','),table.concat(p.checkpoints,','),table.concat(p.gaps,','),table.concat(p.prefixWrites,',')))
 if p.restore then patching=true;for a,v in pairs(p.restore)do memory.writebyte(a,v)end;patching=false end
end
install(0xc001);install(0xf8db)
for address=0x12,0x13 do memory.registerwrite(address,1,function(a,n,v)
 if patching then return end
 local sequence=-1;local opname='outside';local current=0
 if active then sequence=active.sequence;active.sequence=active.sequence+1;opname=phase();if #calls>0 then current=calls[#calls].id end end
 local row=string.format('{"sequence":%d,"pc":%d,"bank":%d,"address":%d,"value":%d,"a":%d,"x":%d,"y":%d,"operation":%d,"phase":"%s"}',sequence,memory.getregister('pc'),read(0x8000),a,v,memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),current,opname)
 if active then active.writes[#active.writes+1]=row elseif staged then gaps[#gaps+1]=row end
end)end
for address=0x700,0x704 do memory.registerwrite(address,1,function(a,n,v)
 if patching or not active or memory.getregister('pc')~=0xe9fc then return end
 active.prefixWrites[#active.prefixWrites+1]=string.format('{"sequence":%d,"pc":%d,"bank":%d,"offset":%d,"keepMask":%d,"value":%d,"work":%d}',active.sequence,memory.getregister('pc'),read(0x8000),a-0x700,read(5),v,word(0x12))
end)end
watch(0xd2d9,'players',nil,true);watch(0xd38a,'player',nil,true)
watch(0xfa0a,'weapons',nil,true);watch(0xfa3a,'scroll',nil,true);watch(0xfa1a,'enemies',nil,true)
watch(0xfa22,'screen-spawn',nil,true);watch(0xfa32,'soldier-spawn',nil,true)
watch(0xd8d9,'player-feet');watch(0xda5e,'player-side');watch(0xd7d1,'player-perspective')
watch(0x83e3,'weapon-perspective',0x1c);watch(0x87f7,'weapon-solid',0x1c)
watch(0xdfb8,'support-query');watch(0xdfec,'query-coordinate');watch(0xdfee,'query-prepared-x');watch(0xdff4,'query-page-toggle')
watch(0xdfe0,'support-index');watch(0xdfe6,'read-index')
watch(0xdd4b,'stream-column');watch(0xddef,'stream-row')
watch(0xc656,'graphic-location');watch(0xeb9b,'entity-probe');watch(0xe989,'collision-clear')
watch(0xb686,'guardian-pair-clear',0x10)
local pair_entry=hooks[0xb686]
hooks[0xb686]=function()
 if active and read(0x8000)==0x10 then active.checkpoints[#active.checkpoints+1]=string.format('{"name":"guardian-read","sequence":%d,"work":%d,"queueUsed":%d}',active.sequence,word(0x12),read(0x21));active.sequence=active.sequence+1 end
 pair_entry()
end
local rowShift=nil
hooks[0xb6a9]=function()
 if not active or read(0x8000)~=0x10 then return end
 local p={id=#active.operations+1,name='guardian-row-shift',address=0xb6a9,bank=0x10,parent=#calls>0 and calls[#calls].id or 0,phase=phase(),first=active.sequence,input=operation_input('guardian-row-shift')}
 active.sequence=active.sequence+1;active.operations[#active.operations+1]=p;rowShift=p
end
hooks[0xb6b6]=function()
 if not rowShift then return end
 local p=rowShift;rowShift=nil;p.last=active.sequence;active.sequence=active.sequence+1;p.after=word(0x12);p.a=memory.getregister('a');p.flags=memory.getregister('p');p.ram=''
end
install(0xb6a9);install(0xb6b6)
local prior_enemy_entry=hooks[0xe54f]
hooks[0xe54f]=function()
 if prior_enemy_entry then prior_enemy_entry()end
 if active then active.checkpoints[#active.checkpoints+1]=string.format('{"name":"enemy-entry","sequence":%d,"work":%d,"queueUsed":%d,"ram":"%s"}',active.sequence,word(0x12),read(0x21),controlledCases and hx(0,2048)or '');active.sequence=active.sequence+1 end
end
install(0xe54f)
function control_frame(n,pads)
 if controlledCases then return end
 for i=0,1 do
  local weapon=AND(read(0xaa+i),15);pads[i+1].right=true;pads[i+1].B=weapon==1 or weapon==4 or n%4==1
  if read(0x90+i)==1 and read(0xa0+i)==0 and read(0xa4+i)==0 and read(0xb2+i)==0 then pads[i+1].A=true end
  if read(0x90+i)==3 and n%30==1 then pads[i+1].A=true end
 end
 if not guardianSaved and read(0x30)==7 then
  for slot=0,15 do if read(0x528+slot)==16 and read(0x4b8+slot)~=0 then
   guardianSaved=true;local save=savestate.create(GUARDIAN_STATE);savestate.save(save);savestate.persist(save)
   writefile('guardian-state.json',string.format('{"frame":%d,"slot":%d,"routine":%d,"screen":%d,"work":%d}',emu.framecount(),slot,read(0x4b8+slot),read(0x64),word(0x12)));break
  end end
 end
end
function finish_probe()assert(not active and #calls==0);frames:close()end
'''

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--name',required=True);parser.add_argument('--stage',type=int,default=8);parser.add_argument('--frames',type=int,default=5000);parser.add_argument('--cases',type=Path);parser.add_argument('--state');parser.add_argument('--full-frames',action='store_true');parser.add_argument('--preserve-lives',action='store_true');args=parser.parse_args()
    assert 1<=args.stage<=8
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    src=ROOT/f'.local/contra-campaign/states/stage-{args.stage}-v2.fcs';dst=BASE/f'states/stage-{args.stage}.fcs'
    if dst.exists():assert src.read_bytes()==dst.read_bytes()
    else:shutil.copyfile(src,dst)
    guardian=BASE/f'states/guardian-{args.name}.fcs';assert not guardian.exists()
    setup=''if args.preserve_lives else"memory.writebyte(0x32,30);memory.writebyte(0x33,30)"
    cases=json.loads(args.cases.read_text())if args.cases else None
    if cases:args.frames=len(cases)
    encoded='nil'if cases is None else'{'+','.join('{'+','.join(f'[{a}]={v}'for a,v in c['writes'].items())+'}'for c in cases)+'}'
    hook=HOOK.replace('INITIAL_SETUP',setup).replace('GUARDIAN_STATE',json.dumps(str(guardian))).replace('CONTROLLED_CASES',encoded).replace('FULL_FRAMES','true'if args.full_frames else'false')
    spec={'name':args.name,'load_state':args.state or f'stage-{args.stage}','frames':args.frames,'segments':[],'captures':[1,args.frames],'hooks_lua':hook,'timeout_seconds':180}
    p=BASE/(args.name+'.json');p.write_text(json.dumps(spec,indent=2)+'\n');out=run(p)
    rows=[json.loads(l)for l in(out/'terrain-work.jsonl').read_text().splitlines()];assert rows
    if cases:assert len(rows)==len(cases),(len(rows),len(cases))
    counts={};last={};operations={}
    for row in rows:
        for w in row['writes']:
            key=f'{w["bank"]:02x}:{w["pc"]:04x}:{w["address"]:02x}';counts[key]=counts.get(key,0)+1;last[key]=w['phase']
        for o in row['operations']:operations[o['name']]=operations.get(o['name'],0)+1
    data={'source_is_original_execution':True,'scope':'Full C001->F8DB observations from retained controlled stage initialization, with only initial reserves30 and subsequent controller policy; no enemy/HP/result injection.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'cases':cases,'frames':rows,'writerCounts':counts,'writerLastPhase':last,'operationCounts':operations,
          'guardian':json.loads((out/'guardian-state.json').read_text())if(out/'guardian-state.json').exists()else None}
    if cases:data['scope']='Declared input setup at each C001 only; no after-state injection or restoration. Native work state is never patched. All intervening writes and named operations observed; not natural gameplay.'
    elif args.preserve_lives:data['scope']='Retained source state with its existing provenance; this run changes no RAM and supplies only controller inputs plus passive observation.'
    target=ROOT/f'docs/research/nes-contra/evidence/terrain-work-{args.name}.json.gz'
    if target.exists():assert json.loads(gzip.decompress(target.read_bytes()))['frames']==rows
    else:target.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    print(json.dumps({'frames':len(rows),'writes':sum(counts.values()),'writers':counts,'operations':operations,'guardian':data['guardian'],'sha256':hashlib.sha256(target.read_bytes()).hexdigest()}))

if __name__=='__main__':main()
