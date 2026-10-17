"""Observe all action-7/11 phases with ownership, exclusion, and RNG context."""
import hashlib
import json
from capture_reference import BASE, ROOT, run
from capture_ball import HOOK


def main():
    reference=BASE/'runs/hoop-control-cpu-long'
    spec=json.loads((reference/'scenario.json').read_text())
    hook=HOOK.replace('deflection=0x564}', 'deflection=0x564,excludedCatcher=0x561,court=0x58b,random0=0x48,random1=0x49,random2=0x4a,random3=0x4b}')
    assert hook!=HOOK
    spec.update(name='loose-ball-cpu-long',hooks_lua=hook);spec.pop('save_slot',None)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    observed=BASE/'runs'/spec['name']
    assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    images=list(reference.glob('frame-*.png'))
    for image in images:assert image.read_bytes()==(observed/image.name).read_bytes()
    all_rows=[json.loads(line) for line in (observed/'ball.jsonl').read_text().splitlines()]
    rows=[r for r in all_rows if r['before']['action']&127 in (7,11)]
    p=ROOT/'games/basketball/engine/testdata/loose-ball.json';p.write_text(json.dumps(rows,separators=(',',':'))+'\n')
    summary=dict(source_is_original_execution=True,records=len(rows),all_ball_phases=len(all_rows),identical_frames=spec['frames'],identical_screenshots=len(images),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((observed/'source.json').read_text()))
    p.with_name('loose-ball-source.json').write_text(json.dumps(summary,indent=2)+'\n')
    out=ROOT/'.local/basketball/rea/dropped-ball';(out/'capture.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
