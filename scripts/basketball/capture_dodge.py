"""Capture all original dodge action calls in two unchanged reference runs."""
import hashlib
import json
from capture_reference import ROOT, BASE, run
from capture_hoop_player import HOOK


def main():
    hook=HOOK.replace('return (a>=0x30 and a<=0x3c) or a==0x57','return a==0x2e')
    hook=hook.replace('passage=0x6d7','passage=0x6d7,dodgeStep=0x565,dodgeSpeed=0x548')
    records=[];sources=[]
    for name in ['hoop-control-cpu-long','ball-control-pass']:
        reference=BASE/'runs'/name
        spec=json.loads((reference/'scenario.json').read_text());spec.pop('save_slot',None)
        spec.update(name='dodge-'+name,hooks_lua=hook)
        path=BASE/(spec['name']+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');run(path)
        observed=BASE/'runs'/spec['name']
        assert (observed/'ram.bin').read_bytes()==(reference/'ram.bin').read_bytes()
        records.extend(dict(scenario=name,**json.loads(line)) for line in (observed/'hoop-player.jsonl').read_text().splitlines())
        sources.append(dict(scenario=name,**json.loads((observed/'source.json').read_text())))
    fixture=ROOT/'games/basketball/engine/testdata/dodge.json'
    fixture.write_text(json.dumps(records,separators=(',',':'))+'\n')
    fixture.with_name('dodge-source.json').write_text(json.dumps(dict(source_is_original_execution=True,entry='fixed:e165',exit='fixed:e168',records=len(records),sha256=hashlib.sha256(fixture.read_bytes()).hexdigest(),sources=sources),indent=2)+'\n')
    print(json.dumps(dict(records=len(records))))


if __name__=='__main__': main()
