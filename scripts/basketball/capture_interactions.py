"""Capture the complete original combat-then-catch response, without RAM writes."""
import csv
import hashlib
import json
from capture_reference import BASE, ROOT, run
from capture_combat import HOOK


def main():
    reference=BASE/'runs/shot-observation'
    spec=json.loads((reference/'scenario.json').read_text())
    hook=HOOK.replace('memory.registerexec(0x84c3,1,function()', 'memory.registerexec(0x801d,1,function()')
    assert hook!=HOOK
    spec.update(name='interaction-shot-observation',hooks_lua=hook);spec.pop('save_slot',None)
    path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
    observed=BASE/'runs'/spec['name']
    assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
    images=list(reference.glob('frame-*.png'))
    for image in images:assert image.read_bytes()==(observed/image.name).read_bytes()
    rows=list(csv.DictReader((observed/'combat-phases.csv').open()));raw=(observed/'combat-phase-ram.bin').read_bytes()
    assert len(rows)==1197 and len(raw)==len(rows)*4096
    summary=dict(complete_phases=len(rows),identical_frames=spec['frames'],identical_screenshots=len(images),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((observed/'source.json').read_text()))
    out=ROOT/'.local/basketball/rea/interactions';out.mkdir(parents=True,exist_ok=True)
    (out/'capture.json').write_text(json.dumps(summary,indent=2)+'\n');print(json.dumps(summary))


if __name__=='__main__':main()
