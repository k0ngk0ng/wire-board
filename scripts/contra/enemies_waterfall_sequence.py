"""Continuous original Waterfall Boss phase, with declared test boundaries.

Before integration verification: repeated initializers may hide chain drift;
children generated in lower slots must run immediately; bullets/explosions must
share slots; budget refusal must not advance animation; earlier arm deaths must
affect later mouth firing; final clear delay must remain source-driven.

This is not a natural Boss fight: the initial actors, frame budget/targets and
three scheduled destruction transitions are explicitly controlled. The complete
enemy pool then carries forward from native E54F output, never oracle after data.
"""
import argparse, ast, gzip, hashlib, json, os, shutil
from pathlib import Path
import enemies_waterfall_capture as base
ROOT,BASE=base.ROOT,base.BASE
from capture_reference import run

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--name',required=True);args=parser.parse_args()
    frames=2400
    ef=base.FIELDS
    wf=base.WORLD+[('queueUsed',0x21)]
    player=base.literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS')
    hook=base.literal(ROOT/'scripts/contra/enemies_collision_tick_capture.py','HOOK')
    hook=hook.replace('PLAYERS','{'+','.join(hex(a)for _,a in player)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in ef)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in wf)+'}').replace('CAPTURE_BIAS','true')
    hook=hook.replace('if p.restore then for address,value in pairs(p.restore)', 'if after_phase then after_phase()end\n   if p.restore then for address,value in pairs(p.restore)')
    hook+='''
local pool={};local kept={};local phaseFrame=0
local fields={FIELDS}
local globals={0x85,0x3b,0x2a,0x2b,0x06}
local function put(slot,values)for address,value in pairs(values)do memory.writebyte(address+slot,value)end end
function configure_phase(n)
 local saved={};for a=0,2047 do if a<0x100 or a>=0x200 then saved[a]=memory.readbyte(a)end end
 phaseFrame=n
 for i=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+i,pool[a+i]or 0)end end
 for a,v in pairs(kept)do memory.writebyte(a,v)end
 if n==1 then
  for slot=14,15 do put(slot,{[0x4b8]=1,[0x528]=21,[0x30a]=1,[0x33e]=slot==15 and 80 or 176,[0x324]=56,[0x598]=14,[0x588]=82,[0x578]=241,[0x5a8]=slot==15 and 1 or 0})end
  put(13,{[0x4b8]=1,[0x528]=20,[0x30a]=1,[0x33e]=128,[0x324]=64,[0x598]=141,[0x588]=131,[0x578]=241,[0x548]=2})
  memory.writebyte(0x85,0);memory.writebyte(0x3b,0);memory.writebyte(0x2a,7);memory.writebyte(0x2b,3)
 end
 if n==1000 or n==1600 then local root=n==1000 and 15 or 14;local hand=memory.readbyte(0x518+root);assert(hand<16);memory.writebyte(0x4b8+hand,5)end
 if n==2000 then memory.writebyte(0x4b8+13,6)end
 memory.writebyte(0x1a,n%256);memory.writebyte(0x34,(n*17)%256)
 memory.writebyte(0x84,1);memory.writebyte(0x74,0);memory.writebyte(0x68,0)
 memory.writebyte(0x30,2);memory.writebyte(0x40,0);memory.writebyte(0x41,1);memory.writebyte(0x59,0)
 memory.writebyte(0x21,n%11==0 and 64 or n%7==0 and 40 or 0);memory.writebyte(0x23,0)
 memory.writebyte(0x8e,1);memory.writebyte(0xdf,2)
 for i=0,1 do memory.writebyte(0x90+i,1);memory.writebyte(0x38+i,0);memory.writebyte(0xae+i,255);memory.writebyte(0xb0+i,0);memory.writebyte(0x334+i,32+((n*(i+1))%192));memory.writebyte(0x31a+i,180-i*16)end
 for a=0x368,0x4b7 do memory.writebyte(a,0)end
 return saved
end
function after_phase()
 for i=0,15 do for _,a in ipairs(fields)do pool[a+i]=memory.readbyte(a+i)end end
 for _,a in ipairs(globals)do kept[a]=memory.readbyte(a)end
end
'''.replace('FIELDS',','.join(hex(a)for _,a in ef))
    spec={'name':args.name,'load_state':'stage-3','frames':frames,'captures':[1,600,1200,1800,2400],'hooks_lua':hook,'timeout_seconds':180}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in(out/'enemy-ticks.jsonl').read_text().splitlines()]
    assert len(rows)==frames
    data={'source_is_original_execution':True,'scope':'Controlled continuous E54F Boss sequence, not a natural battle or campaign clear.',
          'interventions':{'initialSlots':{'15':'left shoulder','14':'right shoulder','13':'mouth'},'handDestroyedTicks':[1000,1600],'mouthDestroyedTick':2000,'frameBudget':'64 every11th,40 every7th,else0','players':'scripted visible targets; spawn invincibility255; empty player projectile pool'},
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'world_fields':wf,'records':rows}
    dest=ROOT/'games/contra/engine/testdata/original-enemies-waterfall-sequence.json.gz'
    if dest.exists():assert json.loads(gzip.decompress(dest.read_bytes()))['records']==rows,'continuous original replay differs'
    else:dest.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    result={'ticks':len(rows),'entityCalls':sum(len(r['slots'])for r in rows),'fixtureSHA256':hashlib.sha256(dest.read_bytes()).hexdigest(),'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()}
    (out/'verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))

if __name__=='__main__':main()
