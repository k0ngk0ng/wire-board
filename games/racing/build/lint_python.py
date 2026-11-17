import ast
import io
import json
import re
import sys
import tokenize


def valid_name(name, kind):
    if name == '_' or name.startswith('__') and name.endswith('__'):
        return True
    if kind == 'class':
        return re.fullmatch(r'[A-Z][A-Za-z0-9]*', name) is not None
    return re.fullmatch(r'[a-z][a-z0-9_]*|[A-Z][A-Z0-9_]*', name) is not None


def inspect_source(source):
    errors = []
    try:
        tree = ast.parse(source)
        if any(item.type == tokenize.COMMENT for item in tokenize.generate_tokens(io.StringIO(source).readline)):
            errors.append('no-comments')
    except (SyntaxError, tokenize.TokenError):
        return ['syntax']
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            if not valid_name(node.name, 'class' if isinstance(node, ast.ClassDef) else 'binding'):
                errors.append('python-name')
        elif isinstance(node, ast.arg):
            if not valid_name(node.arg, 'binding'):
                errors.append('python-name')
        elif isinstance(node, ast.Name) and isinstance(node.ctx, ast.Store):
            if not valid_name(node.id, 'binding'):
                errors.append('python-name')
        elif isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in ['eval', 'exec']:
            errors.append('no-dynamic-evaluation')
    return errors


inputs = json.load(sys.stdin)
json.dump([inspect_source(item['source']) for item in inputs], sys.stdout)
