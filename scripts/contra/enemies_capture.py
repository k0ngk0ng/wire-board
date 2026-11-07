"""Observe Japanese enemy spawning/dispatch through natural dual-player input.

Failure inventory before implementation: mismatched bank/dispatch state,
descending-slot order lost, overwritten/full pool allocation, missed child
spawns or cross-slot mutation, spawn trigger timing drift, lost fixed-point
carry, RNG/world inputs omitted, and collision side effects mixed with AI.
No gameplay memory writes occur in this recorder.
"""
from pathlib import Path
import gzip
import hashlib
import json
import os
import shutil
import sys

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies/oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run

FIELDS=[('sprite',0x30a),('y',0x324),('x',0x33e),('spriteAttributes',0x358),
 ('routine',0x4b8),('yFraction',0x4c8),('xFraction',0x4d8),('velocityY',0x4e8),
 ('velocityYFraction',0x4f8),('velocityX',0x508),('velocityXFraction',0x518),
 ('kind',0x528),('animationDelay',0x538),('variableA',0x548),('attackDelay',0x558),
 ('frame',0x568),('hp',0x578),('scoreCollision',0x588),('stateWidth',0x598),
 ('attributes',0x5a8),('variable1',0x5b8),('variable2',0x5c8),('variable3',0x5d8),('variable4',0x5e8)]
WORLD=[('frame',0x1a),('mode',0x22),('levelRoutine',0x2c),('level',0x30),('loop',0x31),('random',0x34),
 ('bossDefeated',0x3b),('sceneType',0x40),('scrollType',0x41),('screen',0x64),('screenScroll',0x65),
 ('frameScroll',0x68),('screenCleared',0x6a),('scrollState',0x73),('autoScrollA',0x75),('autoScrollB',0x76),
 ('soldierRoutine',0x79),('soldierTimer',0x7a),('soldierX',0x7b),('soldierY',0x7c),
 ('screenReadOffset',0x82),('bossScrollComplete',0x84),('soldierAllowed',0x8e),
 ('activePlayers',0xdf),('scrollY',0xfc),('scrollX',0xfd),('ppuControl',0xff),
 ('soldierScreen',0x195),('soldiersGenerated',0x196)]
HOOK=r'''
local f=assert(io.open(out..'/enemies.jsonl','w'))
local mf=assert(io.open(out..'/enemy-maps.jsonl','w'))
local fields=FIELDS;local wf=WORLD;local pending={};local hooks={};local returns={};local known={};local mapcount=0;local count=0
local function read(a)return memory.readbyte(a)end
local function word(a)return read(a)+256*read(a+1)end
local function arr(fields,i)local t={};for _,a in ipairs(fields)do t[#t+1]=tostring(read(a+i))end;return '['..table.concat(t,',')..']'end
local function state()
 local enemies={};for i=0,15 do enemies[#enemies+1]=arr(fields,i)end
 local map={};for a=0x680,0x6ff do map[#map+1]=string.format('%02x',read(a))end
 local data=table.concat(map);local id=known[data]
 if not id then mapcount=mapcount+1;id=mapcount;known[data]=id;mf:write(string.format('{"id":%d,"bytes":"%s"}\n',id,data))end
 local players={};for i=0,1 do players[#players+1]=string.format('{"state":%d,"x":%d,"y":%d,"hidden":%d,"jump":%d,"water":%d,"spawnInvincible":%d,"invincible":%d,"gameOver":%d}',read(0x90+i),read(0x334+i),read(0x31a+i),read(0xba+i),read(0xa0+i),read(0xb2+i),read(0xae+i),read(0xb0+i),read(0x38+i))end
 return '{"enemies":['..table.concat(enemies,',')..'],"world":'..arr(wf,0)..',"players":['..table.concat(players,',')..'],"map":'..id..'}'
end
local function regs()return string.format('[%d,%d,%d,%d]',memory.getregister('a'),memory.getregister('x'),memory.getregister('y'),memory.getregister('p'))end
local function arm(pc)
 if returns[pc]then return end;returns[pc]=true
 memory.registerexec(pc,1,function()
  local sp=memory.getregister('s')
  for i=#pending,1,-1 do
   local p=pending[i]
   if p.pc==pc and p.sp==sp then
    f:write(string.format('{"id":%d,"kind":"%s","frame":%d,"actor":%d,"input":%s,"output":%s,"before":%s,"after":%s}\n',p.id,p.kind,p.frame,p.actor,p.input,regs(),p.before,state()))
    table.remove(pending,i)
   end
  end
  if hooks[pc]then hooks[pc]()end
 end)
end
local function hook(address,name,bank)
 hooks[address]=function()
  if bank and read(0x8000)~=bank then return end
  local sp=memory.getregister('s');local pc=word(0x100+AND(sp+1,255))+1
  count=count+1
  pending[#pending+1]={id=count,kind=name,pc=pc,sp=AND(sp+2,255),frame=emu.framecount(),actor=memory.getregister('x'),input=regs(),before=state()}
  arm(pc)
 end
 arm(address)
end
hook(0xa97b,'screenSpawn',0x14)
hook(0xaa85,'soldierSpawn',0x14)
hook(0xe58a,'enemyStep',0x10)
hook(0xedad,'initialize')
hook(0xed62,'clear')
function finish_probe()assert(#pending==0,'unfinished enemy routine');f:close();mf:close()end
'''

