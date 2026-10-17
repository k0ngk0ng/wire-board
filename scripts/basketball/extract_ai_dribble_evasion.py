"""Retain every mode-26 AI call and its shot-specific actor state."""
import csv
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def main():
    run = ROOT / '.local/nes-basket/runs/ai-shot-observation'
    raw = (run / 'ai-ram.bin').read_bytes()
    rows = list(csv.DictReader((run / 'ai.csv').open()))
    assert len(raw) == len(rows) * 4096

    def decode(ram, actor):
        def word(lo, hi):
            return ram[lo] + 256 * ram[hi]

        def position(i):
            return dict(x=word(0x68+i, 0x75+i)+65536*ram[0x82+i],
                        depth=word(0x8f+i, 0x9c+i),
                        height=word(0xa9+i, 0xb6+i)+65536*ram[0xc3+i])

        return dict(mode=ram[0x672+actor], nextMode=ram[0x676+actor],
                    counter=ram[0x66e+actor], parameter=ram[0x682+actor],
                    target=ram[0x67a+actor], destination=ram[0x67e+actor],
                    zone=ram[0x66a+actor], action=ram[0x512+actor],
                    request=ram[0x51f+actor], heading=ram[0x4ec+actor],
                    facing=ram[0x46b+actor], holder=ram[0x53f],
                    random=list(ram[0x48:0x4c]), **position(actor),
                    team=ram[0x586+actor//2],
                    teamHasHuman=(ram[0x539+(actor&2)] & ram[0x53a+(actor&2)] & 128) == 0,
                    airborne=bool(ram[0x5f7+actor]&128), court=ram[0x58b],
                    jumpPhase=ram[0x576+actor], verticalSpeed=ram[0x4b9+actor],
                    shotCycle=ram[0x572+actor], shotSkill=ram[0x61d+actor], difficulty=ram[0x58c])

    selected = []
    for index, row in enumerate(rows):
        actor = int(row['actor'])
        before, after = [decode(raw[index*4096+offset:index*4096+offset+2048], actor)
                         for offset in (0, 2048)]
        if before['mode'] & 127 == 26:
            selected.append(dict(actor=actor, frame=int(row['frame']), before=before, after=after))
    assert len(selected) == 5
    target = ROOT / 'games/basketball/engine/testdata/ai-dribble-evasion.json'
    target.write_text(json.dumps(selected, separators=(',', ':')) + '\n')
    target.with_name('ai-dribble-evasion-source.json').write_text(json.dumps(dict(
        source_is_original_execution=True, records=len(selected),
        trace_sha256=hashlib.sha256(raw).hexdigest(),
        sha256=hashlib.sha256(target.read_bytes()).hexdigest(),
        source=json.loads((run/'source.json').read_text())), indent=2)+'\n')
    print('ai-dribble-evasion', len(selected))


if __name__ == '__main__':
    main()
