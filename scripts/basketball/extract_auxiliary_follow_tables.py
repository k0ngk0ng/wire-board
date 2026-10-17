"""Editable EC7B follow-offset data, derived from original bank6/fixed tables."""
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];r=(ROOT/'.local/nes-basket/basketball.nes').read_bytes()[16:16+131072];b=r[6*16384:7*16384]+r[-16384:]
def vals(a,n):return list(b[a-0x8000:a-0x8000+n])
indices=vals(0xecf5,106);assert max(indices)<16
s='package engine\n\n// Original EC7B/ECF5/ED65 data; source: scripts/basketball/extract_auxiliary_follow_tables.py.\nvar auxiliaryOwnerOffsets = [106]uint8{'+','.join(map(str,indices))+'}\nvar auxiliaryFollowOffsets = [16]struct{ X int16; Height, Depth uint8 }{\n'
for i in range(16):
 lo,hi=vals(0xed65+i*2,2);v=vals(lo+256*hi,4);x=v[0]+256*v[1];x=x-65536 if x>=32768 else x;s+=f'{{{x},{v[2]},{v[3]}}},\n'
s+='}\nvar auxiliaryRequestRejections = [16][2]uint8{\n'
for i in range(16):
 lo,hi=vals(0xe898+i*2,2);v=vals(lo+256*hi,2);s+='{'+','.join(map(str,v))+'},\n'
(ROOT/'games/basketball/engine/auxiliary_follow_tables.go').write_text(s+'}\n')
