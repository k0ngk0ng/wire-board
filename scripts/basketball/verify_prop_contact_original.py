"""Run isolated original 8653 player-to-prop calls on explicit synthetic states."""
from pathlib import Path
import hashlib,itertools,json,sys
ROOT=Path(__file__).resolve().parents[2];sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
from verify_special_contact_original import make_case,word
from capture_prop_contact import state
code=(ROOT/'.local/basketball/ai/bank5-visible.bin').read_bytes()
def build(index,source,target,side,height,air,action):
 r=make_case(index,20,0,0,False,896)
 r[0x3c:0x42]=bytes([128]*6);r[0x3c+source]=target;r[0x56b]=(source+1)%4
 for i in [4,5]:
  x=48 if i==4 else 464
  value=x*256+123
  word(r,0x68+i,0x75+i,value);r[0x82+i]=(value>>16)&255
  word(r,0x8f+i,0x9c+i,70*256+39);word(r,0xa9+i,0xb6+i,0);r[0xc3+i]=0
  word(r,0x478+i,0x485+i,123);word(r,0x492+i,0x49f+i,-77);word(r,0x4ac+i,0x4b9+i,456);word(r,0x4c6+i,0x4d3+i,31)
  r[0x512+i]=128;r[0x51f+i]=128;r[0x52c+i]=255;r[0x539+i]=255;r[0x437+i]=5;r[0x444+i]=1;r[0x451+i]=3;r[0x642+i]=64;r[0x56b+i]=255
  r[0x5b+i]=2;r[0x46b+i]=128
 x=48 if target==4 else 464
 position=[x*256-1,x*256+1,(x+1)*256+13][side]
 word(r,0x68+source,0x75+source,position);r[0x82+source]=(position>>16)&255
 r[0x512+source]=0xe0;r[0x57a+source]=[1,8,41,255][index%4]
 r[0x512+target]=action|128;r[0x5f7+target]=128 if air else 0
 word(r,0xa9+target,0xb6+target,height);r[0xc3+target]=(height>>16)&255
 return r
def main():
 records=[]
 for index,args in enumerate(itertools.product(range(4),[4,5],range(3),[0,127,256],[False,True],[0,6])):
  source,target,*_=args;b=build(index,*args);m=MPU();m.memory[0x8000:0x8000+len(code)]=code;m.memory[:2048]=list(b);m.x=source;m.sp=255;m.stPushWord(0x6ffe);m.pc=0x8653;sounds=[]
  for steps in range(1000):
   if m.pc==0x6fff:break
   if m.pc==0xc000:sounds.append(m.a);m.pc=(m.stPopWord()+1)&65535
   else:m.step()
  else:raise RuntimeError('original prop response did not return')
  records.append(dict(name='isolated-'+str(index),source=source,target=target,before=state(b),after=state(bytes(m.memory[:2048])),sounds=sounds,originalInstructions=steps))
 p=ROOT/'games/basketball/engine/testdata/prop-contact-isolated.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n');p.with_name('prop-contact-isolated-source.json').write_text(json.dumps(dict(source_is_original_execution=True,natural_gameplay=False,entry='bank5:8653',method='Original machine-code function, explicit synthetic inputs; C000 sound sink only',records=len(records),target_sha256=hashlib.sha256(code).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print('isolated prop responses',len(records))
if __name__=='__main__':main()
