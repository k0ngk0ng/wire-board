import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
POLICY = json.loads((ROOT / 'games/basketball/build/local-paths-lint.json').read_text())
LOCAL_PATH = re.compile(r'\.local/([A-Za-z0-9_.-]+)(?:/([A-Za-z0-9_.-]+))?')
JOINED_PATH = re.compile(r'''["']\.local["']\s*,\s*["']basketball["'](?:\s*,\s*["']([A-Za-z0-9_.-]+)["'])?''')


def inspect_source(name, source):
    source = JOINED_PATH.sub(lambda match: '.local/basketball' + ('/' + match[1] if match[1] else ''), source)
    issues = []
    for match in LOCAL_PATH.finditer(source):
        game, directory = match.groups()
        alias = game in POLICY['aliases'] or game.startswith(POLICY['game'] + '-')
        misplaced = game == POLICY['game'] and directory is not None and directory not in POLICY['directories']
        if alias or misplaced:
            line = source.count('\n', 0, match.start()) + 1
            issues.append(f'{name}:{line}: game local files must use .local/{POLICY["game"]}/ followed by a purpose directory: {match.group()}')
    return issues


def check_workspace(root):
    extensions = {suffix for values in POLICY['languages'].values() for suffix in values}
    names = subprocess.check_output(['git', 'ls-files', '--cached', '--others', '--exclude-standard', '-z'], cwd=root).decode().split('\0')
    issues = []
    checked = 0
    for name in sorted(set(names)):
        path = root / name
        if path.suffix not in extensions or not path.is_file() or path.is_symlink():
            continue
        checked += 1
        issues.extend(inspect_source(name, path.read_text()))
    local = root / '.local'
    if local.is_dir():
        for path in local.iterdir():
            if path.name in POLICY['aliases'] or path.name.startswith(POLICY['game'] + '-'):
                issues.append(f'{path.relative_to(root)}: obsolete game local directory')
        game = local / POLICY['game']
        if game.is_dir():
            for path in game.iterdir():
                if path.name not in POLICY['directories']:
                    issues.append(f'{path.relative_to(root)}: expected a purpose directory')
    return {'passed': not issues, 'scope': POLICY['game'], 'languages': sorted(POLICY['languages']), 'files': checked, 'issues': issues}


def main():
    args = sys.argv[1:]
    if args:
        if len(args) != 2 or args[0] != '--stdin' or args[1] not in POLICY['languages']:
            return 2
        issues = inspect_source('<stdin>', sys.stdin.read())
        result = {'passed': not issues, 'issues': issues}
    else:
        result = check_workspace(ROOT)
    print(json.dumps(result, ensure_ascii=False))
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
