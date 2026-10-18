"""Export every original full input phase with the shared world projection."""
import csv
import gzip
import hashlib
import json
from pathlib import Path
from extract_world_actions import decode_world_action_state

ROOT=Path(__file__).resolve().parents[2]


def signed(v):return v-256 if v>=128 else v
def control(r,i):return dict(mode=r[0x672+i]&127,entered=bool(r[0x672+i]&128),plan=r[0x676+i],counter=signed(r[0x66e+i]),planParameter=r[0x682+i],trackedObject=signed(r[0x67a+i]),destination=r[0x67e+i])
def decode_world_input_state(r):
    team=dict(teams=[dict(mode=r[0x663+i]&127,entered=bool(r[0x663+i]&128),counter=signed(r[0x668+i])) for i in range(2)],players=[control(r,i) for i in range(4)])
    memory=[dict(tapWindow=r[0x4f9+i],lastTap=r[0x509+i],attackTapWindow=r[0x4fd+i],lastAttackTap=r[0x505+i],buttonDelay=r[0x501+i],pendingButtons=r[0x50d+i]) for i in range(4)]
    human=[dict(evenStatus=signed(r[0x657+i]),request=r[0x653+i],status=signed(r[0x658+i]),buttonWindows=list(r[0x65b+i:0x65d+i]),timers=[signed(v) for v in r[0x65f+i:0x661+i]],messages=list(r[0x6c6+i:0x6c8+i]),overrides=list(r[0x68e+i:0x690+i])) for i in (0,2)]
    for t in range(2):
        if r[0x68a+2*t]:human[t]['accepted']=r[0x68a+2*t]
    return dict(world=decode_world_action_state(r),team=team,memory=memory,held=list(r[0x4e0:0x4e4]),pressed=list(r[0x4e4:0x4e8]),human=human)


def main():
    run=ROOT/'.local/nes-basket/runs/world-inputs-cpu-long'
    rows=list(csv.DictReader((run/'world-inputs.csv').open()));compressed=(run/'world-inputs-ram.bin.gz').read_bytes();raw=gzip.decompress(compressed)
    assert len(rows)==1998 and len(raw)==1998*4096
    records=[]
    for i,row in enumerate(rows):
        before=raw[i*4096:i*4096+2048];after=raw[i*4096+2048:(i+1)*4096]
        config=dict(courtID=before[0x58b],controls=list(before[0x539:0x53d]),characters=list(before[0x3e3:0x3e7]),teams=list(before[0x586:0x588]),runGrades=list(before[0x615:0x619]),jumpGrades=list(before[0x619:0x61d]),shotSkills=list(before[0x61d:0x621]),recoveryGrades=list(before[0x621:0x625]),equipment=before[0x6e4],otherObjectArcFlags=list(before[0x6e9:0x6f0]),enhancementEnabled=before[0x58d]<128,enhancements=[before[0x6e5+3*before[0x3e3+j]] for j in range(4)])
        frames=[]
        for actor in range(4):
            slot=before[0x4e8+actor]
            frames.append(dict(held=before[slot],pressed=before[slot+4]) if slot<4 else dict(held=0,pressed=0))
        context=dict(world=config,frames=frames,difficulty=before[0x58c],scoreDifference=before[0x696],cachedZones=list(before[0x66a:0x66e]),objectKinds=list(before[0x3d6:0x3e3]),hoopLinks=list(before[0x649:0x64f]),humanTactics=list(before[0x666:0x668]))
        records.append(dict(frame=int(row['frame']),before=decode_world_input_state(before),after=decode_world_input_state(after),calls=[int(v) for v in row['objects'].split(';')],context=context))
    payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0)
    out=ROOT/'games/basketball/engine/testdata';p=out/'world-inputs.json.gz';p.write_bytes(packed)
    p.with_name('world-inputs-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(rows),phase_boundary='fixed D32C..D345',all_records_retained=True,uncompressed_sha256=hashlib.sha256(payload).hexdigest(),sha256=hashlib.sha256(packed).hexdigest(),trace_sha256=hashlib.sha256(raw).hexdigest(),compressed_trace_sha256=hashlib.sha256(compressed).hexdigest(),source=json.loads((run/'source.json').read_text())),indent=2)+'\n')
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();bank=rom[16+5*16384:16+6*16384];read=lambda a:bank[a-0x8000];word=lambda a:read(a)+256*read(a+1)
    profiles=[[read(word(0x897e+2*t)+j) for j in range(4)] for t in range(6)]
    q=out/'human-command-profiles.json';q.write_text(json.dumps(profiles)+'\n')
    q.with_name('human-command-profiles-source.json').write_text(json.dumps(dict(source='Original bank5 $897E command message selection',rom_sha256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(q.read_bytes()).hexdigest()),indent=2)+'\n')
    print('world-inputs',len(records),'json bytes',len(payload),'gzip bytes',len(packed))


if __name__=='__main__':main()
