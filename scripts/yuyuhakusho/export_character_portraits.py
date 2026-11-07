"""Export the eleven existing original HUD portraits as one tiny client atlas."""
from pathlib import Path
import gzip, hashlib, json, struct, zlib

ROOT = Path(__file__).resolve().parents[2]
hud_path = ROOT/'games/yuyuhakusho/data/hud-assets.json.gz'
rules_path = ROOT/'games/yuyuhakusho/data/chu-world-rules.json.gz'
hud = json.loads(gzip.decompress(hud_path.read_bytes()))
start = json.loads(gzip.decompress(rules_path.read_bytes()))['match_start']
assert len(hud['portraits']) == len(start['palettes']) == 11
atlas = bytearray(352*32*3)
# Same normal-mode MD -> RGB565 -> RGBA levels as the game's renderer.
levels = [0, 33, 66, 99, 140, 173, 206, 239]
rows = []
for character in range(11):
    source = next(row for row in hud['portraits'] if row['parameter'] == (character+1)*2)
    palette = start['initial_colors'][16:22] + start['palettes'][character][0]
    colors = [(levels[(word>>1)&7],levels[(word>>5)&7],levels[(word>>9)&7]) for word in palette]
    portrait = bytearray(32*32*3)
    for y in range(32):
        for x in range(32):
            tile = source['tiles'][(x//8)*4+y//8]
            offset = (y*32+x)*3
            portrait[offset:offset+3] = bytes(colors[int(tile[(y%8)*8+x%8],16)])
    for y in range(32):
        offset = (y*352+character*32)*3
        atlas[offset:offset+96] = portrait[y*96:(y+1)*96]
    rows.append({'character_id':character,'hud_parameter':source['parameter'],'rgb_sha256':hashlib.sha256(portrait).hexdigest()})
target = ROOT/'games/yuyuhakusho/assets/character-portraits.png'
def chunk(kind, data):
    return struct.pack('>I',len(data))+kind+data+struct.pack('>I',zlib.crc32(kind+data)&0xffffffff)
scanlines = b''.join(b'\0'+atlas[y*1056:(y+1)*1056] for y in range(32))
encoded = zlib.compress(scanlines,9)
assert zlib.decompress(encoded) == scanlines
target.write_bytes(b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',352,32,8,2,0,0,0))+chunk(b'IDAT',encoded)+chunk(b'IEND',b''))
manifest = {'width':352,'height':32,'bytes':target.stat().st_size,'sha256':hashlib.sha256(target.read_bytes()).hexdigest(),'characters':rows,'sources':{str(p.relative_to(ROOT)):hashlib.sha256(p.read_bytes()).hexdigest()for p in [hud_path,rules_path]}}
(target.with_suffix('.json')).write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps({'portraits':11,'bytes':target.stat().st_size,'asset':str(target)}))
