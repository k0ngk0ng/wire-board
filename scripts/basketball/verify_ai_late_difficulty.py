"""Full unmodified A05D calls for playable campaign approach-delay bands.
No actor physics, sound stubs, reseeding between chained calls, or natural-game claim.
"""
import argparse, gzip, hashlib, itertools, json
from verify_ai_mode5_plan11 import ROOT, ROM, PRG, SOURCE, BASE, start, machine
from verify_pass_floor_original import put
from extract_world_inputs import decode_world_input_state
from capture_world_human_input import context


def setup(actor, mode, difficulty, human, dx=10, dz=0, seed=0x12345678):
    r = start(actor)
    r[0x672+actor] = mode
    r[0x58c] = difficulty
    r[0x67a+actor] = actor ^ 2
    r[0x676+actor] = 8
    r[0x48:0x4c] = seed.to_bytes(4, 'big')
    if human:
        r[0x539+(actor^1)] = 0
    # Keep the canonical recorded team/character pursuit tables.
    put(r, 200*256+19, 0x68+actor, 0x75+actor, 0x82+actor)
    put(r, (200+dx)*256+71, 0x68+(actor^2), 0x75+(actor^2), 0x82+(actor^2))
    put(r, (40+dz)*256+53, 0x8f+(actor^2), 0x9c+(actor^2))
    return r


def record(cpu, actor, name):
    before = bytes(cpu.memory[:2048])
    cpu.memory[0x8000:0xc000] = PRG[5*16384:6*16384]
    cpu.pc = 0xa05d
    cpu.x, cpu.sp, cpu.p = actor, 255, 0x20
    cpu.stPushWord(0x6fff)
    rng = 0
    for steps in range(20000):
        if cpu.pc == 0x7000:
            break
        if cpu.pc == 0xf9c3:
            rng += 1
        # No device/unknown-call substitutions: Memory rejects unsupported writes.
        cpu.step()
    else:
        raise RuntimeError(('original dispatcher did not return', name, hex(cpu.pc)))
    after = bytes(cpu.memory[:2048])
    return dict(name=name, actor=actor, before=decode_world_input_state(before),
                after=decode_world_input_state(after), context=context(before),
                rngCalls=rng, originalInstructions=steps,
                rawBeforeSHA256=hashlib.sha256(before).hexdigest(),
                rawAfterSHA256=hashlib.sha256(after).hexdigest())


def main():
    parser=argparse.ArgumentParser(); parser.add_argument('--check', action='store_true'); args=parser.parse_args()
    calls=[]; chains=[]
    for mode,actor,difficulty,human,delta,seed in itertools.product(
            [19,39],range(4),range(28),[False,True],[(10,0),(32,0),(10,15),(10,16),(10,-16),(10,-17)], [0,0x12345678]):
        cpu=machine(setup(actor,mode,difficulty,human,*delta,seed))
        row=record(cpu,actor,f'entry-{mode}-{actor}-{difficulty}-{human}-{delta}-{seed}')
        assert row['rngCalls']==1
        calls.append(row)
    for mode,actor,difficulty,human in itertools.product([19,39],range(4),range(28),[False,True]):
        cpu=machine(setup(actor,mode,difficulty,human)); rows=[]
        for n in range(16):
            rows.append(record(cpu,actor,f'chain-{mode}-{actor}-{difficulty}-{human}-{n}'))
            assert rows[-1]['rngCalls']==(1 if n==0 else 0)
            if cpu.memory[0x672+actor]&127==7:
                # Consume the source engagement itself, preserving the prior RNG/state.
                rows.append(record(cpu,actor,f'chain-engagement-{mode}-{actor}-{difficulty}-{human}'))
                break
        else: raise RuntimeError('delay never handed off')
        chains.append(dict(mode=mode,actor=actor,difficulty=difficulty,human=human,records=rows))
    payload=(json.dumps(dict(calls=calls,chains=chains),separators=(',',':'))+'\n').encode()
    data=gzip.compress(payload,mtime=0)
    out=ROOT/'games/basketball/engine/testdata'
    p=out/'ai-late-difficulty.json.gz'
    if args.check: assert p.read_bytes()==data, 'source fixture not reproducible'
    else: p.write_bytes(data)
    meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),ramSource=str(SOURCE.relative_to(ROOT)),ramSHA256=hashlib.sha256(BASE).hexdigest(),fixtureSHA256=hashlib.sha256(data).hexdigest(),uncompressedSHA256=hashlib.sha256(payload).hexdigest(),calls=len(calls),chains=len(chains),chainCalls=sum(len(c['records']) for c in chains),playableDifficulties=list(range(28)),entries=['bank5:A05D','bank5:A851','bank5:A2E9'],originalInstructionsUnmodified=True,skippedCalls=[],naturalGameplay=False,scope='complete controlled AI dispatcher calls and AI-only continuations; no actor physics or complete campaign',reusedREA='ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d')
    if not args.check: (out/'ai-late-difficulty-source.json').write_text(json.dumps(meta,indent=2)+'\n')
    print(json.dumps(meta))

if __name__=='__main__': main()
