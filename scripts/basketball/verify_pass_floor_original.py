"""Execute original entered-pass action, with explicit isolated flight inputs.

This is not natural-match coverage: it preserves the original ROM instructions,
maps MMC3 PRG writes, and uses a recorded New York RAM image for court tables.
"""
import gzip, hashlib, itertools, json, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / '.local/basketball/python'))
from py65.devices.mpu6502 import MPU


class Memory(list):
    def __init__(self, prg, ram):
        super().__init__([0] * 65536)
        self.prg, self.register = prg, 0
        self[:2048] = ram
        self[0x8000:] = prg[6*16384:7*16384] + prg[-16384:]

    def __setitem__(self, address, value):
        if isinstance(address, slice):
            return super().__setitem__(address, value)
        if address == 0x8000:
            assert value & 0xc0 == 0, ('unsupported MMC3 mode', value)
            self.register = value & 7
        elif address == 0x8001:
            assert self.register in (6, 7), ('unexpected CHR write', self.register)
            start = 0x8000 + (self.register - 6) * 8192
            self[start:start+8192] = self.prg[value*8192:(value+1)*8192]
        else:
            assert address < 0x2000, ('unexpected device or ROM write', hex(address), value)
            super().__setitem__(address, value)


def ball(r):
    fields = dict(action=0x518, request=0x525, pose=0x61, facing=0x471,
                  clock=0x43d, cursor=0x44a, repeatCount=0x457, nextAction=0x532,
                  air=0x5fd, attachment=0x648, bounces=0x511, holder=0x53f,
                  hoop=0x55d, deflection=0x564, excluded=0x561)
    result = {key: r[address] for key, address in fields.items()}
    for key, addresses in dict(x=(0x6e,0x7b,0x88), depth=(0x95,0xa2),
        height=(0xaf,0xbc,0xc9), vx=(0x47e,0x48b), vy=(0x498,0x4a5),
        vz=(0x4b2,0x4bf), gravity=(0x4cc,0x4d9)).items():
        result[key] = sum(r[a] << (8*i) for i, a in enumerate(addresses))
    return result


def put(r, value, *addresses):
    for i, a in enumerate(addresses):
        r[a] = (value >> (i*8)) & 255


def main():
    rom = (ROOT / '.local/nes-basket/basketball.nes').read_bytes()
    digest = hashlib.sha256(rom).hexdigest()
    assert digest == '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    source = ROOT / '.local/nes-basket/runs/ball-pass/ram.bin'
    ram = source.read_bytes()[:2048]
    assert len(ram) == 2048
    records = []
    for height, vx, vz, facing, old_pose_flip in itertools.product([0, 1, 255, 256, 512, 5120], [-257, 0, 257], [-512, 0, 512], [0, 128], [0, 128]):
        r = bytearray(ram)
        for a, value in {0x518:131, 0x525:255, 0x61:1|old_pose_flip, 0x471:facing,
            0x43d:0, 0x44a:0, 0x457:0, 0x532:255, 0x5fd:128, 0x648:0,
            0x511:2, 0x53f:6, 0x564:0, 0x561:2, 0x3dc:8}.items(): r[a] = value
        put(r,200*256+113,0x6e,0x7b,0x88); put(r,40*256+71,0x95,0xa2)
        put(r,height,0xaf,0xbc,0xc9); put(r,vx,0x47e,0x48b)
        put(r,73,0x498,0x4a5); put(r,vz,0x4b2,0x4bf); put(r,64,0x4cc,0x4d9)
        memory = Memory(rom[16:16+131072], r)
        cpu = MPU(memory=memory); cpu.sp = 255; cpu.stPushWord(0x6fff)
        cpu.pc, cpu.x, cpu.p = 0xb5f4, 6, 0x20
        before = ball(memory)
        for steps in range(20000):
            if cpu.pc == 0x7000: break
            cpu.step()
        assert cpu.pc == 0x7000, ('did not return', hex(cpu.pc))
        records.append(dict(before=before, after=ball(memory), instructions=steps))
    out = ROOT / 'games/basketball/engine/testdata'
    payload = (json.dumps(records,separators=(',',':'))+'\n').encode()
    packed = gzip.compress(payload,mtime=0)
    (out/'pass-floor-function.json.gz').write_bytes(packed)
    metadata = dict(source_is_natural_execution=False, isolated_original_function_execution=True,
        entry='bank6 B5F4, entered pass only', records=len(records), rom_sha256=digest,
        ram_source=str(source.relative_to(ROOT)), ram_frame=1, ram_sha256=hashlib.sha256(ram).hexdigest(),
        sha256=hashlib.sha256(packed).hexdigest(), unchanged_ROM_instructions=True,
        device_model='MMC3 PRG registers 6/7 only; unexpected writes fail',
        completed_landings=sum(r['after']['action']==6 for r in records))
    (out/'pass-floor-function-source.json').write_text(json.dumps(metadata,indent=2)+'\n')
    print(json.dumps(metadata))


if __name__ == '__main__': main()
