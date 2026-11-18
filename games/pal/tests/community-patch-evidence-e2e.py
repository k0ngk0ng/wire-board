import hashlib
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SCOPE = VERIFY / 'win98-reference-v1'
NAMED = {145, 191, 1196, 2173, 2763, 3537}
ABYSS = {5071, 5072, 5073, 5074, 5077}


def read(path):
    return json.loads(path.read_bytes())


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


previous = read(SCOPE / 'previous-hidden-progress.json')
current = read(SCOPE / 'accepted-progress-733.json') if (SCOPE / 'accepted-progress-733.json').is_file() else read(VERIFY / 'hidden-progress.json')
assert (previous['processed'], previous['total'], previous['actuallyClaimed'], previous['originalUnreachablePreserved']) == (722, 735, 697, 25)
assert (current['processed'], current['total'], current['actuallyClaimed'], current['originalUnreachablePreserved']) == (733, 735, 697, 36)
assert set(previous['pendingEventIDs']) == NAMED | ABYSS | {1172, 2947}
assert current['pendingEventIDs'] == [1172, 2947] and current['remaining'] == 2
assert current['verifiedPoints'] == previous['verifiedPoints'] and current['catalog'] == previous['catalog'] and current['catalogSupplements'] == previous['catalogSupplements']
retained = {row['event']: row for row in current['preservedPoints']}
assert all(retained[row['event']] == row for row in previous['preservedPoints'])
assert set(retained) - {row['event'] for row in previous['preservedPoints']} == NAMED | ABYSS
report_path = SCOPE / 'community-patch-review.json'
review = read(report_path)
assert review['passed'] and review['readyForProgress'] and not review['originalDataEdits'] and review['scopeStartsWithOriginalWindows98']
assert review['scopeArchivesAreComparisonOnly'] and review['actuallyObtained'] == 0
reference = ROOT / review['source']['decodedPath']
assert digest(reference) == review['source']['decodedSHA256'] and digest(ROOT / review['source']['rawPath']) == review['source']['rawSHA256']
text = reference.read_text()
points = {row['Event']: row for row in read(VERIFY / 'hidden-search-candidates.json')['candidates']}
assert {row['event'] for row in review['records']} == NAMED | ABYSS
for row in review['records']:
    event = row['event']
    assert row['quote'] in text and not row['actuallyObtained'] and row['classification'] == 'original-unreachable-preserved'
    assert row['position'] == [points[event]['X'], points[event]['Y']] and row['scene'] == points[event]['Scene']
    assert row['grant'] == points[event]['Grants'][0]['Item'] and row['patchedSearchStances'] and not row['originalPreMoveDoorRoute']
    assert len(row['originalSnapshots']) >= 1
    for source in row['originalSnapshots']:
        assert digest(ROOT / source['path']) == source['sha256']
        assert source['eventRow'][:14] + source['eventRow'][15:] == row['originalEvent'][:14] + row['originalEvent'][15:]
    assert retained[event]['status'] == row['classification'] and retained[event]['actuallyObtained'] is False
    assert report_path.relative_to(ROOT).as_posix() in retained[event]['evidence']
    if event in NAMED:
        assert row['differsInPosition'] and not row['quote'].startswith('1 修正了无底深渊')
    else:
        assert row['quote'] == '1 修正了无底深渊地图C部分物品得不到的问题' and not row['originalTerrainSearchStances']
assert all(row['winSearchStances'] and row['dosSearchStances'] for row in review['abyssControls'])
assert {row['event'] for row in review['unresolved']} == {1172, 2947}
assert (VERIFY / 'hidden-entry-recheck-v1/entry-routes.json').is_file()
print(json.dumps({'passed': True, 'previousProcessed': 722, 'processed': 733, 'total': 735, 'newOriginalUnreachablePreserved': 11, 'recentNormalPickups': 0, 'remaining': current['pendingEventIDs']}, ensure_ascii=False))
