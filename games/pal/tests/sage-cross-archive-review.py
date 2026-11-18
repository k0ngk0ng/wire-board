from html.parser import HTMLParser
import hashlib
import json
from pathlib import Path
import struct


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
OUT = VERIFY / 'last-hidden-v1'
ARCHIVE = VERIFY / 'win98-reference-v1'
ORIGINAL = ROOT / '.local/pal/original-98'


def bound(path, sha):
    data = path.read_bytes()
    assert hashlib.sha256(data).hexdigest() == sha, path
    return data


def source(path, data):
    return {'path': path.relative_to(ROOT).as_posix(), 'sha256': hashlib.sha256(data).hexdigest()}


class PlainText(HTMLParser):
    def __init__(self):
        super().__init__()
        self.text = []

    def handle_data(self, data):
        self.text.append(data)


def read_archive(sss, messages, encoding):
    offs = struct.unpack_from('<6I', sss)
    events = [None, *struct.iter_unpack('<hHHhHHh9H', sss[offs[0]:offs[1]])]
    scenes = [None, *struct.iter_unpack('<4H', sss[offs[1]:offs[2]])]
    scripts = list(struct.iter_unpack('<4H', sss[offs[4]:offs[5]]))
    cuts = [n for n, in struct.iter_unpack('<I', sss[offs[3]:offs[4]])]
    return events, scenes, scripts, [messages[a:b].decode(encoding, errors='replace') for a,b in zip(cuts,cuts[1:])]


def inspect(archive, cutscene, ordinary, return_scene, protagonists, tree, recast):
    events, scenes, scripts, messages = archive
    assert scenes[cutscene][0] == scenes[ordinary][0] == 201 and scenes[cutscene][2] == 0
    enter = scenes[cutscene][1]
    transition, = [pc for pc, row in enumerate(scripts) if row[:2] == (0x59, cutscene)]
    assert not any(row[:2] == (0x6d, cutscene) for row in scripts)
    stop = next(pc for pc in range(enter, len(scripts)) if scripts[pc][0] <= 3)
    assert scripts[stop-1] == (0x59, return_scene, 0, 0) and scripts[stop] == (1, 0, 0, 0)
    ordinary_rows = range(scenes[ordinary][3]+1, scenes[ordinary+1][3]+1)
    cut_rows = range(scenes[cutscene][3]+1, scenes[cutscene+1][3]+1)
    assert cut_rows[:2] == range(protagonists[0], protagonists[-1]+1)
    assert [events[n][8] for n in protagonists] == [609, 358]
    variants = {event: [x for x in scripts[events[event][4]:events[event][4]+6] if x[:2] == (0x1f, 291)] for event in [*ordinary_rows, *cut_rows] if events[event][7] == 1 and events[event][8] == 519}
    assert variants[tree] == [(0x1f, 291, 2, 0)]
    assert [n for n in variants if n in ordinary_rows] == [tree]
    duplicate = [n for n in variants if n in cut_rows]
    assert all(events[n] == events[tree] for n in duplicate)
    lines = []
    normalized = []
    for op, a, b, c in scripts[enter:stop+1]:
        if op == 0xffff:
            lines.append(messages[a])
            a = 0
        if op in (0x16, 0x24, 0x49):
            assert a in protagonists
            a = protagonists.index(a) + 1
            if op == 0x24:
                assert b == recast
                b = 1
        if op == 0x59:
            assert a == return_scene
            a = 1
        normalized.append((op, a, b, c))
    report = {'map': 201, 'cutscene': cutscene, 'entryPC': enter, 'singleSceneTransitionPC': transition, 'returnPC': stop-1, 'stopPC': stop, 'firstFreeFieldSceneAfterVisit': return_scene, 'ordinaryScene': ordinary, 'ordinaryBananaEvents': [tree], 'cutsceneBananaEvents': duplicate, 'firstExchange': lines[:2], 'lastLine': lines[-1]}
    return normalized, lines, report


