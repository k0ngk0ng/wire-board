"""Derive raster regions from source timing, OAM and immutable pattern data.

Screenshot comparison is verification only. No image/observed after-state is
exported as a runtime frame or used to drive a scene rule.
"""
from pathlib import Path
import csv,gzip,hashlib,json,collections
ROOT=Path(__file__).resolve().parents[2];RUN=ROOT/'.local/contra-raster/oracle/runs/raster-v1';E=ROOT/'docs/research/nes-contra/evidence'
def pixel(ppu,x,y,scroll=0,nt0=0):
 gx=x+scroll+nt0*256;tx=(gx%256)//8;ty=y//8;base=0x2000+(gx//256%2)*1024;tile=ppu[base+ty*32+tx];a=0x1000+tile*16;bit=7-gx%8
 v=(ppu[a+y%8]>>bit&1)|(ppu[a+8+y%8]>>bit&1)<<1;attr=ppu[base+960+(ty//4)*8+tx//4];palette=(attr>>((ty&2)*2+(tx&2)))&3
 return ppu[0x3f00+(palette*4+v if v else 0)],v

def sprite_hit(ppu,oam):
 y,tile,attr,x=oam
 for row in range(16):
  yy=y+1+row
  if yy>=240:break
  rr=15-row if attr&128 else row;base=(tile&1)*4096+((tile&254)+(rr//8))*16
  for col in range(8):
   xx=x+col
   if xx>=255:continue
   bit=col if attr&64 else 7-col
   v=(ppu[base+rr%8]>>bit&1)|(ppu[base+8+rr%8]>>bit&1)<<1
   if v and pixel(ppu,xx,yy)[1]:return [xx,yy]
 raise AssertionError('marker has no hit')

def main():
 from PIL import Image
 original=json.loads(gzip.decompress((E/'campaign-scene-raster-original.json.gz').read_bytes()));by=collections.defaultdict(list)
 for row in original['events']:by[row['frame']].append(row)
 intro=[];inter=[]
 for frame,es in by.items():
  select=lambda k:next(e for e in es if e['kind']==k)
  if any(e['kind']=='pc-f974'for e in es):
   h=next(e for e in es if e['kind']=='hit-observed'and e['pc']==0xf971);x=select('pc-f980');y=select('pc-f985')
   assert x['cycles']-h['cycles']==126 and y['cycles']-h['cycles']==133
   assert h['cycles']-h['lastMiss']==9 and x['oam0']==[61,3,35,80]
   intro.append({'frame':frame,'sequence':x['sequence'],'scroll':x['scroll'],'hitHookToXWrite':126,'hitHookToYWrite':133,'oam0':x['oam0']})
  if any(e['kind']=='pc-9fe1'for e in es):
   h=next(e for e in es if e['kind']=='hit-observed'and e['pc']in[0x9fd8,0x9fde]);m=select('pc-9fe5');c=select('pc-9fec')
   assert m['cycles']-h['cycles']in[11,12] and c['cycles']-m['cycles']==9
   inter.append({'frame':frame,'oam0':m['oam0'],'hitHookToMask':m['cycles']-h['cycles'],'hitHookToControl':c['cycles']-h['cycles'],'mask':m['mask']&0xef,'control':c['control']|1})
 assert len(intro)==619 and len(inter)==122
 ppu=(RUN/'frame-07300-ppu.bin').read_bytes();assert sprite_hit(ppu,[61,3,35,80])==[83,62]
 introLayouts=[]
 for stage in range(1,9):
  name='scene-natural-04'if stage==1 else f'scene-stage-{stage}-02'
  q=(ROOT/'.local/contra-campaign-scene/oracle/runs'/name/'frame-00800-ppu.bin').read_bytes()
  assert sprite_hit(q,[61,3,35,80])==[83,62]
  assert all(pixel(q,x,63,fine)[1]==0 for x in range(120,256)for fine in range(8))
  introLayouts.append({'stage':stage,'hit':[83,62],'fineTransitionRegionTransparent':True})
 # Both original cinematic layouts give the same hit geometry for every
 # valid marker Y, using the actual decoded background/CHR resources.
 groups=[]
 for stage in range(1,8):
  name=f'scene-interlude-{stage}-01';first=64 if stage<6 else 24
  p=(ROOT/'.local/contra-campaign-scene/oracle/runs'/name/'frame-00100-ppu.bin').read_bytes()
  hits=[sprite_hit(p,[y,3,32,5])for y in range(first,186)];assert hits==[[9,y+2]for y in range(first,186)]
  groups.append({'layout':name,'markerY':[first,185],'hit':[9,'markerY+2'],'fullBackgroundStart':'markerY+3','samples':len(hits)})
 rgb=(ROOT/'.local/contra-background/runs/background-v1/background-colors.rgb').read_bytes();ram=(RUN/'ram.bin').read_bytes();frames={int(r['frame']):int(r['emulator_frame'])for r in csv.DictReader((RUN/'frames.csv').open())};checks=[]
 for n in [7250,7300,7500,7800]:
  frame=frames[n];event=next(e for e in by[frame]if e['kind']=='pc-f980');scroll=event['scroll'];p=(RUN/f'frame-{n:05d}-ppu.bin').read_bytes();img=Image.open(RUN/f'frame-{n:05d}.png').convert('RGB');blocked=set()
  for index in [n-2,n-1]:
   row=ram[index*2048:(index+1)*2048]
   for at in range(0x200,0x300,4):
    y,_,_,x=row[at:at+4]
    for yy in range(y+1,min(y+17,240)):
     for xx in range(x,min(x+8,256)):blocked.add((xx,yy))
  mismatch=[];tested=0
  for y in range(240):
   for x in range(256):
    if (x,y)in blocked:continue
    color,_=pixel(p,x,y,scroll if y>=64 else 0);expected=tuple(rgb[color*3:color*3+3]);tested+=1
    if img.getpixel((x,y))!=expected:mismatch.append([x,y])
  assert not mismatch,(n,len(mismatch),mismatch[:8])
  checks.append({'capture':n,'emulatorFrame':frame,'scroll':scroll,'backgroundPixelsCompared':tested,'mismatches':len(mismatch)})
 facts={'scope':'Source instructions plus read-only original timing and normalized-pattern geometry; screenshot pixels are verification only.','introLayouts':introLayouts,'introWrites':len(intro),'interludeWrites':len(inter),'intro':{'hit':[83,62],'markerOAM':[61,3,35,80],'fineXTransitionRow':63,'fullNewScrollFromRow':64,'row63AfterWriteRegionTransparent':True,'firstWriteCyclesAfterStatusRead':128,'secondWriteCyclesAfterStatusRead':135,'pollPeriodCycles':9},'interlude':{'groups':groups,'maskWriteCyclesAfterStatusRead':[13,14],'controlWriteCyclesAfterStatusRead':[22,23],'pollReadGapCycles':[8,9],'maskTransition':'sprite-hit row, midline','fullControlRegion':'sprite-hit row+1','partialRowDotLimitation':'Exact horizontal boundary depends on polling phase; persistent Session fields do not encode a 6502 sub-scanline phase.'},'pixelChecks':checks,'nmiReset':{'source':['F8A7..F8B6','F926..F945','F8E2..F905'],'normal':'FE mask with Busy blanking; FD/FC scroll and FF control each NMI','nested':'same mask/scroll/control restoration, no new main-world work'},'rawEventsSHA256':hashlib.sha256((E/'campaign-scene-raster-original.json.gz').read_bytes()).hexdigest()}
 (E/'campaign-scene-raster-verification.json').write_text(json.dumps(facts,indent=2)+'\n');print(json.dumps({'passed':True,'introLayouts':introLayouts,'introWrites':len(intro),'interludeWrites':len(inter),'pixelChecks':checks}))
if __name__=='__main__':main()
