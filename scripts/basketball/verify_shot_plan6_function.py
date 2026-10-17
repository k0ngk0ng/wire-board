"""Original B4E3 isolated oracle; no original instructions are patched."""
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
    rows = []
    for actor, action, heading, parameter in itertools.product(range(4), range(256), [0,32,64,96,128,160,192,224,255], [6,134]):
        cpu.memory[:2048] = [(i * 17 + 73) & 255 for i in range(2048)]
        cpu.pc, cpu.sp, cpu.x, cpu.p = 0xb4e3, 255, actor, 0x20
        for address, value in [(0x512,action),(0x4ec,heading),(0x682,parameter),(0x672,134)]:
            cpu.memory[address+actor] = value
        cpu.stPushWord(0x6fff)
        before = list(cpu.memory[:2048])
        for step in range(10):
            if cpu.pc == 0x7000: break
            cpu.step()
        assert cpu.pc == 0x7000
        after = list(cpu.memory[:2048])
        expected = list(before)
        expected[0x512+actor], expected[0x672+actor] = 0x17, 1
        assert after == expected
        rows.append(dict(actor=actor, action=action, heading=heading, parameter=parameter, afterAction=after[0x512+actor], afterMode=after[0x672+actor]))
    payload = (json.dumps(rows,separators=(',',':'))+'\n').encode()
    packed = gzip.compress(payload,mtime=0)
    output = ROOT / 'games/basketball/engine/testdata'
    (output / 'ai-shot-plan6-function.json.gz').write_bytes(packed)
    beforepath = ROOT / '.local/basketball/team-roster-after-attacks-20261018/native/court1-teams0-2-characters0-1-0-1/before.json'
    before = beforepath.read_bytes()
    (output / 'ai-shot-plan6-runtime-before.json.gz').write_bytes(gzip.compress(before,mtime=0))
    source = dict(records=len(rows), entry='bank5 B4E3', source_is_natural_execution=False, isolated_original_function_execution=True, rom_sha256=digest, sha256=hashlib.sha256(packed).hexdigest(), runtime_before_path=str(beforepath.relative_to(ROOT)), runtime_before_sha256=hashlib.sha256(before).hexdigest())
    (output / 'ai-shot-plan6-function-source.json').write_text(json.dumps(source,indent=2)+'\n')
    print(json.dumps(source))


if __name__ == '__main__': main()
