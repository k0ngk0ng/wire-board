import ast
import io
import json
import re
import sys
import tokenize


def valid_name(name, kind):
    if name == '_' or name.startswith('__') and name.endswith('__'):
        return True
    if name in {'setUp', 'tearDown', 'setUpClass', 'tearDownClass'} or re.fullmatch(r'do_(GET|POST|PUT|HEAD|PATCH|DELETE|OPTIONS)', name):
        return True
    pattern = r'[A-Z][A-Za-z0-9]*' if kind == 'class' else r'_?[a-z][a-z0-9_]*|[A-Z][A-Z0-9_]*'
    return re.fullmatch(pattern, name) is not None


def inspect_source(item):
    source = item['source']
    issues = []
    changed = set(item['changedLines']) if 'changedLines' in item else None
    def active(node, span=True):
        if changed is None:
            return True
        first = getattr(node, 'lineno', 0)
        last = getattr(node, 'end_lineno', first) if span else first
        return any(line in changed for line in range(first, last + 1))
    def record(issue, node, span=True):
        if active(node, span):
            issues.append(issue)
    try:
        tree = ast.parse(source)
        if any(token.type == tokenize.COMMENT and (changed is None or token.start[0] in changed) for token in tokenize.generate_tokens(io.StringIO(source).readline)):
            issues.append('no-comments')
    except (SyntaxError, tokenize.TokenError):
        return ['syntax']
    testing = '/tests/' in item['file'] or item['file'].rsplit('/', 1)[-1].startswith('test_')
    for node in ast.walk(tree):
        if isinstance(node, (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)) and ast.get_docstring(node):
            record('no-docstrings', node.body[0])
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)) and not valid_name(node.name, 'class' if isinstance(node, ast.ClassDef) else 'binding'):
            record('python-name', node, False)
        if isinstance(node, ast.arg) and not valid_name(node.arg, 'binding'):
            record('python-name', node)
        if isinstance(node, ast.Name) and isinstance(node.ctx, ast.Store) and not valid_name(node.id, 'binding'):
            record('python-name', node)
        if isinstance(node, ast.Call):
            if isinstance(node.func, ast.Name) and node.func.id in {'eval', 'exec', 'compile'}:
                record('no-dynamic-evaluation', node)
            if isinstance(node.func, ast.Name) and node.func.id in {'TemporaryDirectory', 'NamedTemporaryFile', 'mkdtemp', 'mkstemp'} and not any(keyword.arg == 'dir' for keyword in node.keywords):
                record('workspace-temporary-directory', node)
            if isinstance(node.func, ast.Attribute):
                name = node.func.attr
                if name in {'TemporaryDirectory', 'NamedTemporaryFile', 'mkdtemp', 'mkstemp'} and not any(keyword.arg == 'dir' for keyword in node.keywords):
                    record('workspace-temporary-directory', node)
                if name in {'home', 'gettempdir', 'expanduser'}:
                    record('workspace-output-directory', node)
        if isinstance(node, ast.Constant) and isinstance(node.value, str) and not testing:
            if any(match.group(1) != 'cs16' for match in re.finditer(r'\.local/([a-z][a-z0-9_-]*)(?:/|$|\s)', node.value)):
                record('local-output-directory', node)
            if re.match(r'^(?:/tmp/.+|/private/tmp/.+|~/.+)', node.value):
                record('workspace-output-directory', node)
    return issues


json.dump([inspect_source(item) for item in json.load(sys.stdin)], sys.stdout)
