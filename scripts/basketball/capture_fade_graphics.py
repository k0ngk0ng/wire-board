"""Read-only original fade frames, committed palette levels and draw inputs.
Reference timelines include ordinary-input Court4 palette10 period ending.
"""
import json,re,csv
from capture_reference import ROOT,BASE,run
REFS=[('ny-open','opening-from-menu',1),('ny-match','match-clock-complete',1),('court4-match','court-4-palette-complete-reference',4),('court4-palette10','court-4-fade-pause30-reference',4)]
PALETTE=r'''
local display_palette_id=memory.readbyte(0x304);local display_palette_level=math.min(4,math.floor(memory.readbyte(0x303)/16));local display_palette_flags=memory.readbyte(0x301)
local pending_palette=nil;local palette_commit=false;local address_cycle=0;local fine_cycle=0;local timing_frame=0
memory.registerwrite(0x374,1,function(a,z,v)if v==0x3f then pending_palette={id=memory.readbyte(0x304),level=math.floor(memory.readbyte(0x303)/16),flags=memory.readbyte(0x301)}end end)
memory.registerexec(0xcd12,1,function()palette_commit=memory.readbyte(0x374)==0x3f and memory.readbyte(0x375)==0 end)
memory.registerexec(0xcd21,1,function()if palette_commit and pending_palette then display_palette_id=pending_palette.id;display_palette_level=pending_palette.level;display_palette_flags=pending_palette.flags;pending_palette=nil;palette_commit=false end end)
memory.registerwrite(0x2006,1,function(a,z,v)if memory.getregister('pc')==0xf34b then address_cycle=debugger.getcyclescount();timing_frame=emu.framecount()end end)
'''
def main():
 records=[]
 for label,name,court in REFS:
  original=BASE/'runs'/name;spec=json.loads((original/'scenario.json').read_text());raw=(original/'ram.bin').read_bytes();markers=[]
  for n in range(1,len(raw)//2048):
   old=raw[(n-1)*2048:n*2048];state=raw[n*2048:(n+1)*2048]
   if state[0x303]!=old[0x303] and state[0x588]==1 and state[0x589]&127 in [4,6,8] and state[0x304] in ([2] if court==1 else [5,9,10]) and state[0x303] in [0,16,32,48,64]:markers.append({'frame':n+1,'direction':'out' if state[0x301]&128 else 'in','phase':state[0x589]&127,'paletteID':state[0x304]})
  chosen=sorted({m['frame']+d for m in markers for d in [0,1]});spec.update(name=f'fade-{label}-reference',captures=chosen);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');reference=BASE/'runs'/spec['name']
  if not (reference/'capture-complete.json').exists():run(path)
  assert (reference/'ram.bin').read_bytes()==raw
  with (reference/'frames.csv').open() as f:frames={int(x['frame']):int(x['emulator_frame']) for x in csv.DictReader(f)}
  selected={frames[n] for n in chosen};window={n+d for n in selected for d in [-1,0,1]}
  hook=json.loads((BASE/'runs/court-4-observed/scenario.json').read_text())['hooks_lua'];hook=hook[hook.index('local capture_window='):]
  hook=re.sub(r'local capture_window=\{[^}]*\}',lambda _:'local capture_window={'+','.join('[%d]=true'%n for n in sorted(window))+'}',hook);hook=re.sub(r'local selected=\{[^}]*\}',lambda _:'local selected={'+','.join('[%d]=true'%n for n in sorted(selected))+'}',hook)
  hook=hook.replace('if pc==0xf2b7 then scrolly=v end','if pc==0xf2b7 then scrolly=v end\n if pc==0xf353 then fine_cycle=debugger.getcyclescount() end')
  hook=hook.replace('{"state":%s,"court":%d','{"paletteID":%d,"paletteLevel":%d,"paletteFlags":%d,"timingFrame":%d,"addressCycle":%d,"fineCycle":%d,"state":%s,"court":%d')
  hook=hook.replace("',rendered_inputs,memory.readbyte(0x58b)","',display_palette_id,display_palette_level,display_palette_flags,timing_frame,address_cycle,fine_cycle,rendered_inputs,memory.readbyte(0x58b)")
  spec.update(name=f'fade-v2-{label}-observed',hooks_lua=spec.get('hooks_lua','')+'\n'+PALETTE+'\n'+hook);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=BASE/'runs'/spec['name']
  if not (observed/'capture-complete.json').exists():run(path)
  assert (observed/'ram.bin').read_bytes()==raw
  for n in chosen:assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
  records.append({'label':label,'court':court,'run':spec['name'],'slot':spec['load_slot'],'frames':[{'frame':n,'emulator_frame':frames[n],'direction':next(m['direction'] for m in markers if m['frame'] in [n,n-1])} for n in chosen],'markers':markers,'source':json.loads((observed/'source.json').read_text()),'ram_and_png_equal':True})
 out=ROOT/'.local/basketball/fade';(out/'capture.json').write_text(json.dumps({'scope':'original normal-input fade frames and committed palette state; Court4 selected only at initial court setup','records':records},indent=2)+'\n');print(json.dumps({'runs':len(records),'frames':sum(len(r['frames'])for r in records)}))
if __name__=='__main__':main()
