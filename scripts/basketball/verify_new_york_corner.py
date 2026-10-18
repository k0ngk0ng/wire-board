"""Reproduce controlled original carrier corner calls; no Go output is used."""
import gzip
import hashlib
import itertools
import json
from verify_pass_floor_original import ROOT, MPU, put
from verify_ball18 import ROM, PRG, SoundMemory
from extract_world_actions import decode_world_action_state
from extract_world_preparation import context


def main():
    fixture = ROOT / 'games/basketball/engine/testdata'
    metadata = json.loads((fixture / 'new-york-corner-source.json').read_text())
    base = gzip.decompress((fixture / 'new-york-corner-origin.ram.gz').read_bytes())
    assert hashlib.sha256(ROM).hexdigest() == metadata['romSHA256']
    assert hashlib.sha256(base).hexdigest() == metadata['originRAMSHA256']
    rows = []
    # E165 calls the complete action, including heading-based velocity updates.
    # Input velocity variants do not claim distinct final trajectories.
    for x, depth, vx, vz in itertools.product(
        [135, 136, 140, 388, 391, 392], [72, 73, 74, 79],
        [-1408, -128, 128, 1408], [-512, -256, 0, 256, 512]
    ):
        ram = bytearray(base)
        put(ram, x * 256 + 128, 0x6a, 0x77, 0x84)
        put(ram, depth * 256 + 64, 0x91, 0x9e)
        put(ram, vx, 0x47a, 0x487)
        put(ram, vz, 0x494, 0x4a1)
        memory = SoundMemory(PRG, ram)
        cpu = MPU(memory=memory, pc=0xe165)
        cpu.x, cpu.sp, cpu.p = 2, 255, 0x20
        for _ in range(30000):
            if cpu.pc == 0xe168:
                break
            cpu.step()
        else:
            raise RuntimeError(f'original action failed to return: {cpu.pc:04x}')
        rows.append(dict(x=x, depth=depth, vx=vx, vz=vz, actor=2,
                         before=decode_world_action_state(ram),
                         after=decode_world_action_state(bytes(memory[:2048])),
                         context=context(ram)['actions']))
    raw = (json.dumps(rows, separators=(',', ':')) + '\n').encode()
    # gzip framing differs across Python versions; the uncompressed identity is
    # the reproducibility contract, and the committed gzip has its own digest.
    expected = (fixture / 'new-york-corner.json.gz').read_bytes()
    assert hashlib.sha256(expected).hexdigest() == metadata['fixtureSHA256']
    assert raw == gzip.decompress(expected)
    assert hashlib.sha256(raw).hexdigest() == metadata['sourceSHA256']
    print(json.dumps(dict(calls=len(rows), matched=True,
                          sourceSHA256=metadata['sourceSHA256'])))


if __name__ == '__main__':
    main()
