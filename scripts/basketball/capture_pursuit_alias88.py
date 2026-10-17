"""Read-only alias88 search calls and F4/DA writes across retained natural timelines."""
import csv,gzip,hashlib,json
from capture_reference import ROOT,BASE,run
from capture_pursuit_search import HOOK
from extract_world_actions import decode_world_action_state


def main():
 hook=HOOK.replace('{0x55b,0xe5,0xe7}','{0xda,0xf4}');assert hook!=HOOK
 selected=[];sources=[]
 for name in ['hoop-control-cpu-long','special-ball-cpu-command','match-clock-complete']:
  reference=BASE/'runs'/name
  spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None);spec.update(name='pursuit88-'+name,hooks_lua=hook)
  path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
  observed=BASE/'runs'/spec['name'];assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
  for image in reference.glob('frame-*.png'):assert image.read_bytes()==(observed/image.name).read_bytes()
  rows=[json.loads(line) for line in (observed/'pursuit.jsonl').read_text().splitlines()];raw=(observed/'pursuit-ram.bin').read_bytes();assert len(rows)*2048<=len(raw)<=len(rows)*2048+2048
  initial=(observed/'initial-ram.bin').read_bytes()
  for i,row in enumerate(rows):
   if not any(p['index']==88 for p in row['probes']):continue
   b=raw[i*2048:(i+1)*2048]
   selected.append(dict(row,scenario=name,before=decode_world_action_state(b),reserved=dict(action73=b[0x55b],depth73=b[0xe5],depth75=b[0xe7],depth88=b[0xf4],xHigh88=b[0xda])))
  sources.append(dict(name=name,frames=spec['frames'],all_pursuit_calls=len(rows),alias88_calls=sum(any(p['index']==88 for p in r['probes']) for r in rows),initial=dict(depth88=initial[0xf4],xHigh88=initial[0xda]),writes=list(csv.DictReader((observed/'reserved-writes.csv').open())),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text())))
 payload=(json.dumps(selected,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);out=ROOT/'games/basketball/engine/testdata';(out/'pursuit-alias88.json.gz').write_bytes(packed)
 source=dict(source_is_original_execution=True,selection='all complete B100 calls that actually probe88; complete raw traces retained',records=len(selected),sha256=hashlib.sha256(packed).hexdigest(),uncompressed_sha256=hashlib.sha256(payload).hexdigest(),sources=sources)
 (out/'pursuit-alias88-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(dict(records=len(selected),sources=[{k:v for k,v in s.items() if k not in ['source','writes']}|dict(write_count=len(s['writes'])) for s in sources])))


if __name__=='__main__':main()
