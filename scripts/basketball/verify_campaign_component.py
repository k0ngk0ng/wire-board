"""Original campaign component oracles. CPU calls are explicitly isolated.
Normal menu snapshots are extracted separately from retained controller traces.
"""
import argparse,gzip,hashlib,itertools,json
from pathlib import Path
from verify_ai_mode5_plan11 import ROOT,ROM,PRG,BASE,start,MPU
from verify_pass_floor_original import Memory
OUT=ROOT/'games/basketball/campaign/testdata'

def progress(r):
 return dict(mode=r[0x58d],period=r[0x58c],winBits=list(r[0x6d2:0x6d4]),rewardSet=r[0x6d4],inventory=[x if x<128 else x-256 for x in r[0xd3:0xe3]],equipment=[[x if x<128 else x-256 for x in r[0x6e4+i*3:0x6e7+i*3]] for i in range(4)])
def base():
 r=start(0);r[0x58d]=r[0x58c]=0;r[0x6d2:0x6d5]=bytes(3);r[0xd3:0xe3]=bytes([255]*16);r[0x6e4:0x6f0]=bytes([255]*12);return r

def execute(r,bank,entry,stops=(),presentation=()):
 m=Memory(PRG,r);m[0x8000:0xc000]=PRG[bank*16384:(bank+1)*16384];cpu=MPU(memory=m,pc=entry);cpu.sp=255;cpu.p=0x20;cpu.stPushWord(0x6fff)
 for tick in range(30000):
  if cpu.pc==0x7000 or cpu.pc in stops:return bytearray(m[:2048]),cpu.pc
  if cpu.pc==0xc000 or cpu.pc in presentation:cpu.pc=(cpu.stPopWord()+1)&65535
  else:cpu.step()
 raise RuntimeError(('no return',bank,hex(entry),hex(cpu.pc)))

def decode_code(code):
 r=base();r[0x589]=137;r[0x3cb:0x3d1]=bytes(int(c,16) for c in code)
 r,pc=execute(r,1,0xbf52)
 accepted=r[0x589]==8
 if accepted:r,_=execute(r,3,0xa8c1)
 return dict(code=code,accepted=accepted,after=progress(r))

def encode_source(r):
 # Stop after actual six display glyph writes, before BF3B's NMI wait.
 out,_=execute(r,1,0xbecd,(0xbf3b,))
 glyphs=list(out[0x30e:0x314]);table=list(PRG[16384+0x3f42:16384+0x3f52])
 assert len(table)==16
 return ''.join(format(table.index(v),'X') for v in glyphs),glyphs

def equipment_call(r,kind,character,index):
 r[0x6d9]=character;r[0x6dc]=index;r[0x6ac]=0;r[0]=0;r[4]=0x40 if kind=='cancel' else 0x80
 r[0x589]=155 if kind in ['equip','cancel'] else 156
 before=progress(r)
 # The first display update occurs after semantic inventory writes. Main
 # input/sound instructions are unchanged; rendering is outside this probe.
 after,stop=execute(r,3,0x91dd if kind in ['equip','cancel'] else 0x931d,(0x942c,0x8088))
 return dict(kind=kind,character=character,index=index,before=before,after=progress(after),stop=f'{stop:04X}',menuPhase=after[0x589])

