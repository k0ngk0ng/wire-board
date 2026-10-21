#!/usr/bin/env python3
"""Verify immutable recovered mount assets; failure cases: .local/bubble-mounts/PLAN.md."""
import hashlib,json,struct
from pathlib import Path
root=Path(__file__).resolve().parents[2]
assets=root/'games/bubbleparty/assets'
c=json.loads((assets/'mounts-v4.json').read_text())
m=json.loads((assets/'manifest-mounts-v4.json').read_text())
assert c['version']=='bubbleparty/v4/mounts'
assert len(c['mounts'])==8 and len(c['riders'])==10
assert [x['pixelsPerTick'] for x in c['mounts']]==[10,5,2,9,2,2,9,2]
assert [x['sourceItem'] for x in c['mounts']][2:4]==['Item/ItemTurtle0','Item/ItemTurtle1']
entries={e['file']:e for e in m['entries']}
for e in entries.values():
 b=(assets/'public/bubbleparty/v4/mounts'/e['file']).read_bytes()
 assert hashlib.sha256(b).hexdigest()==e['sha256']
 assert b[:8]==b'\x89PNG\r\n\x1a\n'
 assert struct.unpack_from('>II',b,16)==(e['width'],e['height'])
for obj in c['mounts']+c['riders']:
 assert set(obj['clips'])=={'up','down','left','right'}
 for clip in obj['clips'].values():
  for variant in ['red','blue'] if 'red' in clip else ['file']:
   e=entries[clip[variant]]
   assert all(0<=f['x']<f['x']+f['w']<=e['width'] and 0<=f['y']<f['y']+f['h']<=e['height'] for f in clip['frames'])
  assert all(0<=i<len(clip['frames']) and w>0 for i,w in clip['sequence'])
print(json.dumps({'passed':True,'mounts':len(c['mounts']),'riders':len(c['riders']),'files':len(entries),'bytes':sum(e['bytes'] for e in entries.values())}))
