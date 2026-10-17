"""Convert original configured-court observations to semantic renderer cases.
HUD glyph IDs are observation adapters only; no RAM/PPU/OAM enters the browser.
"""
from pathlib import Path
import json
from derive_irq_timing import derive
ROOT=Path(__file__).resolve().parents[2]
def prepare(holdout=False):
 timings=derive()['cases']
 out=ROOT/'.local/basketball/renderer/browser';capture=json.loads((ROOT/('.local/basketball/irq-raster/holdout-capture.json' if holdout else '.local/basketball/courts/capture.json')).read_text());cases=[]
 for record in capture['records']:
  court=record['court'];directory=ROOT/'.local/nes-basket/runs'/record.get('run',f'court-{court}-observed')
  for item in record['frames']:
   if holdout and item['frame'] in [1600,1680,1750]:continue
   stem=f"ppu-{item['emulator_frame']:05d}";source=json.loads((directory/(stem+'-semantic.json')).read_text());ppu=(directory/(stem+'-ppu.bin')).read_bytes()
   def row(y,a,b):return list(ppu[0x2000+y*32+a:0x2000+y*32+b])
   def digit(t):
    assert 1<=t<=10;return t-1
   panels=[]
   for start,end,edge in [(2,11,1),(21,30,30)]:
    panels.append({'lines':[row(y,start,end) for y in range(1,4)],'edgeGlyphs':[row(y,edge,edge+1)[0] for y in range(1,4)],'headerGlyphs':row(0,start,end),'turnMarker':None})
   hud={'panels':panels,'clockLabel':row(1,12,16),'clock':{'minutes':digit(row(1,16,17)[0]),'seconds':digit(row(1,18,19)[0])*10+digit(row(1,19,20)[0])},'scores':[sum(digit(t)*10**(2-i) for i,t in enumerate(row(3,x,x+3))) for x in [12,17]]}
   snapshot={**source['state'],'court':court,'paletteID':(directory/'ram.bin').read_bytes()[(item['frame']-1)*2048+0x304],'camera':source['camera'],'reverseSprites':source['reverseSprites'],'ballCHR':source['ballCHR'],'hud':hud}
   if f'{court}:{item["frame"]}' in timings:snapshot['hudFetchTiming']=timings[f'{court}:{item["frame"]}']['timing']
   cases.append({'court':court,'frame':item['frame'],'courtEvents':source['courtEvents'],'snapshot':snapshot,'reference':f'/{"irq-reference" if holdout else "court-reference"}/{court}/{item["frame"]:05d}.png'})
 (out/('irq-cases.json' if holdout else 'court-cases.json')).write_text(json.dumps({'scope':capture['scope'],'raster_boundary_source':'Two tile-fetch timing boundaries derived from original CPU clocks and saved phase, without reading reference image data; ordinary semantic camera/court/HUD supply all pixels.','cases':cases},separators=(',',':'))+'\n')
if __name__=='__main__':prepare()
