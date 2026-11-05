#!/usr/bin/env python3
"""Export computational renderer metadata; retain no client pixel payload."""
import gzip,hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];DATA=ROOT/'games/yuyuhakusho/data'
def sha(b):return hashlib.sha256(b).hexdigest()
def read(name):
 p=DATA/(name+'.json.gz');raw=gzip.decompress(p.read_bytes());return raw,json.loads(raw)
rr,r=read('chu-world-rules');sr,s=read('world-sprites');hr,h=read('hud-assets')
frames=[]
for f in s['frames']:
 frames.append({'key':f['key'],'tile_bank':f['tile_bank'],'preloaded_only':f['preloaded_only'],'transfers':f['transfers'],'piece_count':len(f['pieces'])})
assert len(frames)==1500 and len(s['default_static_slots'])==320
payload={'schema':1,'rules_sha256':sha(rr),'sprites_sha256':sha(sr),'hud_sha256':sha(hr),'tile_count':len(s['tile_bytes'])//64,'frames':frames,'default_static_slots':s['default_static_slots'],'arena_resources':r['match_start']['arena_resources'],'hud':{'resource_list':h['resource_list'],'base_count':len(h['base_tiles']),'bar_count':len(h['bar_tiles']),'glyph_codes':[v['code']for v in h['glyphs']],'portrait_parameters':[v['parameter']for v in h['portraits']],'window_stripe':h['window_stripe']}}
raw=(json.dumps(payload,separators=(',',':'))+'\n').encode();packed=gzip.compress(raw,mtime=0)
(DATA/'renderstate-metadata.json.gz').write_bytes(packed)
manifest={'schema':1,'pass':True,'scope':'Pixel-free computational frame/culling/cache/transfer/preload counts, stable tile identities and HUD ref domains; no tile pixels, piece geometry, palettes, ROM, CPU snapshots or client runtime','bytes':len(raw),'gzip_bytes':len(packed),'sha256':sha(raw),'gzip_sha256':sha(packed),'frames':len(frames),'tile_identities':payload['tile_count'],'default_slots':len(payload['default_static_slots']),'glyph_codes':len(payload['hud']['glyph_codes']),'portraits':len(payload['hud']['portrait_parameters']),'source_sha256':{'chu-world-rules.json.gz':sha((DATA/'chu-world-rules.json.gz').read_bytes()),'world-sprites.json.gz':sha((DATA/'world-sprites.json.gz').read_bytes()),'hud-assets.json.gz':sha((DATA/'hud-assets.json.gz').read_bytes())},'exporter_sha256':sha(Path(__file__).read_bytes())}
(DATA/'renderstate-metadata-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps(manifest,indent=2))
