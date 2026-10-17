"""Read original phase-6 art and semantic draw inputs using normal match input."""
from pathlib import Path
import csv,json,hashlib
from capture_reference import ROOT,BASE,run
from capture_renderer import EXTRA

def main():
 original=BASE/'runs/match-clock-complete';spec=json.loads((original/'scenario.json').read_text())
 spec.update(name='result-art-reference',captures=[22120,22140,22160,22180,22200,22230,22260,22290,22320,22350,22380,22410,22440,22470,22500]);spec.pop('hooks_lua',None)
 path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
 existing=BASE/'runs'/spec['name']
 if not (existing/'capture-complete.json').exists() or json.loads((existing/'scenario.json').read_text())!=spec:run(path)
 reference=BASE/'runs'/spec['name'];assert (reference/'ram.bin').read_bytes()==(original/'ram.bin').read_bytes();assert (reference/'frame-22500.png').read_bytes()==(original/'frame-22500.png').read_bytes()
 with (reference/'frames.csv').open() as f:frames={int(x['frame']):int(x['emulator_frame']) for x in csv.DictReader(f)}
 selected={frames[n] for n in spec['captures']}
 hook=json.loads((BASE/'runs/graphics-right/scenario.json').read_text())['hooks_lua']
 window={n+delta for n in selected for delta in (-1,0,1)}
 hook='local capture_window={'+','.join('[%d]=true'%n for n in window)+'}\n'+hook
 hook=hook.replace('if field_phase and memory.getregister("pc")==0xf281 then','if field_phase and capture_window[emu.framecount()] and memory.getregister("pc")==0xf281 then')
 hook=hook.replace('local rendered_order=false','local rendered_order=false\n'+EXTRA+'\nlocal rendered_ball_chr=memory.readbyte(0x54);local field_page=1')
 hook=hook.replace('function() order_reversed=false end)','function() order_reversed=false;draw_inputs=semantic_inputs() end)')
 hook=hook.replace('rendered_order=order_reversed','rendered_order=order_reversed;rendered_inputs=draw_inputs;rendered_ball_chr=memory.readbyte(0x54)')
 hook=hook.replace('fieldx,fieldy=scrollx,scrolly','fieldx,fieldy=scrollx,scrolly;field_page=memory.readbyte(0x39a)')
 hook=hook.replace('emu.registerafter(function()', 'local selected={'+','.join('[%d]=true'%n for n in selected)+'}\nemu.registerafter(function()\n if not selected[emu.framecount()] then return end')
 hook=hook.replace('writefile(stem.."-oam.bin",oam)','writefile(stem.."-oam.bin",oam)\n writefile(stem.."-semantic.json",string.format(\'{"state":%s,"camera":{"x":%d,"y":%d},"reverseSprites":%s,"ballCHR":%d,"result":{"court":%d,"digits":[[%d,%d,%d],[%d,%d,%d]],"winningTeam":%d},"courtCommits":[%s]}\',rendered_inputs,fieldx,fieldy+256*field_page,tostring(rendered_order),rendered_ball_chr,memory.readbyte(0x58b),memory.readbyte(0x160),memory.readbyte(0x161),memory.readbyte(0x162),memory.readbyte(0x163),memory.readbyte(0x164),memory.readbyte(0x165),memory.readbyte(0x3c5)>=128 and 1 or 0,table.concat(commit_history,",")))')
 spec.update(name='result-art-observed',hooks_lua=hook);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
 observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
 for im in reference.glob('frame-*.png'):assert im.read_bytes()==(observed/im.name).read_bytes()
 out=ROOT/'.local/basketball/result-art';out.mkdir(parents=True,exist_ok=True);manifest={'source':json.loads((observed/'source.json').read_text()),'frames':[{'frame':n,'emulator_frame':frames[n]} for n in spec['captures']],'ram_and_png_match_uninstrumented':True,'scope':'original phase6 result rendering; not Go whole-game proof'};(out/'capture.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps({'frames':len(selected),'matched':True}))
if __name__=='__main__':main()
