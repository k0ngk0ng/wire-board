"""Isolated original attack calls; synthetic starts, unchanged ROM instructions."""
import argparse
import gzip
import hashlib
import itertools
import json

from verify_ai_mode5_plan11 import ROOT, ROM, PRG, SOURCE, BASE, start, machine
from verify_pass_floor_original import put
from capture_player_attack import decode


def word(address):
    offset = 6 * 16384 + address - 0x8000
    return int.from_bytes(PRG[offset:offset + 2], "little")


def byte(address):
    return PRG[6 * 16384 + address - 0x8000]


def selections(handler):
    return [(0x60 + kind, role // 4, role % 4, byte(word(0xa022 + kind * 2) + role * 2))
            for kind in range(9) for role in range(32)
            if byte(word(0xa022 + kind * 2) + role * 2 + 1) == handler]


def call(cpu, actor, name):
    cpu.memory[0x8000:0xc000] = PRG[6 * 16384:7 * 16384]
    cpu.x, cpu.pc, cpu.sp, cpu.p = actor, 0x9fa7, 255, 0x20
    cpu.stPushWord(0x6fff)
    before = bytes(cpu.memory[:2048])
    sounds = []
    for instructions in range(20000):
        if cpu.pc == 0x7000:
            after = bytes(cpu.memory[:2048])
            return dict(name=name, actor=actor, before=decode(before, actor),
                        after=decode(after, actor), sounds=sounds, instructions=instructions,
                        secondPlayerX=sum(before[a] << (8*i) for i,a in enumerate((0x69,0x76,0x83))),
                        launchTargetBefore=dict(x=(before[0x69f]+256*before[0x6a0])*256,depth=before[0x6a1]*256,height=(before[0x6a2]+256*before[0x6a3])*256),
                        launchTargetAfter=dict(x=(after[0x69f]+256*after[0x6a0])*256,depth=after[0x6a1]*256,height=(after[0x6a2]+256*after[0x6a3])*256),
                        arcHeightBefore=before[0x6a4]*256,arcHeightAfter=after[0x6a4]*256,
                        holderPosition=dict(x=sum(before[a] << (8*i) for i,a in enumerate((0x68+before[0x53f],0x75+before[0x53f],0x82+before[0x53f]))),
                                            depth=sum(before[a] << (8*i) for i,a in enumerate((0x8f+before[0x53f],0x9c+before[0x53f]))),
                                            height=sum(before[a] << (8*i) for i,a in enumerate((0xa9+before[0x53f],0xb6+before[0x53f],0xc3+before[0x53f])))))
        if cpu.pc in (0xc000, 0xfbcc):
            sounds.append(cpu.a)
            cpu.pc = (cpu.stPopWord() + 1) & 65535
        else:
            cpu.step()
    raise RuntimeError((name, "original attack did not return", hex(cpu.pc)))


def initial(actor, selected, entered, facing, motion, cursor):
    action, team, character, variant = selected
    r = start(actor)
    r[0x586 + actor // 2] = team
    r[0x3e3 + actor] = character
    r[0x512 + actor] = action | (128 if entered else 0)
    if entered:
        animation = word(0xac8f + variant * 2)
        count = 0
        while byte(animation + count * 2) < 253:
            count += 1
        # Explicit retained-variant isolation: exercise cursor-dependent
        # handlers without constructing an invalid animation cursor. This is
        # not evidence that gameplay naturally changes the chosen variant.
        if cursor >= count:
            variant = 51
    r[0x57a + actor] = variant if entered else 0
    r[0x46b + actor] = 128 if facing else 0
    r[0x5b + actor] = 3 | (0 if facing else 128)  # prior pose differs from facing
    r[0x437 + actor] = 1 if entered else 7
    r[0x444 + actor] = cursor if entered else 3
    r[0x451 + actor] = 2
    r[0x52c + actor] = action
    r[0x576 + actor] = 27
    r[0x4ec + actor] = 64 if facing else 1
    r[0x58d] = 128
    height, air, vertical = motion
    put(r, height, 0xa9 + actor, 0xb6 + actor, 0xc3 + actor)
    r[0x5f7 + actor] = 128 if air else 0
    put(r, -517 if facing else 517, 0x478 + actor, 0x485 + actor)
    put(r, 127, 0x492 + actor, 0x49f + actor)
    put(r, vertical, 0x4ac + actor, 0x4b9 + actor)
    put(r, 113, 0x4c6 + actor, 0x4d3 + actor)
    return r


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--handler", type=int, required=True)
    args = parser.parse_args()
    rows = []
    for selected in selections(args.handler):
        for actor, entered, facing, motion, cursor in itertools.product(
                range(4), (False, True), (False, True),
                ((0, False, 0), (1, True, -513), (8192, True, 513)), (0, 1, 2)):
            r = initial(actor, selected, entered, facing, motion, cursor)
            rows.append(call(machine(r), actor, f"matrix-{len(rows)}"))
        # Original repeated calls retain the preceding handler's real state.
        for actor, facing in itertools.product((0, 3), (False, True)):
            cpu = machine(initial(actor, selected, False, facing, (8192, True, 0), 0))
            for tick in range(80):
                row = call(cpu, actor, f"chain-{selected}-{actor}-{facing}-{tick}")
                rows.append(row)
                if row["after"]["action"] & 127 != selected[0]:
                    break
    if args.handler == 27:
        for actor, holder, fraction in itertools.product(range(4),range(7),(0,1,37,105,255)):
            r=initial(actor,selections(27)[0],False,False,(8192,True,0),0)
            r[0x53f]=holder;r[0x4ec+actor]=64
            put(r,200*256+17,0x68+actor,0x75+actor,0x82+actor)
            put(r,60*256+29,0x8f+actor,0x9c+actor)
            put(r,220*256+43,0x68+holder,0x75+holder,0x82+holder)
            put(r,40*256+53,0x8f+holder,0x9c+holder)
            r[0x69]=fraction
            rows.append(call(machine(r),actor,f'holder-diagonal-{actor}-{holder}-{fraction}'))
    if args.handler == 25:
        # Failures to isolate: selecting ball instead of real holder, discarded
        # actor fractions, negative-height upper-byte clamp, and self-holder alias.
        for actor, holder, height in itertools.product(range(4), range(7), (-65537,-257,-1,0,255,247*256+37,255*256+37,65536+37)):
            r = initial(actor, selections(25)[0], True, False, (8192,True,513), 2)
            r[0x53f] = holder
            r[0x437+actor] = 1
            r[0x444+actor] = 1
            r[0x57a+actor] = 60
            r[0x52c+actor] = 4
            put(r, 333*256+197, 0x68+holder, 0x75+holder, 0x82+holder)
            put(r, 61*256+91, 0x8f+holder, 0x9c+holder)
            put(r, height, 0xa9+holder, 0xb6+holder, 0xc3+holder)
            rows.append(call(machine(r), actor, f"holder-{actor}-{holder}-{height}"))
    output = ROOT / "games/basketball/engine/testdata"
    path = output / f"attack-handler-{args.handler}.json.gz"
    packed = gzip.compress((json.dumps(rows, separators=(",", ":")) + "\n").encode(), mtime=0)
    path.write_bytes(packed)
    source = dict(handler=args.handler, selections=selections(args.handler), records=len(rows),
                  isolated_original_execution=True, natural_gameplay=False,
                  unchanged_original_instructions=True, entry="bank6:9FA7 dispatcher",
                  handlerAddress=hex(word(0xa274 + args.handler * 2)),
                  soundBoundary="FBCC/C000 sinks record sound ID and return without audio-driver execution",
                  retainedVariantScope="Continuation cursors outside the selected animation use valid variant51 to isolate retained variant/cursor semantics; not natural reachability evidence",
                  reaEvidence="ev_535248b0e94c2c008702b2638bdcd014b1a5ccfad5b795c1c1af22262b189e6e",
                  romSHA256=hashlib.sha256(ROM).hexdigest(),
                  sourceRAM=str(SOURCE.relative_to(ROOT)), sourceRAMSHA256=hashlib.sha256(BASE).hexdigest(),
                  sha256=hashlib.sha256(packed).hexdigest())
    path.with_name(f"attack-handler-{args.handler}-source.json").write_text(json.dumps(source, indent=2) + "\n")
    print(json.dumps(source, indent=2))


if __name__ == "__main__":
    main()
