import hashlib
import json
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
SSS = (ROOT / '.local/pal/original-98/sss.mkf').read_bytes()
SSS_HASH = hashlib.sha256(SSS).hexdigest()
assert SSS_HASH == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
OFFSETS = struct.unpack_from('<6I', SSS)
WORDS = (ROOT / '.local/pal/original-98/word.dat').read_bytes()
direct_path = VERIFY / 'hidden-search-candidates.json'
direct_bytes = direct_path.read_bytes()
direct = json.loads(direct_bytes)
assert direct['passed'] and direct['sssSHA256'] == SSS_HASH and len(direct['candidates']) == 732
known = {row['Event'] for row in direct['candidates']}


def script(pc):
    return struct.unpack_from('<4H', SSS, OFFSETS[4]+pc*8)


def word(item):
    return WORDS[item*10:item*10+10].rstrip(b' \x00').decode('gbk')


rows = []
for event, scene, source_item, check_pc, swap_pc, entry, grant_pc, magic_pc in [(2166, 119, 287, 39722, 39723, 19879, 19906, 0), (2333, 138, 271, 39718, 39719, 16538, 16609, 16622)]:
    original = struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(event-1)*32)
    place = struct.unpack_from('<4H', SSS, OFFSETS[1]+(scene-1)*8)
    following = struct.unpack_from('<4H', SSS, OFFSETS[1]+scene*8)
    item = struct.unpack_from('<7H', SSS, OFFSETS[2]+source_item*14)
    assert place[3] < event <= following[3] and event not in known and 0 < original[7] < 4
    assert item[2] <= check_pc and item[6] & 16
    assert script(check_pc)[0:3] == (0x81, event, 1) and script(swap_pc) == (0x25, event, entry, 0)
    assert script(swap_pc+1) == (0x20, source_item, 0, 0)
    assert script(entry) == (0x40, 65535, 2, 0)
    grant = script(grant_pc)
    assert grant[0] == 0x1f and grant[1] > 0 and grant[2] == 0 and grant[3] == 0
    assert all(script(pc)[0] > 3 for pc in range(entry, grant_pc+1))
    if source_item == 271:
        assert script(item[2]) == (0x81, 1783, 1, check_pc)
    else:
        assert item[2] == check_pc
    row = {'Scene': scene, 'Map': place[0], 'Event': event, 'X': original[1], 'Y': original[2], 'Sprite': original[8], 'InitialState': original[6], 'TriggerMode': original[7], 'Script': entry, 'OriginalTriggerScript': original[4], 'Classification': 'conditional-item-use-quest', 'Grants': [{'PC': grant_pc, 'Item': grant[1], 'Amount': 1, 'Cash': 0, 'Name': word(grant[1])}], 'LearnedSpells': [], 'SourceItem': source_item, 'SourceItemName': word(source_item), 'UseScript': item[2], 'FacingCheckPC': check_pc, 'TriggerSwapPC': swap_pc, 'ConsumptionPC': swap_pc+1}
    if magic_pc:
        magic = script(magic_pc)
        assert magic == (0x55, 384, 1, 0)
        row['LearnedSpells'].append({'PC': magic_pc, 'Magic': magic[1], 'RoleOperand': magic[2], 'Name': word(magic[1])})
    rows.append(row)
assert direct_path.read_bytes() == direct_bytes
report = {'passed': True, 'scope': 'two original item-use quest reward candidates omitted by the bounded direct-search inventory; target event IDs deduplicated, not a claim that all conditional branches are enumerated or played', 'sssSHA256': SSS_HASH, 'directCatalogUnchanged': True, 'directCatalogSHA256': hashlib.sha256(direct_bytes).hexdigest(), 'directCandidates': len(known), 'additionalCandidates': len(rows), 'combinedCandidates': len(known)+len(rows), 'candidates': rows}
(VERIFY / 'hidden-conditional-candidates.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'directCandidates': len(known), 'additionalCandidates': len(rows), 'combinedCandidates': len(known)+len(rows)}, ensure_ascii=False))
