"""Project every complete original 13-object action stage; never trim failures."""
import csv
import gzip
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]


def decode_world_action_state(r):
    def word(a,b):return r[a]+256*r[b]
    def signed(value,bits):return value-(1<<bits) if value&(1<<(bits-1)) else value
    def byte(v):return signed(v,8)
    def body(i):
        return dict(position=dict(x=signed(word(0x68+i,0x75+i)+65536*r[0x82+i],24),depth=word(0x8f+i,0x9c+i),height=signed(word(0xa9+i,0xb6+i)+65536*r[0xc3+i],24)),velocity=dict(x=signed(word(0x478+i,0x485+i),16),depth=signed(word(0x492+i,0x49f+i),16),height=signed(word(0x4ac+i,0x4b9+i),16)),gravity=signed(word(0x4c6+i,0x4d3+i),16))
    def common(i):
        return dict(action=r[0x512+i]&127,entered=bool(r[0x512+i]&128),pose=dict(frame=r[0x5b+i]&127,mirrored=bool(r[0x5b+i]&128)),facingLeft=bool(r[0x46b+i]&128),animation=dict(cursor=r[0x444+i],elapsed=r[0x437+i],loops=r[0x451+i],nextAction=r[0x52c+i] if r[0x52c+i]<128 else 0,hasNextAction=r[0x52c+i]<128),airborne=bool(r[0x5f7+i]&128),request=dict(action=r[0x51f+i]&127,pending=r[0x51f+i]<128))
    def player(i):return dict(common(i),body=body(i),heading=r[0x4ec+i]//32 if r[0x4ec+i]!=255 else 8,newYorkPassage=r[0x6d7+i])
    actors=[dict(player=player(i),shotCycle=byte(r[0x572+i]),aimWindow=r[0x54c+i],jumpPhase=byte(r[0x576+i]),dodge=dict(step=r[0x565+i],initialXHigh=byte(r[0x548+i])),endurance=r[0x611+i],linkedActor=byte(r[0x550+i]),specialShot=r[0x554+i],attackVariant=r[0x57a+i]) for i in range(4)]
    auxiliary=[dict(common(i),body=body(i),heading=r[0x4ec+i]//32 if r[0x4ec+i]!=255 else 8,owner=byte(r[0x539+i]),attachment=r[0x642+i],newYorkPassage=r[0x6d7+i],objectLink=byte(r[0x56b+i])) for i in (4,5)]
    ball=dict(common(6),body=body(6),bouncesRemaining=r[0x511],deflection=r[0x564])
    hoops=[]
    for i in range(7,13):
        link=r[0x539+i];h=dict(common(i),**body(i),owner=link,scoreLock=r[0x626+i],durability=byte(r[0x577+i]))
        hoops.append(h)
    return dict(actors=actors,auxiliary=auxiliary,ball=ball,hoops=hoops,playerLinks=list(r[0x642:0x646]),ballLink=r[0x648],poseSource=r[0x56a],holder=byte(r[0x53f]),excludedCatcher=byte(r[0x561]),lastTouch=byte(r[0x6a8]),specialHolder=byte(r[0x56b]),random=int.from_bytes(r[0x48:0x4c],'big'),mountFlags=list(r[0x6f0:0x6f6]),detachedCount=r[0xe4],contactLatch=r[0x571],selectedHoop=byte(r[0x55d]),launchTarget=dict(x=word(0x69f,0x6a0)*256,depth=r[0x6a1]*256,height=word(0x6a2,0x6a3)*256),launchArcHeight=r[0x6a4]*256,ballLanding=dict(x=word(0x699,0x69a)*256,depth=r[0x698]*256,height=0),shotOriginX=word(0x55e,0x55f)*256,lastShooter=byte(r[0x6d1]),dunkTargetObject=r[0x56e],auxiliaryRelease=bool(r[0x547]),carriedPlayer=byte(r[0x569]), **(dict(unownedSpecialShot=r[0x55a]) if r[0x55a] else {}), **(dict(captureObject=r[0x56c]) if r[0x56c] else {}))


def main():
    run=ROOT/'.local/nes-basket/runs/world-actions-cpu-long'
    rows=list(csv.DictReader((run/'world-actions.csv').open()));raw=(run/'world-actions-ram.bin').read_bytes()
    assert len(rows)==1998 and len(raw)==len(rows)*4096
    schedules=json.loads((ROOT/'games/basketball/engine/testdata/action-schedule.json').read_text())
    by_frame={r['frame']:r for r in schedules}
    records=[]
    for i,row in enumerate(rows):
        b=raw[i*4096:i*4096+2048];a=raw[i*4096+2048:(i+1)*4096];frame=int(row['frame'])
        records.append(dict(frame=frame,before=decode_world_action_state(b),after=decode_world_action_state(a),calls=[c['actor'] for c in by_frame[frame]['calls']],context=dict(courtID=b[0x58b],controls=list(b[0x539:0x53d]),characters=list(b[0x3e3:0x3e7]),teams=list(b[0x586:0x588]),runGrades=list(b[0x615:0x619]),jumpGrades=list(b[0x619:0x61d]),shotSkills=list(b[0x61d:0x621]),recoveryGrades=list(b[0x621:0x625]),equipment=b[0x6e4],cachedZones=list(b[0x66a:0x66e]),shotEquipment=[b[0x6e4+3*b[0x3e3+i]] for i in range(4)],ballShotInputButtons=b[0x4e6],otherObjectArcFlags=list(b[0x6e9:0x6f0]),enhancementEnabled=b[0x58d]<128,enhancements=[b[0x6e5+3*b[0x3e3+i]] for i in range(4)])))
    out=ROOT/'games/basketball/engine/testdata';p=out/'world-actions.json.gz';payload=(json.dumps(records,separators=(',',':'))+'\n').encode();packed=gzip.compress(payload,mtime=0);p.write_bytes(packed)
    assert gzip.decompress(packed)==payload
    (out/'world-actions.json').unlink(missing_ok=True)
    p.with_name('world-actions-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(records),phase_boundary='fixed D345..D348',all_records_retained=True,uncompressed_sha256=hashlib.sha256(payload).hexdigest(),trace_sha256=hashlib.sha256(raw).hexdigest(),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),source=json.loads((run/'source.json').read_text())),indent=2)+'\n')
    print('world-actions',len(records),'bytes',p.stat().st_size)


if __name__=='__main__':main()
