"""Export editable original court animation tile tables; no runtime ROM access."""
from pathlib import Path
import hashlib
ROOT=Path(__file__).resolve().parents[2]
r=(ROOT/'.local/nes-basket/basketball.nes').read_bytes();assert hashlib.sha256(r).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
b=r[16+4*16384:16+5*16384]
def values(address,n):return list(b[address-0x8000:address-0x8000+n])
def pointer(address):v=values(address,2);return v[0]+256*v[1]
def fmt(v):return '{'+', '.join(fmt(x) if isinstance(x,list) else str(x) for x in v)+'}'
tables={
 'courtWaterTiles':('[4][3][14]uint8',[[values(pointer(0x9c59+row*8+frame*2),14) for row in range(3)] for frame in range(4)]),
 'courtSpringTiles':('[2][2][3][5]uint8',[[[values(pointer(0x9e0e+row*8+side*4+frame*2),5) for row in range(3)] for frame in range(2)] for side in range(2)]),
 'courtSpringAddress':('[2]uint8',values(0x9e0c,2)),
 'courtGateTiles':('[2][3][8]uint8',[[values(pointer(0x9f7c+row*4+frame*2),8) for row in range(3)] for frame in range(2)]),
 'courtGateIdleTiles':('[2][2][2]uint8',[[values(0x9fb2+row*4+frame*2,2) for row in range(2)] for frame in range(2)]),
 'courtBridgeTiles':('[2][2][8]uint8',[[values(pointer(0xa0fa+row*4+frame*2),8) for row in range(2)] for frame in range(2)]),
 'courtBridgeAddress':('[2]uint8',values(0xa122,2)),
 'courtSlotAddresses':('[32]uint8',values(0xa335,32)),
 'courtSlotCounts':('[32]uint8',values(0xa355,32)),
 'courtSlotOffsets':('[9]uint8',values(0xa2cf,9)),
 'courtSlotSymbols':('[3][5]uint8',[values(0xa3da+i*5,5) for i in range(3)]),
}
out='package engine\n\n// Original bank 4 tables. Regenerate with scripts/basketball/extract_court_animation_tables.py.\n'
for name,(kind,v) in tables.items():out+='var '+name+' = '+kind+fmt(v)+'\n'
(ROOT/'games/basketball/engine/court_animation_tables.go').write_text(out)
