"""Record immutable table provenance for rider geometry and request legality."""
import hashlib
import json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
rom=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()
prg=rom[16:16+131072]
mapped=prg[6*16384:7*16384]+prg[7*16384:8*16384]
read=lambda address:mapped[address-0x8000]
word=lambda address:read(address)+256*read(address+1)
request_table=word(0xe200)
allowed=[]
for request in range(106):
    table=word(request_table+2*request)
    if read(table+0x3d//8)&(128>>(0x3d%8))==0:allowed.append(request)
animation=word(word(0x86fd)+2*0x3d)
ball_group=read(0xecf5+0x3d)
ball_offset=word(0xed65+2*ball_group)
result=dict(rom_sha256=hashlib.sha256(rom).hexdigest(),
    support_heights=dict(address='fixed EFF2',values=[read(0xeff2+i) for i in range(16)]),
    rider_animation=dict(address=hex(animation),bytes=[read(animation+i) for i in range(3)]),
    accepted_requests_from_riding=dict(table=hex(request_table),values=allowed),
    held_ball_offset=dict(action=61,group=ball_group,address=hex(ball_offset),bytes=[read(ball_offset+i) for i in range(4)]))
(ROOT/'games/basketball/engine/testdata/rider-tables-source.json').write_text(json.dumps(result,indent=2)+'\n')
print(json.dumps(result))
