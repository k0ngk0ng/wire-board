"""Record the complete Japanese $E54F enemy phase for integration-first tests.

Failure inventory before Tick implementation: stale player targets after a
previous slot's hit; ascending order or copied slots; missing OnEnemy reset;
incorrect post-AI gates; delayed Falcon effects; reordered sound requests;
fake graphics admission/results; lost queue/terrain mutations and alias reads.
No RAM or game outcomes are modified by this natural recorder.
"""
import argparse,ast,csv,gzip,hashlib,json,os,shutil
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-collision/tick-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

def literal(path,name):
    for n in ast.parse(path.read_text()).body:
        if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name)and t.id==name for t in n.targets):return ast.literal_eval(n.value)
    raise ValueError(name)

PLAYER=literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS')
ENEMY=literal(ROOT/'scripts/contra/enemies_capture.py','FIELDS')
WORLD=[('frame',0x1a),('gameRoutine',0x18),('level',0x30),('gameLoop',0x31),('random',0x34),
 ('sceneType',0x40),('vertical',0x41),('weaponStrength',0x2f),('attackEnabled',0x8e),('frameScroll',0x68),
 ('solidFlags',0x59),('scrollX',0xfd),('scrollY',0xfc),('control',0xff),('demo',0x1c),('flash',0x7d),
 ('screenReadOffset',0x82),('screen',0x64),('screenScroll',0x65),('queueUsed',0x21),('bossDefeated',0x3b)]

