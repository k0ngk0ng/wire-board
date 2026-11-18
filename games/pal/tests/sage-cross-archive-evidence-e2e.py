import hashlib
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
OUT = VERIFY / 'last-hidden-v1'
BEFORE = VERIFY / 'win98-reference-v1/accepted-progress-733.json'
EVIDENCE = OUT / 'sage-cross-archive.json'
previous = json.loads(BEFORE.read_bytes())
progress = json.loads((VERIFY / 'hidden-progress.json').read_bytes())
assert previous['processed'] == 733 and previous['pendingEventIDs'] == [1172, 2947]
assert progress['processed'] >= 734 and 2947 not in progress['pendingEventIDs']
assert progress['total'] == previous['total'] == 735 and progress['verifiedPoints'] == previous['verifiedPoints']
before_rows = {row['event']: row for row in previous['preservedPoints']}
after_rows = {row['event']: row for row in progress['preservedPoints']}
assert all(after_rows[key] == row for key, row in before_rows.items())
row = after_rows[2947]
assert row['scene'] == 178 and row['map'] == 201 and row['position'] == [1264, 712] and row['actuallyObtained'] is False
assert row['status'] == 'original-unreachable-preserved' and EVIDENCE.relative_to(ROOT).as_posix() in row['evidence']
review = json.loads(EVIDENCE.read_bytes())
assert review['passed'] and review['readyForProgress'] and not review['nativeEdits']
assert review['originalRecord'] == 2947 and review['lengthOfMatchingCutscene'] == 59 and review['sameDialogueLineCount'] == 27
assert review['windows']['firstFreeFieldSceneAfterVisit'] == 175 and review['dos']['firstFreeFieldSceneAfterVisit'] == 174
assert review['windows']['ordinaryBananaEvents'] == [2932] and review['dos']['ordinaryBananaEvents'] == [3086]
assert review['windows']['cutsceneBananaEvents'] == [2947] and review['dos']['cutsceneBananaEvents'] == []
assert review['liveWindowsEvidence']['freshRuntimeSaves'] == 14 and review['liveWindowsEvidence']['directionalTimerCalls'] == 32
for source in review['sources']:
    path = ROOT / source['path']
    assert path.is_file() and hashlib.sha256(path.read_bytes()).hexdigest() == source['sha256']
print(json.dumps({'passed': True, 'event': 2947, 'processed': progress['processed'], 'total': progress['total'], 'normalPickupsAdded': 0}, ensure_ascii=False))
