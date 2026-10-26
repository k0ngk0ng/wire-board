"""Analyze native Canvas evidence without calling production timeline code.

Failure cases considered before this analyzer: one-second projectile clumps;
missing or repeated shots; server cooldown quantization; unrecorded initial
events; frame stalls hiding interval errors; vitality jumping to a packet's last
HP; vitality preceding projectile disappearance; distinct impacts merged by a
packet; and initial/reconnect history replayed as new events.

Limits: one room, one stationary target, one tower of each recorded kind; use a
fresh page with probe initialization before navigation. A capture reset during a
running match must set historyBoundary=False and is NOT reconnect evidence.
The default 1000ms playback anchor is checked independently of fitted intervals.
"""
from collections import defaultdict
from statistics import median


def analyze_combat_probe(raw):
    checks, failures = [], []

    def check(name, passed, evidence=None):
        item = {'name': name, 'passed': bool(passed)}
        if evidence is not None:
            item['evidence'] = evidence
        checks.append(item)
        if not passed:
            failures.append(item)

    report = {'passed': False, 'checks': checks, 'failures': failures,
              'limitations': [
                  'Requires one room, one long-lived target and one tower per kind.',
                  'Equal-time impacts may share a frame; different impacts may merge only within a measured frame interval.',
                  'Ratio verification measures native fillRect arguments, not physical display pixels.',
              ]}
    snapshots = sorted(raw.get('snapshots', []), key=lambda item: item['at'])
    frames = sorted(raw.get('frames', []), key=lambda item: item['id'])
    room_ids = {s['roomId'] for s in snapshots if s.get('roomId')}
    check('one room recorded', len(room_ids) <= 1, sorted(room_ids))
    check('server snapshots and actual Canvas frames recorded', bool(snapshots) and len(frames) >= 30,
          {'snapshots': len(snapshots), 'frames': len(frames)})
    if not snapshots or len(frames) < 30:
        return report
    initial = snapshots[0]
    frames = [f for f in frames if f['at'] >= initial['at']]
    check('frames follow initial authoritative snapshot', len(frames) >= 30)
    if len(frames) < 30:
        return report
    check('native viewport geometry recorded', all('viewport' in frame for frame in frames))
    steps = [b['at'] - a['at'] for a, b in zip(frames, frames[1:])]
    ordered_steps = sorted(steps)
    frame_ms = median(steps)
    frame_p95 = ordered_steps[min(len(ordered_steps) - 1, int(len(ordered_steps) * .95))]
    tolerance = max(50., min(100., frame_p95 * 2))
    check('recording has usable frame cadence', frame_ms <= 40 and frame_p95 <= 75,
          {'medianMS': frame_ms, 'p95MS': frame_p95, 'maxMS': max(steps)})
    report['toleranceMS'] = tolerance
    history = raw.get('historyBoundary', True)
    baseline_id = max([0] + [e['id'] for e in initial['events']]) if history else -1
    report['initialCursor'] = {'eventID': baseline_id, 'at': initial['at'],
                               'elapsedMS': initial.get('elapsedMS'), 'historyBoundary': history}
    events = {}
    for snapshot in snapshots:
        for event in snapshot['events']:
            events[event['id']] = event
    events = sorted(events.values(), key=lambda e: e['id'])
    shots = [e for e in events if e['type'] == 'shot' and e['id'] > baseline_id]
    check('shots expose exact logical time and tower identity', bool(shots) and all(
        isinstance(e.get('atMS'), (float, int)) and e.get('towerId') for e in shots))
    if not shots or any('atMS' not in e or not e.get('towerId') for e in shots):
        return report
    target_ids = {e.get('targetId') for e in shots}
    check('single target fixture', len(target_ids) == 1 and None not in target_ids, sorted(target_ids, key=str))
    by_kind = defaultdict(list)
    for e in shots:
        by_kind[e['kind']].append(e)
    check('unique tower identity per projectile kind', all(
        len({e['towerId'] for e in values}) == 1 for values in by_kind.values()),
        {kind: sorted({e['towerId'] for e in values}) for kind, values in by_kind.items()})

    # Consecutive real paint frames form a projectile run. A stalled renderer
    # does not split a run merely because wall time exceeded 16ms.
    runs = defaultdict(list)
    previous = {}
    clumps = []
    target_visible_frames = []

    def visible_draw(frame, draw):
        canvas = frame['canvas']
        viewport = frame.get('viewport', {'width': float('inf'), 'height': float('inf')})
        scale = canvas['width'] / frame['size']['width']
        matrix = draw['matrix']
        half_width = (abs(matrix[0]) * draw['width'] + abs(matrix[2]) * draw['height']) * scale / 2
        half_height = (abs(matrix[1]) * draw['width'] + abs(matrix[3]) * draw['height']) * scale / 2
        return (max(0, canvas['x']) <= draw['screenX'] - half_width and
                draw['screenX'] + half_width <= min(viewport['width'], canvas['x'] + canvas['width']) and
                max(0, canvas['y']) <= draw['screenY'] - half_height and
                draw['screenY'] + half_height <= min(viewport['height'], canvas['y'] + canvas['height']))

    for frame in frames:
        projectiles = defaultdict(list)
        for draw in frame['draws']:
            if draw['name'].startswith('projectile-'):
                projectiles[draw['name'][len('projectile-'):]].append(draw)
            elif draw['name'].startswith('enemy-') and visible_draw(frame, draw):
                target_visible_frames.append(frame['id'])
        for kind, draws in projectiles.items():
            visible = any(visible_draw(frame, draw) for draw in draws)
            if len(draws) > 1:
                clumps.append({'frame': frame['id'], 'at': frame['at'], 'kind': kind, 'count': len(draws)})
            if previous.get(kind) != frame['id'] - 1:
                runs[kind].append({'firstFrame': frame['id'], 'firstAt': draws[0]['at'],
                                   'lastFrame': frame['id'], 'lastAt': draws[-1]['at'], 'frames': 1,
                                   'visibleFrames': int(visible)})
            else:
                runs[kind][-1].update(lastFrame=frame['id'], lastAt=draws[-1]['at'],
                                      frames=runs[kind][-1]['frames'] + 1,
                                      visibleFrames=runs[kind][-1]['visibleFrames'] + int(visible))
            previous[kind] = frame['id']
    check('no same-kind batch projectiles', not clumps, clumps[:20])
    check('all server tower kinds actually painted', set(by_kind) == set(runs),
          {'server': sorted(by_kind), 'painted': sorted(runs)})
    check('unique combat target actually visible in the viewport', len(set(target_visible_frames)) >= 30,
          {'visiblePaintFrames': len(set(target_visible_frames))})
    pairs, offsets = [], []
    report['towers'] = {}
    minimum_runs = raw.get('minimumRunsPerKind', 3)
    for kind, kind_shots in by_kind.items():
        kind_runs = runs[kind]
        check(f'{kind}: sufficient distinct projectile starts', len(kind_runs) >= minimum_runs,
              {'runs': len(kind_runs), 'minimum': minimum_runs})
        check(f'{kind}: projectile run visible within Canvas and viewport', any(
            run['visibleFrames'] >= 2 for run in kind_runs),
            {'visibleFramesPerRun': [run['visibleFrames'] for run in kind_runs]})
        check(f'{kind}: no surplus projectile starts', len(kind_runs) <= len(kind_shots),
              {'runs': len(kind_runs), 'newServerShots': len(kind_shots)})
        kind_pairs = []
        for event, run in zip(kind_shots, kind_runs):
            pair = {'event': event, 'run': run, 'offsetMS': run['firstAt'] - event['atMS']}
            pairs.append(pair)
            kind_pairs.append(pair)
            offsets.append(pair['offsetMS'])
        intervals = []
        for before, after in zip(kind_pairs, kind_pairs[1:]):
            expected = after['event']['atMS'] - before['event']['atMS']
            drawn = after['run']['firstAt'] - before['run']['firstAt']
            intervals.append({'fromID': before['event']['id'], 'toID': after['event']['id'],
                              'serverMS': expected, 'drawMS': drawn, 'errorMS': drawn - expected})
        check(f'{kind}: actual shot intervals follow atMS', bool(intervals) and all(
            abs(item['errorMS']) <= tolerance for item in intervals), intervals)
        tower_id = kind_shots[0]['towerId']
        advertised = {t.get('effectiveInterval') for s in snapshots for t in s.get('towers', [])
                      if t['id'] == tower_id and t.get('effectiveInterval') is not None}
        if len(advertised) == 1:
            interval = next(iter(advertised))
            check(f'{kind}: server shots match advertised interval', all(
                b['atMS'] - a['atMS'] == interval for a, b in zip(kind_shots, kind_shots[1:])),
                {'advertisedMS': interval, 'actualMS': [b['atMS'] - a['atMS'] for a, b in zip(kind_shots, kind_shots[1:])]})
        report['towers'][kind] = {'towerId': tower_id, 'runs': kind_runs,
                                   'shotIDs': [e['id'] for e in kind_shots], 'intervals': intervals,
                                   'advertisedIntervalsMS': sorted(advertised)}
    if not offsets:
        return report
    offset = median(offsets)
    report['fittedLogicalOffsetMS'] = offset
    check('all tower starts share one logical playback clock', all(abs(value - offset) <= tolerance for value in offsets),
          {'minOffsetMS': min(offsets), 'maxOffsetMS': max(offsets), 'medianOffsetMS': offset})
    matched_ids = {pair['event']['id'] for pair in pairs}
    expected_ids = {e['id'] for e in shots if frames[0]['at'] <= offset + e['atMS'] <= frames[-1]['at'] - tolerance}
    check('every shot due in capture has a native draw', expected_ids <= matched_ids,
          {'missingIDs': sorted(expected_ids - matched_ids), 'dueCount': len(expected_ids)})
    if history:
        expected_offset = initial['at'] + raw.get('playbackBufferMS', 1000) - initial.get('elapsedMS', initial.get('tick', 0) * 200)
        advancing = next((s for s in snapshots if s.get('elapsedMS',0) > initial.get('elapsedMS',0)), None)
        if advancing:
            expected_offset = max(expected_offset, advancing['at'] + raw.get('playbackBufferMS',1000) - advancing['elapsedMS'])
        anchor_tolerance = raw.get('anchorToleranceMS', 250)
        check('fresh-page cursor excludes historical playback', abs(offset - expected_offset) <= anchor_tolerance,
              {'initialMaxEventID': baseline_id, 'expectedOffsetMS': expected_offset,
               'actualOffsetMS': offset, 'toleranceMS': anchor_tolerance})
    else:
        report['limitations'].append('Mid-match capture: historical replay suppression was not verified.')

    def health_ratio(target):
        return (target.get('health', target.get('hp', 0)) /
                max(1, target.get('maxHealth', target.get('maxHP', 1))))

    impacts = [e for e in events if e['id'] > baseline_id and e['type'] in ('hit', 'heal', 'death', 'base_hit')
               and e.get('targetId') in target_ids]
    check('impact events carry authoritative target health', bool(impacts) and all(
        e.get('target') is not None and 'atMS' in e for e in impacts))
    if not impacts or any(e.get('target') is None or 'atMS' not in e for e in impacts):
        return report
    # Equal-time damage is intentionally rendered together. Closely spaced
    # events can only combine when their complete span fits one measured frame.
    groups = []
    for event in sorted(impacts, key=lambda e: (e['atMS'], e['id'])):
        if not groups or event['atMS'] - groups[-1][0]['atMS'] > frame_p95 + 2:
            groups.append([event])
        else:
            groups[-1].append(event)
    observations = []
    prior = None
    for frame in frames:
        bars = [bar for bar in frame['bars'] if bar.get('enemy')]
        if len(bars) > 1:
            check('one enemy vitality bar per frame', False, {'frame': frame['id'], 'bars': len(bars)})
            return report
        value = bars[0]['ratio'] if bars else 0.
        if prior is not None and abs(value - prior) > 1e-10:
            observations.append({'frame': frame['id'], 'at': frame['at'], 'before': prior, 'after': value})
        if bars or prior is not None:
            prior = value
    by_shot = {(p['event']['towerId'], p['event']['atMS'], p['event'].get('targetId')): p for p in pairs}
    vitality = []
    used_observations = set()
    for group in groups:
        event = group[-1]
        due = offset + event['atMS'] + raw.get('flightMS', 180)
        if due < frames[0]['at'] + tolerance or due > frames[-1]['at'] - tolerance:
            continue
        expected = 0 if event['type'] in ('death', 'base_hit') else health_ratio(event['target'])
        candidates = [(index, item) for index, item in enumerate(observations)
                      if index not in used_observations and abs(item['after'] - expected) <= 1e-9
                      and abs(item['at'] - due) <= tolerance]
        matched = min(candidates, key=lambda pair: abs(pair[1]['at'] - due)) if candidates else None
        evidence = {'eventIDs': [e['id'] for e in group], 'atMS': event['atMS'],
                    'expectedRatio': expected, 'dueAt': due, 'observed': matched[1] if matched else None}
        check('impact vitality matches its target state and due frame', matched is not None, evidence)
        if matched:
            index, observation = matched
            used_observations.add(index)
            related = [by_shot.get((e.get('towerId'), e['atMS'], e.get('targetId'))) for e in group if e['type'] == 'hit']
            related = [p for p in related if p]
            if related:
                check('health changes after related projectile disappears', all(
                    observation['frame'] > p['run']['lastFrame'] for p in related),
                    {'eventIDs': [e['id'] for e in group], 'healthFrame': observation['frame'],
                     'projectileLastFrames': [p['run']['lastFrame'] for p in related]})
        vitality.append(evidence)
    # Two impacts less than one p95 frame apart can still straddle actual
    # frames. Account for their intermediate authoritative states as well;
    # never accept a change before its own projectile has disappeared.
    matched_intermediate = set()
    for index, observation in enumerate(observations):
        if index in used_observations:
            continue
        for event in impacts:
            if event['id'] in matched_intermediate:
                continue
            expected = 0 if event['type'] in ('death','base_hit') else health_ratio(event['target'])
            due = offset + event['atMS'] + raw.get('flightMS',180)
            shot = by_shot.get((event.get('towerId'),event['atMS'],event.get('targetId')))
            if (abs(observation['after']-expected)<=1e-9 and abs(observation['at']-due)<=tolerance
                    and (not shot or observation['frame']>shot['run']['lastFrame'])):
                used_observations.add(index)
                matched_intermediate.add(event['id'])
                break
    interior_unmatched = [item for index, item in enumerate(observations) if index not in used_observations
                          and frames[0]['at'] + tolerance <= item['at'] <= frames[-1]['at'] - tolerance]
    check('no unaccounted health jumps', not interior_unmatched, interior_unmatched)
    check('multiple authoritative impacts observed separately', len(vitality) >= raw.get('minimumHealthChanges', 3),
          {'observedImpactGroups': len(vitality), 'nativeHealthChanges': len(observations)})
    report['vitality'] = vitality
    report['nativeHealthChanges'] = observations
    report['passed'] = not failures
    return report
