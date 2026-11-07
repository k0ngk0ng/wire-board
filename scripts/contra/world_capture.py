"""Record the original player/projectile frame phase before implementing it.

Failure inventory: advancing P2 before P1; clearing scroll or exposure too late;
decrementing timers before sprite/fire logic; wrong peer-camera correction;
losing an empty/inactive player's state; bullet stepping before player movement;
drawing every projectile instead of the original alternating logical slots.
This captures an original phase, not a complete independent game acceptance.
"""
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import sys

ROOT=Path(__file__).resolve().parents[2]
os.environ.setdefault('CONTRA_REFERENCE_ROOT',str(ROOT/'.local/contra-world/oracle'))
from capture_reference import BASE,run

HOOK=r'''
local file=assert(io.open(out..'/frame-phase.jsonl','w'))
local pfields=PLAYER_FIELDS
local wfields=WORLD_FIELDS
local pending=nil;local exits={};local entryHooks={};local serial=0
local function read(a)return memory.readbyte(a)end
local function fields(addresses,actor)
 local out={};for _,a in ipairs(addresses)do out[#out+1]=tostring(read(a+actor))end
 return '['..table.concat(out,',')..']'
end
local function hex(address,count)
 local out={};for a=address,address+count-1 do out[#out+1]=string.format('%02x',read(a))end
 return table.concat(out)
end
local function snapshot()
 return string.format('{"players":[%s,%s],"world":%s,"projectiles":"%s","collision":"%s","display":{"codes":"%s","x":"%s","y":"%s","attributes":"%s"}}',fields(pfields,0),fields(pfields,1),fields(wfields,0),hex(0x368,336),hex(0x680,128),hex(0x300,10),hex(0x334,10),hex(0x31a,10),hex(0x34e,10))
end
local function arm(pc)
 if exits[pc] then return end;exits[pc]=true
 memory.registerexec(pc,1,function()
  if pending and pending.pc==pc and memory.getregister('s')==pending.sp then
   local p=pending;pending=nil
   file:write(string.format('{"id":%d,"frame":%d,"before":%s,"after":%s,"order":[%s]}\n',p.id,p.frame,p.before,snapshot(),table.concat(p.order,',')))
  end
  if entryHooks[pc] then entryHooks[pc]() end
 end)
end
entryHooks[0xd296]=function()
 assert(not pending,'nested player/projectile phase')
 local s=memory.getregister('s');local pc=read(0x100+AND(s+1,255))+256*read(0x100+AND(s+2,255))+1
 serial=serial+1;pending={id=serial,frame=emu.framecount(),pc=pc,sp=AND(s+2,255),before=snapshot(),order={}}
 arm(pc)
end
arm(0xd296)
for pc,name in pairs({[0xd2d9]='players',[0xd38a]='actor',[0xd356]='peerCamera',[0xfa0a]='projectiles'})do
 entryHooks[pc]=function()
  if pending then pending.order[#pending.order+1]=string.format('{"phase":"%s","actor":%d}',name,memory.getregister('x')) end
 end
 arm(pc)
end
function finish_probe()assert(not pending);file:close()end
'''


def main():
    label=sys.argv[1] if len(sys.argv)>1 else 'v1'
    metadata_path=ROOT/'docs/research/nes-contra/evidence/static-playerstep-complete-oracle.json.gz'
    metadata=json.loads(gzip.decompress(metadata_path.read_bytes()))
    pfields=metadata['player_fields']
    wfields=metadata['world_fields']+[
        {'name':'weaponStrength','address':'0x2f'},{'name':'vulnerablePlayer','address':'0x8e'},
        {'name':'indoorScroll','address':'0x73'},{'name':'screenCleared','address':'0x6a'},
        {'name':'solidCollision','address':'0x59'}]
    lua=HOOK.replace('PLAYER_FIELDS','{'+','.join(f['address'] for f in pfields)+'}')
    lua=lua.replace('WORLD_FIELDS','{'+','.join(f['address'] for f in wfields)+'}')
    (BASE/'states').mkdir(parents=True,exist_ok=True)
    for mode in ('solo','coop'):
        src=ROOT/f'.local/contra/states/{mode}-play.fcs';dst=BASE/f'states/{mode}-play.fcs'
        if dst.exists():assert dst.read_bytes()==src.read_bytes()
        else:shutil.copyfile(src,dst)
    scenarios=[
        ('solo','solo',200,[{'start':1,'end':90,'keys':['right']},{'start':5,'end':5,'keys':['A']},{'start':120,'end':180,'keys':['left']}] + [{'start':n,'end':n,'keys':['B']} for n in range(1,200,8)]),
        ('coop-peer','coop',420,[{'start':1,'end':420,'player':1,'keys':['right','B']},{'start':1,'end':40,'player':2,'keys':['left']},{'start':240,'end':420,'player':2,'keys':['right','B']}]),
        ('coop-run','coop',600,[{'start':1,'end':600,'player':p,'keys':['right','B']} for p in (1,2)]+[{'start':n,'end':n+1,'player':p,'keys':['A']} for p in (1,2) for n in range(60,600,90)]),
    ]
    results=[]
    for name,mode,frames,segments in scenarios:
        spec={'name':f'world-phase-{label}-{name}','load_state':mode+'-play','frames':frames,'segments':segments,'captures':[1,frames],'hooks_lua':lua,'timeout_seconds':90}
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
        observed=run(path)
        records=[json.loads(line) for line in (observed/'frame-phase.jsonl').read_text().splitlines()]
        assert records
        results.append({'name':name,'source':json.loads((observed/'source.json').read_text()),'scenario':spec,'records':records,'ramSHA256':hashlib.sha256((observed/'ram.bin').read_bytes()).hexdigest()})
    fixture={'source_is_original_execution':True,'scope':'Original $D296 player/projectile frame phase; enemy and map phases execute later and are not included.',
             'metadataSourceSHA256':hashlib.sha256(metadata_path.read_bytes()).hexdigest(),'player_fields':pfields,'world_fields':wfields,'scenarios':results}
    target=ROOT/'docs/research/nes-contra/evidence/world-phase-original.json.gz'
    packed=gzip.compress(json.dumps(fixture,separators=(',',':')).encode(),mtime=0)
    if target.exists():
        old=json.loads(gzip.decompress(target.read_bytes()))
        for a,b in zip(old['scenarios'],results,strict=True):assert a['records']==b['records'] and a['ramSHA256']==b['ramSHA256']
    else:target.write_bytes(packed)
    print(json.dumps({'frames':sum(len(s['records']) for s in results),'sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'label':label}))


if __name__=='__main__':main()