win_sss = ORIGINAL / 'sss.mkf'
win_msg = ORIGINAL / 'm.msg'
dos_sss = ARCHIVE / 'fan-dos-sss.mkf'
dos_msg = OUT / 'fan-dos-m.msg'
guide = OUT / 'xiesean-sage.html'
live_report = VERIFY / 'transient-scene-original-review.json'
live_stage = VERIFY / 'transient-scene-stage.json'
raw = [bound(path, expected) for path, expected in [(win_sss,'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'), (win_msg,'5e553754fa0dcdab3b25590a01cb3f08a510c17ee9d59afd1c9f62b02841b846'), (dos_sss,'38252bae8dabaebe9c780f00e8e6829bce56f89d862ecf9d44cc7b135047ed5d'), (dos_msg,'29cb8ce28b1ae6281ab21d970d02316bd6f94f839362005e1a8b5bc418cdbe4d'), (guide,'a23e92921695f0ee77f8336b1ad6f2a98edfff9cee45cd2efefd281cae5fe8a6'), (live_report,'64e7c1f766ce3c34b064222290560371b445ae2c3c4373d65d7cc788d1c40f16')]]
win_code, win_lines, win_info = inspect(read_archive(*raw[:2], 'cp936'), 178, 177, 175, (2945, 2946), 2932, 28023)
dos_code, dos_lines, dos_info = inspect(read_archive(*raw[2:4], 'big5'), 177, 176, 174, (3099, 3100), 3086, 32428)
assert win_code == dos_code and len(win_code) == 59
assert len(win_lines) == len(dos_lines) == 27
assert win_info['firstExchange'] == ['剑圣∶', '圣姑，他们的情况如何？'] and dos_info['firstExchange'] == ['劍聖：', '聖姑，他們的情況如何？']
assert win_info['lastLine'] == '别让他们知道我来过这里' and dos_info['lastLine'] == '別讓他們知道我來過這裡'
assert win_info['cutsceneBananaEvents'] == [2947] and not dos_info['cutsceneBananaEvents']
text = PlainText()
text.feed(raw[4].decode('utf-8'))
quote = '逍遥痊愈后出门在门口可摘的香蕉'
assert quote in ''.join(text.text)
native = json.loads(raw[5])
stage = json.loads(live_stage.read_bytes())
assert native['passed'] and native['entryAlwaysLeavesBeforeReturning'] and native['distinctDuplicateEvent']
assert native['originalRecord'][1:3] == [1264, 712] and not native['normalPickupVerified']
assert stage['passed'] and stage['event'] == 2947 and stage['freshRuntimeSaves'] == 14 and stage['directionalTimerCalls'] == 32
assert not stage['nativeEdits'] and not stage['newProcessed']
report = {'passed': True, 'readyForProgress': True, 'nativeEdits': False, 'actuallyObtained': False, 'classification': 'original-unreachable-preserved', 'event': 2947, 'originalRecord': 2947, 'scene': 178, 'map': 201, 'position': [1264, 712], 'lengthOfMatchingCutscene': len(win_code), 'sameDialogueLineCount': len(win_lines), 'windows': win_info, 'dos': dos_info, 'scope': 'same scripted cutscene in two independently distributed game-data variants; Windows live continuation preserves the unused event, DOS comparison puts no collectible in the cutscene, textual route locates banana on the later field visit', 'liveWindowsEvidence': {'freshRuntimeSaves': stage['freshRuntimeSaves'], 'directionalTimerCalls': stage['directionalTimerCalls'], 'report': live_report.relative_to(ROOT).as_posix()}, 'guide': {'url': 'https://xiesean.github.io/palweb/xjgl/p23.htm', 'quote': quote}, 'sources': [source(path, data) for path,data in zip([win_sss,win_msg,dos_sss,dos_msg,guide,live_report],raw)]+[source(live_stage,live_stage.read_bytes())]}
(OUT / 'sage-cross-archive.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'passed': True, 'matched': len(win_code), 'windowRecord': 2947, 'normalPickups': 0}, ensure_ascii=False))
