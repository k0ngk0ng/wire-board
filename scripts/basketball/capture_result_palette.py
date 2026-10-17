"""Original Court 4 result frames retaining the live palette selection (9)."""
import json,re,csv
from capture_reference import ROOT,BASE,run

def main():
 original=BASE/'runs/court-4-palette-complete-reference';spec=json.loads((original/'scenario.json').read_text());spec.update(name='court-4-result-reference',frames=23800,captures=[23460,23500,23550,23600,23650,23700,23750,23800]);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');reference=BASE/'runs'/spec['name']
 if not (reference/'capture-complete.json').exists():run(path)
 assert (reference/'ram.bin').read_bytes()==(original/'ram.bin').read_bytes()[:spec['frames']*2048]
 with (reference/'frames.csv').open() as f:frames={int(row['frame']):int(row['emulator_frame']) for row in csv.DictReader(f)}
 selected={frames[n] for n in spec['captures']};window={n+d for n in selected for d in [-1,0,1]}
 hook=json.loads((BASE/'runs/result-art-observed/scenario.json').read_text())['hooks_lua'];hook=re.sub(r'local capture_window=\{[^}]*\}',lambda _:'local capture_window={'+','.join('[%d]=true'%n for n in sorted(window))+'}',hook);hook=re.sub(r'local selected=\{[^}]*\}',lambda _:'local selected={'+','.join('[%d]=true'%n for n in sorted(selected))+'}',hook)
 spec.update(name='court-4-result-observed',hooks_lua=spec['hooks_lua']+'\n'+hook);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=BASE/'runs'/spec['name']
 if not (observed/'capture-complete.json').exists():run(path)
 assert (reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
 for n in spec['captures']:
  assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
  ram=(observed/'ram.bin').read_bytes()[(n-1)*2048:n*2048];assert ram[0x589]==134 and ram[0x304]==9 and ram[0x58b]==4
 out=ROOT/'.local/basketball/palette-timing';(out/'result-capture.json').write_text(json.dumps({'source':json.loads((observed/'source.json').read_text()),'run':spec['name'],'frames':[{'frame':n,'emulator_frame':frames[n]} for n in spec['captures']],'court':4,'paletteID':9,'ram_and_png_equal':True,'scope':'original result after normal configured Court4 match; no injected scores/result state'},indent=2)+'\n')
if __name__=='__main__':main()
