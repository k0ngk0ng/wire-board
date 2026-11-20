import argparse
import ast
import io
import json
from pathlib import Path
import re
import tokenize

ROOT = Path(__file__).resolve().parents[2]


def python_errors(source):
    errors = []
    try:
        tree = ast.parse(source)
        if any(token.type == tokenize.COMMENT for token in tokenize.generate_tokens(io.StringIO(source).readline)):
            errors.append('no-comments')
    except (SyntaxError, tokenize.TokenError):
        return ['python-syntax']
    for node in ast.walk(tree):
        name = None
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            name = node.name
        elif isinstance(node, ast.arg):
            name = node.arg
        elif isinstance(node, ast.Name) and isinstance(node.ctx, ast.Store):
            name = node.id
        if name and not re.fullmatch(r'[a-z_][a-z0-9_]*|[A-Z][A-Z0-9_]*|(?:do_(?:GET|HEAD|PUT|POST)|setUp|tearDown)|[A-Z][A-Za-z0-9]*', name):
            errors.append('python-name')
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in ['exec', 'eval']:
            errors.append('no-dynamic-evaluation')
        if isinstance(node, ast.Constant) and isinstance(node.value, str) and node.value.startswith('.local/') and node.value != '.local/' and not node.value.startswith(('.local/ci-traffic', '.local/ci-resource-cache')) and not re.fullmatch(r'\.local/[a-z][a-z0-9_-]*/(?:cache/ci-resources|verification/ci-traffic/resources\.json)', node.value):
            errors.append('traffic-output-directory')
    return sorted(set(errors))


def workflow_errors(source, maintenance=False, composite=False):
    errors = []
    if re.search(r'resource_store\.py\s+hydrate|fetch-assets\.py[^\n]*--oss|(?:curl|wget)[^\n]*(?:OSS_ENDPOINT|CDN_ORIGIN|ASSETS_BASE_URL|/bga/|https?://cdn\.)', source):
        errors.append('direct-resource-download')
    if not maintenance and re.search(r'resources\.py[^\n]*\bprime\b|args=\(prime\b', source):
        errors.append('manual-cache-only')
    if composite:
        if 'continue-on-error' in source or 'OSS_ACCESS_KEY' in source or 'secrets.' in source:
            errors.append('cache-failure')
        if 'restore --scope "$RESOURCE_SCOPE" --max-download-bytes 0' not in source:
            errors.append('zero-routine-budget')
        if 'actions/cache/restore@v4' not in source or 'actions/upload-artifact@v4' not in source:
            errors.append('cache-evidence')
    if maintenance:
        trigger = re.search(r'^on:\s*\n(.*?)(?=^[a-zA-Z][a-zA-Z_-]*:|\Z)', source, re.M | re.S)
        names = re.findall(r'^  ([a-z_]+):', trigger[1], re.M) if trigger else []
        if names != ['workflow_dispatch'] or 'continue-on-error' in source:
            errors.append('manual-cache-only')
        if '--max-download-bytes "$MAX_DOWNLOAD_BYTES"' not in source or 'actions/cache/save@v4' not in source:
            errors.append('bounded-cache-preparation')
    for block in re.split(r'^\s+- (?:name:|uses:)', source, flags=re.M):
        if './.github/actions/restore-resources' in block and ('OSS_ACCESS_KEY' in block or 'continue-on-error' in block):
            errors.append('cache-failure')
        if not maintenance and 'OSS_ACCESS_KEY' in block and 'scripts/cdn_delivery.py upload' not in block and 'Smoke test Linux binary' not in block:
            errors.append('paid-read-credentials')
    return sorted(set(errors))


def publication_errors(source):
    tree = ast.parse(source)
    publish = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == 'publish')
    errors = []
    for node in ast.walk(publish):
        if isinstance(node, ast.Constant) and isinstance(node.value, str) and node.value.startswith('GET\n'):
            errors.append('publication-get')
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id == 'Request':
            method = next((keyword.value for keyword in node.keywords if keyword.arg == 'method'), None)
            if not isinstance(method, ast.Constant) or method.value not in ['HEAD', 'PUT']:
                errors.append('publication-get')
        if isinstance(node, ast.Attribute) and node.attr == 'get' and isinstance(node.value, ast.Name) and node.value.id in ['client', 'store']:
            errors.append('publication-get')
    return sorted(set(errors))


def inspect(root):
    rows = []
    policy = json.loads((root / 'scripts/ci/traffic-policy.json').read_text())
    errors = []
    if any(type(policy.get(key)) is not int or policy[key] != 0 for key in ['normalDownloadBytes', 'publicationGetBytes']):
        errors.append('zero-routine-budget')
    if type(policy.get('maintenanceMaxDownloadBytes')) is not int or not 0 < policy['maintenanceMaxDownloadBytes'] <= 2147483648:
        errors.append('maintenance-hard-limit')
    rows.append({'path': 'scripts/ci/traffic-policy.json', 'errors': errors})
    for path in sorted((root / '.github/workflows').glob('*.yml')):
        rows.append({'path': path.relative_to(root).as_posix(), 'errors': workflow_errors(path.read_text(), maintenance=path.name == 'resource-cache.yml')})
    composite = root / '.github/actions/restore-resources/action.yml'
    rows.append({'path': composite.relative_to(root).as_posix(), 'errors': workflow_errors(composite.read_text(), composite=True)})
    publication = root / 'scripts/cdn_delivery.py'
    rows.append({'path': publication.relative_to(root).as_posix(), 'errors': publication_errors(publication.read_text())})
    for name in policy['pythonFiles']:
        path = root / name
        if not path.exists():
            continue
        errors = python_errors(path.read_text())
        if not re.fullmatch(r'scripts/ci/[a-z][a-z0-9_]*\.py', name):
            errors.append('python-file-directory')
        rows.append({'path': name, 'errors': errors})
    return rows


def self_test():
    samples = [('def valid_name(value):\n return value\n', []), ('def badName():\n pass\n', ['python-name']), ('# forbidden\npass\n', ['no-comments']), ('eval("1")', ['no-dynamic-evaluation']), ('path=".local/unscoped/file"', ['traffic-output-directory'])]
    for source, expected in samples:
        if python_errors(source) != expected:
            raise ValueError('Python traffic lint self-test failed')
    if workflow_errors('run: python3 -B scripts/resource_store.py hydrate --oss') != ['direct-resource-download']:
        raise ValueError('Workflow traffic lint self-test failed')
    if publication_errors("def publish():\n canonical='GET\\n'\n") != ['publication-get']:
        raise ValueError('Publication traffic lint self-test failed')
    return len(samples) + 2


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--root', type=Path, default=ROOT)
    parser.add_argument('--self-test', action='store_true')
    args = parser.parse_args()
    root = args.root.resolve()
    if not root.is_relative_to(ROOT):
        raise ValueError('Traffic lint workspace must remain inside the repository')
    examples = self_test() if args.self_test else 0
    rows = inspect(root)
    report = {'passed': not any(row['errors'] for row in rows), 'scope': 'CI workflows, resource cache action, publication GET prohibition and registered Python naming, directories and forbidden constructs', 'files': rows, 'examples': examples}
    output = root / '.local/ci-traffic/lint.json'
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report))
    raise SystemExit(0 if report['passed'] else 1)


if __name__ == '__main__':
    main()
