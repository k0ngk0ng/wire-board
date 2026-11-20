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
    try:
        tree = ast.parse(source)
        if any(token.type == tokenize.COMMENT for token in tokenize.generate_tokens(io.StringIO(source).readline)):
            issues.append('no-comments')
    except (SyntaxError, tokenize.TokenError):
        return ['syntax']
    testing = '/tests/' in item['file'] or item['file'].rsplit('/', 1)[-1].startswith('test_')
    for node in ast.walk(tree):
        if isinstance(node, (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)) and ast.get_docstring(node):
            issues.append('no-docstrings')
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)) and not valid_name(node.name, 'class' if isinstance(node, ast.ClassDef) else 'binding'):
            issues.append('python-name')
        if isinstance(node, ast.arg) and not valid_name(node.arg, 'binding'):
            issues.append('python-name')
        if isinstance(node, ast.Name) and isinstance(node.ctx, ast.Store) and not valid_name(node.id, 'binding'):
            issues.append('python-name')
        if isinstance(node, ast.Call):
            if isinstance(node.func, ast.Name) and node.func.id in {'eval', 'exec', 'compile'}:
                issues.append('no-dynamic-evaluation')
            if isinstance(node.func, ast.Attribute):
                name = node.func.attr
                if name in {'TemporaryDirectory', 'NamedTemporaryFile', 'mkdtemp', 'mkstemp'} and not any(keyword.arg == 'dir' for keyword in node.keywords):
                    issues.append('workspace-temporary-directory')
                if name in {'home', 'gettempdir', 'expanduser'}:
                    issues.append('workspace-output-directory')
        if isinstance(node, ast.Constant) and isinstance(node.value, str) and not testing:
            if any(match.group(1) != 'hotlinemiami' for match in re.finditer(r'\.local/([a-z][a-z0-9_-]*)(?:/|$|\s)', node.value)):
                issues.append('local-output-directory')
            if re.match(r'^(?:/tmp/.+|/private/tmp/.+|~/.+)', node.value):
                issues.append('workspace-output-directory')
    return issues


json.dump([inspect_source(item) for item in json.load(sys.stdin)], sys.stdout)
