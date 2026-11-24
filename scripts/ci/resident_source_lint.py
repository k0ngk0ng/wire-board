import ast
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tokenize

ROOT = Path(__file__).resolve().parents[2]
POLICY = json.loads((ROOT / 'scripts/ci/resident-source-lint.json').read_text())


def python_issues(source):
    issues = []
    try:
        tree = ast.parse(source)
        if any(token.type == tokenize.COMMENT for token in tokenize.generate_tokens(io.StringIO(source).readline)):
            issues.append('no-comments')
    except (SyntaxError, tokenize.TokenError, IndentationError):
        return ['syntax']
    parents = {child: node for node in ast.walk(tree) for child in ast.iter_child_nodes(node)}
    for node in ast.walk(tree):
        name = None
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            name = node.name
        elif isinstance(node, ast.arg):
            name = node.arg
        elif isinstance(node, ast.Name) and isinstance(node.ctx, ast.Store):
            name = node.id
        if name and not re.fullmatch(r'[a-z_][a-z0-9_]*|[A-Z][A-Z0-9_]*|setUp|tearDown|[A-Z][A-Za-z0-9]*', name):
            issues.append('python-name')
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in ['eval', 'exec']:
            issues.append('no-dynamic-evaluation')
        if isinstance(node, ast.Constant) and isinstance(node.value, str):
            parent = parents.get(node)
            if isinstance(parent, (ast.Tuple, ast.List)):
                parent = parents.get(parent)
            if isinstance(parent, ast.Call) and isinstance(parent.func, ast.Attribute) and parent.func.attr in ['startswith', 'endswith']:
                continue
            if node.value.startswith(('/tmp/', '/private/tmp/', '~/')):
                issues.append('workspace-output')
            if node.value.startswith('.local/') and not any(node.value == prefix or node.value.startswith(prefix + '/') for prefix in POLICY['outputRoots']):
                issues.append('output-directory')
    return sorted(set(issues))


def checked_lines(name, source):
    excluded = set()
    for fragment in POLICY['go'].get('legacyFragments', []):
        if fragment['file'] != name:
            continue
        match = re.search(fragment['pattern'], source)
        if match and hashlib.sha256(match[0].encode()).hexdigest() == fragment['sha256']:
            first = source.count('\n', 0, match.start()) + 1
            excluded.update(range(first, first + match[0].count('\n') + 1))
    return [line for line in range(1, len(source.splitlines()) + 1) if line not in excluded]


def inspect(items):
    files = []
    go_sources = []
    go_indexes = []
    for item in items:
        name = item['file']
        language = 'go' if name.endswith('.go') else 'python' if name.endswith('.py') else ''
        issues = []
        rule = POLICY.get(language)
        path = ROOT / name
        if not rule or not path.resolve().is_relative_to(ROOT) or not re.match(rule['directory'], name) or not re.fullmatch(rule['filename'], path.name) or name not in rule['sources']:
            issues.append('source-location')
        if not issues and language == 'python':
            issues.extend(python_issues(item['source']))
        if not issues and language == 'go':
            go_indexes.append(len(files))
            entry = {'file': name, 'source': item['source'], 'expectedPackage': 'server', 'strictIntegers': name in rule['strictIntegerSources']}
            if name == 'internal/server/static_delivery.go':
                entry['changedLines'] = checked_lines(name, item['source'])
            go_sources.append(entry)
        files.append({'file': name, 'sourceSHA256': hashlib.sha256(item['source'].encode()).hexdigest(), 'issues': issues})
    if go_sources:
        temporary = ROOT / '.local/yuyuhakusho/resident-assets/lint'
        env = dict(os.environ, GOCACHE=str(temporary / 'go-cache'), GOMODCACHE=str(temporary / 'go-mod'), GOTMPDIR=str(temporary / 'tmp'), GOTELEMETRY='off', GOPROXY='off')
        for name in ['GOCACHE', 'GOMODCACHE', 'GOTMPDIR']:
            Path(env[name]).mkdir(parents=True, exist_ok=True)
        result = subprocess.run(['go', 'run', 'games/cs16/build/lint_go.go'], cwd=ROOT, env=env, input=json.dumps(go_sources), text=True, capture_output=True)
        if result.returncode:
            raise ValueError('Shared Go source lint could not run')
        for index, issues in zip(go_indexes, json.loads(result.stdout), strict=True):
            files[index]['issues'].extend(issues)
    for item in files:
        item['passed'] = not item['issues']
    return {'passed': all(item['passed'] for item in files), 'files': files}


def main():
    streamed = '--stdin' in sys.argv
    rows = json.load(sys.stdin) if streamed else [{'file': name, 'source': (ROOT / name).read_text()} for language in ['go', 'python'] for name in POLICY[language]['sources']]
    report = inspect(rows)
    print(json.dumps(report))
    if not streamed and not report['passed']:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
