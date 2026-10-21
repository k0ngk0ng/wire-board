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
LEGACY = ('splendor', 'rail', 'catan', 'carcassonne', 'sanguosha', 'dota')
# Old mixed-package scenarios are run with an exact game subtest selector.
MULTI = {
    'TestBotsFinishBothGames': ('splendor', 'rail'),
    'TestSoloGamesAIContinuationPersistenceAndRematch': ('splendor', 'rail'),
    'TestBotFinalTurnFinishesRoom': ('splendor', 'rail'),
    'TestBaseGamesRemainBaseAfterExpansionSettings': ('splendor', 'catan'),
    'TestAutoPlayAllGamesFinishAndCreditHumanPlayers': LEGACY[:-1],
    'TestTimeoutAutoplayAllGamesContinueToNaturalFinish': LEGACY,
}
PAIRED = {'TestBaseData', 'TestBotsDoNotUseOtherPlayersSecretsOrDeckOrder', 'TestBotHandlesSubstepsAndInvalidSeats'}

def game_names(root):
    return sorted(p.name for p in (root/'games').iterdir() if p.is_dir() and not p.name.startswith(('.', '_')))

def plan(paths, games, manual='', legacy_sources=None):
    selected = {}
    def add(game, backend=False):
        if game in games: selected[game] = selected.get(game, False) or backend
    for path in paths:
        if path.endswith('.md') or path.startswith('docs/'): continue
        parts = path.split('/')
        if path.startswith('internal/') and path.endswith('_test.go'):
            source = (legacy_sources or {}).get(path)
            if source is None: source = (ROOT/path).read_text() if (ROOT/path).exists() else ''
            for name in re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', source, re.M):
                for game in owner(Path(path), name, games): add(game, True)
        if len(parts) > 2 and parts[0] == 'games':
            add(parts[1], path.endswith('.go') or parts[2] == 'server' or '/tests/testdata/' in path)
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
    return {'games': rows, 'matrix': {'include': matrix}, 'has_games': bool(rows)}

def changed_names(raw):
    return [p.decode('utf-8') for p in raw.split(b'\0') if p]

def owner(path, name, games):
    parts = path.parts
    if parts[0] == 'games': return {parts[1]}
    if name in MULTI: return set(MULTI[name])
    if name in PAIRED: return {'rail', 'splendor'}
    named = {g for g in games if g in name.lower() or path.name.startswith(g + '_')}
    return named

def go_commands(root, game, index=0, count=1, race=False):
    if count < 1 or not 0 <= index < count: raise ValueError('Invalid shard')
    games = game_names(root)
    if game not in (*games, 'shared'): raise ValueError(f'Unknown game: {game}')
    grouped, special = {}, []
    files = sorted([*root.glob('*_test.go'), *root.glob('internal/**/*_test.go')])
    if game != 'shared': files += sorted((root/'games'/game).rglob('*_test.go'))
    for file in files:
        relative = file.relative_to(root)
        if relative.parts[:2] == ('internal', 'gametest'): continue # opt-in browser servers
        for name in re.findall(r'^func ((?:Test|Example|Fuzz)\w*)\(', file.read_text(), re.M):
            owners = owner(relative, name, games)
            if (game == 'shared' and owners) or (game != 'shared' and game not in owners): continue
            if int.from_bytes(hashlib.sha256(name.encode()).digest()[:8], 'big') % count != index: continue
            if race and game != 'shared':
                selected_race = bool(re.search(r'Concurrent|AutoPlay|Timeout|Spectator|LongGameArtifact', name))
                if game == 'basketball':
                    selected_race = selected_race or name.startswith(('TestBasketballRealtime', 'TestCampaignPresentation', 'TestCampaignEnding', 'TestRuntimePresentation', 'TestDunkSharedTarget', 'TestHighPassHeight')) or name == 'TestBasketballHTTPCompleteOriginalSelection'
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
    env = dict(os.environ if base is None else base)
    if backend and game == 'snooker': env['WIRE_BOARD_SNOOKER_HTTP_FULL'] = '1'
    if backend and game == 'catan': env['WIRE_BOARD_NESTING_ACCEPTANCE'] = '1'
    if backend and game == 'basketball':
        env.update(BASKETBALL_PREPARATION_MATRIX='1', BASKETBALL_SELECTION_ADMISSION='1', BASKETBALL_FULL_HTTP='1')
    return env

def run(args):
    output = ROOT/'.local/ci'/args.game
    output.mkdir(parents=True, exist_ok=True)
    if args.game == 'shared':
        commands = go_commands(ROOT, 'shared', race=True)
    else:
        commands = []
        if args.shard == 0:
            frontend = sorted((ROOT/'games'/args.game/'tests').glob('*.test.mjs'))
            if frontend:
                commands.append(['node', '--experimental-strip-types', '--test', *(str(p) for p in frontend)])
            assets = ROOT/'scripts'/args.game/'verify-assets.py'
            if assets.exists(): commands.append(['python3', '-B', str(assets), '--dist'])
        if args.backend:
            commands += go_commands(ROOT, args.game, args.shard, args.count)
            commands += go_commands(ROOT, args.game, args.shard, args.count, race=True)
    env = verification_env(args.game, args.backend)
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

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    p = sub.add_parser('plan')
    p.add_argument('--base', default='')
    p.add_argument('--head', default='HEAD')
    p.add_argument('--manual', default='')
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
    for path in paths:
        if path.startswith('internal/') and path.endswith('_test.go'):
            source = (ROOT/path).read_text() if (ROOT/path).exists() else ''
            if args.base:
                old = subprocess.run(['git', 'show', args.base + ':' + path], cwd=ROOT, text=True, capture_output=True)
                if old.returncode == 0: source += '\n' + old.stdout
            legacy_sources[path] = source
    result = plan(paths, game_names(ROOT), args.manual, legacy_sources)
    result.update(base=args.base, head=args.head, paths=changed_names(raw))
    out = ROOT/'.local/ci'
    out.mkdir(parents=True, exist_ok=True)
    (out/'plan.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    if os.environ.get('GITHUB_OUTPUT'):
        with open(os.environ['GITHUB_OUTPUT'], 'a') as f:
            f.write('matrix=' + json.dumps(result['matrix']) + '\n')
            f.write('has_games=' + str(result['has_games']).lower() + '\n')
    if os.environ.get('GITHUB_STEP_SUMMARY'):
        with open(os.environ['GITHUB_STEP_SUMMARY'], 'a') as f:
            f.write('## Verification scope\n\nBaseline: `' + (args.base or 'bootstrap') + '`\n\n')
            f.write('Shared platform checks always run. Selected games:\n\n')
            f.write('\n'.join('- ' + r['game'] + (': frontend, rules, full matches and race checks' if r['backend'] else ': frontend/assets only') for r in result['games']) or '- None')
            f.write('\n')
    print(json.dumps(result, ensure_ascii=False))

if __name__ == '__main__': main()
