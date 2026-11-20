import json
from pathlib import Path
import re


def check(root):
    issues = []
    for source in sorted((root / 'cmd').rglob('*.go')):
        text = source.read_text()
        blocks = re.findall(r'^[ \t]*import[ \t]*(\([^)]*\)|(?:[A-Za-z_.][A-Za-z0-9_]*\s+)?"[^"\n]+")', text, re.M | re.S)
        owners = {name for block in blocks for name in re.findall(r'"github.com/k0ngk0ng/wire-board/games/([^/"\n]+)(?:/[^"\n]*)?"', block) if not name.startswith('_')}
        issues.extend({'file': source.relative_to(root).as_posix(), 'game': owner, 'rule': 'game-tool-outside-game'} for owner in sorted(owners))
    return issues


if __name__ == '__main__':
    issues = check(Path(__file__).resolve().parents[2])
    print(json.dumps({'passed': not issues, 'issues': issues}))
    raise SystemExit(bool(issues))
