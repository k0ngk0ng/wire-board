"""Retain all complete original attack-response and catch phases."""
import csv
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]


def main():
    run=ROOT/'.local/nes-basket/runs/interaction-shot-observation'
    rows=list(csv.DictReader((run/'combat-phases.csv').open()));raw=(run/'combat-phase-ram.bin').read_bytes()
    assert len(rows)==1197 and len(raw)==len(rows)*4096
    def state(r):
        def word(a,b):return r[a]+256*r[b]
        def body(i):
            fields=dict(action=0x512,request=0x51f,pose=0x5b,facing=0x46b,clock=0x437,cursor=0x444,repeatCount=0x451,nextAction=0x52c,air=0x5f7,attachment=0x642,heading=0x4ec,passage=0x6d7)
            value={name:r[base+i] for name,base in fields.items()}
            value.update(x=word(0x68+i,0x75+i)+65536*r[0x82+i],depth=word(0x8f+i,0x9c+i),height=word(0xa9+i,0xb6+i)+65536*r[0xc3+i],vx=word(0x478+i,0x485+i),vy=word(0x492+i,0x49f+i),vz=word(0x4ac+i,0x4b9+i),gravity=word(0x4c6+i,0x4d3+i))
            return value
        players=[dict(body(i),variant=r[0x57a+i],attackGrade=r[0x625+i],resistance=r[0x611+i]) for i in range(4)]
        ball=dict(body(6),bounces=r[0x511],holder=r[0x53f],hoop=r[0x55d],deflection=r[0x564])
        return dict(players=players,ball=ball,candidates=list(r[0x3c:0x43]),court=r[0x58b],holder=r[0x53f],lastTouch=r[0x6a8],previousHolder=r[0x546],excludedCatcher=r[0x561],controls=list(r[0x539:0x53d]),attachments=list(r[0x642:0x646]),auxiliaryOwners=list(r[0x53d:0x53f]),hoopOwners=list(r[0x540:0x546]),random=list(r[0x48:0x4c]),ai=list(r[0x66a:0x686]))
    records=[dict(frame=int(row['frame']),before=state(raw[i*4096:i*4096+2048]),after=state(raw[i*4096+2048:(i+1)*4096]),sounds=[int(x) for x in row['sounds'].split(';') if x]) for i,row in enumerate(rows)]
    out=ROOT/'games/basketball/engine/testdata';p=out/'interactions.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n')
    p.with_name('interactions-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(rows),trace_sha256=hashlib.sha256(raw).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((run/'source.json').read_text())),indent=2)+'\n')
    rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();b=rom[16+5*16384:16+6*16384]
    p=out/'player-catch-flags.json';p.write_text(json.dumps(list(b[0x1321:0x1321+96]))+'\n')
    p.with_name('player-catch-flags-source.json').write_text(json.dumps(dict(source='Original bank 5 $9321, actions 0..95; catcher checks bit 16',rom_sha256=hashlib.sha256(rom).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest()),indent=2)+'\n')
    print('interactions',len(records))


if __name__=='__main__':main()
