"""Additional original frames reserved for validation of the HUD split model."""
import json,re,csv
from capture_reference import ROOT,BASE,run

def main():
 records=[]
 for court in [3,4]:
  reference_spec=json.loads((BASE/f'runs/court-{court}-reference/scenario.json').read_text());reference_spec.update(name=f'court-{court}-irq-holdout-reference',captures=list(range(1560,1751,10)))
  path=BASE/(reference_spec['name']+'.json');path.write_text(json.dumps(reference_spec,indent=2)+'\n');reference=BASE/'runs'/reference_spec['name']
  if not (reference/'capture-complete.json').exists():run(path)
  assert (reference/'ram.bin').read_bytes()==(BASE/f'runs/court-{court}-reference/ram.bin').read_bytes()
  with (reference/'frames.csv').open() as f:frames={int(row['frame']):int(row['emulator_frame']) for row in csv.DictReader(f)}
  selected={frames[n] for n in reference_spec['captures']};window={n+d for n in selected for d in [-1,0,1]}
  spec=json.loads((BASE/f'runs/court-{court}-observed/scenario.json').read_text());hook=spec['hooks_lua'];hook=re.sub(r'local capture_window=\{[^}]*\}',lambda _:'local capture_window={'+','.join('[%d]=true'%n for n in sorted(window))+'}',hook);hook=re.sub(r'local selected=\{[^}]*\}',lambda _:'local selected={'+','.join('[%d]=true'%n for n in sorted(selected))+'}',hook)
  spec.update(name=f'court-{court}-irq-holdout-observed',captures=reference_spec['captures'],hooks_lua=hook);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');observed=BASE/'runs'/spec['name']
  if not (observed/'capture-complete.json').exists():run(path)
  assert (reference/'ram.bin').read_bytes()==(observed/'ram.bin').read_bytes()
  for n in spec['captures']:assert (reference/f'frame-{n:05d}.png').read_bytes()==(observed/f'frame-{n:05d}.png').read_bytes()
  records.append({'court':court,'run':spec['name'],'source':json.loads((observed/'source.json').read_text()),'frames':[{'frame':n,'emulator_frame':frames[n]} for n in spec['captures']],'ram_and_png_equal':True})
 out=ROOT/'.local/basketball/irq-raster';(out/'holdout-capture.json').write_text(json.dumps({'scope':'40 additional configured original frames; not Go whole-game proof','records':records},indent=2)+'\n')
if __name__=='__main__':main()
