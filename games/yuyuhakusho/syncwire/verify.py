"""Verify exact syncwire tests and native/Wasm complete-match facts."""
from pathlib import Path
import base64,datetime,hashlib,json
ROOT=Path(__file__).resolve().parents[3]
HERE=Path(__file__).resolve().parent
DATA=HERE/'testdata'

def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def read_log(name):
 path=DATA/name;events=[json.loads(line)for line in path.read_text().splitlines()]
 assert events and not any(event['Action']=='fail'for event in events),name
 assert any(event['Action']=='pass'and not event.get('Test')for event in events),('incomplete package',name)
 passed=sorted(event['Test']for event in events if event.get('Test')and'/'not in event['Test']and event['Action']=='pass')
 assert len(passed)==8,(name,passed)
 assert not any(event['Action']=='skip'for event in events),name
 facts=[];sizes=None
 for event in events:
  line=event.get('Output','')
  if 'SYNCWIRE_MATCH_FACTS 'in line:
   value=json.loads(line.split('SYNCWIRE_MATCH_FACTS ',1)[1]);assert value['complete']and not value['reference_state_loaded'];assert value['frames']>100 and value['callbacks']>0;value['character_ids']=list(base64.b64decode(value['characters']));facts.append(value)
  if 'SYNCWIRE_SIZE_FACTS 'in line:sizes=json.loads(line.split('SYNCWIRE_SIZE_FACTS ',1)[1])
 assert len(facts)==5 and sizes
 return {'path':str(path.relative_to(ROOT)),'sha256':sha(path),'tests':passed,'complete_matches':facts,'payload_sizes':sizes}

freeze=json.loads((DATA/'test-first.json').read_text())
for name,expected in freeze['sha256'].items():assert sha(ROOT/name)==expected,('frozen source changed',name)
native=read_log('native-first.jsonl');wasm=read_log('wasm-first.jsonl')
assert native['tests']==wasm['tests']
assert native['complete_matches']==wasm['complete_matches'],'native/Wasm complete world/event/wire traces differ'
assert native['payload_sizes']==wasm['payload_sizes']
assert sorted({code for fact in native['complete_matches']for code in fact['character_ids']})==list(range(11))
assert native['payload_sizes']['dense']['window_json_bytes']<=1800
assert native['payload_sizes']['idle']['rle_bytes']==10
assert json.loads((DATA/'wasm-command.json').read_text())['exit_code']==0
assert not (DATA/'vet.txt').read_text().strip()
report={'pass':True,'verified_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Canonical12-bit bounded input encoding/history and complete independent fixed-automation rule replay; no network/graphics/audio/takeover application claim','test_first':{'record':str((DATA/'test-first.json').relative_to(ROOT)),'sha256':sha(DATA/'test-first.json'),'frozen_sources_unchanged':True,'actual_missing_red':str((DATA/'frozen-api-red.jsonl').relative_to(ROOT))},'native':native,'wasm':wasm,'native_wasm_complete_facts_equal':True,'max_frames':120,'all_eleven_characters_covered':True,'whole_world_controllers_compared_without_mask':True,'automation':'Input masks preserved exactly; complete matches use fixed authorized flags, dynamic tick application belongs to sessiontick','versioning':'Version1 only; uint64firstFrame JSON decimal string; future versions explicitly rejected','source_sha256':{str(path.relative_to(ROOT)):sha(path)for path in [HERE/'inputs.go',HERE/'window_test.go',HERE/'replay_test.go',HERE/'README.md',Path(__file__).resolve()]},'limits':['Regular packets contain only inputs/frame metadata; checkpoints and authenticated host control are separate.','Does not apply automation, predict ticks, compute or normalize world digests, render media or implement slow-peer recovery.','The complete replay oracle is another independent Go session receiving the exact same physical inputs, not original CPU execution.']}
(DATA/'validation.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print('syncwire8 tests/runtime,5 complete native/Wasm matches with full state/events/wire equality; bounded payload facts verified')