HOOK=r'''
local f=assert(io.open(out..'/enemy-ticks.jsonl','w'))
local pf=PLAYERS;local ef=ENEMIES;local wf=WORLD
local active=nil;local returns={};local ordinal=0
local captureBias=CAPTURE_BIAS
local function read(a)return memory.readbyte(a)end
local function hex(a,n)local t={};for i=a,a+n-1 do t[#t+1]=string.format('%02x',read(i))end;return table.concat(t)end
local function array(fields,slot)local t={};for _,a in ipairs(fields)do t[#t+1]=tostring(read(a+slot))end;return '['..table.concat(t,',')..']'end
local function state()
 local enemies={};for i=0,15 do enemies[#enemies+1]=array(ef,i)end
 return '{"players":['..array(pf,0)..','..array(pf,1)..'],"enemies":['..table.concat(enemies,',')..'],"world":'..array(wf,0)..',"weapons":"'..hex(0x368,336)..'","score":"'..hex(0x7e0,6)..hex(0x3c,4)..'","graphics":"'..hex(0x700,128)..'","supertiles":"'..hex(0x600,128)..'","collision":"'..hex(0x680,128)..'"}'
end
memory.registerexec(0xe54f,1,function()
 assert(not active,'nested enemy tick')
 local restore=nil
 if configure_phase then restore=configure_phase(ordinal+1)end
 local sp=memory.getregister('s')
 local pc=read(0x100+AND(sp+1,255))+256*read(0x100+AND(sp+2,255))+1
 ordinal=ordinal+1
 active={id=ordinal,frame=emu.framecount(),pc=pc,sp=AND(sp+2,255),before=state(),sounds={},slots={},requests={},restore=restore,biasWrites={}}
 if not returns[pc]then
  returns[pc]=true
  memory.registerexec(pc,1,function()
   if not active or active.pc~=pc or active.sp~=memory.getregister('s')then return end
   local p=active;active=nil
   local audio=''
   if p.audioResets then audio=string.format(',"audioReset":true,"audioResets":%d,"discardedSounds":[%s]',p.audioResets,table.concat(p.discarded,','))end
   if captureBias then audio=audio..',"generationBiasWrites":['..table.concat(p.biasWrites,',')..']'end
   f:write(string.format('{"id":%d,"frame":%d,"before":%s,"after":%s,"sounds":[%s],"slots":[%s],"requests":[%s]%s}\n',p.id,p.frame,p.before,state(),table.concat(p.sounds,','),table.concat(p.slots,','),table.concat(p.requests,','),audio))
   if p.restore then for address,value in pairs(p.restore)do memory.writebyte(address,value)end end
  end)
 end
end)
memory.registerwrite(0x06,1,function(a,size,value)
 if active and captureBias then active.biasWrites[#active.biasWrites+1]=string.format('{"pc":"0x%04x","bank":%d,"slot":%d,"value":%d}',memory.getregister('pc'),read(0x8000),read(0x83),value)end
end)
memory.registerexec(0xe58a,1,function()
 if active then local slot=memory.getregister('x');active.slots[#active.slots+1]=string.format('{"slot":%d,"kind":%d,"routine":%d}',slot,read(0x528+slot),read(0x4b8+slot))end
end)
memory.registerexec(0xf9bc,1,function()if active then active.sounds[#active.sounds+1]=tostring(memory.getregister('a'))end end)
memory.registerexec(0xf9d4,1,function()
 if active then
  active.audioResets=(active.audioResets or 0)+1;active.discarded=active.discarded or {}
  for _,code in ipairs(active.sounds)do active.discarded[#active.discarded+1]=code end
  active.sounds={}
 end
end)
memory.registerexec(0xc3d1,1,function()
 if active then active.requests[#active.requests+1]=string.format('{"x":%d,"y":%d,"code":%d,"used":%d}',memory.getregister('a'),memory.getregister('y'),read(0x10),read(0x21))end
end)
function finish_probe()assert(not active,'unfinished whole enemy phase');f:close()end
'''

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--name',required=True);parser.add_argument('--route',choices=['machine','ordinary','traverse','controlled','boss'],required=True);parser.add_argument('--cases',type=Path);parser.add_argument('--bias',action='store_true');args=parser.parse_args()
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    source=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    if target.exists():assert target.read_bytes()==source.read_bytes()
    else:shutil.copyfile(source,target)
    if args.route=='machine':
        frames=700
        segments=([{'start':1,'end':500,'player':p,'keys':['right']}for p in (1,2)]+
          [{'start':n,'end':n,'player':p,'keys':['A']}for p in (1,2)for n in (15,85,155)]+
          [{'start':220,'end':270,'player':p,'keys':['down','right']}for p in (1,2)]+
          [{'start':n,'end':n,'player':p,'keys':['B']}for p in (1,2)for n in range(1,351,4)]+
          [{'start':351,'end':700,'player':p,'keys':['B']}for p in (1,2)])
    elif args.route=='ordinary':
        frames=1200
        segments=([{'start':1,'end':1200,'player':p,'keys':['right','B']}for p in (1,2)]+
          [{'start':n,'end':n+1,'player':p,'keys':['A']}for p in (1,2)for n in range(60,1200,90)])
    elif args.route=='traverse':
        frames=5000;segments=[]
    else:
        assert args.cases,'controlled route requires declared case file'
        cases=json.loads(args.cases.read_text());frames=len(cases);segments=[]
    fields=WORLD+([('delayLow',0x2a),('delayHigh',0x2b)]if args.route=='boss'else[])
    if args.bias:fields+=[('generationBias',0x06)]
    hook=HOOK.replace('PLAYERS','{'+','.join(hex(a)for _,a in PLAYER)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ENEMY)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in fields)+'}').replace('CAPTURE_BIAS','true'if args.bias else'false')
    timeline=None
    if args.route in ('controlled','boss'):
        hook+='\nlocal cases={'+','.join('{'+','.join(f'[{address}]={value}'for address,value in case['writes'].items())+'}'for case in cases)+'}\n'
        hook+='''function configure_phase(index)
 local c=assert(cases[index]);local saved={}
 for a=0,2047 do if a<0x100 or a>=0x200 then saved[a]=memory.readbyte(a)end end
 for a,v in pairs(c)do memory.writebyte(a,v)end
 return saved
end
'''
    if args.route=='traverse':
        retained=ROOT/'games/contra/engine/testdata/original-enemy-collision-tick-traverse.json.gz'
        if retained.exists():timeline=json.loads(gzip.decompress(retained.read_bytes()))['recorded_inputs']
        else:
            input_path=ROOT/'.local/contra-enemies-shooters/natural/runs/shooters-natural-v2/inputs.csv'
            assert hashlib.sha256(input_path.read_bytes()).hexdigest()=='aaf2f2bf19861a7d5673931efee3eb128d16a8e5da5ae4be69febb37f74d8219'
            timeline=[[int(r['p1']),int(r['p2'])]for r in csv.DictReader(input_path.read_text().splitlines())]
        assert len(timeline)==frames
        hook+='\nlocal timeline={'+','.join('{'+','.join(map(str,row))+'}'for row in timeline)+'}\n'
        hook+='''function control_frame(n,pads)
 local names={"right","left","down","up","start","select","B","A"}
 for p=1,2 do for bit,name in ipairs(names)do pads[p][name]=AND(timeline[n][p],2^(bit-1))~=0 end end
end
'''
    spec={'name':args.name,'load_state':'coop-play','frames':frames,'segments':segments,'captures':[1,frames],'hooks_lua':hook,'timeout_seconds':120}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=run(path)
    records=[json.loads(line)for line in (observed/'enemy-ticks.jsonl').read_text().splitlines()]
    assert 0<len(records)<=frames
    if args.route!='traverse':assert len(records)==frames
    scope=args.route+('-bias'if args.bias else'')
    data={'source_is_original_execution':True,'scope':scope,'interventions':'Declared full-phase inputs staged at E54F and restored on return; not natural play.'if args.route in ('controlled','boss')else'none; ordinary dual-player input only','source':json.loads((observed/'source.json').read_text()),'scenario':spec,'recorded_inputs':timeline,'cases':cases if args.route in ('controlled','boss')else None,
      'player_fields':[{'name':n,'address':hex(a)}for n,a in PLAYER],'enemy_fields':[{'name':n,'address':hex(a)}for n,a in ENEMY],'world_fields':[{'name':n,'address':hex(a)}for n,a in fields],'records':records}
    dest=ROOT/f'games/contra/engine/testdata/original-enemy-collision-tick-{scope}.json.gz'
    if dest.exists():
        previous=json.loads(gzip.decompress(dest.read_bytes()));assert previous['records']==records,'whole phase repeat differs'
    else:dest.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    summary={'passed':True,'route':args.route,'ticks':len(records),'entityCalls':sum(len(r['slots'])for r in records),'graphicsRequests':sum(len(r['requests'])for r in records),
      'soundRequests':sum(len(r['sounds'])for r in records),'fixtureSHA256':hashlib.sha256(dest.read_bytes()).hexdigest(),'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()}
    (observed/'verification.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))

if __name__=='__main__':main()
