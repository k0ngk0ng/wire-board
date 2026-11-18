import hashlib
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
OUT = VERIFY / 'last-hidden-v1'
previous = json.loads((OUT / 'accepted-progress-734.json').read_bytes())
progress = json.loads(((OUT/'accepted-progress-735.json') if (OUT/'accepted-progress-735.json').is_file() else (VERIFY/'hidden-progress.json')).read_bytes())
assert previous['processed'] == 734 and previous['pendingEventIDs'] == [1172]
assert (progress['processed'], progress['total'], progress['actuallyClaimed'], progress['originalUnreachablePreserved'], progress['remaining']) == (735, 735, 697, 38, 0)
assert not progress['pendingEventIDs'] and progress['verifiedPoints'] == previous['verifiedPoints'] and progress['catalog'] == previous['catalog']
before = {row['event']: row for row in previous['preservedPoints']}
after = {row['event']: row for row in progress['preservedPoints']}
assert set(after) - set(before) == {1172} and all(after[key] == row for key,row in before.items())
record = after[1172]
assert record['status'] == 'original-unreachable-preserved' and record['actuallyObtained'] is False and record['scene'] == 61 and record['position'] == [768,1840]
proof = OUT / 'tomb-upper-reward.json'
assert proof.relative_to(ROOT).as_posix() in record['evidence']
report = json.loads(proof.read_bytes())
assert report['passed'] and report['event'] == 1172 and report['readyForProgress'] and not report['nativeEdits'] and not report['actuallyObtained']
assert len(report['originalEntries']) == 12 and report['bounds']['partyOffset'] == [160,112] and not report['reachableOriginalSearchStances']
assert report['illegalPaddedRoute']['firstOutOfBounds'] and report['patchedDOS']['reachableSearchStances']
assert report['independentMap']['offset'] == [128,128] and report['independentMap']['target']['pixels'] == [640,1712] and report['independentMap']['target']['redAnnotationPixels'] == 0
assert {row['event'] for row in report['independentMap']['controls']} == set(range(1168,1175)) - {1172}
assert all(row['redAnnotationPixels'] >= 300 and row['reachableOriginalSearchStances'] for row in report['independentMap']['controls'])
assert report['teleport']['exitScene'] == 65 and report['unchangedSnapshot']['event'] == 1172
for source in report['sources']:
    assert hashlib.sha256((ROOT / source['path']).read_bytes()).hexdigest() == source['sha256']
print(json.dumps({'passed': True, 'originalUnreachablePreserved': 38, 'normalPickups': 697, 'processed': 735, 'total': 735}, ensure_ascii=False))
