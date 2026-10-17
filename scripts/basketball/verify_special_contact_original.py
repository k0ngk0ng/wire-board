"""Isolated original 84C3 gameplay oracle; synthetic inputs, no live-game RAM injection."""
from pathlib import Path
import hashlib,itertools,json,sys
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'.local/basketball/python'))
from py65.devices.mpu6502 import MPU
from capture_special_contact import contact_state,contact_context
code=(ROOT/'.local/basketball/ai/bank5-visible.bin').read_bytes()
def word(r,lo,hi,v):r[lo]=v&255;r[hi]=(v>>8)&255
def make_case(index,action,player_action,variant,facing,vx,**options):
 r=bytearray(2048)
 for i in range(4):
  word(r,0x68+i,0x75+i,(160+i*24)*256+37);r[0x82+i]=0
  word(r,0x8f+i,0x9c+i,40*256+17);word(r,0xa9+i,0xb6+i,0);r[0xc3+i]=0
  word(r,0x478+i,0x485+i,123);word(r,0x492+i,0x49f+i,-77);word(r,0x4ac+i,0x4b9+i,456);word(r,0x4c6+i,0x4d3+i,31)
  r[0x512+i]=128;r[0x51f+i]=128;r[0x52c+i]=255;r[0x437+i]=9;r[0x444+i]=2;r[0x451+i]=3;r[0x4ec+i]=255;r[0x611+i]=(19+index)%256;r[0x625+i]=2
 target=index%4
 r[0x512+target]=player_action|128;r[0x57a+target]=variant;r[0x46b+target]=128 if facing else 0;r[0x5b+target]=3|(128 if facing else 0)
 r[0x539+target]=options.get('control',128);r[0x5f7+target]=128 if index%2 else 0
 word(r,0xa9+target,0xb6+target,12*256 if index%2 else 0)
 word(r,0x8f+target,0x9c+target,options.get('depth',72)*256+17)
 r[0x518]=action|128;r[0x525]=128;r[0x532]=255;r[0x457]=9;r[0x43d]=5;r[0x44a]=1;r[0x471]=128 if (index//2)%2 else 0;r[0x61]=2;r[0x5fd]=128;r[0x511]=3
 word(r,0x6e,0x7b,200*256+13);word(r,0x95,0xa2,40*256+15);word(r,0xaf,0xbc,24*256+23)
 word(r,0x47e,0x48b,vx);word(r,0x498,0x4a5,99);word(r,0x4b2,0x4bf,-331);word(r,0x4cc,0x4d9,77)
 r[0x53f]=options.get('holder',6);r[0x6a8]=2;r[0x546]=255;r[0x561]=target if options.get('excluded') else 255
 r[0x55d]=[7,8,11,12,0,255][index%6];r[0x58b]=options.get('court',1)
 r[0x53d:0x53f]=bytes([255,255]);r[0x540:0x546]=bytes(range(7,13));r[0x3c:0x43]=bytes([128]*6+[options.get('candidate',target)])
 return r
def run_case(r):
 m=MPU();m.memory[0x8000:0x8000+len(code)]=code;m.memory[:2048]=list(r);m.sp=255;m.stPushWord(0x6ffe);m.pc=0x84c3
 sounds=[]
 for n in range(2000):
  if m.pc==0x6fff:return bytes(m.memory[:2048]),sounds,n
  if m.pc==0xc000:
   sounds.append(m.a);m.pc=(m.stPopWord()+1)&65535
  else:m.step()
 raise RuntimeError('original contact did not return')
def main():
 records=[]
 cases=list(itertools.product([13,14,17,20,21,22,23],[0,0x60],[0,3,71,62],[False,True],[0x0380,-704,128,-192,0]))
 for index,args in enumerate(cases):
  b=make_case(index,*args);a,sounds,steps=run_case(b)
  records.append(dict(name='matrix-'+str(index),before=contact_state(b),after=contact_state(a),context=contact_context(b),sounds=sounds,originalInstructions=steps))
 extra=[dict(candidate=128),dict(holder=0),dict(control=192),dict(excluded=True)]+[dict(court=c,depth=d) for c in [2,5,7] for d in [71,72]]
 for j,opts in enumerate(extra):
  b=make_case(len(cases)+j,20,0,0,bool(j%2),896,**opts);a,sounds,steps=run_case(b)
  records.append(dict(name='boundary-'+str(j),before=contact_state(b),after=contact_state(a),context=contact_context(b),sounds=sounds,originalInstructions=steps))
 p=ROOT/'games/basketball/engine/testdata/special-contact-isolated.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n')
 p.with_name('special-contact-isolated-source.json').write_text(json.dumps(dict(source_is_original_execution=True,natural_gameplay=False,method='Isolated original 6502 function with explicit synthetic RAM inputs',entry='bank5:84c3',exit='RTS to isolated sentinel',intercept='C000 sound driver logs A and returns; CPU/gameplay function bytes unchanged',sound_intercept_scope='gameplay projection only; audio driver scratch memory and timing excluded',records=len(records),target_sha256=hashlib.sha256(code).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n');print('isolated original contacts',len(records))
if __name__=='__main__':main()
