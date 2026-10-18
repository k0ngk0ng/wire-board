"""Controlled reset/input/action chains using unchanged original CPU routines."""
import gzip,hashlib,json
from verify_ball18 import ROOT,ROM,BASE,PRG,SoundMemory,MPU,put
from extract_world_inputs import decode_world_input_state
from extract_world_preparation import context

def main():
 chains=[]
 for actor in range(4):
  for selected in [4,5]:
   r=bytearray(BASE)
   for i in range(4):
    r[0x512+i]=128;r[0x51f+i]=128;r[0x642+i]=64;r[0x437+i]=0;r[0x444+i]=0;r[0x451+i]=0;r[0x52c+i]=255;r[0x5f7+i]=0
    put(r,100*256 if i==actor else (350+i*20)*256,0x68+i,0x75+i,0x82+i);put(r,40*256,0x8f+i,0x9c+i);put(r,0,0xa9+i,0xb6+i,0xc3+i)
    for addrs in [(0x478+i,0x485+i),(0x492+i,0x49f+i),(0x4ac+i,0x4b9+i),(0x4c6+i,0x4d3+i)]:put(r,0,*addrs)
   r[0x672+actor]=13;r[0x516]=0;r[0x53d]=255;r[0x517]=0;r[0x53e]=255;r[0x518]=128;r[0x525]=128;r[0x53f]=6
   if selected==5:r[0x53d]=actor^1
   put(r,100*256,0x68+selected,0x75+selected,0x82+selected);put(r,40*256,0x8f+selected,0x9c+selected);put(r,0,0xa9+selected,0xb6+selected,0xc3+selected)
   r[0x539:0x53d]=[0]*4;r[0x539+actor]=128;r[0x4e8:0x4ec]=[0,1,2,3];r[0:8]=bytes(8)
   m=SoundMemory(PRG,r);records=[]
   def run(pc,end,bank):
    m[0x8000:0xc000]=PRG[bank*16384:(bank+1)*16384];c=MPU(memory=m,pc=pc);c.sp=255;c.stPushWord(0x6fff)
    for n in range(100000):
     if c.pc==end:return
     c.step()
    raise RuntimeError(hex(c.pc))
   for n in range(100):
    before=bytes(m[:2048]);run(0x8469,0x7000,5);reset=bytes(m[:2048]);run(0xd32c,0xd345,6);afterInput=bytes(m[:2048]);run(0xd345,0xd348,6);after=bytes(m[:2048])
    records.append(dict(before=decode_world_input_state(before),afterReset=decode_world_input_state(reset),afterInput=decode_world_input_state(afterInput),after=decode_world_input_state(after)))
    if m[0x672+actor]&127==0:break
   else:raise RuntimeError(('chain unfinished',actor,selected))
   ctx=dict(world=context(r)['actions'],difficulty=r[0x58c],scoreDifference=r[0x696],cachedZones=list(r[0x66a:0x66e]),objectKinds=list(r[0x3d6:0x3e3]),hoopLinks=list(r[0x649:0x64f]),humanTactics=list(r[0x666:0x668]))
   chains.append(dict(actor=actor,selected=selected,context=ctx,records=records))
 out=ROOT/'games/basketball/engine/testdata';packed=gzip.compress((json.dumps(chains,separators=(',',':'))+'\n').encode(),mtime=0);(out/'ai-auxiliary-cooperation-chains.json.gz').write_bytes(packed)
 meta=dict(scope='single controlled initialization, unchanged original contact reset + full input + full action phases, no reinjection between steps; not full-world/menu evidence',chains=len(chains),steps=[len(c['records']) for c in chains],romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(packed).hexdigest());(out/'ai-auxiliary-cooperation-chains-source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps(meta))
if __name__=='__main__':main()
