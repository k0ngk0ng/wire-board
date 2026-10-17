"""Original EEFF for the two saved NY ball bodies which reached corner guards.

The bodies are from reconstructed-runtime failures and are explicitly injected
into a captured original NY context. This is an isolated function oracle, not
an original natural whole-match recording.
"""
import gzip, hashlib, json
from pathlib import Path
from verify_pass_floor_original import Memory, MPU, ball, put

ROOT = Path(__file__).resolve().parents[2]


def main():
    rom = (ROOT/'.local/nes-basket/basketball.nes').read_bytes()
    digest = hashlib.sha256(rom).hexdigest()
    assert digest == '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
    baseline = (ROOT/'.local/nes-basket/runs/ball-pass/ram.bin').read_bytes()[:2048]
    rows, trace = [], bytearray()
    for path in sorted((ROOT/'.local/basketball/rea/preparation-matrix').glob('court1-roster3*.json')):
        r = json.loads(path.read_text())
        b = r['state']['playing']['world']['Preparation']['Actions']['ball']
        ram = bytearray(baseline)
        body = b['body']
        for group, name, addresses in [('position','x',(0x6e,0x7b,0x88)),('position','depth',(0x95,0xa2)),('position','height',(0xaf,0xbc,0xc9)),('velocity','x',(0x47e,0x48b)),('velocity','depth',(0x498,0x4a5)),('velocity','height',(0x4b2,0x4bf))]:
            put(ram,body[group][name],*addresses)
        put(ram,body['gravity'],0x4cc,0x4d9)
        for address,value in [(0x518,b['action']|(128 if b['entered'] else 0)),(0x525,b['request']['action']|(0 if b['request']['pending'] else 128)),(0x61,b['pose']['frame']|(128 if b['pose']['mirrored'] else 0)),(0x471,128 if b['facingLeft'] else 0),(0x5fd,128 if b['airborne'] else 0),(0x648,0),(0x53f,6),(0x564,b['deflection']),(0x3dc,8),(0x6dd,0)]: ram[address]=value
        memory = Memory(rom[16:16+131072],ram)
        before = bytes(memory[:2048])
        cpu = MPU(memory=memory);cpu.sp=255;cpu.pc=0xeeff;cpu.x=6;cpu.p=0x20;cpu.stPushWord(0x6fff)
        for count in range(20000):
            if cpu.pc == 0x7000: break
            cpu.step()
        assert cpu.pc == 0x7000, hex(cpu.pc)
        after = bytes(memory[:2048]);trace.extend(before);trace.extend(after)
        rows.append(dict(name=path.stem,before=ball(before),after=ball(after),instructions=count))
    assert len(rows)==2
    out=ROOT/'games/basketball/engine/testdata'
    packed=gzip.compress(trace,mtime=0)
    (out/'prepared-corner-motion.ram.gz').write_bytes(packed)
    (out/'prepared-corner-motion.json').write_text(json.dumps(rows,indent=2)+'\n')
    source=dict(isolated_original_function_execution=True,source_is_natural_execution=False,entry='fixed:EEFF',records=len(rows),rom_sha256=digest,baseline_sha256=hashlib.sha256(baseline).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),unchanged_ROM_instructions=True,inputs='saved failing native runtime ball bodies in captured original NY context')
    (out/'prepared-corner-motion-source.json').write_text(json.dumps(source,indent=2)+'\n');print(json.dumps(source))


if __name__=='__main__':main()
