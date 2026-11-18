import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
BASE = '.local/pal/verification/'
WAVE = BASE + 'runtime-candidates/scene-wave/development-health/hp-9999/'
AUDIO = BASE + 'runtime-candidates/audio/development-health/hp-9999/'
catalog_path = ROOT / (BASE + 'hidden-search-candidates.json')
catalog = json.loads(catalog_path.read_text())
assert catalog['passed'] and catalog['sssSHA256'] == 'ce448a90c9797648eea315fe589b2bd434139ffc423bf0a5954dae6f4c8b4756'
points = {row['Event']: row for row in catalog['candidates']}
assert len(points) == len(catalog['candidates'])
supplements = []
for supplement_name in ['hidden-conditional-candidates.json', 'hidden-touch-candidates.json', 'hidden-followup-candidates.json']:
    supplement_path = ROOT / (BASE+supplement_name)
    if not supplement_path.is_file():
        continue
    supplement_bytes = supplement_path.read_bytes()
    supplement = json.loads(supplement_bytes)
    assert supplement['passed'] and supplement['sssSHA256'] == catalog['sssSHA256'] and supplement['directCatalogUnchanged']
    assert supplement['directCatalogSHA256'] == hashlib.sha256(catalog_path.read_bytes()).hexdigest()
    assert supplement['directCandidates'] == len(catalog['candidates']) and supplement['additionalCandidates'] == len(supplement['candidates'])
    for row in supplement['candidates']:
        assert row['Event'] not in points
        if row['Classification'] == 'conditional-item-use-quest':
            assert row['SourceItem'] > 0 and row['UseScript'] > 0
        elif row['Classification'] == 'touch-script-quest':
            assert row['TriggerMode'] >= 4 and row['BattleFormation'] == 221 and row['FollowingSceneOnEnter'] == 27553
        else:
            assert row['Classification'] == 'followup-dialogue-reward' and row['Event'] in {85,192,202,434} and row['OriginalTriggerScript'] > 0 and len(row['Grants']) == 1
        points[row['Event']] = row
    assert supplement['combinedCandidates'] == len(points)
    supplements.append({'path': str(supplement_path.relative_to(ROOT)), 'sha256': hashlib.sha256(supplement_bytes).hexdigest(), 'scope': supplement['scope'], 'additionalCandidates': supplement['additionalCandidates']})
for row in points.values():
    assert row['Scene'] > 0 and row['Map'] > 0 and 0 <= row['X'] < 2048 and 0 <= row['Y'] < 2048
evidence = {}
verified = {}
preserved = {}

def report(name):
    path = ROOT / name
    raw = path.read_bytes()
    value = json.loads(raw)
    assert value['passed'], name
    evidence[name] = hashlib.sha256(raw).hexdigest()
    return value

def accept(event, group, source, position=None, scene=None):
    assert event in points and event not in preserved
    point = points[event]
    if position is not None:
        assert list(position) == [point['X'], point['Y']], (event, position)
    if scene is not None:
        assert scene == point['Scene']
    entry = verified.setdefault(event, {'event': event, 'scene': point['Scene'], 'map': point['Map'], 'position': [point['X'], point['Y']], 'status': 'pickup-and-saved-no-duplication-verified', 'groups': [], 'evidence': []})
    if group not in entry['groups']:
        entry['groups'].append(group)
    if source not in entry['evidence']:
        entry['evidence'].append(source)

def pickup_battle_cash(row, case_battles=None):
    total = 0
    for battle in row.get('battleCashDuringPickup', []):
        assert battle['rewardCash'] >= 0 and battle['cashAfter']-battle['cashBefore'] == battle['rewardCash']
        if case_battles is not None:
            assert battle in case_battles
        total += battle['rewardCash']
    return total

def rendered_text(view):
    chars = []
    if view.get('raster'):
        raster, seen = view['raster'], set()
        def visit(index):
            if index <= 0 or index > len(raster['nodes']) or index in seen:
                return
            seen.add(index)
            node = raster['nodes'][index-1]
            visit(node[1])
            visit(node[2])
            if node[0] == 2:
                chars.append(chr(node[5]))
        visit(raster['root'])
    else:
        chars.extend(chr(op['id']) for op in view.get('overlay') or [] if op['kind'] == 0)
    return ''.join(chars)+(view.get('dialogue') or {}).get('text', '')

def claimed(value, source, group):
    rows = value['claimed']
    assert len({row['event'] for row in rows}) == len(rows)
    for row in rows:
        assert row['repeatSearches'] >= 3
        if 'grants' in row:
            assert row['notification'] and row['grants']
            assert row['grants'] == points[row['event']]['Grants']
            assert all((grant['Item'] > 0 and grant['Amount'] > 0 and grant['Cash'] == 0) or (grant['Item'] == 0 and grant['Amount'] == 0 and grant['Cash'] > 0) for grant in row['grants'])
            cash = sum(grant['Cash'] for grant in row['grants'])
            if cash or 'battleCashDuringPickup' in row:
                assert row['cashAfter']-row['cashBefore'] == cash+pickup_battle_cash(row)
        else:
            assert row['amount'] > 0
        accept(row['event'], group, source, row['position'], row['scene'])
        if 'followup' in row:
            verified[row['event']]['followupRewards'] = row['followup']
        if row.get('originalRenewableHarvest'):
            assert row['event'] == 2922 and value['verificationVersion'] >= 12 and row['grants'] == [{'PC': 28193, 'Item': 104, 'Amount': 1, 'Cash': 0, 'Name': '鼠儿果'}]
            assert row['inventoryAfter']-row['inventoryBefore'] == 4 and len(row['freshSearches']) == 3 and row['repeatSearches'] == 3
            assert all(step['after']-step['before'] == 1 for step in row['freshSearches'])
            assert all(value['proof']['Saved'][f'reward-2922-renewable-search-{n}'] for n in [1, 2, 3])
            verified[row['event']]['status'] = 'renewable-harvest-and-saved-no-replay-verified'
            verified[row['event']]['harvestRules'] = {'repeatable': True, 'quantityPerSearch': 1, 'verifiedFreshSearches': 4}

for name, group in [('hidden-inn-rewards-1.json', 'yuhang-inn'), ('hidden-village-rewards-1.json', 'yuhang-village-harbor')]:
    source = BASE + name
    value = report(source)
    assert value['saves'] >= len(value['sites'])*3
    assert len({row['event'] for row in value['sites']}) == len(value['sites'])
    for row in value['sites']:
        assert row['notification'] and row['repeatAfterSaveRejected']
        accept(row['event'], group, source, row['position'], row['scene'])

source = WAVE + 'shu-hidden-rewards-1.json'
value = report(source)
assert value['proof']['Saves'] >= len(value['claimed'])*4
claimed(value, source, 'shu-alchemy')
for name, group in [('capital-hidden-rewards-1.json', 'capital-hidden-shoes'), ('poison-forest-rewards-1.json', 'poison-forest')]:
    source = WAVE + name
    claimed(report(source), source, group)

source = WAVE + 'campaign-capital-manor-1.json'
value = report(source)
assert not value['resumedTestRoute']
proof = value['proof']
assert proof['RitualBefore']['檀香'] == proof['RitualBefore']['蜡烛'] == proof['RitualBefore']['符纸'] == 1
assert proof['CashChanges']['manor-chest'][1]-proof['CashChanges']['manor-chest'][0] == 3000
for event, label in [(2187, 'incense-candle-inventory'), (2188, 'paper-inventory'), (2189, 'manor-chest-collected')]:
    assert proof['ChestFrames'][str(event)] == 1 and proof['Saved'][label]
    accept(event, 'capital-original-chests', source)

source = WAVE + 'campaign-toad-mountain-1.json'
value = report(source)
assert not value['resumedTestRoute'] and value['fivePoisonOrbAmount'] == 1
for label in ['orb-inventory', 'orb-reading', 'orb-repeat-search']:
    assert value['proof']['Saved'][label]
accept(1911, 'toad-corpse-orb', source)

for scene in [149, 169]:
    source = BASE + f'tower-hidden-{scene}-stage.json'
    value = report(source)['result']
    assert value['passed']
    claimed(value, source, 'tower-retained-' + str(scene))
for scene in [148, 155, 157, 166, 168]:
    source = AUDIO + f'tower-rewards-{scene}-result-1.json'
    value = report(source)
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
    assert value['proof']['Battles'] == value['proof']['Wins']
    claimed(value, source, 'tower-current-' + str(scene))
