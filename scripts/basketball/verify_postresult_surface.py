"""Original complete CB0E upload processing with declared PPU memory mapping.
The CPU executes queue ordering/increment selection; the adapter models two
physical nametables and standard palette aliases, not a video/CPU runtime.
"""
import argparse,gzip,hashlib,json
from verify_pass_floor_original import ROOT,Memory,MPU
ROM=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();PRG=ROM[16:131088]
class Surface(Memory):
 def __init__(self,ram,mirror):
  super().__init__(PRG,ram);self.mirror=mirror;self.tables=[[((bank*97+i*11)&255) for i in range(1024)] for bank in range(2)];self.palette=[(i*3)&63 for i in range(32)]
  for i in [0,4,8,12]:self.palette[i+16]=self.palette[i]
  self.address=0;self.high=True;self.increment=1;self.ppuwrites=[]
 def __getitem__(self,a):
  if a==0x2002:self.high=True;return 0
  return super().__getitem__(a)
 def __setitem__(self,a,v):
  if a==0x2000:self.increment=32 if v&4 else 1;return
  if a==0x2006:
   self.address=((v&63)<<8)|(self.address&255) if self.high else (self.address&0xff00)|v;self.high=not self.high;return
  if a==0x2007:
   address=self.address&0x3fff;self.ppuwrites.append([address,v]);self.address=(self.address+self.increment)&0x3fff
   if address>=0x3f00:
    index=address&31
    if index>=16 and index%4==0:index-=16
    self.palette[index]=v&63
    if index<16 and index%4==0:self.palette[index+16]=v&63
   elif address>=0x2000:
    offset=(address-0x2000)%4096;table=offset//1024;bank=table&1 if self.mirror==0 else table>>1
    self.tables[bank][offset%1024]=v
   # Pattern window is immutable CHR ROM; source writes do not change it.
   return
  super().__setitem__(a,v)
 def surface(self):return dict(nametables=[x.copy() for x in self.tables],palette=self.palette.copy(),mirroring=self.mirror)

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args();rows=[]
 headers=[0x30b,0x32e,0x351,0x374]
 for mirror in [0,1]:
  for slot,header in enumerate(headers):
   for address in [0x2000,0x23ff,0x27f0,0x2fe0,0x3f00,0x3f10,0x3f1d,0x3ff8,0x7f00]:
    for length in [0,1,3,16,32]:
     for control in [0,4]:
      ram=bytearray(2048);ram[0x100]=128;ram[0x56]=136
      for h in headers:ram[h]=255
      ram[header]=address>>8;ram[header+1]=address&255;ram[header+2]=length;ram[0x307+slot]=control
      data=[(slot*17+i*11+address)&255 for i in range(32)];ram[header+3:header+35]=bytes(data)
      m=Surface(ram,mirror);before=m.surface();cpu=MPU(memory=m,pc=0xcb0e);cpu.sp=255;cpu.p=32;cpu.stPushWord(0x6fff)
      for n in range(10000):
       if cpu.pc==0x7000:break
       cpu.step()
      else:raise RuntimeError('queue call did not return')
      assert m[header]==255 and m[0x307+slot]==0
      rows.append(dict(before=before,address=address,data=data[:length],control=control,after=m.surface(),writes=m.ppuwrites,slot=slot))
 payload=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);out=ROOT/'games/basketball/engine/testdata/postresult-surface.json.gz'
 meta=dict(cases=len(rows),romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(payload).hexdigest(),skippedCalls=[],scope=__doc__)
 if args.check:assert out.read_bytes()==payload
 else:out.write_bytes(payload);out.with_name('postresult-surface-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
