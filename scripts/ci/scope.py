#!/usr/bin/env python3
"""Changed-game CI planning and test ownership; no cross-game rule execution."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[2]
POKER = ('doudizhu', 'guandan', 'shengji')
LEGACY = ('splendor', 'rail', 'catan', 'carcassonne', 'sanguosha')
# Old mixed-package scenarios are run with an exact game subtest selector.
MULTI = {
    'TestBotsFinishBothGames': ('splendor', 'rail'),
    'TestSoloGamesAIContinuationPersistenceAndRematch': ('splendor', 'rail'),
    'TestBotFinalTurnFinishesRoom': ('splendor', 'rail'),
    'TestBaseGamesRemainBaseAfterExpansionSettings': ('splendor', 'catan'),
    'TestAutoPlayAllGamesFinishAndCreditHumanPlayers': LEGACY,
    'TestTimeoutAutoplayAllGamesContinueToNaturalFinish': LEGACY,
}
PAIRED = {'TestBaseData', 'TestBotsDoNotUseOtherPlayersSecretsOrDeckOrder', 'TestBotHandlesSubstepsAndInvalidSeats'}

def game_names(root):
    return sorted(p.name for p in (root/'games').iterdir() if p.is_dir() and not p.name.startswith(('.', '_')))

def plan(paths, games, manual='', legacy_sources=None, contra_campaigns=False):
    selected = {}
    # The user authorized human-only Contra delivery. Computer campaigns are
    # retained as explicit research, never inferred from rule changes or all.
    if contra_campaigns and 'contra' not in games:
        raise ValueError('Contra research requested without Contra sources')
    def add(game, backend=False):
        if game in games: selected[game] = selected.get(game, False) or backend
    for path in paths:
        if path.endswith('.md') or path.startswith('docs/'): continue
        parts = path.split('/')
        if path.startswith('internal/') and path.endswith('_test.go'):
            source = (legacy_sources or {}).get(path)
            if source is None: source = (ROOT/path).read_text() if (ROOT/path).exists() else ''
            for name in re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', source, re.M):
                for game in owner(Path(path), name, games):
                    add(game, True)
        if len(parts) > 2 and parts[0] == 'games':
            # Contra keeps rule tables and visual-AI fixtures under engine,
            # including JSON/gzip/indexed images rather than Go files alone.
            engine_data = parts[1] == 'contra' and parts[2] == 'engine'
            add(parts[1], path.endswith('.go') or parts[2] == 'server' or engine_data or '/tests/testdata/' in path)
            continue
        if path.startswith(('web/src/poker/', 'scripts/poker/', 'internal/gametest/poker_', 'internal/gametest/browser_e2e')):
            for game in POKER: add(game, path.endswith('.go'))
            continue
        if path.startswith(('scripts/', 'internal/')):
            for game in games:
                if re.search(r'(?:/|_)' + re.escape(game) + r'(?:/|_|\.)', path):
                    add(game, path.endswith('.go'))
        if path in ('internal/game/bot_test.go', 'internal/game/game_test.go'):
            for game in ('rail', 'splendor'): add(game, True)
    if manual:
        if manual not in (*games, 'all', 'shared'): raise ValueError(f'Unknown game: {manual}')
        if manual != 'shared':
            for game in games if manual == 'all' else [manual]: add(game, True)
    rows = [{'game': g, 'backend': selected[g]} for g in sorted(selected)]
    matrix = []
    for row in rows:
        count = 20 if row['game'] == 'catan' and row['backend'] else 1
        matrix.extend({**row, 'shard': i, 'count': count} for i in range(count))
    campaigns = [{'mode': mode, 'players': players}
                 for mode in ('standard-terminal', 'assisted-clear') for players in (1, 2)] if contra_campaigns else []
    return {'games': rows, 'matrix': {'include': matrix}, 'has_games': bool(rows),
            'contra_campaign_requested': bool(contra_campaigns),
            'contra_campaign_matrix': {'include': campaigns}, 'has_contra_campaigns': bool(campaigns)}

def changed_names(raw):
    return [p.decode('utf-8') for p in raw.split(b'\0') if p]

def owner(path, name, games):
    parts = path.parts
    if parts[:2] == ('internal', 'production'):
        return set() # Application startup contracts, not game rule campaigns.
    if parts[0] == 'games' and len(parts) > 2: return {parts[1]}
    if name in MULTI: return set(MULTI[name])
    if name in PAIRED: return {'rail', 'splendor'}
    named = {g for g in games if g in name.lower() or path.name.startswith(g + '_')}
    return named

def go_commands(root, game, index=0, count=1, race=False):
    if count < 1 or not 0 <= index < count: raise ValueError('Invalid shard')
    games = game_names(root)
    if game not in (*games, 'shared'): raise ValueError(f'Unknown game: {game}')
    grouped, special = {}, []
    files = sorted([*root.glob('*_test.go'), *root.glob('internal/**/*_test.go'),
                    *(root/'games').glob('*_test.go')])
    if game != 'shared': files += sorted((root/'games'/game).rglob('*_test.go'))
    for file in files:
        relative = file.relative_to(root)
        if relative.parts[:2] == ('internal', 'gametest'): continue # opt-in browser servers
        for name in re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', file.read_text(), re.M):
            if name == 'TestBotStandardCampaignE2E' and relative.parts[:2] == ('games', 'contra'):
                continue # Explicit campaign runner validates execution and provenance.
            owners = owner(relative, name, games)
            if (game == 'shared' and owners) or (game != 'shared' and game not in owners): continue
            if int.from_bytes(hashlib.sha256(name.encode()).digest()[:8], 'big') % count != index: continue
            if race and game != 'shared':
                selected_race = bool(re.search(r'Concurrent|AutoPlay|Timeout|Spectator|LongGameArtifact', name))
                if game == 'basketball':
                    selected_race = selected_race or name.startswith(('TestBasketballRealtime', 'TestCampaignPresentation', 'TestCampaignEnding', 'TestRuntimePresentation', 'TestDunkSharedTarget', 'TestHighPassHeight')) or name == 'TestBasketballHTTPCompleteOriginalSelection'
                if game == 'contra':
                    selected_race = selected_race or name.startswith(('TestContraHTTP', 'TestContraModule', 'TestRuntimePaused', 'TestRuntimeBundle', 'TestRuntimeHTTP'))
                    area = relative.parts[2] if len(relative.parts) > 3 and relative.parts[:2] == ('games', 'contra') else ''
                    selected_race = selected_race or (
                        area == 'server' and name.startswith(('TestProduction', 'TestRuntimeArtifact')) or
                        area == 'engine' and name.startswith('TestSessionBattleReview') or
                        area == 'runtime' and name == 'TestRuntimeTerminalUsesPersistedBattleReview'
                    )
                if not selected_race: continue
            package = './' + relative.parent.as_posix() if relative.parent.as_posix() != '.' else '.'
            if name in MULTI:
                suffix = '-' if name == 'TestBotsFinishBothGames' else '$'
                special.append((package, '^' + name + '$/^' + game + suffix))
            else:
                grouped.setdefault(package, []).append(name)
    base = ['go', 'test', '-json', '-count=1', '-timeout=45m']
    if race: base.append('-race')
    commands = [base + ['-run', '^(?:' + '|'.join(re.escape(n) for n in sorted(set(names))) + ')$', package]
                for package, names in sorted(grouped.items())]
    commands += [base + ['-run', pattern, package] for package, pattern in special]
    return commands

def verification_env(game, backend, base=None):
    env = {k: v for k, v in (os.environ if base is None else base).items()
           if not k.startswith('CONTRA_BOT_')}
    if backend and game == 'snooker': env['WIRE_BOARD_SNOOKER_HTTP_FULL'] = '1'
    if backend and game == 'catan': env['WIRE_BOARD_NESTING_ACCEPTANCE'] = '1'
    if backend and game == 'basketball':
        env.update(BASKETBALL_PREPARATION_MATRIX='1', BASKETBALL_SELECTION_ADMISSION='1', BASKETBALL_FULL_HTTP='1')
    return env

def frontend_tests(root, game):
    base = root/'games'/game
    return sorted([*(base/'tests').glob('*.test.mjs'),
                   *(base/'web').glob('*.test.mjs')])

def build_commands(root, game, index=0):
    if game != 'contra' or index != 0:
        return []
    return ([['python3', '-B', str(root/'games/contra/build'/name)]
             for name in ('identity_test.py', 'media_test.py', 'media_archive_test.py', 'builder_inputs_test.py', 'runtime_assets_test.py', 'delivery_test.py', 'ci_runtime_test.py')]
            + [['node', '--test', str(root/'games/contra/build/runtime-delivery.test.mjs')]])

def run(args):
    output = ROOT/'.local/ci'/args.game
    output.mkdir(parents=True, exist_ok=True)
    if args.game == 'shared':
        commands = go_commands(ROOT, 'shared', race=True)
    else:
        commands = []
        if args.shard == 0:
            frontend = frontend_tests(ROOT, args.game)
            if frontend:
                commands.append(['node', '--experimental-strip-types', '--test', *(str(p) for p in frontend)])
            assets = ROOT/'scripts'/args.game/'verify-assets.py'
            if assets.exists(): commands.append(['python3', '-B', str(assets), '--dist'])
        if args.backend:
            commands += go_commands(ROOT, args.game, args.shard, args.count)
            commands += go_commands(ROOT, args.game, args.shard, args.count, race=True)
    env = verification_env(args.game, args.backend)
    commands += build_commands(ROOT, args.game, args.shard)
    manifest = {'game': args.game, 'backend': args.backend, 'shard': args.shard, 'commands': commands, 'passed': False}
    destination = output/f'result-{args.shard}.json'
    destination.write_text(json.dumps(manifest, indent=2) + '\n')
    for i, command in enumerate(commands):
        print('RUN', ' '.join(command), flush=True)
        with (output/f'{args.shard}-{i}.log').open('w') as log:
            subprocess.run(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)
    manifest['passed'] = True
    destination.write_text(json.dumps(manifest, indent=2) + '\n')
    print(f'PASS {args.game}: {len(commands)} scoped checks', flush=True)

def test_blocks(source):
    # Mask Go comments/string literals before matching braces or declarations.
    # Original bodies are retained for comparison, including literal changes.
    literals = re.compile(r"//[^\n]*|/\*[\s\S]*?\*/|`[^`]*`|\"(?:\\.|[^\"\\])*\"|'(?:\\.|[^'\\])*'")
    masked = literals.sub(lambda match: re.sub(r'[^\n]', ' ', match.group()), source)
    blocks, spans = {}, []
    for match in re.finditer(r'^func ((?:Test|Example|Fuzz)\w*)\(', masked, re.M):
        if spans and match.start() < spans[-1][1]: continue
        start = masked.find('{', match.end())
        if start < 0: return None
        depth, end = 0, None
        for index in range(start, len(masked)):
            if masked[index] == '{': depth += 1
            elif masked[index] == '}':
                depth -= 1
                if depth == 0:
                    end = index + 1
                    break
        if end is None: return None
        blocks[match.group(1)] = source[match.start():end]
        while end < len(source) and source[end].isspace(): end += 1
        spans.append((match.start(), end))
    outside, cursor = [], 0
    for start, end in spans:
        outside.append(source[cursor:start])
        cursor = end
    outside.append(source[cursor:])
    return blocks, ''.join(outside).strip()

def changed_test_source(before, after, retired=()):
    # A retired table member has no surviving rules to verify. Ignore exactly
    # that deletion, while keeping all changes to scenario bodies and helpers.
    if retired:
        def table(match):
            members = re.findall(r'"([^"\\]*)"', match.group(1))
            return '[]string{' + ','.join(json.dumps(g) for g in members if g not in retired) + '}'
        pattern = r'\[\]string\{\s*((?:"[^"\\]*"\s*,?\s*)*)\}'
        before = re.sub(pattern, table, before)
        after = re.sub(pattern, table, after)
    old, new = test_blocks(before), test_blocks(after)
    # Imports, helpers and fixtures can affect every test in a mixed file.
    if old is None or new is None or old[1] != new[1]: return before + '\n' + after
    names = sorted(name for name in old[0].keys() | new[0].keys() if old[0].get(name) != new[0].get(name))
    return '\n'.join('func ' + name + '(' for name in names)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    p = sub.add_parser('plan')
    p.add_argument('--base', default='')
    p.add_argument('--head', default='HEAD')
    p.add_argument('--manual', default='')
    p.add_argument('--contra-campaigns', action='store_true', help='Opt into non-blocking Contra computer research campaigns')
    r = sub.add_parser('run')
    r.add_argument('--game', required=True, choices=['shared', *game_names(ROOT)])
    r.add_argument('--backend', action='store_true')
    r.add_argument('--shard', type=int, default=0)
    r.add_argument('--count', type=int, default=1)
    args = parser.parse_args()
    if args.command == 'run': return run(args)
    if args.base:
        subprocess.run(['git', 'merge-base', '--is-ancestor', args.base, args.head], cwd=ROOT, check=True)
        # No rename folding: both old and new owners must be validated.
        raw = subprocess.check_output(['git', 'diff', '--no-renames', '--name-only', '-z', args.base, args.head], cwd=ROOT)
    else:
        raw = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT)
    paths = changed_names(raw)
    legacy_sources = {}
    active_games = game_names(ROOT)
    retired = {path.split('/')[1] for path in paths if path.startswith('games/') and len(path.split('/')) > 2 and path.split('/')[1] not in active_games}
    for path in paths:
        if path.startswith('internal/') and path.endswith('_test.go'):
            current = subprocess.run(['git', 'show', args.head + ':' + path], cwd=ROOT, text=True, capture_output=True)
            source = current.stdout if current.returncode == 0 else ''
            if args.base:
                old = subprocess.run(['git', 'show', args.base + ':' + path], cwd=ROOT, text=True, capture_output=True)
                source = changed_test_source(old.stdout if old.returncode == 0 else '', source, retired=retired)
            legacy_sources[path] = source
    result = plan(paths, game_names(ROOT), args.manual, legacy_sources, contra_campaigns=args.contra_campaigns)
    result.update(base=args.base, head=args.head, paths=changed_names(raw))
    out = ROOT/'.local/ci'
    out.mkdir(parents=True, exist_ok=True)
    (out/'plan.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a') as f:
            f.write('matrix=' + json.dumps(result['matrix']) + '\n')
            f.write('has_games=' + str(result['has_games']).lower() + '\n')
            f.write('contra_campaign_matrix=' + json.dumps(result['contra_campaign_matrix']) + '\n')
            f.write('has_contra_campaigns=' + str(result['has_contra_campaigns']).lower() + '\n')
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as f:
            f.write('## Verification scope\n\nBaseline: `' + (args.base or 'bootstrap') + '`\n\n')
            f.write('Shared platform checks always run. Selected games:\n\n')
            f.write('\n'.join('- ' + r['game'] + (': frontend/build and Go contracts; optional computer research listed separately' if r['game'] == 'contra' and r['backend'] else ': frontend, rules, full matches and race checks' if r['backend'] else ': frontend/assets only') for r in result['games']) or '- None')
            f.write('\n')
            if result['has_contra_campaigns']:
                f.write('\nOptional Contra computer research explicitly requested: standard terminal and assisted clear, each with one and two seats. These jobs do not gate human gameplay delivery.\n')
    print(json.dumps(result, ensure_ascii=False))

if __name__ == '__main__': main()
