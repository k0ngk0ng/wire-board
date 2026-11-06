"""Validate the declared passive native VDP observation before implementation."""
from pathlib import Path
import hashlib, json
ROOT=Path(__file__).resolve().parents[2]
path=ROOT/'games/yuyuhakusho/verification/world-video/native/capture.json'
assert path.is_file(), 'Missing actual passive VDP capture'
j=json.loads(path.read_text())
assert j['pass'] and j['checked_frames']==2400
assert j['all_reference_equal'] and j['all_pcm_equal'] and j['observer_overflow']==0
assert {2071,2396}.issubset(set(j['selected_frames']))
assert j['identity']['rom_sha256']=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
assert j['identity']['core_sha256']=='e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
for row in j['artifacts']:
 p=ROOT/row['path'];assert p.is_file() and hashlib.sha256(p.read_bytes()).hexdigest()==row['sha256']
assert j['production_modified'] is False and j['global_irq_acceptance_proved'] is False
print(json.dumps({'pass':True,'frames':j['checked_frames'],'selected':len(j['selected_frames']),'events':j['events']}))
