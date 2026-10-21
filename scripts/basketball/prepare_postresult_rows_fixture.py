"""Test-only original raster observations for pure Go tile-row projection.
Recorded event clocks are explicit observed-input fixtures, not a production
IRQ schedule. Expected rows have independently matched every source pixel.
"""
import argparse,gzip,hashlib,json
from pathlib import Path
from postresult_fetch import derive_frame,observed,ROOT,BASE
from prepare_postresult_raster_reference import state_fields

FRAMES=[('reward',540),('reward',660),('ending',600),('ending',1800),('ending',6000),('ending',10500),('ending',12000),('ending',13500),('reward',600),('reward',720),('ending',900),('ending',1200),('ending',3000),('ending',4800),('ending',9000),('ending',14000)]

def main():
 p=argparse.ArgumentParser();p.add_argument('--check',action='store_true');args=p.parse_args();cases=[]
 for kind,frame in FRAMES:
  expected=derive_frame(kind,frame);directory,events,frames=observed(kind);absolute=frames[frame]
  fields=state_fields(BASE/f'runs/postresult-{kind}-raster-states/before-{absolute:05d}.fcs');ram=fields['RAM'];ppu=(directory/f'ppu-{absolute:05d}-ppu.bin').read_bytes()
  source=dict(scene=ram[0x588],phase=ram[0x589],machineControl=ram[0x4d],cameraX=int.from_bytes(ram[0x397:0x399],'little'),cameraY=int.from_bytes(ram[0x399:0x39b],'little'),stageOffset=ram[0x6d9],credits=list(ram[0x6da:0x6de]),backgroundCHR=list(ram[0x4f:0x51]),spriteCHR=list(ram[0x51:0x55]),control=ram[0x56],mask=ram[0x57])
  mirror=fields['A000'][0]
  surface=dict(nametables=[list(ppu[0x2000:0x2400]),list(ppu[0x2400:0x2800] if mirror==0 else ppu[0x2800:0x2c00])],palette=list(ppu[0x3f00:0x3f20]),mirroring=mirror)
  effects=[];register=fields['CMD'][0]&7;high=None;scrollX=True
  for e in events:
   if e['frame']!=absolute or e['kind']!='deviceWrite':continue
   a,v=e['address'],e['value'];operation=None;value=v
   if a==0x8000:register=v&7
   elif a==0x8001 and register<6:operation='chr'+str(register)
   elif a==0x2000:operation='control'
   elif a==0x2001:operation='mask'
   elif a==0xa000:operation='mirroring'
   elif a==0x2006:
    if high is None:high=v
    else:operation='address';value=high*256+v;high=None;scrollX=True
   elif a==0x2005:operation='scrollX' if scrollX else 'scrollY';scrollX=not scrollX
   if operation:effects.append(dict(dot=e['cycles']*3-expected['clock']['originDot'],kind=operation,value=value))
  assert high is None
  cases.append(dict(kind=kind,frame=frame,surface=surface,plan=dict(source=source,oddFrame=bool(fields['KOOK'][0]),timingMode='source-observed',timingSource=f'controlled original {kind} video{frame}; captured writes are test-only',effects=effects),want=dict(palette=expected['palette'],rows=expected['rows'])))
 packed=gzip.compress(json.dumps(cases,separators=(',',':')).encode(),mtime=0);out=ROOT/'games/basketball/engine/testdata/postresult-raster-rows.json.gz'
 meta=dict(cases=len(cases),sourcePixels=16*61440,fixtureSHA256=hashlib.sha256(packed).hexdigest(),scope=__doc__)
 if args.check:assert out.read_bytes()==packed
 else:out.write_bytes(packed);out.with_name('postresult-raster-rows-source.json').write_text(json.dumps(meta,indent=2)+'\n')
 print(json.dumps(meta))
if __name__=='__main__':main()
