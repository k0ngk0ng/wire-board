"""Observe audio IRQ / main-world ordering on the retained 300-frame scenario.

Read-only hooks; no RAM injection, PCM recording, or gameplay edits. Preserve
the original scenario and assert byte-identical per-frame RAM after capture.
"""
import hashlib
import json
from capture_reference import ROOT, BASE, run

HOOK = r'''
local log=assert(io.open(out..'/audio-work-order.jsonl','w'))
local order=0
local function event(kind)
 order=order+1
 log:write(string.format('{"order":%d,"frame":%d,"kind":"%s","work":%d,"phase":%d,"request":%d,"flags":%d,"ball_action":%d,"selected_hoop":%d}\n',order,emu.framecount(),kind,memory.readbyte(0xff),memory.readbyte(0x589),memory.getregister('a'),memory.readbyte(0x7e5),memory.readbyte(0x518),memory.readbyte(0x55d)))
end
memory.registerexec(0xf089,1,function()event('nmi')end)
memory.registerexec(0xf368,1,function()event('audio_irq_before')end)
memory.registerexec(0xf36b,1,function()event('audio_irq_after')end)
memory.registerexec(0xfbcc,1,function()event('request_before')end)
memory.registerexec(0xfbde,1,function()event('bank_return')end)
memory.registerexec(0xd345,1,function()event('world_before')end)
memory.registerexec(0xd348,1,function()event('world_after')end)
memory.registerexec(0xbdd2,1,function()
 if memory.readbyte(0xbfff)==6 then event('ball_compare') end
end)
function finish_probe()log:close()end
'''


def main():
    reference = BASE / 'runs/runtime-pause-audio'
    spec = json.loads((reference/'scenario.json').read_text())
    spec.update(name='audio-work-order-v2', hooks_lua=HOOK)
    out = BASE / 'runs' / spec['name']
    spec_path = ROOT / '.local/basketball/audio/work-byte/order-scenario.json'
    spec_path.parent.mkdir(parents=True, exist_ok=True)
    spec_path.write_text(json.dumps(spec, indent=2)+'\n')
    # A completed matching capture is reusable. Never start another process
    # merely because an earlier observer timed out.
    if not ((out/'capture-complete.json').exists() and json.loads((out/'scenario.json').read_text()) == spec):
        run(spec_path)
    assert (out/'ram.bin').read_bytes() == (reference/'ram.bin').read_bytes()
    records = [json.loads(line) for line in (out/'audio-work-order.jsonl').read_text().splitlines()]
    counts = {kind: sum(row['kind']==kind for row in records) for kind in sorted({row['kind'] for row in records})}
    assert counts['audio_irq_before'] == counts['audio_irq_after'] == 300
    assert counts['nmi'] == 300
    pending = None
    nested_nmis = []
    request_pairs = []
    request_pending = None
    for row in records:
        if row['kind'] == 'audio_irq_before':
            assert pending is None
            pending = row
        elif row['kind'] == 'audio_irq_after':
            assert pending is not None
            pending = None
        elif row['kind'] == 'nmi' and pending is not None:
            nested_nmis.append({'irq_before': pending, 'nmi': row})
        elif row['kind'] == 'request_before':
            assert request_pending is None
            request_pending = row
        elif row['kind'] == 'bank_return' and request_pending is not None:
            request_pairs.append({'before': request_pending, 'after': row})
            request_pending = None
    assert pending is None
    assert request_pending is None
    # Select the nearest preceding and following sound-update observations for
    # each main-world entry; do not invent a universal per-frame ordering.
    world = []
    for index, row in enumerate(records):
        if row['kind'] != 'world_before':
            continue
        previous = next((r for r in reversed(records[:index]) if r['kind']=='audio_irq_after'), None)
        following = next((r for r in records[index+1:] if r['kind']=='audio_irq_before'), None)
        world.append({'world': row, 'preceding_audio': previous, 'following_audio': following})
    summary = {'scope': 'natural retained slot5 input with read-only observation; not a Court2 special-ball whole-match proof',
        'same_ram_as_retained_run': True, 'reference': str(reference.relative_to(ROOT)),
        'ram_sha256': hashlib.sha256((out/'ram.bin').read_bytes()).hexdigest(),
        'trace': str((out/'audio-work-order.jsonl').relative_to(ROOT)),
        'trace_sha256': hashlib.sha256((out/'audio-work-order.jsonl').read_bytes()).hexdigest(),
        'counts': counts, 'nmi_during_audio_irq': nested_nmis,
        'request_pairs': request_pairs, 'world_entries': world,
        'ordering_note': '$FBDE is shared bank-return code; only entries paired with $FBCC are sound request completion. NMI may interrupt the audio IRQ; the main thread resumes after IRQ completion.'}
    (ROOT/'.local/basketball/audio/work-byte/order-summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps({'same_ram': True, 'counts': counts, 'nested_nmis': len(nested_nmis), 'request_pairs': len(request_pairs), 'trace': summary['trace']}))


if __name__ == '__main__':
    main()