for scene in [2, 4, 7, 9, 10, 17, 18, 20, 21, 25, 28, 37, 39, 41, 42, 45, 46, 47, 48, 49, 53, 54, 59, 60, 61, 62, 63, 65, 66, 70, 71, 72, 73, 74, 75, 76, 77, 81, 87, 93, 96, 97, 101, 102, 103, 104, 105, 109, 138, 175, 176, 177, 180, 181, 185, 187, 189, 190, 191, 192, 208, 210, 220, 221, 223, 224, 226, 227, 251, 253, 263, 264, 265]:
    sources = [(1, AUDIO + f'regional-hidden-{scene}-result-1.json')]
    sources.extend((version, AUDIO + f'regional-hidden-v{version}-{scene}-result-1.json') for version in range(2, 19))
    available = [(version, path) for version, path in sources if (ROOT / path).is_file()]
    if not available:
        continue
    version, source = available[-1]
    value = report(source)
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture']
    if version >= 2:
        assert value['verificationVersion'] == version
    if version >= 6:
        assert not value['auditOnly'] and value['auditedEvents'] == [] and value['claimed']
    if version >= 12 and scene in [175, 177, 180, 185, 189]:
        original_review = report(BASE+'sacred-rewards-original-review.json')
        assert original_review['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not original_review['nativeEdits']
        assert len(original_review['records']) == 43 and {row['scene'] for row in original_review['records']} == {175, 177, 180, 185, 189}
    if version >= 13 and scene == 187:
        original_review = report(BASE+'forest-peach-original-review.json')
        assert original_review['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not original_review['nativeEdits']
        assert len(original_review['records']) == 13
    if version >= 14 and scene in [176, 181, 190, 191, 192]:
        original_review = report(BASE+'peach-rewards-original-review.json')
        assert original_review['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not original_review['nativeEdits']
        assert len(original_review['records']) == 41 and {row['scene'] for row in original_review['records']} == {176, 181, 190, 191, 192}
        assert {row['event'] for row in original_review['records'] if row['scene'] == scene} == {row['event'] for row in value['claimed']}
    if version >= 15 and scene in [208, 210]:
        original_review = report(BASE+'dali-rewards-original-review.json')
        assert original_review['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not original_review['nativeEdits'] and len(original_review['records']) == 37
        assert {row['event'] for row in original_review['records'] if row['scene'] == scene} == {row['event'] for row in value['claimed']}
        inventory = dict(value['inventoryInitial'])
        for row in value['claimed']:
            for grant in row['grants']:
                if grant['Item'] > 0:
                    inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
        assert inventory == value['inventory'] and value['battleItemGrants'] == []
        assert value.get('pendingOriginalEvents', []) == ([3537] if scene == 208 else [])
        assert not original_review['pendingBoots']['readyForProgress'] and not original_review['pendingBoots']['actuallyObtained']
    if version >= 16 and scene in [251, 253, 263, 264, 265]:
        original_review = report(BASE+'past-rewards-original-review.json')
        assert original_review['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not original_review['nativeEdits'] and len(original_review['records']) == 30
        assert {row['event'] for row in original_review['records'] if row['scene'] == scene} == {row['event'] for row in value['claimed']}
        inventory = dict(value['inventoryInitial'])
        for row in value['claimed']:
            for grant in row['grants']:
                if grant['Item'] > 0:
                    inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
        assert inventory == value['inventory'] and value['battleItemGrants'] == []
        assert len(original_review['originalWaterChasePreserved']) == 4 and {row['scene'] for row in original_review['originalWaterChasePreserved']} == {251, 253}
    if version >= 17 and scene in [220, 221, 223, 224, 226, 227]:
        deep = scene in [223, 224, 226, 227]
        original_review = report(BASE+('trial-deep-original-review.json' if deep else 'yuhang-trial-original-review.json'))
        assert original_review['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not original_review['nativeEdits'] and len(original_review['records']) == (78 if deep else 58)
        assert {row['event'] for row in original_review['records'] if row['scene'] == scene} == {row['event'] for row in value['claimed']}
        route_raw = (ROOT / (BASE+('trial-deep-rewards-plan.json' if deep else 'trial-rewards-plan.json'))).read_bytes()
        route = json.loads(route_raw)
        assert route['passed'] and route['diagnosticOnly'] and not route['originalDataEdits'] and value['routePlanSHA256'] == hashlib.sha256(route_raw).hexdigest()
        inventory = dict(value['inventoryInitial'])
        for row in value['claimed']:
            for grant in row['grants']:
                inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
        drops = value['battleItemGrants']
        native_drops = [row for row in original_review['battleItemGrants'] if row['scene'] == scene]
        assert [{key: row[key] for key in drops[0]} for row in native_drops] == drops
        for drop in drops:
            assert drop['endScript'] == 42828 and drop['item'] == 152 and drop['name'] == '傀儡虫' and drop['amount'] > 0 and value['proof']['Saved'][f"battle-{drop['battle']}-worm-drop"]
            inventory['傀儡虫'] = inventory.get('傀儡虫', 0)+drop['amount']
        assert inventory == value['inventory'] and inventory['土灵珠'] == 1
        case = next(row for row in route['cases'] if row['scene'] == scene)
        assert len(value['trialRoute']) == case['portalSteps']+case['earthReturns'] and value['trialPortalSteps'] == case['portalSteps'] and value['trialEarthReturns'] == case['earthReturns']
    assert value['proof']['Saves'] >= len(value['claimed'])*4
    cash = sum(grant['Cash'] for row in value['claimed'] for grant in row['grants'])
    battle_cash = 0
    if version >= 3:
        assert value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == 0
        assert len(value['battleCashChanges']) == value['proof']['Battles']-value['preCollectionBattles']
        assert value['combatAllowed'] == (scene in [7, 18] or version >= 5 and 41 <= scene <= 48 or version >= 7 and 60 <= scene <= 66 or version >= 8 and 70 <= scene <= 77 or version >= 9 and scene in [81, 87, 93] or version >= 10 and 102 <= scene <= 105 or version >= 12 and scene in [180, 185, 189] or version >= 14 and scene in [181, 190, 191] or version >= 16 and scene in [251, 253] or version >= 17 and scene in [220, 221, 223, 224, 226, 227])
        for battle in value['battleCashChanges']:
            assert battle['rewardCash'] >= 0 and battle['cashAfter']-battle['cashBefore'] == battle['rewardCash']
            battle_cash += battle['rewardCash']
        assert [(p['role'], p['name'], p['controller']) for p in value['partyInitial']] == [(p['role'], p['name'], p['controller']) for p in value['party']]
        if not value['combatAllowed']:
            assert value['partyInitial'] == value['party'] and value['proof']['Battles'] == 0
    else:
        assert value['proof']['Battles'] == 0
    if cash or version >= 2:
        assert value['cashFinal']-value['cashInitial'] == cash+battle_cash
    if version >= 14 and scene == 181:
        drops = value['battleItemGrants']
        assert drops == [{'amount': 1, 'battle': battle, 'endScript': 41008, 'enemy': 402, 'item': 104, 'name': '鼠儿果'} for battle in [10, 16, 17]]
        assert [{key: row[key] for key in drops[0]} for row in original_review['battleItemGrants']] == drops
        inventory = dict(value['inventoryInitial'])
        for row in value['claimed']:
            for grant in row['grants']:
                if grant['Item'] > 0:
                    inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
        for drop in drops:
            inventory[drop['name']] = inventory.get(drop['name'], 0)+drop['amount']
            battle = drop['battle']
            assert value['proof']['Saved'][f'battle-{battle}'] and value['proof']['Saved'][f'battle-{battle}-fruit-drop']
            native = next(row for row in original_review['battleItemGrants'] if row['battle'] == battle)
            assert native['nativePC'] == 41010
            for key in ['battleState', 'noticeState']:
                raw = (ROOT / native[key]['path']).read_bytes()
                assert hashlib.sha256(raw).hexdigest() == native[key]['sha256']
            start = json.loads((ROOT / native['battleState']['path']).read_bytes())['public']
            notice = json.loads((ROOT / native['noticeState']['path']).read_bytes())['public']
            assert sum(enemy['object'] == 402 for enemy in start['battle']['enemies']) == drop['amount']
            assert notice['phase'] == 'battle' and notice['battle']['phase'] == 1 and '捡到一颗鼠儿果' in rendered_text(notice)
        assert inventory == value['inventory'] and inventory['鼠儿果'] == 5
    if version >= 2:
        for row in value['claimed']:
            assert row['invisible'] == (points[row['event']]['Sprite'] == 0)
            assert row['originalHerbDisappearance'] == (points[row['event']]['Sprite'] in ([68, 69, 70] if version >= 12 else [68, 69]))
            during_cash = 0
            if version >= 10:
                assert isinstance(row['battleCashDuringPickup'], list)
                during_cash = pickup_battle_cash(row, value['battleCashChanges'])
            assert row['cashAfter']-row['cashBefore'] == sum(grant['Cash'] for grant in row['grants'])+during_cash
            if version >= 3:
                assert row['searchMode'] == points[row['event']]['TriggerMode']
                sprite = points[row['event']]['Sprite']
                vanished = version >= 5 and row['disappearsAfterPickup']
                if sprite in [10, 389] and not vanished:
                    assert row['openFrame'] >= 0 and row['closedFrame'] >= 0 and row['openFrame'] != row['closedFrame']
                elif sprite in [10, 389] and vanished:
                    assert row['closedFrame'] >= 0 and row['openFrame'] == -1
                else:
                    assert row['openFrame'] == row['closedFrame'] == -1
                if version >= 5:
                    assert row['repeatedFromSavedPose'] and len(row['repeatPose']) == 3 and 0 <= row['repeatPose'][2] < 4
                    if sprite == 0:
                        assert not vanished
                if version >= 7 and row['event'] == 1048:
                    assert row['persistentBodyAfterSavedSearches'] and not vanished
                    assert [(grant['Item'], grant['Amount']) for grant in row['grants']] == [(152, 2)]
                    assert all(value['proof']['Saved'][f'blood-pool-optional-switch-{event}'] for event in [1042, 1047])
                if version >= 8 and row['event'] == 1486:
                    assert row['originalGroundWeaponDisappearance'] and vanished and sprite == 230
                    assert row['searchMode'] == 3 and [(grant['Item'], grant['Amount']) for grant in row['grants']] == [(174, 1)]
                if version >= 9 and row['event'] == 1631:
                    assert row['originalQuestBundleDisappearance'] and vanished and sprite == 380
                    assert [(grant['Item'], grant['Amount']) for grant in row['grants']] == [(271, 1)]
                if version >= 10 and row['event'] == 1845:
                    assert row['originalMerchantCorpsePreserved'] and not vanished and sprite == 387
                if version >= 10 and row['event'] == 1883:
                    assert row['originalEscortContract'] and vanished and sprite == 83
                    assert row['escortFollowerSprite'] == 82 and row['decisions'] == ['是'] and row['searchesBeforeReward'] == 1
                    assert value['proof']['Saved']['reward-1883-decision-1']
                if version >= 10 and row['event'] == 1911:
                    assert row['originalPoisonCorpseFrame'] == 2
                if version >= 11 and row['event'] == 1818:
                    assert row['originalLetterRequest'] and not vanished and sprite == 134 and row['decisions'] == ['是']
                    assert value['proof']['Saved']['reward-1818-decision-1']
                if version >= 11 and row['event'] == 2035:
                    assert row['originalGroundVialDisappearance'] and vanished and sprite == 308
                if version >= 12 and row['event'] == 2932:
                    assert row['originalBananaTreeFrame'] == 1 and not vanished and sprite == 519
                    assert [(g['Item'], g['Amount']) for g in row['grants']] == [(291, 2)]
                if version >= 12 and row['event'] == 2886:
                    follow = row['followup']
                    assert (follow['PC'], follow['Item'], follow['Amount'], follow['afterRepeat']) == (28168, 268, 1, 1)
                    assert follow['after']-follow['before'] == 1 and follow['noticeSeen'] and follow['noDuplicateSearches'] >= 3
                    assert value['proof']['Saved']['reward-2886-vessel-notice'] and value['proof']['Saved']['reward-2886-vessel-collected']
                if version >= 4 and row['event'] == 364:
                    branch = row['followup']
                    assert (branch['PC'], branch['Item'], branch['Amount'], branch['Name']) == (5149, 179, 1, '苗刀')
                    assert branch['noticeSeen'] and branch['notice'] and branch['afterRepeat'] == 2
                    assert branch['after']-branch['before'] == 1 and branch['noDuplicateSearches'] >= 3
                    assert value['proof']['Saved']['reward-364-blade-notice']
                    for step in range(1, 4):
                        assert value['proof']['Saved'][f'reward-364-followup-{step}']
            for stage in ['unpicked', 'notice', 'collected', 'repeated']:
                assert value['proof']['Saved'][f"reward-{row['event']}-{stage}"]
    if version >= 9:
        audits = value['emptyChestAudits']
        assert len(audits) == (1 if scene == 93 else 0)
        if audits:
            empty = audits[0]
            assert empty['event'] == 1704 and empty['event'] not in points
            assert empty['originalZeroItemChest'] and not empty['countedCandidate'] and empty['grants'] == []
            assert empty['zeroItemGrantPC'] == 37482 and empty['originalNotice'] == '获得紫青玉蓉膏'
            assert empty['inventoryBefore'] == empty['inventoryAfter'] and empty['cashBefore'] == empty['cashAfter']
            assert empty['openFrame'] == 1 and empty['closedFrame'] == 0
            assert empty['repeatSearches'] >= 3 and empty['repeatedFromSavedPose']
            for stage in ['unpicked', 'notice', 'collected', 'repeated']:
                assert value['proof']['Saved'][f'reward-1704-{stage}']
    claimed(value, source, 'regional-current-' + str(scene))

source = AUDIO + 'regional-hidden-v10-103-renegotiated-result-1.json'
if (ROOT / source).is_file():
    alternate = report(source)
    assert alternate['verificationVersion'] == 10 and alternate['candidate'] == 'audio' and alternate['developmentProfile'] == 'hp-9999'
    assert alternate['authorityFixture'] and alternate['scholarOffer'] == 'renegotiated' and alternate['scene'] == 103
    assert not alternate['auditOnly'] and alternate['auditedEvents'] == alternate['emptyChestAudits'] == []
    assert len(alternate['claimed']) == 1 and 1883 in verified
    row = alternate['claimed'][0]
    assert row['event'] == 1883 and row['originalEscortContract'] and row['escortFollowerSprite'] == 83
    assert row['decisions'] == ['否'] and row['searchesBeforeReward'] == 3 and row['repeatSearches'] >= 3 and row['repeatedFromSavedPose']
    assert [(g['PC'], g['Item'], g['Amount'], g['Cash']) for g in row['grants']] == [(21261, 0, 0, 1500)]
    assert row['cashAfter']-row['cashBefore'] == 1500+pickup_battle_cash(row, alternate['battleCashChanges'])
    assert alternate['proof']['Battles'] == alternate['proof']['Wins'] and alternate['proof']['ScriptedFinishes'] == 0
    assert alternate['cashFinal']-alternate['cashInitial'] == 1500+sum(b['rewardCash'] for b in alternate['battleCashChanges'])
    for label in ['unpicked', 'notice', 'collected', 'repeated', 'decision-1', 'declined-field-1', 'declined-field-2']:
        assert alternate['proof']['Saved'][f'reward-1883-{label}']
    assert alternate['proof']['Saves'] >= 7
    direct_source = AUDIO + 'regional-hidden-v10-103-result-1.json'
    assert direct_source in verified[1883]['evidence']
    verified[1883]['groups'].append('regional-scholar-renegotiated')
    verified[1883]['evidence'].append(source)
    verified[1883]['conditionalRewards'] = [{'condition': 'accept-first-offer', 'cash': 1000, 'followerSprite': 82, 'evidence': direct_source}, {'condition': 'decline-then-talk-twice', 'cash': 1500, 'followerSprite': 83, 'evidence': source}]

forest_sources = [(AUDIO+'regional-hidden-v13-193-pre-lift-result-1.json', [3313, 3314, 3316, 3317]), (AUDIO+'regional-hidden-v13-193-post-lift-result-1.json', [3315])]
if any((ROOT / source).is_file() for source, _ in forest_sources):
    native_source = BASE+'forest-peach-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits'] and len(native['records']) == 13
    for source, events in forest_sources:
        value = report(source)
        assert value['verificationVersion'] == 13 and value['scene'] == 193 and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture']
        assert [row['event'] for row in value['claimed']] == events and value['proof']['Saves'] >= len(events)*4
        assert value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == 0 and value['combatAllowed']
        assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges'])
        assert len(value['battleCashChanges']) == value['proof']['Battles']-value['preCollectionBattles']
        assert [row['role'] for row in value['partyInitial']] == [row['role'] for row in value['party']] == [0]
        for row in value['claimed']:
            assert row['originalHerbDisappearance'] and row['disappearsAfterPickup'] and row['searchMode'] == points[row['event']]['TriggerMode']
            assert row['cashAfter']-row['cashBefore'] == pickup_battle_cash(row, value['battleCashChanges'])
            assert row['repeatedFromSavedPose'] and all(value['proof']['Saved'][f"reward-{row['event']}-{label}"] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        claimed(value, source, 'east-forest-'+('pre-lift' if len(events) == 4 else 'post-lift'))

source = AUDIO+'peach-quest-v1-result-1.json'
if (ROOT / source).is_file():
    value = report(source)
    native_source = BASE+'forest-peach-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits']
    assert value['event'] == 2980 and value['kind'] == 'original-peach-village-quest' and not value['resumedTestRoute'] and not value['nativeEdits'] and value['resourceUploads'] == 0
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['verificationVersion'] == 1
    assert value['bossBattles'] == 1 and value['forceInputs'] > 0 and value['returnAttempts'] == 3 and value['temporarySoloSaved']
    assert value['finalInventory'] == {'风灵珠': 1, '金罡珠': 1, '寿葫芦': 1}
    assert value['inventoryChecks']['initial'] == value['inventoryChecks']['wrong-wind-orb-retained'] == value['inventoryChecks']['wind-orb-retained'] == {'风灵珠': 1, '金罡珠': 0, '寿葫芦': 0}
    assert all(value['inventoryChecks'][f'rescue-repeat-{n}'] == value['finalInventory'] for n in [1, 2, 3])
    assert [row['role'] for row in value['partyInitial']] == [0, 4] and [row['role'] for row in value['partyFinal']] == [4, 0]
    assert value['proof']['Battles'] == value['proof']['Wins'] == len(value['battleCashChanges']) and value['proof']['ScriptedFinishes'] == 0
    assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges'])
    assert all(value['proof']['Saved'][label] for label in ['wind-orb-selected-in-fog', 'wooden-taoist-battle-1', 'village-solo-in-progress', 'gourd-return-solo', 'anu-rejoined', 'rescue-repeat-1', 'rescue-repeat-2', 'rescue-repeat-3'])
    accept(2980, 'peach-original-touch-quest', source)
    accept(2980, 'peach-original-touch-quest', native_source)
    verified[2980]['status'] = 'touch-quest-chain-and-saved-no-duplicate-verified'
    verified[2980]['grants'] = points[2980]['Grants']

source = BASE+'dali-rewards-original-review.json'
if (ROOT / source).is_file():
    native = report(source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits']
    kirin = native['kirin']
    assert kirin['event'] == 3354 and kirin['reusedExistingStoryEvidence'] and kirin['newGameplayRuns'] == 0
    assert (kirin['originalEntry'], kirin['battlePC'], kirin['battleFormation'], kirin['fireOrbGrantPC'], kirin['hornGrantPC']) == (29953, 29972, 224, 29976, 30093)
    main_source = AUDIO+'campaign-dali-kirin-1.json'
    value = report(main_source)
    assert kirin['report']['path'] == main_source and kirin['report']['sha256'] == evidence[main_source]
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and not value['resumedTestRoute']
    assert value['sourceCheckpoint'] == 'campaign-sacred-tree-state.json' and value['sourceCheckpointSHA256'] == value['rootStage18SHA256'] == hashlib.sha256((ROOT / (AUDIO+'campaign-sacred-tree-state-1.json')).read_bytes()).hexdigest()
    checks, proof = value['checks'], value['proof']
    assert checks['KirinObserved'] and checks['KirinAcceptedSaved'] and checks['RepeatInputs'] == 3 and checks['RewardOrder'] == ['获得火灵珠', '获得火眼麒麟角']
    assert checks['InventoryBefore'] == {'风灵珠': 1} and checks['InventoryAfter'] == {'火灵珠': 1, '火眼麒麟角': 1, '风灵珠': 1}
    assert proof['Battles'] == proof['Wins'] == 43 and proof['ScriptedFinishes'] == 0 and proof['Saves'] == len(proof['Saved']) == 103 and all(proof['Saved'].values())
    for stage in kirin['nativeStages']:
        assert proof['Saved'][stage['label']] and hashlib.sha256((ROOT / stage['state']['path']).read_bytes()).hexdigest() == stage['state']['sha256']
    assert all(proof['Saved'][f'ended-kirin-confirm-{n}'] for n in [1, 2, 3])
    assert any(stage['label'] == 'sacred-reward-2' and stage['roles'] == [0] for stage in kirin['nativeStages'])
    assert any(stage['label'] == 'sacred-items-collected' and stage['roles'] == [4, 0] and stage['nativeEvent'][6] == 0 for stage in kirin['nativeStages'])
    accept(3354, 'original-kirin-reward-chain', main_source)
    accept(3354, 'original-kirin-reward-chain', source)
    verified[3354]['status'] = 'original-kirin-reward-chain-and-saved-no-duplicate-verified'
    verified[3354]['grants'] = points[3354]['Grants']
    verified[3354]['followupRewards'] = [{'PC': 30093, 'Item': 276, 'Amount': 1, 'Cash': 0, 'Name': '火眼麒麟角'}]

source = BASE+'past-rewards-original-review.json'
if (ROOT / source).is_file():
    native = report(source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits']
    staff = native['staff']
    assert staff['event'] == 4649 and staff['reusedExistingStoryEvidence'] and staff['newGameplayRuns'] == 0
    assert (staff['originalEntry'], staff['grantPC'], staff['item']) == (34080, 34082, 195)
    main_source = AUDIO+'campaign-temple-past-1.json'
    value = report(main_source)
    assert staff['report']['path'] == main_source and staff['report']['sha256'] == evidence[main_source]
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and not value['resumedTestRoute']
    assert value['sourceCheckpoint'] == 'campaign-dali-kirin-state.json' and value['sourceCheckpointSHA256'] == value['rootStage19SHA256'] == hashlib.sha256((ROOT / (AUDIO+'campaign-dali-kirin-state-1.json')).read_bytes()).hexdigest()
    checks, proof = value['checks'], value['proof']
    assert checks['StaffRepeatInputs'] == 3 and checks['InventoryWithStaff']['天蛇杖'] == 1 and checks['InventoryBefore'].get('天蛇杖', 0) == checks['InventoryAfter'].get('天蛇杖', 0) == 0
    assert proof['Battles'] == proof['Wins'] == 22 and proof['ScriptedFinishes'] == 0 and proof['Saves'] == len(proof['Saved']) == 164 and all(proof['Saved'].values())
    assert proof['Seen']['staff-delivered'] and proof['Seen']['queen-love']
    for stage in staff['nativeStages']:
        assert proof['Saved'][stage['label']] and hashlib.sha256((ROOT / stage['state']['path']).read_bytes()).hexdigest() == stage['state']['sha256']
    assert any(stage['label'] == 'staff-no-duplication' and stage['nativeEvent'][6] == 0 for stage in staff['nativeStages'])
    assert any(stage['label'] == 'field-254-step-30' and stage['roles'] == [0, 3] for stage in staff['nativeStages'])
    accept(4649, 'original-staff-pickup-and-delivery', main_source)
    accept(4649, 'original-staff-pickup-and-delivery', source)
    verified[4649]['status'] = 'original-staff-pickup-delivery-and-saved-no-duplicate-verified'
    verified[4649]['grants'] = points[4649]['Grants']

source = BASE+'yuhang-trial-original-review.json'
if (ROOT / source).is_file():
    native = report(source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits']
    old = native['oldYuhang']
    assert old['events'] == [4674, 4663] and old['reusedExistingStoryEvidence'] and old['newMainStoryRuns'] == 0
    main_source = AUDIO+'campaign-old-yuhang-1.json'
    value = report(main_source)
    assert old['report']['path'] == main_source and old['report']['sha256'] == evidence[main_source]
    assert value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and not value['resumedTestRoute']
    assert value['sourceCheckpoint'] == 'campaign-temple-past-state.json' and value['sourceCheckpointSHA256'] == value['rootStage20SHA256'] == hashlib.sha256((ROOT / (AUDIO+'campaign-temple-past-state-1.json')).read_bytes()).hexdigest()
    checks, proof = value['checks'], value['proof']
    assert checks['WrongUseChecked'] and checks['ClothUsed'] and checks['RepeatInputs'] == 3
    assert checks['InventoryWithCloth']['凤纹手绢'] == checks['InventoryAfterWrongUse']['凤纹手绢'] == 1 and checks['InventoryAfterCloth'].get('凤纹手绢', 0) == 0
    assert checks['InventoryBeforeExchange']['木剑'] == 1 and checks['InventoryAfter'].get('木剑', 0) == 0 and checks['InventoryAfter']['水灵珠'] == checks['InventoryAfter']['金凤凰蛋壳'] == 1
    assert proof['Battles'] == proof['Wins'] == 8 and proof['ScriptedFinishes'] == 0 and proof['Saves'] == len(proof['Saved']) == 88 and all(proof['Saved'].values())
    assert all(proof['Seen'][label] for label in ['cloth-granted', 'cloth-recognized', 'toy-chest', 'wooden-sword-request', 'exchange', 'water-orb', 'egg-shell', 'anu-reunion'])
    for stage in old['nativeStages']:
        assert proof['Saved'][stage['label']] and hashlib.sha256((ROOT / stage['state']['path']).read_bytes()).hexdigest() == stage['state']['sha256']
    repeat_source = AUDIO+'old-yuhang-rewards-v1-result-1.json'
    repeat = report(repeat_source)
    assert repeat['passed'] and repeat['event'] == 4674 and repeat['repeatConversations'] == 3 and not repeat['nativeEdits']
    assert repeat['inventoryBefore'] == repeat['inventoryAfter'] == {'凤纹手绢': 1, '木剑': 1} and repeat['proof']['Saves'] == 4 and repeat['proof']['Battles'] == 0 and all(n > 0 for n in repeat['readingsPerRepeat'])
    assert native['newClothRepeat']['report']['sha256'] == evidence[repeat_source]
    for event in [4674, 4663]:
        accept(event, 'old-yuhang-original-quest-chain', main_source)
        accept(event, 'old-yuhang-original-quest-chain', source)
        verified[event]['grants'] = points[event]['Grants']
    accept(4674, 'old-yuhang-original-cloth-repeat', repeat_source)
    verified[4674]['status'] = 'original-cloth-use-and-saved-no-replay-verified'
    verified[4663]['status'] = 'original-wooden-sword-exchange-and-saved-no-duplicate-verified'
    verified[4663]['consumedItems'] = [{'PC': 34918, 'Item': 166, 'Amount': 1, 'Name': '木剑'}]
    verified[4663]['followupRewards'] = [{'PC': 30511, 'Item': 275, 'Amount': 1, 'Cash': 0, 'Name': '金凤凰蛋壳'}]

economy_sources = {2003: AUDIO+'capital-funding-v1-result-1.json', 2086: AUDIO+'capital-merchant-v1-sugar-result-1.json', 2225: AUDIO+'capital-merchant-v1-wine-result-1.json'}
if any((ROOT / source).is_file() for source in economy_sources.values()):
    native_source = BASE+'capital-side-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits']
    assert {row['event'] for row in native['records']} == {1818, 2035, 2336, 2003, 2086, 2225}
    for event, source in economy_sources.items():
        value = report(source)
        assert value['event'] == event and value['verificationVersion'] == 1 and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
        assert not value['nativeEdits'] and value['resourceUploads'] == 0 and value['proof']['Battles'] == 0
        if event == 2003:
            assert value['kind'] == 'original-capped-funding' and value['grants'] == 3 and value['grantAmount'] == 30000
            assert value['shopItem'] == 183 and value['shopPrice'] == 12000 and value['shopQuantity'] == 7 and len(value['steps']) == 14
            assert value['inventoryFinal']['金童剑']-value['inventoryInitial'].get('金童剑', 0) == 7
            assert value['cashFinal']-value['cashInitial'] == 3*30000-7*12000
            for grant in [1, 2, 3]:
                reward = next(row for row in value['steps'] if row['label'] == f'grant-{grant}')
                high = next(row for row in value['steps'] if row['label'] == f'grant-{grant}-high-balance')
                assert reward['delta'] == reward['cashAfter']-reward['cashBefore'] == 30000 and reward['receipt'] == '得三万文钱'
                assert high['cashBefore'] >= 30000 and high['cashAfter'] == high['cashBefore']
                spent = next(row for row in value['steps'] if row['label'] == f'spend-{grant}')
                assert spent['cashAfter'] < 30000 and len(spent['purchases']) == (3 if grant == 3 else 2)
                assert spent['cashBefore']-spent['cashAfter'] == len(spent['purchases'])*12000
            for label in ['grant-2-declined', 'grant-3-declined']+[f'funding-exhausted-{n}' for n in [1, 2, 3]]:
                step = next(row for row in value['steps'] if row['label'] == label)
                assert step['cashBefore'] < 30000 and step['cashBefore'] == step['cashAfter'] and step['delta'] == 0
            status = 'capped-funding-and-saved-no-replay-verified'
        else:
            price, item = (30, '糖葫芦') if event == 2086 else (100, '酒')
            assert value['kind'] == 'original-paid-repeatable-purchase' and value['price'] == price and value['item'] == item and value['purchases'] == 2
            assert value['cashInitial']-value['cashFinal'] == 2*price and value['inventoryFinal'][item]-value['inventoryInitial'].get(item, 0) == 2
            assert [row['choice'] for row in value['steps']] == ['否', '是', '是', '否'] and [row['delta'] for row in value['steps']] == [0, -price, -price, 0]
            status = 'paid-repeatable-transaction-and-saved-no-replay-verified'
        for step in value['steps']:
            if step['label'].startswith('spend-'):
                continue
            assert step['cashAfter']-step['cashBefore'] == step['delta']
            for stage in ['before', 'after']+(['choice'] if step['choice'] else [])+(['reading'] if step['receipt'] else []):
                assert value['proof']['Saved'][step['label']+'-'+stage]
        accept(event, 'capital-original-economy', source)
        accept(event, 'capital-original-economy', native_source)
        verified[event]['status'] = status
        verified[event]['transactionRules'] = {'maxGrants': 3, 'grantAmount': 30000, 'subsequentBalanceBelow': 30000} if event == 2003 else {'repeatable': True, 'price': price, 'item': item, 'verifiedPurchases': 2}

quest_sources = [AUDIO+'capital-quests-v1-letter-accept-result-1.json', AUDIO+'capital-quests-v1-letter-decline-result-1.json', AUDIO+'capital-quests-v1-bundle-result-1.json']
if any((ROOT / source).is_file() for source in quest_sources):
    native_source = BASE+'capital-quests-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits']
    assert len(native['records']) == 3 and {row['event'] for row in native['records']} == {2166, 2333}
    for source in quest_sources:
        value = report(source)
        event = value['event']
        assert event in [2166, 2333] and value['verificationVersion'] == 1 and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999'
        assert not value['nativeEdits'] and value['resourceUploads'] == 0 and value['repeatTalks'] == 3
        assert value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == 0
        items = value['inventoryChecks']
        if event == 2166:
            assert points[event]['SourceItem'] == 287 and points[event]['Grants'][0]['Item'] == 288
            assert value['kind'] == 'original-letter-jade-chain' and value['completeChain'] and value['cashInitial'] == value['cashFinal'] and value['proof']['Battles'] == 0
            assert value['reconsideredAfterDeclines'] == (value['choice'] == '否')
            assert items['letter-wrong-use-retained'] == {'情书': 1, '玉佩': 0}
            assert items['jade-wrong-use-retained'] == items['jade-hui-retained'] == {'情书': 0, '玉佩': 1}
            assert items['jade-returned'] == value['finalInventory'] == {'情书': 0, '玉佩': 0}
            assert all(value['readings'][token] for token in ['letter-read', 'jade-offer', 'jade-not-hui', 'jade-is-de', 'xiang-confession'])
            if value['choice'] == '否':
                assert all(items[f'xiang-declined-repeat-{n}'] == {'情书': 0, '玉佩': 0} for n in [1, 2, 3])
                assert items['xiang-reconsidered'] == {'情书': 0, '玉佩': 1}
        else:
            assert points[event]['SourceItem'] == 271 and points[event]['Grants'][0]['Item'] == 270
            assert value['kind'] == 'original-corpse-bundle-gourd-spell-chain' and value['sourceCorpseEvent'] == 1845
            assert value['learnedMagic'] == 384 and not value['spellBefore'] and value['spellAfter'] and value['spellAfterRepeat']
            assert items['corpse-bundle-collected'] == items['bundle-wrong-use-retained'] == {'布包': 1, '紫金葫芦': 0}
            assert items['gourd-received'] == value['finalInventory'] == {'布包': 0, '紫金葫芦': 1}
            assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges'])
            assert len(value['battleCashChanges']) == value['proof']['Battles']
            assert all(row['cashAfter']-row['cashBefore'] == row['rewardCash'] >= 0 for row in value['battleCashChanges'])
        assert value['proof']['Saves'] == len(value['proof']['Saved']) >= 20 and all(value['proof']['Saved'].values())
        accept(event, 'capital-original-item-use-quests', source)
        accept(event, 'capital-original-item-use-quests', native_source)
        verified[event]['status'] = 'item-use-quest-and-saved-no-duplicate-verified'
        verified[event]['sourceItem'] = points[event]['SourceItem']
        if event == 2333:
            verified[event]['learnedSpells'] = [384]

source = BASE + 'hidden-text-route-review.json'
review = report(source)
assert review['diagnosticOnly'] and not review['nativeEdits']
assert not review['bounty']['reachableSearchStances'] and review['bounty']['classification'].startswith('original inaccessible')
assert not review['chemical']['reachableSearchStances'] and review['chemical']['classification'].startswith('original inaccessible')
assert review['chemicalAfterCollection'] is not None
for event, key in [(1519, 'bounty'), (2576, 'chemical')]:
    assert event in points and event not in verified
    row = review[key]
    assert row['event'] == event and not row['normalPickupVerified']
    preserved[event] = {'event': event, 'scene': points[event]['Scene'], 'map': points[event]['Map'], 'position': row['eventWorldXY'], 'status': 'original-unreachable-preserved', 'actuallyObtained': False, 'evidence': [source]}

source = BASE + 'cave-original-unreachable.json'
if (ROOT / source).is_file():
    cave = report(source)
    assert cave['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
    assert not cave['nativeEdits'] and cave['readyForProgress'] and cave['reference']['url'] == 'https://gamegene.cn/wiki/359'
    assert {row['event'] for row in cave['records']} == {666, 667, 670, 714, 715, 744}
    for row in cave['records']:
        event = row['event']
        assert event not in verified and event not in preserved and event in points
        assert row['classification'] == 'original-unreachable-preserved' and not row['normalPickupVerified']
        assert row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter'] and not row['reachableSearchStances']
        assert row['allSceneTransitions'] == len(row['entrances']) > 0 and row['referenceQuote']
        assert [points[event]['X'], points[event]['Y']] == row['position'] and points[event]['Scene'] == row['scene']
        preserved[event] = {'event': event, 'scene': row['scene'], 'map': row['map'], 'position': row['position'], 'status': row['classification'], 'actuallyObtained': False, 'evidence': [source]}

source = BASE + 'rice-shop-original-unreachable.json'
if (ROOT / source).is_file():
    rice = report(source)
    assert rice['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
    assert not rice['nativeEdits'] and rice['readyForProgress'] and rice['reference']['url'] == 'https://gamegene.cn/wiki/360'
    assert rice['sssSHA256'] == catalog['sssSHA256'] and rice['catalogSHA256'] == hashlib.sha256(catalog_path.read_bytes()).hexdigest()
    assert rice['allSceneTransitions'] == len(rice['entrances']) == 1
    assert rice['graph']['ignoringExitPositions'] == 12414 and rice['graph']['sameScenePositions'] == 342
    assert rice['exit']['event'] == 844 and rice['exit']['state'] == 1 and rice['exit']['radiusExclusive'] == 48
    audit_source = rice['result']['path']
    audit = report(audit_source)
    assert evidence[audit_source] == rice['result']['sha256']
    assert audit['verificationVersion'] == 6 and audit['auditOnly'] and audit['claimed'] == []
    assert audit['auditedEvents'] == list(range(861, 871)) and audit['proof']['Saved']['rice-shop-records-preserved']
    assert audit['proof']['Saves'] >= 1 and audit['cashInitial'] == audit['cashFinal'] and audit['partyInitial'] == audit['party']
    assert {row['event'] for row in rice['records']} == set(range(861, 871))
    for row in rice['records']:
        event = row['event']
        assert event in points and event not in verified and event not in preserved
        assert row['classification'] == 'original-unreachable-preserved' and not row['normalPickupVerified']
        assert row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter'] and not row['reachableSearchStances']
        assert row['ignoringExitSearchStances'] and row['originalTileDWORD'] == 0 and row['referenceQuote'] == rice['reference']['quote']
        assert row['position'] == [points[event]['X'], points[event]['Y']] and row['scene'] == points[event]['Scene'] == 51
        preserved[event] = {'event': event, 'scene': row['scene'], 'map': row['map'], 'position': row['position'], 'status': row['classification'], 'actuallyObtained': False, 'evidence': [source, audit_source]}

source = BASE+'forest-original-unreachable.json'
if (ROOT / source).is_file():
    forest = report(source)
    assert forest['readyForProgress'] and not forest['nativeEdits'] and forest['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
    assert len(forest['records']) == 1 and len(forest['graph']['climbs']) == len(forest['persistentClimbNativeChecks']) == 52 and len(forest['graph']['lifts']) == 2
    assert forest['graph']['retainsOriginalMandatoryClimbTriggers'] and forest['reference']['url'] == 'https://gamegene.cn/wiki/364' and forest['reference']['quote'] == '7.灵山仙芝（拿不到）'
    row = forest['records'][0]
    event = row['event']
    assert event == 3318 and event not in verified and event not in preserved and 3314 in verified
    assert row['classification'] == 'original-unreachable-preserved' and not row['actuallyObtained'] and not row['normalPickupVerified'] and not row['reachableSearchStances']
    assert row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter'] and row['allSceneTransitions'] == 2 and row['position'] == [points[event]['X'], points[event]['Y']]
    assert all(evidence[item['path']] == item['sha256'] for item in forest['actualOtherRewards'])
    preserved[event] = {'event': event, 'scene': row['scene'], 'map': row['map'], 'position': row['position'], 'status': row['classification'], 'actuallyObtained': False, 'evidence': [source]}

source = BASE+'peach-original-unreachable.json'
if (ROOT / source).is_file():
    peach = report(source)
    assert peach['readyForProgress'] and not peach['nativeEdits'] and peach['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
    assert peach['sssSHA256'] == catalog['sssSHA256'] and peach['reference']['url'] == 'https://gamegene.cn/wiki/364'
    assert hashlib.sha256((ROOT / peach['reference']['path']).read_bytes()).hexdigest() == peach['reference']['sha256']
    assert len(peach['records']) == 2 and {row['event'] for row in peach['records']} == {3190, 3212}
    assert [(row['scene'], row['optimisticWalkablePositions']) for row in peach['graphs']] == [(190, 3571), (191, 2395)]
    assert 3211 in verified and 3215 in verified
    for row in peach['records']:
        event = row['event']
        assert event in points and event not in verified and event not in preserved
        assert row['classification'] == 'original-unreachable-preserved' and not row['actuallyObtained'] and not row['normalPickupVerified'] and not row['reachableSearchStances']
        assert row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter'] and row['nativeEventBefore'][1:13] == row['nativeEventAfter'][1:13]
        assert row['allSceneTransitions'] == 2 and row['position'] == [points[event]['X'], points[event]['Y']] and row['scene'] == points[event]['Scene']
        assert row['referenceQuote'] == ('2.女娲石（拿不到）' if event == 3190 else '2.蜂王蜜（拿不到）') and row['referenceQuote'] in (ROOT / peach['reference']['path']).read_text()
        graph = next(graph for graph in peach['graphs'] if graph['scene'] == row['scene'])
        assert graph['ignoresAllDynamicCollisions'] and graph['evenAllowsWalkingAcrossSceneExitAreas'] and len(graph['entrances']) == len(graph['exits']) == 2
        for key in ['before', 'after', 'otherSceneRewards']:
            assert hashlib.sha256((ROOT / row[key]['path']).read_bytes()).hexdigest() == row[key]['sha256']
        preserved[event] = {'event': event, 'scene': row['scene'], 'map': row['map'], 'position': row['position'], 'status': row['classification'], 'actuallyObtained': False, 'evidence': [source]}

source = BASE+'trial-deep-original-unreachable.json'
if (ROOT / source).is_file():
    deep = report(source)
    assert deep['readyForProgress'] and not deep['nativeEdits'] and deep['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa'
    assert len(deep['records']) == 1 and deep['reference']['url'] == 'https://gamegene.cn/wiki/365' and deep['reference']['quote'] == '9.隐蛊（拿不到）'
    assert deep['graph']['optimisticWalkablePositions'] == 2733 and deep['graph']['impossibleShadowedEntranceWalkablePositions'] == 2853 and deep['graph']['ignoresAllDynamicCollisions'] and deep['graph']['evenAllowsWalkingAcrossAllTeleportTriggerAreas']
    assert [(row['firstEvent'], row['shadowedEvent']) for row in deep['shadowedPortals']] == [(3949, 3954), (3950, 3955)] and deep['explicitStateModeAutoScriptWrites'] == []
    assert hashlib.sha256((ROOT / deep['priorityWitness']['path']).read_bytes()).hexdigest() == deep['priorityWitness']['sha256']
    row = deep['records'][0]
    event = row['event']
    assert event == 4000 and event in points and event not in verified and event not in preserved and row['position'] == [points[event]['X'], points[event]['Y']]
    assert row['classification'] == 'original-unreachable-preserved' and not row['actuallyObtained'] and not row['normalPickupVerified'] and not row['reachableSearchStances']
    assert row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter'] and row['allRawIncomingPlacements'] == 28 and row['allActuallyTriggerableIncomingPlacements'] == 26
    for stage in row['nativeStages']:
        assert hashlib.sha256((ROOT / stage['state']['path']).read_bytes()).hexdigest() == stage['state']['sha256']
    preserved[event] = {'event': event, 'scene': row['scene'], 'map': row['map'], 'position': row['position'], 'status': row['classification'], 'actuallyObtained': False, 'evidence': [source]}

bottom_sources = [AUDIO+f'trial-bottom-v1-{branch}-result-1.json' for branch in ['two', 'three']]
if any((ROOT / source).is_file() for source in bottom_sources):
    native_source = BASE+'trial-bottom-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits'] and len(native['records']) == 12
    assert {row['event'] for row in native['records']} == {3607, 3641, 3642, 3643, 3644, 3645}
    for branch, source, roles, fire_role in zip(['two', 'three'], bottom_sources, [[0, 4], [1, 0, 4]], [4, 1]):
        value = report(source)
        assert not value['nativeEdits'] and value['nativeBuildID'] == native['nativeBuildID'] and value['branch'] == branch and value['roles'] == roles and value['fireRole'] == fire_role
        assert value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == 0 and value['proof']['Saves'] == len(value['proof']['Saved'])
        assert {row['event'] for row in value['claimed']} == {3607, 3641, 3642, 3643, 3644, 3645}
        assert [row['event'] for row in value['boatTrips']] == [3607, 3614, 3630, 3631, 3632, 3634, 3635, 3635] and value['boatSaves'] == 8
        assert value['inventory'].get('芦苇漂', 0) == 0 and all(value['inventory'][name]-value['inventoryInitial'].get(name, 0) == 1 for name in ['玄冥宝刀', '冥蛇杖', '无尘剑', '金罡珠', '魅影神靴'])
        assert all(value['proof']['Saved'][label] for label in ['reed-invalid-selected', 'reed-invalid-finished', 'reed-bank-selected', 'reed-bank-placed', 'reed-first-crossed', 'complete'])
        branch_review = next(row for row in native['branches'] if row['branch'] == branch)
        assert branch_review['roles'] == roles and branch_review['fireRole'] == fire_role and len(branch_review['gods']) == 5 and len(branch_review['transits']) == 8
        claimed(value, source, 'trial-bottom-original-rewards')
        for row in value['claimed']:
            accept(row['event'], 'trial-bottom-original-rewards', native_source)

final_sources = [AUDIO+f'regional-hidden-v19-{scene}-result-1.json' for scene in [280, 287, 288, 289, 290, 291, 292, 293, 294]]
if any((ROOT / source).is_file() for source in final_sources):
    native_source = BASE+'final-maze-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits'] and len(native['records']) == 48
    plan_source = BASE+'final-maze-rewards-plan.json'
    plan = report(plan_source)
    assert plan['diagnosticOnly'] and not plan['originalDataEdits'] and {event for case in plan['cases'] for event in case['unplannedEvents']} == {5071, 5072, 5073, 5074, 5077}
    for source in final_sources:
        value = report(source)
        scene = value['scene']
        case = next(row for row in plan['cases'] if row['scene'] == scene)
        assert value['verificationVersion'] == 19 and value['authorityFixture'] and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['combatAllowed']
        assert value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == value['preCollectionBattles'] == 0
        assert [row['event'] for row in value['claimed']] == case['events'] and {row['event'] for row in native['records'] if row['scene'] == scene} == set(case['events'])
        assert value['sourceCheckpoint'] == case['sourceCheckpoint'] and value['sourceSHA256'] == case['sourceSHA256'] and value['routePlanSHA256'] == evidence[plan_source]
        assert len(value['trialRoute']) == value['trialPortalSteps'] == case['portalSteps'] and value['trialEarthReturns'] == 0
        inventory = dict(value['inventoryInitial'])
        for reward in value['claimed']:
            assert all(value['proof']['Saved'][f"reward-{reward['event']}-{label}"] for label in ['unpicked', 'notice', 'collected', 'repeated'])
            for grant in reward['grants']:
                inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
        for grant in value['battleItemGrants']:
            assert grant['endScript'] == 42828 and grant['item'] == 152 and grant['amount'] > 0
            inventory['傀儡虫'] = inventory.get('傀儡虫', 0)+grant['amount']
        assert inventory == value['inventory'] and inventory['土灵珠'] == 1
        assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges']) and len(value['battleCashChanges']) == value['proof']['Battles']
        claimed(value, source, 'final-maze-original-rewards')
        for row in value['claimed']:
            accept(row['event'], 'final-maze-original-rewards', native_source)
    empty_source = AUDIO+'final-maze-empty-v1-result-1.json'
    empty = report(empty_source)
    assert empty['event'] == 5049 and empty['event'] not in points and empty['originalItemID'] == 0 and empty['itemBefore'] == empty['itemAfter'] and empty['repeatSearches'] == 3
    unreachable_source = BASE+'final-maze-original-unreachable.json'
    unreachable = report(unreachable_source)
    assert unreachable['readyForProgress'] and not unreachable['nativeEdits'] and unreachable['nativeBuildID'] == native['nativeBuildID']
    assert {row['event'] for row in unreachable['records']} == {4846, 4847, 4917} and len(unreachable['unverifiedRecords']) == 5
    assert all(not row['readyForProgress'] for row in unreachable['unverifiedRecords'])
    for row in unreachable['records']:
        event = row['event']
        assert event in points and event not in verified and event not in preserved and row['position'] == [points[event]['X'], points[event]['Y']]
        assert row['classification'] == 'original-unreachable-preserved' and not row['actuallyObtained'] and not row['normalPickupVerified'] and row['reachableSearchStances'] == []
        assert row['nativeRecordPreservedBefore'] and row['nativeRecordPreservedAfter'] and row['referenceQuote'] in (ROOT / unreachable['reference']['path']).read_text()
        preserved[event] = {'event': event, 'scene': row['scene'], 'map': row['map'], 'position': row['position'], 'status': row['classification'], 'actuallyObtained': False, 'evidence': [unreachable_source]}

tower_scope = AUDIO+'scopes/tower-remainder-v1/'
source = tower_scope+'regional-hidden-v20-167-result-1.json'
if (ROOT / source).is_file():
    native_source = BASE+'tower-remainder-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits'] and len(native['records']) == 11
    assert native['pendingOriginalEvent']['event'] == 2763 and not native['pendingOriginalEvent']['readyForProgress'] and not native['pendingOriginalEvent']['actuallyObtained']
    plan_source = BASE+'tower-remainder-plan.json'
    plan = report(plan_source)
    assert plan['diagnosticOnly'] and not plan['originalDataEdits'] and plan['case']['unplannedEvents'] == [2763]
    value = report(source)
    assert value['verificationVersion'] == 20 and value['scene'] == 167 and value['candidate'] == 'audio' and value['developmentProfile'] == 'hp-9999' and value['authorityFixture'] and value['combatAllowed']
    assert value['sourceCheckpoint'] == plan['case']['sourceCheckpoint'] and value['sourceSHA256'] == plan['case']['sourceSHA256'] and value['routePlanSHA256'] == evidence[plan_source]
    assert [row['event'] for row in value['claimed']] == plan['case']['events'] and {row['event'] for row in native['records']} == set(plan['case']['events'])
    assert value['proof']['Battles'] == value['proof']['Wins'] and value['proof']['ScriptedFinishes'] == value['preCollectionBattles'] == 0
    inventory = dict(value['inventoryInitial'])
    for row in value['claimed']:
        for grant in row['grants']:
            inventory[grant['Name']] = inventory.get(grant['Name'], 0)+grant['Amount']
    assert inventory == value['inventory'] and value['battleItemGrants'] == value['wormAppearances'] == []
    assert value['cashFinal']-value['cashInitial'] == sum(row['rewardCash'] for row in value['battleCashChanges']) and len(value['battleCashChanges']) == value['proof']['Battles']
    claimed(value, source, 'tower-seventh-remainder')
    for row in value['claimed']:
        accept(row['event'], 'tower-seventh-remainder', native_source)
    empty = report(tower_scope+'tower-remainder-empty-v1-result-1.json')
    assert empty['event'] == 2764 and 2764 not in points and empty['originalItemID'] == 0 and empty['originalGrantPC'] == 37482 and empty['itemBefore'] == empty['itemAfter'] and empty['repeatSearches'] == 3
    for event in [2677, 2408]:
        repeat_source = tower_scope+f'shu-reward-repeat-v1-{event}-result-1.json'
        repeat = report(repeat_source)
        assert repeat['event'] == event and repeat['existingEarnedBranch'] and not repeat['nativeEdits'] and repeat['repeatSearches'] == 3
        assert repeat['inventoryBefore'] == repeat['inventoryAfter'] and repeat['inventoryAfter'][repeat['item']] == 1 and repeat['proof']['Saves'] == 4 and repeat['proof']['Battles'] == 0
        original = next(row for row in native['storyRewards'] if row['event'] == event)
        original_source = original['reusedCompleteStory']['path']
        parent = report(original_source)
        assert parent['passed'] and not parent['resumedTestRoute'] and evidence[original_source] == original['reusedCompleteStory']['sha256']
        accept(event, 'shu-original-story-rewards', original_source)
        accept(event, 'shu-original-story-rewards', native_source)
        accept(event, 'shu-original-story-repeat', repeat_source)
        verified[event]['grants'] = points[event]['Grants']
        if event == 2408:
            assert repeat['inventoryAfter']['天师道袍'] == 1 and original['originalBattleFollowup']['endScript'] == 42420
            verified[event]['followupRewards'] = [{'PC': 42420, 'Item': 216, 'Amount': 1, 'Cash': 0, 'Name': '天师道袍'}]

misc_scope = AUDIO+'scopes/misc-rewards-v1/'
misc_sources = [misc_scope+f'misc-market-v1-{event}-result-1.json' for event in [129, 438, 846, 847]]
if any((ROOT / source).is_file() for source in misc_sources):
    native_source = BASE+'misc-rewards-original-review.json'
    native = report(native_source)
    assert native['nativeBuildID'] == '2bacd1cb805bdda815f581f9b9d893ddbab6e73df1827a5bb0290f98201b13aa' and not native['nativeEdits']
    assert {row['event'] for row in native['merchants']} == {129, 438, 846, 847} and {row['event'] for row in native['pickups']} == {399, 798}
    for source, event, price, decline_cost, item in zip(misc_sources, [129, 438, 846, 847], [25, 20, 100, 100], [0, 20, 0, 0], ['水果', '糖葫芦', '糯米', '糯米']):
        value = report(source)
        assert value['event'] == event and not value['nativeEdits'] and value['kind'] == 'original-paid-repeatable-merchant'
        assert value['price'] == price and value['declineCost'] == decline_cost and value['chargeBeforeChoice'] == (event == 438) and value['purchases'] == value['declines'] == 2
        assert [row['choice'] for row in value['steps']] == ['否', '是', '是', '否'] and [row['delta'] for row in value['steps']] == [-decline_cost, -price, -price, -decline_cost]
        assert value['cashFinal']-value['cashInitial'] == -2*price-2*decline_cost and value['inventoryAfter'][item]-value['inventoryBefore'].get(item, 0) == 2 and value['proof']['Battles'] == 0
        for row in value['steps']:
            assert all(value['proof']['Saved'][row['label']+'-'+label] for label in ['before', 'search-pose', 'reading', 'choice', 'after'])
        merchant = next(row for row in native['merchants'] if row['event'] == event)
        assert merchant['actualSearchTargetVerified'] and len(merchant['steps']) == 4 and merchant['rawOriginalChargeBeforeChoice'] == (event == 438)
        accept(event, 'early-original-merchants', source)
        accept(event, 'early-original-merchants', native_source)
        verified[event]['status'] = 'paid-repeatable-transaction-and-saved-no-replay-verified'
        verified[event]['transactionRules'] = {'repeatable': True, 'price': price, 'item': item, 'declineCost': decline_cost, 'chargeBeforeChoice': event == 438, 'verifiedPurchases': 2, 'verifiedDeclines': 2}
    for event, item in [(399, 173), (798, 285)]:
        source = misc_scope+f'misc-pickup-v1-{event}-result-1.json'
        value = report(source)
        assert value['event'] == event and value['itemID'] == item and not value['nativeEdits'] and value['notification'] and value['repeatSearches'] == 3
        assert value['amountBefore'] == 0 and value['amountAfter'] == value['amountAfterRepeat'] == 1 and value['proof']['Battles'] == 0
        assert all(value['proof']['Saved'][label] for label in ['unpicked', 'notice', 'collected', 'repeated'])
        accept(event, 'misc-original-pickups', source, value['position'], value['scene'])
        accept(event, 'misc-original-pickups', native_source)
        verified[event]['grants'] = points[event]['Grants']

community_source = BASE+'win98-reference-v1/community-patch-review.json'
if (ROOT / community_source).is_file():
    community = report(community_source)
    assert community['readyForProgress'] and community['scopeStartsWithOriginalWindows98'] and community['scopeArchivesAreComparisonOnly'] and not community['originalDataEdits'] and community['actuallyObtained'] == 0
    assert community['originalSSS'] == catalog['sssSHA256']
    note = ROOT / community['source']['decodedPath']
    assert hashlib.sha256(note.read_bytes()).hexdigest() == community['source']['decodedSHA256']
    assert hashlib.sha256((ROOT / community['source']['rawPath']).read_bytes()).hexdigest() == community['source']['rawSHA256']
    text = note.read_text()
    assert {row['event'] for row in community['records']} == {145, 191, 1196, 2173, 2763, 3537, 5071, 5072, 5073, 5074, 5077} and len(community['records']) == 11
    for row in community['records']:
        event = row['event']
        point = points[event]
        assert event not in verified and event not in preserved
        assert row['scene'] == point['Scene'] and row['map'] == point['Map'] and row['position'] == [point['X'], point['Y']]
        assert row['grant'] == point['Grants'][0]['Item'] and row['quote'] in text
        assert row['classification'] == 'original-unreachable-preserved' and not row['actuallyObtained'] and not row['originalPreMoveDoorRoute'] and row['patchedSearchStances'] and row['originalSnapshots']
        assert row['differsInPosition'] if row['map'] != 192 else not row['originalTerrainSearchStances']
        for source in row['originalSnapshots']:
            assert hashlib.sha256((ROOT / source['path']).read_bytes()).hexdigest() == source['sha256']
            assert source['eventRow'][:14] + source['eventRow'][15:] == row['originalEvent'][:14] + row['originalEvent'][15:]
        preserved[event] = {'event': event, 'scene': row['scene'], 'map': row['map'], 'position': row['position'], 'status': row['classification'], 'actuallyObtained': False, 'evidence': [community_source]}

sage_source = BASE+'last-hidden-v1/sage-cross-archive.json'
if (ROOT / sage_source).is_file():
    sage = report(sage_source)
    event = sage['event']
    point = points[event]
    assert event == 2947 and event not in verified and event not in preserved
    assert sage['readyForProgress'] and not sage['nativeEdits'] and not sage['actuallyObtained'] and sage['classification'] == 'original-unreachable-preserved'
    assert sage['lengthOfMatchingCutscene'] == 59 and sage['sameDialogueLineCount'] == 27
    assert sage['windows']['ordinaryBananaEvents'] == [2932] and sage['windows']['cutsceneBananaEvents'] == [event]
    assert sage['dos']['ordinaryBananaEvents'] == [3086] and sage['dos']['cutsceneBananaEvents'] == []
    assert sage['scene'] == point['Scene'] and sage['map'] == point['Map'] and sage['position'] == [point['X'], point['Y']]
    for reference in sage['sources']:
        assert hashlib.sha256((ROOT / reference['path']).read_bytes()).hexdigest() == reference['sha256']
    preserved[event] = {'event': event, 'scene': sage['scene'], 'map': sage['map'], 'position': sage['position'], 'status': sage['classification'], 'actuallyObtained': False, 'evidence': [sage_source]}

tomb_source = BASE+'last-hidden-v1/tomb-upper-reward.json'
if (ROOT / tomb_source).is_file():
    tomb = report(tomb_source)
    event = tomb['event']
    point = points[event]
    assert event == 1172 and event not in verified and event not in preserved
    assert tomb['readyForProgress'] and not tomb['nativeEdits'] and not tomb['actuallyObtained'] and tomb['classification'] == 'original-unreachable-preserved'
    assert tomb['scene'] == point['Scene'] and tomb['map'] == point['Map'] and tomb['position'] == [point['X'], point['Y']] and tomb['grant'] == point['Grants'][0]['Item']
    assert len(tomb['originalEntries']) == 12 and not tomb['reachableOriginalSearchStances'] and tomb['illegalPaddedRoute']['firstOutOfBounds'] == [1856, 1648]
    assert tomb['teleport']['exitScene'] == 65 and tomb['unchangedSnapshot']['originalRecordUnchanged']
    assert tomb['independentMap']['offset'] == [128, 128] and tomb['independentMap']['target']['redAnnotationPixels'] == 0
    assert len(tomb['independentMap']['controls']) == 6 and all(control['redAnnotationPixels'] >= 300 and control['reachableOriginalSearchStances'] for control in tomb['independentMap']['controls'])
    for reference in tomb['sources']:
        assert hashlib.sha256((ROOT / reference['path']).read_bytes()).hexdigest() == reference['sha256']
    preserved[event] = {'event': event, 'scene': tomb['scene'], 'map': tomb['map'], 'position': tomb['position'], 'status': tomb['classification'], 'actuallyObtained': False, 'evidence': [tomb_source]}

followup_source = BASE+'hidden-followup-candidates.json'
if (ROOT / followup_source).is_file():
    followup = report(followup_source)
    assert followup['additionalCandidates'] == 4 and followup['priorCandidates'] == 735 and followup['combinedCandidates'] == 739
    assert {row['Event'] for row in followup['candidates']} == {85,192,202,434}
    for name, digest in followup['priorCatalogsSHA256'].items():
        assert hashlib.sha256((ROOT / (BASE + name)).read_bytes()).hexdigest() == digest
    cases = [(85,'script-swap-boots-v1','script-swap-v1-85'), (192,'script-swap-doctor-v3','script-swap-v1-192'), (202,'followup-rewards-v1','followup-v1-202'), (434,'followup-rewards-v1','followup-v1-434')]
    for event, scope, prefix in cases:
        folder = AUDIO+'scopes/'+scope+'/'
        source = folder+prefix+'-result-1.json'
        if not (ROOT / source).is_file():
            continue
        row = report(source)
        point = points[event]
        assert event == row['event'] and point['Scene'] == row['scene'] and event not in verified and event not in preserved
        assert not row['nativeEdits'] and row['candidate'] == 'audio' and row['developmentProfile'] == 'hp-9999' and row['resourceUploads'] == 0
        assert row['itemID'] == point['Grants'][0]['Item'] and row['item'] == point['Grants'][0]['Name']
        assert row['amountAfter'] == row['amountAfterRepeat'] == row['amountBefore'] + 1 and row['repeatSearches'] == 3
        assert row['cashBefore'] - row['cashAfter'] == row['price'] == point['OriginalCharge'] and row['proof']['Battles'] == 0
        assert hashlib.sha256((ROOT / (folder + row['sourceCheckpoint'].removesuffix('.json') + '-1.json')).read_bytes()).hexdigest() == row['sourceSHA256']
        for label in ['unstarted','reward-handover','received','repeated','repeat-1-search-pose','repeat-2-search-pose','repeat-3-search-pose']:
            assert row['proof']['Saved'][label]
            saved = json.loads((ROOT / (folder + prefix+'-'+label+'-state-1.json')).read_bytes())
            assert saved['native'] and saved['public']['scene'] == row['scene']
            if label == 'reward-handover':
                assert saved['public']['cash'] == row['cashAfter'] and saved['public']['dialogue']['text']
        accept(event, 'original-followup-dialogue-rewards', source)
        accept(event, 'original-followup-dialogue-rewards', followup_source)
        verified[event]['grants'] = point['Grants']
        if row['price']:
            verified[event]['status'] = 'paid-once-and-saved-no-replay-verified'
            verified[event]['transactionRules'] = {'repeatable': False, 'price': row['price'], 'item': row['item']}

handled = set(verified) | set(preserved)
remaining = sorted(set(points)-handled)
progress = {'processed': len(handled), 'total': len(points), 'actuallyClaimed': len(verified), 'originalUnreachablePreserved': len(preserved), 'remaining': len(remaining)}
assert progress['processed']+progress['remaining'] == progress['total']
result = {'passed': True, 'scope': 'deduplicated progress from registered accepted pickup/save/no-duplication or original conditional funding/paid transaction save/no-replay evidence and separately source-confirmed original unreachable records; current bounded reward candidate list, conditional branches still under review; this aggregation does not execute new gameplay', 'display': f"已处理：{progress['processed']}/{progress['total']}", **progress, 'catalog': {'path': str(catalog_path.relative_to(ROOT)), 'sha256': hashlib.sha256(catalog_path.read_bytes()).hexdigest(), 'scope': catalog['scope']}, 'verifiedPoints': [verified[key] for key in sorted(verified)], 'preservedPoints': [preserved[key] for key in sorted(preserved)], 'pendingEventIDs': remaining, 'evidence': [{'path': name, 'sha256': digest} for name, digest in evidence.items()]}
if supplements:
    result['catalogSupplements'] = supplements
out = ROOT / (BASE + 'hidden-progress.json')
out.write_text(json.dumps(result, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({'display': result['display'], **progress}, ensure_ascii=False))
