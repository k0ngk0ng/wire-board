"""Decode complete original preparation stages using the shared action decoder."""
from pathlib import Path
import csv
import gzip
import hashlib
import json
from extract_world_actions import decode_world_action_state

ROOT=Path(__file__).resolve().parents[2]


def state(r):
    signed=lambda v:v if v<128 else v-256
    team=dict(teams=[dict(mode=r[0x663+i]&127,entered=bool(r[0x663+i]&128),counter=signed(r[0x668+i])) for i in range(2)],
              players=[dict(mode=r[0x672+i]&127,entered=bool(r[0x672+i]&128),plan=r[0x676+i],counter=signed(r[0x66e+i]),trackedObject=signed(r[0x67a+i]),destination=r[0x67e+i],planParameter=r[0x682+i]) for i in range(4)])
    contacts=dict(rotation=r[0x633],marks=list(r[0x634:0x63b]),candidates=[v if v<128 else -1 for v in r[0x3c:0x43]],startingBallAction=r[0x665]&127,parity=r[0x6a7])
    return dict(actions=decode_world_action_state(r),teamAI=team,contacts=contacts,previousHolder=signed(r[0x546]),
                zones=list(r[0x66a:0x66e]),scoreDifference=r[0x696],scores=[r[0x166]+256*r[0x168],r[0x167]+256*r[0x169]],
                scoreSoundDelay=r[0xe3],previousBallAction=r[0x6cf]&127,observedBallAction=r[0x6d0]&127,
                selectedBoard=r[0x6f6],displayFlags=r[0x6d6],courtScoreFlags=list(r[0x6dd:0x6df]))


def context(r):
    actions=dict(courtID=r[0x58b],controls=list(r[0x539:0x53d]),characters=list(r[0x3e3:0x3e7]),teams=list(r[0x586:0x588]),runGrades=list(r[0x615:0x619]),jumpGrades=list(r[0x619:0x61d]),shotSkills=list(r[0x61d:0x621]),recoveryGrades=list(r[0x621:0x625]),equipment=r[0x6e4],shotEquipment=[r[0x6e4+3*r[0x3e3+i]] for i in range(4)],otherObjectArcFlags=list(r[0x6e9:0x6f0]),enhancementEnabled=r[0x58d]<128,enhancements=[r[0x6e5+3*r[0x3e3+i]] for i in range(4)])
    return dict(actions=actions,kinds=list(r[0x3d6:0x3dd]),suppressed=[v!=0 for v in r[0x3f4:0x3fb]],attackGrades=list(r[0x625:0x629]),
                humanTactics=list(r[0x666:0x668]),humanCommands=[r[0x653],r[0x655]],humanStatus=[r[a] if r[a]<128 else r[a]-256 for a in (0x658,0x65a)],
                backboardStrength=r[0x56d]>>6,scoringBonuses=[i<2 and r[0x6e6+3*r[0x3e3+i]]==27 for i in range(4)])


def main():
    source=ROOT/'.local/nes-basket/runs/world-preparation-cpu-long'
    rows=list(csv.DictReader((source/'world-preparation.csv').open()))
    raw=(source/'world-preparation-ram.bin').read_bytes()
    assert len(rows)==1997 and len(raw)==len(rows)*4096
    records=[]
    for i,row in enumerate(rows):
        b=raw[4096*i:4096*i+2048];a=raw[4096*i+2048:4096*i+4096]
        records.append(dict(frame=int(row['frame']),before=state(b),after=state(a),context=context(b),soundIDs=[int(x) for x in row['sounds'].split(';') if x]))
    output=ROOT/'games/basketball/engine/testdata/world-preparation.json.gz'
    output.write_bytes(gzip.compress((json.dumps(records,separators=(',',':'))+'\n').encode(),mtime=0))
    output.with_name('world-preparation-typed-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(rows),entry='fixed:d317',exit='fixed:d326',sha256=hashlib.sha256(output.read_bytes()).hexdigest(),trace_sha256=hashlib.sha256(raw).hexdigest(),source=json.loads((source/'source.json').read_text())),indent=2)+'\n')
    print(len(records),output.stat().st_size)


if __name__=='__main__':main()
