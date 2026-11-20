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
    sections = [text.split('jobs:', 1)[0]]
    for name in ['frontend', 'shared', 'games', 'verify', 'complete']:
        match = re.search(r'^  ' + name + r':\n.*?(?=^  [A-Za-z_][A-Za-z0-9_-]*:|\Z)', text, re.M | re.S)
        if not match:
            raise ValueError('Missing execution job: ' + name)
        section = match[0]
        sections.append(section.split('    steps:\n', 1)[0] if name == 'verify' else section)
    return sections


def recover(root, head, runs, jobs_for):
    current = module_at(root, head)
    games = current.game_names(root)
    workflow = git(root, 'show', head + ':.github/workflows/build.yml')
    sections = execution_sections(workflow)
    candidates = []
    positions = {sha: index for index, sha in enumerate(git(root, 'rev-list', head).splitlines())}
    for run in runs:
        base = run.get('head_sha', '')
        if run.get('head_branch') != 'main' or run.get('event') != 'push' or run.get('path') != '.github/workflows/build.yml' or run.get('status') != 'completed' or run.get('conclusion') not in ['success', 'cancelled', 'failure', 'timed_out'] or base == head or not re.fullmatch(r'[a-f0-9]{40}', base):
            continue
        if subprocess.run(['git', 'merge-base', '--is-ancestor', base, head], cwd=root, capture_output=True).returncode:
            continue
        paths = git(root, 'diff', '--no-renames', '--name-only', base, head).splitlines()
        delta = current.plan(paths, games)
        if delta['has_shared'] or delta['frontend_checks']:
            continue
        if sections != execution_sections(git(root, 'show', base + ':.github/workflows/build.yml')):
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
        if named.get('Build application assets', {}).get('conclusion') != 'success':
            continue
        candidates.append({'run': run, 'base': base, 'paths': paths, 'affected': {row['game'] for row in delta['games']}, 'named': named, 'module': module_at(root, base)})
    candidates.sort(key=lambda row: (positions[row['base']], -row['run']['id']))
    shared_name = 'Shared platform HTTP, persistence and race checks'
    shared_commands = current.go_commands(root, 'shared', race=True)
    shared_results = [row for row in candidates if row['named'].get(shared_name, {}).get('conclusion') in ['success', 'failure', 'timed_out'] and row['module'].go_commands(root, 'shared', race=True) == shared_commands]
    platform = shared_results[0] if shared_results and shared_results[0]['named'][shared_name]['conclusion'] == 'success' else None
    if candidates and platform:
        reused, pending = [], []
        for game in games:
            digest = signature(current, root, game)
            selected = None
            for candidate in candidates:
                job = candidate['named'].get('Game ' + game)
                if game in candidate['affected'] or not job or digest != signature(candidate['module'], root, game):
                    continue
                if job['conclusion'] in ['failure', 'timed_out']:
                    break
                if job['conclusion'] == 'success':
                    selected = {'game': game, 'job': job['id'], 'signatureSHA256': digest, 'sourceRun': candidate['run']['id'], 'sourceSHA': candidate['base']}
                    break
            if selected:
                reused.append(selected)
            else:
                pending.append(game)
        baseline = candidates[0]
        return {'eligible': True, 'head': head, 'base': baseline['base'], 'sourceRun': baseline['run']['id'], 'changedPaths': baseline['paths'], 'affectedGames': sorted(baseline['affected']), 'reusedGames': reused, 'pendingGames': pending, 'platformSourceRun': platform['run']['id'], 'platformSourceSHA': platform['base'], 'sharedJob': platform['named'][shared_name]['id'], 'frontendJob': baseline['named']['Build application assets']['id']}
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
