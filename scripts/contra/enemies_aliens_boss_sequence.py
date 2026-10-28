"""Continuous native Guardian/Heart phase verification, with declared inputs.

The live pool and named shared state persist across original E54F calls. Only
initial actors and one scheduled destruction transition are injected; native
after snapshots never drive the independent engine. Not a natural Boss victory.
"""
from pathlib import Path
import argparse,gzip,hashlib,json
import enemies_aliens_boss_capture as source
from capture_reference import run
ROOT,BASE=source.ROOT,source.BASE

def main():
    p=argparse.ArgumentParser();p.add_argument('--name',required=True);p.add_argument('--boss',choices=['guardian','heart'],required=True);args=p.parse_args()
    kind=16 if args.boss=='guardian'else 22;frames=1200 if kind==16 else 1000;hit=700 if kind==16 else 600
    wf=source.WORLD+[('queueUsed',0x21),('alienAimIndex',0x8b)]
    pf=source.literal(ROOT/'scripts/contra/static_capture_playerstep.py','PLAYER_FIELDS')
    h=source.literal(ROOT/'scripts/contra/enemies_collision_tick_capture.py','HOOK')
    h=h.replace('PLAYERS','{'+','.join(hex(a)for _,a in pf)+'}').replace('ENEMIES','{'+','.join(hex(a)for _,a in source.FIELDS)+'}').replace('WORLD','{'+','.join(hex(a)for _,a in wf)+'}').replace('CAPTURE_BIAS','true')
    h=h.replace('if p.restore then for address,value in pairs(p.restore)','if after_phase then after_phase()end\n   if p.restore then for address,value in pairs(p.restore)')
    h+='''
local pool={};local kept={};local terrain={};local fields={FIELDS};local globals={0x76,0x5d,0x4d,0x4f,0x3b,0x2a,0x2b,0x06,0x12,0x13,0x8b}
function configure_phase(n)
 local saved={};for a=0,2047 do if a<0x100 or a>=0x200 then saved[a]=memory.readbyte(a)end end
 for i=0,15 do for _,a in ipairs(fields)do memory.writebyte(a+i,pool[a+i]or 0)end end
 for a,v in pairs(kept)do memory.writebyte(a,v)end
 for a,v in pairs(terrain)do memory.writebyte(a,v)end
 if n==1 then
  local values={[0x4b8]=1,[0x528]=KIND,[0x30a]=1,[0x33e]=176,[0x324]=96,[0x598]=141,[0x588]=131,[0x578]=100}
  for a,v in pairs(values)do memory.writebyte(a+15,v)end
  memory.writebyte(0x3b,0);memory.writebyte(0x2a,7);memory.writebyte(0x2b,3);memory.writebyte(0x8b,0);memory.writebyte(0x12,160);memory.writebyte(0x13,33)
 end
 if n==HIT then memory.writebyte(0x4b8+15,4)end
 memory.writebyte(0x1a,n%256);memory.writebyte(0x34,(n*17)%256);memory.writebyte(0x2f,3)
 memory.writebyte(0x68,0);memory.writebyte(0x30,7);memory.writebyte(0x40,0);memory.writebyte(0x41,0);memory.writebyte(0x59,0)
 memory.writebyte(0x21,n%11==0 and 64 or n%7==0 and 40 or 0);memory.writebyte(0x23,0)
 memory.writebyte(0x8e,1);memory.writebyte(0xdf,2)
 for i=0,1 do memory.writebyte(0x90+i,1);memory.writebyte(0x38+i,0);memory.writebyte(0xae+i,255);memory.writebyte(0xb0+i,0);memory.writebyte(0x334+i,32+((n*(i+1))%192));memory.writebyte(0x31a+i,180-i*16)end
 for a=0x368,0x4b7 do memory.writebyte(a,0)end
 return saved
end
function after_phase()
 for i=0,15 do for _,a in ipairs(fields)do pool[a+i]=memory.readbyte(a+i)end end
 for _,a in ipairs(globals)do kept[a]=memory.readbyte(a)end
 for a=0x680,0x6ff do terrain[a]=memory.readbyte(a)end
end
'''.replace('FIELDS',','.join(hex(a)for _,a in source.FIELDS)).replace('KIND',str(kind)).replace('HIT',str(hit))
    spec={'name':args.name,'load_state':'stage-8','frames':frames,'captures':[1,frames],'hooks_lua':h,'timeout_seconds':180}
    path=BASE/(args.name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
    rows=[json.loads(line)for line in(out/'enemy-ticks.jsonl').read_text().splitlines()];assert len(rows)==frames
    data={'source_is_original_execution':True,'scope':'Controlled continuous Japanese '+args.boss+' E54F phases; not natural completion.',
          'source':json.loads((out/'source.json').read_text()),'scenario':spec,'destroyedTick':hit,'kind':kind,'world_fields':wf,'records':rows}
    dest=ROOT/f'games/contra/engine/testdata/original-enemies-aliens-boss-{args.boss}-sequence.json.gz'
    if dest.exists():assert json.loads(gzip.decompress(dest.read_bytes()))['records']==rows,'continuous native replay differs'
    else:dest.write_bytes(gzip.compress(json.dumps(data,separators=(',',':')).encode(),mtime=0))
    result={'ticks':len(rows),'entityCalls':sum(len(r['slots'])for r in rows),'fixtureSHA256':hashlib.sha256(dest.read_bytes()).hexdigest(),'ramSHA256':hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest()};(out/'verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))

if __name__=='__main__':main()
