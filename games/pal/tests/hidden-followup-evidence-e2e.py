import hashlib
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
BASE = VERIFY / 'runtime-candidates/audio/development-health/hp-9999/scopes'


def read(path):
    return json.loads(path.read_bytes())


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


old = read(VERIFY/'last-hidden-v1/accepted-progress-735.json')
now = read(VERIFY/'hidden-progress.json')
catalog = read(VERIFY/'hidden-followup-candidates.json')
points = {row['Event']:row for row in catalog['candidates']}
assert old['processed']==old['total']==735 and now['processed']==now['total']==739 and now['remaining']==0
assert now['actuallyClaimed']==701 and now['originalUnreachablePreserved']==38
assert now['preservedPoints']==old['preservedPoints'] and now['catalog']==old['catalog']
old_verified = {row['event']:row for row in old['verifiedPoints']}
new_verified = {row['event']:row for row in now['verifiedPoints']}
assert all(new_verified[event]==row for event,row in old_verified.items())
assert set(new_verified)-set(old_verified)==set(points)=={85,192,202,434}
counts = []
for number,scope,prefix,required in [
    (85,'script-swap-boots-v1','script-swap-v1-85',['first-sister-handover','second-sister-handover','invitation-finished']),
    (192,'script-swap-doctor-v3','script-swap-v1-192',['entry-neighbor-search-pose','neighbor-moved','door-101-arrived']),
    (202,'followup-rewards-v1','followup-v1-202',['greeting-handover','between-visits']),
    (434,'followup-rewards-v1','followup-v1-434',['suzhou-arrived','repeat-1-handover']),
]:
    base=BASE/scope
    result = base/f'{prefix}-result-1.json'
    value = read(result)
    proof = value['proof']
    point = points[number]
    assert value['passed'] and not value['nativeEdits'] and value['resourceUploads']==0
    assert value['event']==number and value['scene']==point['Scene'] and value['itemID']==point['Grants'][0]['Item']
    assert value['amountAfter']==value['amountAfterRepeat']==value['amountBefore']+1 and value['repeatSearches']==3
    assert value['price']==point['OriginalCharge'] and value['cashBefore']-value['cashAfter']==point['OriginalCharge']
    assert proof['Battles']==0 and proof['Saves']==len(proof['Saved'])
    assert sha(base/(value['sourceCheckpoint'].removesuffix('.json')+'-1.json'))==value['sourceSHA256']
    assert result.relative_to(ROOT).as_posix() in new_verified[number]['evidence']
    for label in [*required,'unstarted','reward-handover','received','repeated','repeat-1-search-pose','repeat-2-search-pose','repeat-3-search-pose']:
        assert proof['Saved'][label]
        state=read(base/f'{prefix}-{label}-state-1.json')
        assert state['native'] and state['public']['party'][0]['controller']==0
        if label=='reward-handover':
            assert state['public']['cash']==value['cashAfter'] and state['public']['dialogue']['text']
    counts.append(proof['Saves'])
expected_parent='93055a38f3b2f0771bc4b31643479fc08bf31b49d52cdc422d7b749bfd2599f3'
assert sha(BASE/'script-swap-doctor-v2/script-swap-v1-192-door-45-arrived-state-1.json')==expected_parent
assert sha(BASE/'script-swap-doctor-v3/script-swap-v1-192-door-45-arrived-state-1.json')==expected_parent
assert sum(counts)==49
print(json.dumps({'passed':True,'processed':739,'total':739,'addedOptionalRewards':sorted(points),'freshContinuationChecks':sum(counts)},ensure_ascii=False))
