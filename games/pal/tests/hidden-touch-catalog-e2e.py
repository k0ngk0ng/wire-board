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
conditional_path = VERIFY / 'hidden-conditional-candidates.json'
direct_bytes, conditional_bytes = direct_path.read_bytes(), conditional_path.read_bytes()
direct, conditional = json.loads(direct_bytes), json.loads(conditional_bytes)
assert direct['passed'] and direct['sssSHA256'] == SSS_HASH and len(direct['candidates']) == 732
assert conditional['passed'] and conditional['additionalCandidates'] == 2 and conditional['directCatalogSHA256'] == hashlib.sha256(direct_bytes).hexdigest()


def script(pc):
    return struct.unpack_from('<4H', SSS, OFFSETS[4]+pc*8)


def word(item):
    return WORDS[item*10:item*10+10].rstrip(b' \x00').decode('gbk')


event, scene = 2980, 182
original = struct.unpack_from('<hHHhHHh9H', SSS, OFFSETS[0]+(event-1)*32)
place = struct.unpack_from('<4H', SSS, OFFSETS[1]+(scene-1)*8)
assert place[3] < event <= struct.unpack_from('<4H', SSS, OFFSETS[1]+scene*8)[3]
assert event not in {row['Event'] for row in direct['candidates']+conditional['candidates']}
assert original[4] == 27203 and original[6] == 1 and original[7] == 8 and original[8] == 0
assert script(27203) == (0x49, 65535, 0, 0) and script(27241) == (7, 221, 41075, 0)
assert script(27246) == (0x1f, 261, 0, 0) and script(27412) == (0x59, 184, 0, 0)
scene_entry = struct.unpack_from('<4H', SSS, OFFSETS[1]+183*8)
assert scene_entry[1] == 27553 and script(27554) == (0x75, 1, 0, 0)
assert script(27674) == (0x1f, 269, 0, 0) and script(27675) == (0x59, 183, 0, 0)
assert script(27448) == (0x25, 2961, 25096, 0) and script(27507) == script(27533) == (0x75, 5, 1, 0)
grants = [{'PC': pc, 'Item': item, 'Amount': 1, 'Cash': 0, 'Name': word(item)} for pc, item in [(27246, 261), (27674, 269)]]
row = {'Scene': scene, 'Map': place[0], 'Event': event, 'X': original[1], 'Y': original[2], 'Sprite': original[8], 'InitialState': original[6], 'TriggerMode': original[7], 'Script': original[4], 'OriginalTriggerScript': original[4], 'Classification': 'touch-script-quest', 'Grants': grants, 'SourceItem': 0, 'BattlePC': 27241, 'BattleFormation': 221, 'TriggerClosePC': 27203, 'FollowingScenePC': 27412, 'FollowingScene': 184, 'FollowingSceneOnEnter': 27553, 'FinalScenePC': 27675, 'FinalScene': 183, 'ClosedVillageDoorPC': 27448, 'ReunionPC': [27507, 27533]}
assert direct_path.read_bytes() == direct_bytes and conditional_path.read_bytes() == conditional_bytes
report = {'passed': True, 'scope': 'one original touch-trigger Peach optional quest chain omitted by the direct search trigger-mode filter, metal orb after original battle and longevity gourd in its automatic village scene entry counted as one event, not full enumeration of all touch rewards or gameplay acceptance', 'sssSHA256': SSS_HASH, 'directCatalogUnchanged': True, 'conditionalCatalogUnchanged': True, 'directCatalogSHA256': hashlib.sha256(direct_bytes).hexdigest(), 'conditionalCatalogSHA256': hashlib.sha256(conditional_bytes).hexdigest(), 'directCandidates': 732, 'priorConditionalCandidates': 2, 'additionalCandidates': 1, 'combinedCandidates': 735, 'candidates': [row]}
(VERIFY / 'hidden-touch-candidates.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'additionalTouchCandidates': 1, 'combinedCandidates': 735, 'event': event, 'gifts': [261, 269]}, ensure_ascii=False))
