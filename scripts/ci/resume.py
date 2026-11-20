import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
import tempfile


def git(root, *args):
    return subprocess.check_output(['git', *args], cwd=root, text=True).strip()


def module_at(root, sha):
    source = git(root, 'show', sha + ':scripts/ci/scope.py')
    parent = root / '.local/ci-resume/modules'
    parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(dir=parent) as folder:
        path = Path(folder) / 'scope.py'
        path.write_text(source)
        spec = importlib.util.spec_from_file_location('validated_scope', path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        module.ROOT = root
        return module


def signature(module, root, game):
    value = {'normal': module.go_commands(root, game), 'race': module.go_commands(root, game, race=True), 'frontend': [str(path.relative_to(root)) for path in module.frontend_tests(root, game)], 'binding': module.build_commands(root, game, 0), 'environment': module.verification_env(game, True, {}), 'partitions': getattr(module, 'GAME_PARTITIONS', {}).get(game, (1, 1))}
    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()


def execution_sections(text):
    sections = []
    for name in ['frontend', 'shared', 'games', 'verify', 'complete']:
        match = re.search(r'^  ' + name + r':\n.*?(?=^  [A-Za-z_][A-Za-z0-9_-]*:|\Z)', text, re.M | re.S)
        if not match:
            raise ValueError('Missing execution job: ' + name)
        sections.append(match[0])
    return sections


def recover(root, head, runs, jobs_for):
    current = module_at(root, head)
    games = current.game_names(root)
    workflow = git(root, 'show', head + ':.github/workflows/build.yml')
    for run in runs:
        base = run.get('head_sha', '')
        if run.get('head_branch') != 'main' or run.get('event') != 'push' or run.get('path') != '.github/workflows/build.yml' or run.get('status') != 'completed' or run.get('conclusion') not in ['cancelled', 'failure', 'timed_out'] or base == head or not re.fullmatch(r'[a-f0-9]{40}', base):
            continue
        if subprocess.run(['git', 'merge-base', '--is-ancestor', base, head], cwd=root, capture_output=True).returncode:
            continue
        paths = git(root, 'diff', '--no-renames', '--name-only', base, head).splitlines()
        delta = current.plan(paths, games)
        if delta['has_shared'] or delta['frontend_checks']:
            continue
        affected = {row['game'] for row in delta['games']}
        if execution_sections(workflow) != execution_sections(git(root, 'show', base + ':.github/workflows/build.yml')):
            continue
        jobs = jobs_for(run['id'])
        named = {}
        ambiguous = False
        for job in jobs:
            name = job['name']
            if name in named:
                ambiguous = True
            named[name] = job
        if ambiguous:
            continue
        required = ['Build application assets', 'Shared platform HTTP, persistence and race checks']
        if any(named.get(name, {}).get('conclusion') != 'success' for name in required):
            continue
        previous = module_at(root, base)
        if previous.go_commands(root, 'shared', race=True) != current.go_commands(root, 'shared', race=True):
            continue
        reused, pending = [], []
        for game in games:
            job = named.get('Game ' + game)
            digest = signature(current, root, game)
            if game not in affected and job and job['conclusion'] == 'success' and digest == signature(previous, root, game):
                reused.append({'game': game, 'job': job['id'], 'signatureSHA256': digest})
            else:
                pending.append(game)
        return {'eligible': True, 'head': head, 'base': base, 'sourceRun': run['id'], 'changedPaths': paths, 'affectedGames': sorted(affected), 'reusedGames': reused, 'pendingGames': pending, 'sharedJob': named[required[1]]['id'], 'frontendJob': named[required[0]]['id']}
    return {'eligible': False, 'head': head, 'base': '', 'reusedGames': [], 'pendingGames': []}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--repo', required=True)
    parser.add_argument('--head', required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--github-output', type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r'[a-f0-9]{40}', args.head) or not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', args.repo):
        raise ValueError('Invalid source identity')
    root = Path.cwd().resolve()
    def gh(path):
        return json.loads(subprocess.check_output(['gh', 'api', path], text=True))
    prefix = 'repos/' + args.repo + '/actions/runs'
    runs = gh(prefix + '?branch=main&event=push&per_page=30')['workflow_runs']
    report = recover(root, args.head, runs, lambda run: gh(prefix + '/' + str(run) + '/jobs?per_page=100')['jobs'])
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=2) + '\n')
    if args.github_output:
        with args.github_output.open('a') as output:
            output.write('sha=' + report['base'] + '\n')
    print(json.dumps({'eligible': report['eligible'], 'base': report['base'], 'reusedGames': len(report['reusedGames']), 'pendingGames': report['pendingGames']}))


if __name__ == '__main__':
    main()
