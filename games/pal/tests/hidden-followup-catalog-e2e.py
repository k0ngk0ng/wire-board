from collections import defaultdict
import hashlib
import json
from pathlib import Path
import struct


ROOT = Path(__file__).resolve().parents[3]
VERIFY = ROOT / '.local/pal/verification'
DATA = ROOT / '.local/pal/original-98'
SSS = (DATA/'sss.mkf').read_bytes()
HASH = hashlib.sha256(SSS).hexdigest()
assert HASH == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
TABLE = struct.unpack_from('<6I',SSS)
SCRIPTS = list(struct.iter_unpack('<4H',SSS[TABLE[4]:TABLE[5]]))
EVENTS = [None,*struct.iter_unpack('<hHHhHHh9H',SSS[TABLE[0]:TABLE[1]])]
SCENES = [None,*struct.iter_unpack('<4H',SSS[TABLE[1]:TABLE[2]])]
WORDS = (DATA/'word.dat').read_bytes()
BRANCHES = {6:[1],7:[1,2],10:[0],0x1e:[1],0x20:[2],0x2e:[2],0x33:[0],0x34:[0],0x38:[0],0x3a:[0],0x58:[2],0x5d:[1],0x5e:[1],0x61:[0],0x64:[1],0x68:[0],0x74:[0],0x79:[1],0x81:[2],0x83:[2],0x84:[2],0x86:[2],0x91:[0],0x94:[2],0x95:[1],0x9c:[1],0x9e:[2]}
REFERENCE = ROOT/'.local/pal/hidden-items-research/original-98-text-guide.md'


def successors(pc, operation, operands, keeps_entry):
    a,b,_ = operands
    if operation == 0:
        return []
    if operation == 1:
        return [(pc+1,keeps_entry)] if keeps_entry else []
    if operation == 2:
        return ([(a,keeps_entry)] if keeps_entry else []) + ([(pc+1,keeps_entry)] if b else [])
    if operation == 3:
        return [(a,keeps_entry)] + ([(pc+1,keeps_entry)] if b else [])
    if operation == 0xa2:
        return [(point,keeps_entry) for point in range(pc+1,pc+a+1)]
    return [(pc+1,keeps_entry)] + ([(a,False)] if operation == 4 else []) + [(operands[field],keeps_entry) for field in BRANCHES.get(operation,[])]


def grants(start):
    seen = set()
    queue = [(start,True)]
    found = set()
    while queue:
        pc,keeps_entry = queue.pop()
        if (pc,keeps_entry) in seen or not 0 < pc < len(SCRIPTS):
            continue
        seen.add((pc,keeps_entry))
        op,a,b,c = SCRIPTS[pc]
        if (op==0x1e and 0<a<32768) or (op==0x1f and 0<a<len(WORDS)//10 and b<32768):
            found.add(pc)
        queue.extend(successors(pc,op,(a,b,c),keeps_entry))
    return found


old = {}
digests = {}
for name in ['hidden-search-candidates.json','hidden-conditional-candidates.json','hidden-touch-candidates.json']:
    raw=(VERIFY/name).read_bytes()
    digests[name]=hashlib.sha256(raw).hexdigest()
    for row in json.loads(raw)['candidates']:
        assert row['Event'] not in old
        old[row['Event']]=row
assert len(old)==735
scene_of = {number:scene for scene in range(1,len(SCENES)-1) for number in range(SCENES[scene][3]+1,SCENES[scene+1][3]+1)}
assert len(scene_of)==5077
starting = {}
for event in scene_of:
    row = EVENTS[event]
    if row[4] and row[7]:
        reached = grants(row[4])
        if reached:
            starting[event]=reached
new_starting = set(starting)-set(old)
assert new_starting=={16,39,57,202,343,434,761,959,1049,3116}
old_extra={event:sorted(starting[event]-{g['PC'] for g in old[event]['Grants']}) for event in set(starting)&set(old)}
assert {event for event,pcs in old_extra.items() if pcs}=={364,1883,2003,2886,3354}
replacement = defaultdict(dict)
for pc,(op,event,new,_) in enumerate(SCRIPTS):
    if op in [0x24,0x25] and event in scene_of and event not in old and new:
        if reached := grants(new):
            replacement[event][pc] = sorted(reached)
assert set(replacement)=={20,60,85,192,797,831,877,898,2494}
assert SCRIPTS[1279]==(0x25,84,1237,0) and SCRIPTS[1250]==(0x25,85,1285,0)
assert SCRIPTS[1062]==(0x25,192,2018,0) and SCRIPTS[1061]==(0x49,192,2,0)
assert SCRIPTS[4253]==SCRIPTS[4258]==(1,0,0,0)
assert SCRIPTS[6343]==(0x79,37,6349,0) and SCRIPTS[6349]==(0x1e,65536-400,6344,0)
assert SCRIPTS[6377]==(2,6344,0,0) and SCRIPTS[6347]==(0x26,7,0,0)
text = REFERENCE.read_text()
rows = []
for event, scene, pc, grant, item, cash, quote, mode in [
    (85,5,1285,1294,237,0,'秀兰','chained-neighbor-gift'),
    (192,11,2018,2023,107,0,'和药铺的洪大夫交谈，获得灵山仙芝','post-diagnosis-gift'),
    (202,15,4251,4257,100,0,'和张四哥交谈2次，获得行军丹','repeated-dialogue-gift'),
    (434,24,6343,6376,199,400,'第一次和杂货小贩交谈，会自动花400文钱买一支银钗给灵儿','conditional-one-time-purchase'),
]:
    assert scene_of[event]==scene and SCRIPTS[grant]==(0x1f,item,0,0)
    assert quote in text
    assert grant in starting.get(event,set()) or any(grant in pcs for pcs in replacement[event].values())
    original=EVENTS[event]
    rows.append({'Event':event,'Scene':scene,'Map':SCENES[scene][0],'X':original[1],'Y':original[2],'Sprite':original[8],'InitialState':original[6],'TriggerMode':original[7],'Script':pc,'OriginalTriggerScript':original[4],'Classification':'followup-dialogue-reward','RewardMode':mode,'OriginalCharge':cash,'Grants':[{'PC':grant,'Item':item,'Amount':1,'Cash':0,'Name':WORDS[item*10:item*10+10].rstrip(b' \x00').decode('gbk')}],'GuideQuote':quote})
report = {'passed':True,'scope':'bounded original search/touch entry follow-ups, conditional branches, calls and literal script replacements; excludes main-story progress rewards, battle drops and dynamic reachability claims','sssSHA256':HASH,'directCatalogUnchanged':True,'directCatalogSHA256':digests['hidden-search-candidates.json'],'priorCatalogsSHA256':digests,'priorCandidates':735,'additionalCandidates':4,'combinedCandidates':739,'candidates':rows,'mainStoryRewardEventIDs':[16,20,39,57,60,343,761,797,831,877,898,959,1049,2494,3116],'knownRootEventsWithAuditedSecondaryGrants':{event:pcs for event,pcs in old_extra.items() if pcs},'reference':{'url':'https://steamcommunity.com/sharedfiles/filedetails/?id=3654311548','path':REFERENCE.relative_to(ROOT).as_posix(),'sha256':hashlib.sha256(REFERENCE.read_bytes()).hexdigest()}}
report['directCandidates'] = 732
(VERIFY/'hidden-followup-candidates.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'passed':True,'optionalFollowupEvents':[row['Event'] for row in rows],'candidateCount':739},ensure_ascii=False))
