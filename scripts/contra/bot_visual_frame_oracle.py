"""Package retained original raster pixels as a provider-only test oracle.

No ROM is opened or executed. No after-state is supplied to game/session logic.
"""
import base64,csv,gzip,hashlib,json
from pathlib import Path
from PIL import Image
ROOT=Path(__file__).resolve().parents[2];RUN=ROOT/'.local/contra-raster/oracle/runs/raster-v1'
def b64(b):return base64.b64encode(b).decode()
def indexed(raw):
 out=bytearray(4096)
 for t in range(64):
  for y in range(8):
   for x in range(8):out[((t>>3)*8+y)*64+(t&7)*8+x]=(raw[t*16+y]>>(7-x)&1)|((raw[t*16+y+8]>>(7-x)&1)<<1)
 return bytes(out)
def main():
 events=json.loads(gzip.decompress((ROOT/'docs/research/nes-contra/evidence/campaign-scene-raster-original.json.gz').read_bytes()))['events'];frames={int(r['frame']):int(r['emulator_frame'])for r in csv.DictReader((RUN/'frames.csv').open())};ram=(RUN/'ram.bin').read_bytes();pages={};samples=[]
 for n in [6500,6550,6600,7250,7300,7500,7800]:
  es=[e for e in events if e['frame']==frames[n]];raster=next((e for e in es if e['kind']=='pc-9fe5'),None);split=next((e for e in es if e['kind']=='pc-f980'),None);effect=raster or split
  old=ram[(n-2)*2048:(n-1)*2048];ppu=(RUN/f'frame-{n:05d}-ppu.bin').read_bytes();mapping=list(old[0x7f0:0x7f8])
  for slot,page in enumerate(mapping):
   data=indexed(ppu[slot*1024:(slot+1)*1024]);key=str(page)
   if key in pages:assert pages[key]==b64(data)
   pages[key]=b64(data)
  png=RUN/f'frame-{n:05d}.png';expected=Image.open(png).convert('RGBA').tobytes()
  view={'Background':list(ppu[0x2000:0x2800]),'OAM':list(old[0x200:0x300]),'Patterns':mapping,'Palette':list(ppu[0x3f00:0x3f20]),'Scroll':{'X':effect['shadowX'],'Y':effect['shadowY'],'NameTable':effect['control']},'Display':{'AppliedMask':effect['mask'],'SplitActive':bool(split),'RasterActive':bool(raster),'SplitScroll':split['scroll']if split else 0,'RasterMask':raster['mask']&0xef if raster else 0,'RasterControl':raster['control']|1 if raster else 0}}
  samples.append({'Frame':n,'View':view,'ExpectedRGBA':b64(expected),'SourcePNGSHA256':hashlib.sha256(png.read_bytes()).hexdigest()})
 payload={'SourceIsOriginalExecution':True,'scope':'Isolated visible-pixel provider; recorded rendering input is not a game or Session input.','RGB':b64((ROOT/'.local/contra-background/runs/background-v1/background-colors.rgb').read_bytes()),'Pages':pages,'Samples':samples}
 out=ROOT/'games/contra/engine/testdata/bot-visual-frame-raster.json.gz';out.write_bytes(gzip.compress(json.dumps(payload,separators=(',',':')).encode(),mtime=0));print(json.dumps({'frames':len(samples),'sha256':hashlib.sha256(out.read_bytes()).hexdigest()}))
if __name__=='__main__':main()
