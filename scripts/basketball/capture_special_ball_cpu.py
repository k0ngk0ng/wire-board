"""Observe original computer shots after natural human pass/shoot commands."""
import csv,hashlib,json,sys
from capture_reference import BASE,ROOT,run
from capture_special_shot import HOOK,decode
spec=dict(name='special-ball-cpu-command',load_slot=7,frames=18000,segments=[dict(start=1,end=6,keys=['A'])]+[dict(start=i,end=i+20,keys=['B']) for i in range(40,17900,90)],captures=[18000])
path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
if '--extract' not in sys.argv:run(path)
reference=BASE/'runs'/spec['name'];spec.update(name='special-ball-cpu-command-capture',hooks_lua=HOOK);path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n')
if '--extract' not in sys.argv:run(path)
observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
raw=(observed/'special-shot-ram.bin').read_bytes();rows=list(csv.DictReader((observed/'special-shot.csv').open()));assert len(raw)==len(rows)*4096
records=[dict(scenario='special-ball-cpu-command',frame=int(v['frame']),actor=int(v['actor']),kind=v['kind'],before=decode(raw[j*4096:j*4096+2048],int(v['actor'])),after=decode(raw[j*4096+2048:(j+1)*4096],int(v['actor']))) for j,v in enumerate(rows)]
p=ROOT/'games/basketball/engine/testdata/special-ball-cpu.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n');p.with_name('special-ball-cpu-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(records),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())),indent=2)+'\n');print('special-ball-cpu',len(records))
