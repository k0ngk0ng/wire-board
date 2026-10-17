"""Extract semantic AI-control oracles from the retained original RAM trace."""
import csv
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/nes-basket'

def main():
    run=BASE/'runs/ai-shot-observation'
    rows=list(csv.DictReader((run/'ai.csv').open()))
    raw=(run/'ai-ram.bin').read_bytes()
    assert len(raw)==len(rows)*4096
    assert (run/'ram.bin').read_bytes()==(BASE/'runs/shot-observation/ram.bin').read_bytes()
    def decode(ram,actor):
        def word(lo,hi):return ram[lo]+ram[hi]*256
        def wide(lo,mid,hi):return word(lo,mid)+ram[hi]*65536
        hoops=[]
        for i in range(7,13):hoops.append(dict(action=ram[0x512+i],owner=ram[0x539+i],x=wide(0x68+i,0x75+i,0x82+i),depth=word(0x8f+i,0x9c+i),height=wide(0xa9+i,0xb6+i,0xc3+i)))
        players=[dict(x=wide(0x68+i,0x75+i,0x82+i),depth=word(0x8f+i,0x9c+i),height=wide(0xa9+i,0xb6+i,0xc3+i)) for i in range(4)]
        ball=dict(x=wide(0x6e,0x7b,0x88),depth=word(0x95,0xa2),height=wide(0xaf,0xbc,0xc9))
        return dict(mode=ram[0x672+actor],nextMode=ram[0x676+actor],counter=ram[0x66e+actor],parameter=ram[0x682+actor],target=ram[0x67a+actor],destination=ram[0x67e+actor],zone=ram[0x66a+actor],action=ram[0x512+actor],request=ram[0x51f+actor],heading=ram[0x4ec+actor],facing=ram[0x46b+actor],x=wide(0x68+actor,0x75+actor,0x82+actor),depth=word(0x8f+actor,0x9c+actor),height=wide(0xa9+actor,0xb6+actor,0xc3+actor),holder=ram[0x53f],hoops=hoops,players=players,ball=ball,random=list(ram[0x48:0x4c]))
    selected=[];zones=[];possession=[];positioning=[];jump_setup=[]
    profiles=json.loads((ROOT/'games/basketball/engine/testdata/ai-pursuit-profiles.json').read_text())
    for index,row in enumerate(rows):
        actor=int(row['actor']);before=decode(raw[index*4096:index*4096+2048],actor);after=decode(raw[index*4096+2048:index*4096+4096],actor)
        zones.append(dict(actor=actor,frame=int(row['frame']),x=before['x'],depth=before['depth'],zone=before['zone']))
        if before['mode']&127 in (1,2,8,19,18):
            for state,ram in [(before,raw[index*4096:index*4096+2048]),(after,raw[index*4096+2048:index*4096+4096])]:
                team=ram[0x586+actor//2];character=ram[0x3e3+actor]
                state.update(jumpPhase=ram[0x576+actor],verticalSpeed=ram[0x4b9+actor],depthVelocity=ram[0x492+actor]+ram[0x49f+actor]*256,difficulty=ram[0x58c],team=team,teamHasHuman=(ram[0x539+(actor&2)]&ram[0x53a+(actor&2)]&128)==0,airborne=bool(ram[0x5f7+actor]&128),playerActions=[ram[0x512+j]&127 for j in range(4)],ballAction=ram[0x518],ballFacing=ram[0x471],landingX=ram[0x699]+ram[0x69a]*256,landingDepth=ram[0x698],options=next(p['options'] for p in profiles if p['team']==team and p['character']==character))
            (possession if before['mode']&127 in (1,2) else jump_setup if before['mode']&127==18 else positioning).append(dict(actor=actor,frame=int(row['frame']),before=before,after=after))
        if before['mode']&127 in (0,9,20,27,40):selected.append(dict(actor=actor,frame=int(row['frame']),before=before,after=after))
    source=dict(trace_sha256=hashlib.sha256(raw).hexdigest(),ram_matches_uninstrumented=True,source=json.loads((run/'source.json').read_text()))
    for stem,records in [('ai-control',selected),('ai-entry-zones',zones),('ai-possession-control',possession),('ai-positioning',positioning),('ai-jump-setup',jump_setup)]:
        p=ROOT/f'games/basketball/engine/testdata/{stem}.json';p.write_text(json.dumps(records,separators=(',',':'))+'\n')
        p.with_name(stem+'-source.json').write_text(json.dumps(dict(source_is_original_execution=True,records=len(records),sha256=hashlib.sha256(p.read_bytes()).hexdigest(),**source),indent=2)+'\n')
        print(stem,len(records))

if __name__=='__main__':main()
