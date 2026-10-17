"""Original eight-court rendering with an isolated configuration selection.
Only $058B is selected at the first D1B5 court initializer. This is deliberately
not a claim that the original menu naturally selected each court. The original
loader, column streamer, palette, sprites and gameplay then execute unchanged.
Instrumented RAM and screenshots must equal the same configured reference run.
"""
from pathlib import Path
import csv,json
from capture_reference import ROOT,BASE,run
from capture_renderer import EXTRA

def main():
 out=ROOT/'.local/basketball/courts';out.mkdir(parents=True,exist_ok=True);records=[]
 for court in range(8):
  spec=json.loads((BASE/'opening-from-menu.json').read_text());spec.pop('hooks_lua',None)
  selection='''local selected=false
memory.registerexec(0xd1b5,1,function()
 if not selected and AND(memory.readbyte(0x589),128)==0 then
  memory.writebyte(0x58b,COURT);selected=true
  writefile('configured-court.json',string.format('{"court":%d,"layout":%d,"emulator_frame":%d,"scope":"isolated original court configuration"}',COURT,memory.readbyte(0x588),emu.framecount()))
 end
end)
'''.replace('COURT',str(court))
  spec.update(name=f'court-{court}-reference',frames=1750,captures=[1420,1480,1550,1600,1680,1750],hooks_lua=selection)
  spec['segments'] += [{'start':1555,'end':1620,'keys':['right']},{'start':1650,'end':1720,'keys':['left']}]
  reference=BASE/'runs'/spec['name'];path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
  if not (reference/'capture-complete.json').exists() or json.loads((reference/'scenario.json').read_text())!=spec:run(path)
  with (reference/'frames.csv').open() as f:frames={int(x['frame']):int(x['emulator_frame']) for x in csv.DictReader(f)}
  selected={frames[n] for n in spec['captures']};window={n+d for n in selected for d in (-1,0,1)}
  hook=json.loads((BASE/'runs/graphics-right/scenario.json').read_text())['hooks_lua']
  hook='local capture_window={'+','.join('[%d]=true'%n for n in window)+'}\n'+hook
  hook=hook.replace('if field_phase and memory.getregister("pc")==0xf281 then','if field_phase and capture_window[emu.framecount()] and memory.getregister("pc")==0xf281 then')
  hook=hook.replace('local rendered_order=false','local rendered_order=false\n'+EXTRA)
  hook+=r'''
local live_video_control=memory.readbyte(0x56)
memory.registerwrite(0x2000,1,function(a,z,v)live_video_control=v end)
local function capture_court_patch(base)
 local address=memory.readbyte(base)*256+memory.readbyte(base+1)
 local count=memory.readbyte(base+2);local step=AND(live_video_control,4)==0 and 1 or 32
 for i=0,count-1 do
  local a=address+i*step;local plane=AND(a,0x800)~=0 and 1 or 0;local offset=AND(a,0x3ff)
  if a>=0x2000 and a<0x3000 and offset<960 and (plane==1 or offset>=192) then
   commit_history[#commit_history+1]=string.format('{"patch":{"plane":%d,"row":%d,"column":%d,"tiles":[%d]}}',plane,math.floor(offset/32),offset%32,memory.readbyte(base+3+i))
  end
 end
end
memory.registerexec(0xcc4c,1,function()capture_court_patch(0x30b)end)
memory.registerexec(0xcc8e,1,function()capture_court_patch(0x32e)end)
memory.registerexec(0xccd0,1,function()capture_court_patch(0x351)end)
'''
  hook=hook.replace('function() order_reversed=false end)' ,'function() order_reversed=false;draw_inputs=semantic_inputs() end)')
  hook=hook.replace('rendered_order=order_reversed','rendered_order=order_reversed;rendered_inputs=draw_inputs;rendered_ball_chr=memory.readbyte(0x54)')
  hook=hook.replace('fieldx,fieldy=scrollx,scrolly','fieldx,fieldy=scrollx,scrolly;field_page=memory.readbyte(0x39a)')
  hook=hook.replace('emu.registerafter(function()', 'local selected={'+','.join('[%d]=true'%n for n in selected)+'}\nemu.registerafter(function()\n if not selected[emu.framecount()] then return end')
  hook=hook.replace('writefile(stem.."-oam.bin",oam)','writefile(stem.."-oam.bin",oam)\n writefile(stem.."-semantic.json",string.format(\'{"state":%s,"court":%d,"layout":%d,"camera":{"x":%d,"y":%d},"reverseSprites":%s,"ballCHR":%d,"courtEvents":[%s]}\',rendered_inputs,memory.readbyte(0x58b),memory.readbyte(0x588),fieldx,fieldy+256*field_page,tostring(rendered_order),rendered_ball_chr,table.concat(commit_history,",")))')
  spec.update(name=f'court-{court}-observed',hooks_lua=selection+'\n'+hook);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=BASE/'runs'/spec['name']
  if not (observed/'capture-complete.json').exists() or json.loads((observed/'scenario.json').read_text())!=spec:run(path)
  assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  for n in spec['captures']:
   assert (observed/f'frame-{n:05d}.png').read_bytes()==(reference/f'frame-{n:05d}.png').read_bytes()
   ram=(observed/'ram.bin').read_bytes()[(n-1)*2048:n*2048];assert ram[0x58b]==court and ram[0x588]==1 and ram[0x589]&127==1,(court,n,ram[0x58b],ram[0x588],ram[0x589])
  records.append({'court':court,'source':json.loads((observed/'source.json').read_text()),'frames':[{'frame':n,'emulator_frame':frames[n]} for n in spec['captures']],'ram_and_png_match_configured_reference':True})
 (out/'capture.json').write_text(json.dumps({'scope':'isolated court selection, original loader and gameplay; not natural-menu or Go match proof','records':records},indent=2)+'\n')
if __name__=='__main__':main()
