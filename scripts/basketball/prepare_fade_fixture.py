"""Adapt committed original fade displays to semantic renderer inputs.
All pixels remain original references. Prior-menu black frames are retained in
capture evidence but excluded from the court-only renderer contract.
"""
from pathlib import Path
import json
ROOT=Path(__file__).resolve().parents[2]
def saved_clock(slot):
 raw=(ROOT/f'.local/nes-basket/states/slot-{slot}.fcs').read_bytes();assert raw[:4]==b'FCSX' and raw[12:16]==b'\xff'*4;fields={};position=16
 while position<len(raw):
  end=position+5+int.from_bytes(raw[position+1:position+5],'little');cursor=position+5
  while cursor+8<=end:
   name=raw[cursor:cursor+4].rstrip(b'\0');size=int.from_bytes(raw[cursor+4:cursor+8],'little');fields[name]=raw[cursor+8:cursor+8+size];cursor+=8+size
  position=end
 return int.from_bytes(fields[b'TSBS'],'little')*3+int.from_bytes(fields[b'ICou'],'little',signed=True)//16,fields[b'KOOK'][0]
def prepare():
 capture=json.loads((ROOT/'.local/basketball/fade/capture.json').read_text());cases=[];excluded=[]
 fallback=json.loads((ROOT/'.local/basketball/renderer/browser/cases.json').read_text())['cases'][0]['snapshot']['hud']
 for record in capture['records']:
  directory=ROOT/'.local/nes-basket/runs'/record['run'];ram=(directory/'ram.bin').read_bytes();base,kook0=saved_clock(record['slot']);last_hud=fallback
  for item in record['frames']:
   stem=f"ppu-{item['emulator_frame']:05d}";source=json.loads((directory/(stem+'-semantic.json')).read_text());court=record['court'];palette=source['paletteID'];level=source['paletteLevel']
   if palette not in ([2] if court==1 else [5,9,10]):
    assert level==0;excluded.append({'run':record['run'],'frame':item['frame'],'paletteID':palette,'paletteLevel':level,'reason':'previous-menu palette still committed before court fade-in'});continue
   assert level in range(5) and source['paletteFlags'] in [0,128]
   ppu=(directory/(stem+'-ppu.bin')).read_bytes();state=ram[(item['frame']-1)*2048:item['frame']*2048];is_result=source['state']['phase']==6 and source['camera']=={'x':0,'y':296} and source['ballCHR']==13
   def row(y,a,b):return list(ppu[0x2000+y*32+a:0x2000+y*32+b])
   def digit(t):
    assert 1<=t<=10,(record['run'],item['frame'],t);return t-1
   if not is_result and not(source['state']['phase']==6 and level==0):
    panels=[{'lines':[row(y,a,b) for y in range(1,4)],'edgeGlyphs':[row(y,edge,edge+1)[0]for y in range(1,4)],'headerGlyphs':row(0,a,b),'turnMarker':None} for a,b,edge in [(2,11,1),(21,30,30)]]
    last_hud={'panels':panels,'clockLabel':row(1,12,16),'clock':{'minutes':digit(row(1,16,17)[0]),'seconds':digit(row(1,18,19)[0])*10+digit(row(1,19,20)[0])},'scores':[sum(digit(t)*10**(2-i)for i,t in enumerate(row(3,x,x+3)))for x in [12,17]]}
   snapshot={**source['state'],'court':court,'paletteID':palette,'paletteLevel':level,'camera':source['camera'],'reverseSprites':source['reverseSprites'],'ballCHR':source['ballCHR'],'hud':last_hud}
   if is_result:snapshot['result']={'court':court,'digits':[list(state[0x160:0x163]),list(state[0x163:0x166])],'winningTeam':1 if state[0x3c5]>=128 else 0}
   if not is_result and source['timingFrame']==item['emulator_frame']:
    prior=item['frame']-1;origin=base+prior*89342-(prior//2+(prior%2)*kook0)+(22+193)*341-16-(kook0^(prior&1));address=(source['addressCycle']*3-origin)//8;fine=(source['fineCycle']*3-origin)//8
    assert 5<=address<=6 and 8<=fine<=10,(record['label'],item['frame'],address,fine)
    snapshot['hudFetchTiming']={'addressResetFetch':address,'fineScrollResetFetch':fine}
   cases.append({'label':record['label'],'frame':item['frame'],'direction':item['direction'],'courtEvents':source['courtEvents'],'snapshot':snapshot,'reference':f'/fade-reference/{record["label"]}/{item["frame"]:05d}.png'})
 out=ROOT/'.local/basketball/renderer/browser';doc={'scope':capture['scope'],'excluded':excluded,'cases':cases};(out/'fade-cases.json').write_text(json.dumps(doc,separators=(',',':'))+'\n');print(json.dumps({'fade_cases':len(cases),'excluded_previous_menu':len(excluded)}));return doc
if __name__=='__main__':prepare()
