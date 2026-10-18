"""Verify committed campaign handoff fields against retained original menu RAM."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def main():
    fixture = ROOT / 'games/basketball/engine/testdata/campaign-home-entry.json'
    expected = json.loads(fixture.read_text())
    raw = (ROOT / '.local/nes-basket/runs' / expected['run'] / 'ram.bin').read_bytes()
    assert hashlib.sha256(raw).hexdigest() == expected['sourceSHA256']
    a = raw[(expected['entryFrame'] - 1) * 2048:expected['entryFrame'] * 2048]
    b = raw[(expected['playingFrame'] - 1) * 2048:expected['playingFrame'] * 2048]
    assert len(a) == len(b) == 2048
    assert a[0x588] == b[0x588] == 1 and b[0x589] == 129
    signed = lambda v: v if v < 128 else v - 256
    actual = dict(
        config=dict(Court=a[0x58b], Teams=list(a[0x586:0x588]),
                    Characters=list(a[0x3e3:0x3e7]),
                    ControllerAssignments=[signed(v) for v in a[0x4e8:0x4ec]],
                    Mode=a[0x58d],
                    Equipment=[[signed(v) for v in a[0x6e4+3*i:0x6e7+3*i]] for i in range(4)],
                    Seed=int.from_bytes(a[0x48:0x4c], 'big')),
        humanTactics=list(a[0x666:0x668]), period=a[0x58c],
        winBits=list(a[0x6d2:0x6d4]),
        grades=[{key: b[address+i] for key, address in zip(
            ['Run', 'Jump', 'Shot', 'Recovery', 'Attack'],
            [0x615, 0x619, 0x61d, 0x621, 0x625])} for i in range(4)],
        controls=list(b[0x539:0x53d]),
        entryRAMSHA256=hashlib.sha256(a).hexdigest(),
        playingRAMSHA256=hashlib.sha256(b).hexdigest())
    for key, value in actual.items():
        assert value == expected[key], (key, value, expected[key])
    print(json.dumps(dict(matched=True, sourceSHA256=expected['sourceSHA256'],
                          fixtureSHA256=hashlib.sha256(fixture.read_bytes()).hexdigest())))


if __name__ == '__main__':
    main()