def main():
    name=sys.argv[1] if len(sys.argv)>1 else 'natural-enemies-v1'
    source=ROOT/'.local/contra/states/coop-play.fcs';target=BASE/'states/coop-play.fcs'
    target.parent.mkdir(parents=True,exist_ok=True)
    if target.exists():assert source.read_bytes()==target.read_bytes()
    else:shutil.copyfile(source,target)
    hook=HOOK.replace('FIELDS','{'+','.join(hex(a) for _,a in FIELDS)+'}').replace('WORLD','{'+','.join(hex(a) for _,a in WORLD)+'}')
    spec={'name':name,'load_state':'coop-play','frames':1200,'captures':[1,300,600,900,1200],
          'segments':[{'start':1,'end':1200,'player':p,'keys':['right','B']}for p in (1,2)]+
                     [{'start':n,'end':n+1,'player':p,'keys':['A']}for p in (1,2)for n in range(60,1200,90)],
          'hooks_lua':hook,'timeout_seconds':90}
    path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    records=[json.loads(line)for line in (out/'enemies.jsonl').read_text().splitlines()]
    maps=[json.loads(line)for line in (out/'enemy-maps.jsonl').read_text().splitlines()]
    counts={};states={}
    for row in records:
        counts[row['kind']]=counts.get(row['kind'],0)+1
        if row['kind']=='enemyStep':
            e=row['before']['enemies'][row['actor']];states.setdefault(str(e[11]),set()).add(e[4])
    fixture={'source_is_original_execution':True,'source':json.loads((out/'source.json').read_text()),
             'interventions':'none; ordinary two-player input only','scenario':spec,
             'fields':[{'name':n,'address':hex(a)}for n,a in FIELDS],
             'world_fields':[{'name':n,'address':hex(a)}for n,a in WORLD],
             'maps':maps,'records':records,'counts':counts,'states':{k:sorted(v)for k,v in states.items()}}
    data=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    dest=ROOT/'docs/research/nes-contra/evidence/enemies-natural.json.gz'
    if dest.exists():
        old=json.loads(gzip.decompress(dest.read_bytes()))
        assert old['records']==records and old['maps']==maps,'repeat differs'
    else:dest.write_bytes(data)
    summary={'passed':True,'counts':counts,'states':fixture['states'],'fixture_sha256':hashlib.sha256(dest.read_bytes()).hexdigest(),
             'ram_sha256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    (out/'enemy-source.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))

if __name__=='__main__':main()
