"""Run unchanged bank5 8C23 with explicit diagonal/quadrant inputs.

These are configured original-function executions, not natural full games.
The ROM's 97C9 opcode is A5 69 (load player 1 X fraction), unlike the A9 69
immediate in the other equal-distance quadrants.
"""
import gzip, hashlib, itertools, json, sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / '.local/basketball/python'))
from py65.devices.mpu6502 import MPU


def main():
    rom = (ROOT / '.local/nes-basket/basketball.nes').read_bytes()
    digest = hashlib.sha256(rom).hexdigest()
    assert digest == '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    prg = rom[16:16+131072]
    cpu = MPU()
    cpu.memory[0x8000:0xc000] = prg[5*16384:6*16384]
    cpu.memory[0xc000:] = prg[-16384:]
    assert cpu.memory[0x97c9:0x97cb] == [0xa5, 0x69]
    records = []
    for distance, sx, sy, fraction, height, arc in itertools.product([0, 1, 13, 64], [-1, 1], [-1, 1], [0, 1, 64, 105, 128, 255], [0, 30, 62], [0, 24]):
        cpu.memory[:2048] = [0]*2048
        cpu.sp, cpu.pc, cpu.x, cpu.y, cpu.p = 255, 0x8c23, 6, 0, 0x20
        cpu.stPushWord(0x6fff)
        x, depth = 256, 100
        target_x, target_depth, target_height = x + sx*distance, depth + sy*distance, 88
        for value, addresses in [(x, [0x7b,0x88]), (depth,[0xa2]), (height,[0xbc,0xc9]), (target_x,[0x69f,0x6a0]), (target_depth,[0x6a1]), (target_height,[0x6a2,0x6a3]), (arc,[0x6a4]), (fraction,[0x69])]:
            for i, address in enumerate(addresses): cpu.memory[address] = value >> (i*8) & 255
        for steps in range(20000):
            if cpu.pc == 0x7000: break
            cpu.step()
        assert cpu.pc == 0x7000, hex(cpu.pc)
        def word(lo, hi):
            value = cpu.memory[lo] | cpu.memory[hi]<<8
            return value-65536 if value >= 32768 else value
        records.append(dict(position=dict(x=x*256,depth=depth*256,height=height*256),target=dict(x=target_x*256,depth=target_depth*256,height=target_height*256),arc=arc*256,secondPlayerX=400*256+fraction,velocity=dict(x=word(0x47e,0x48b),depth=word(0x498,0x4a5),height=word(0x4b2,0x4bf)),gravity=word(0x4cc,0x4d9)))
    out = ROOT/'games/basketball/engine/testdata'
    payload = json.dumps(records,separators=(',',':')).encode()
    packed = gzip.compress(payload,mtime=0)
    (out/'diagonal-launch-function.json.gz').write_bytes(packed)
    source = dict(isolated_original_function_execution=True,source_is_natural_execution=False,entry='bank5:8C23',records=len(records),rom_sha256=digest,sha256=hashlib.sha256(packed).hexdigest(),source_instruction='97C9 A5 69: LDA $0069; player seat 1 X fraction',unchanged_ROM_instructions=True)
    (out/'diagonal-launch-function-source.json').write_text(json.dumps(source,indent=2)+'\n')
    print(json.dumps(source))


if __name__ == '__main__': main()