def main():
 OUT.mkdir(parents=True,exist_ok=True)
 passwords=[];encodes=[]
 # Every legitimate count ordering, every 7-bit mask, both modes and all four
 # normally generated reward sets. These are isolated, not earned-win runs.
 for a,b in itertools.product(range(128),repeat=2):
  if a==127 and b==127 or a.bit_count()-b.bit_count() not in (0,1):continue
  for mode,reward in itertools.product(range(2),range(4)):
   r=base();r[0x58d]=mode;r[0x6d2]=a;r[0x6d3]=b;r[0x6d4]=reward
   code,glyphs=encode_source(r);row=decode_code(code);assert row['accepted'];passwords.append(row)
   encodes.append(dict(progress=row['after'],code=code,glyphs=glyphs))
 # Invalid checksum, impossible masks/count order and completed game.
 invalid=[]
 for raw in [0xFFFFFF,0x000001,0x000100,0xFE7FE0,0x020000,0x000010,0x008080,0xFE00F0]:invalid.append(decode_code(f'{raw:06X}'))
 # Keep a valid-checksum selector4 observation, but do not execute the adjacent
 # reward pointer; it is outside the normal generated table domain.
 r=base();r[0x589]=137;r[0x3cb:0x3d1]=bytes([0,0,0,0,4,4]);unsupported,_=execute(r,1,0xbf52)
 equipment=[]
 for char,item,old,index in itertools.product(range(4),range(32),[False,True],[0,1,3]):
  r=base();inv=[5,5,12,22];inv[index]=item;r[0xd3:0xd7]=bytes(inv)
  slot=0 if item<10 else 1 if item<21 else 2
  if old:r[0x6e4+char*3+slot]=[3,14,24][slot]
  equipment.append(equipment_call(r,'equip',char,index))
 for char,slot,count,empty in itertools.product(range(4),range(3),[0,1,15,16],[False,True]):
  r=base();r[0xd3:0xd3+count]=bytes([5]*count)
  if not empty:r[0x6e4+char*3+slot]=[2,13,23][slot]
  equipment.append(equipment_call(r,'unequip',char,slot))
 for char,index in itertools.product(range(4),[0,1,15]):
  r=base();r[0xd3:0xd5]=bytes([1,1]);r[0x6e4+char*3]=3
  equipment.append(equipment_call(r,'cancel',char,index))
 for char,index in itertools.product(range(4),[0,15]):equipment.append(equipment_call(base(),'equip',char,index))
 maps=[]
 masks=[0,1,2,3,7,63,64,85,126,127]
 for a,b,mode in itertools.product(masks,masks,range(2)):
  if a==127 and b==127 or a.bit_count()-b.bit_count() not in (0,1):continue
  r=base();r[0x58d]=mode;r[0x6d2]=a;r[0x6d3]=b;r[0x58c]=2*(a.bit_count()+b.bit_count());r[0x6d7]=0
  source=progress(r);choices=[];courts=[]
  for _ in range(8):
   r[0x589]=129;r[4]=1
   r,_=execute(r,3,0x821a,(0x8481,),(0xfd64,))
   opponent=PRG[3*16384+0x82be-0x8000+r[0x6d7]]
   if opponent in choices:break
   choices.append(opponent)
   c=bytearray(r);c[0x58b]=opponent;c[0x589]=0
   c,_=execute(c,3,0xb29f,(0xb32d,),(0xc021,));courts.append(c[0x58b])
  maps.append(dict(progress=source,opponents=choices,courts=courts))
 rewards=[]
 for reward,opponent,period in itertools.product(range(4),range(1,8),[2,4,26,28]):
  r=base();r[0x6d4]=reward;r[0x587]=opponent;r[0x58c]=period;r[0x589]=7
  r,_=execute(r,4,0xbd8e,(0xbdd4,))
  rewards.append(dict(rewardSet=reward,opponent=opponent,postMatchPeriod=period,item=r[0xd3]))
 files={}
 for name,rows in [('password-decode',passwords),('password-encode',encodes),('password-invalid',invalid),('equipment-calls',equipment),('map-calls',maps),('reward-calls',rewards)]:
  payload=gzip.compress((json.dumps(rows,separators=(',',':'))+'\n').encode(),mtime=0);f=OUT/(name+'.json.gz');f.write_bytes(payload);files[f.name]=dict(records=len(rows),sha256=hashlib.sha256(payload).hexdigest())
 normal=[]
 for name,frames in [('campaign-audit-password-equip-commit',[1200,2800]),('campaign-audit-equipped-home-match',[4700]),('campaign-audit-equipment-remove',[2800,3600]),('campaign-audit-equipment-swap-final',[1200,2600,3700]),('campaign-audit-loss-continue',[24000,26400])]:
  path=ROOT/'.local/nes-basket/runs'/name;d=(path/'ram.bin').read_bytes();rows=[]
  for frame in frames:
   r=d[(frame-1)*2048:frame*2048];rows.append(dict(frame=frame,progress=progress(r),scene=r[0x588],phase=r[0x589],court=r[0x58b],teams=list(r[0x586:0x588])))
  normal.append(dict(run=name,ramSHA256=hashlib.sha256(d).hexdigest(),frames=rows))
 (OUT/'normal-menu.json').write_text(json.dumps(normal,indent=2)+'\n')
 meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),baseSHA256=hashlib.sha256(BASE).hexdigest(),isolated=True,entries={'decode':'bank1 BF52','encode':'bank1 BECD..BF3B','inventoryRestore':'bank3 A8C1','equip':'bank3 91DD..first942C','unequip':'bank3 931D..first942C/8088'},sourceInstructionsUnchanged=True,renderingScope='stop at next display call, never emulate a menu by arbitrary RAM defaults',outputs=files,rawSelector4DecodeAccepted=unsupported[0x589]==8,normalMenuSource='retained controller-only traces, password imports identified separately from earned wins',fullInventoryRemoval=[r for r in equipment if r['kind']=='unequip' and all(v>=0 for v in r['before']['inventory']) and r['before']['equipment'][r['character']][r['index']]>=0])
 (OUT/'source.json').write_text(json.dumps(meta,indent=2)+'\n');print(json.dumps({k:v for k,v in meta.items() if k!='fullInventoryRemoval'}))
if __name__=='__main__':main()
