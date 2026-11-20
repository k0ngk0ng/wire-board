import ast
import io
import json
import re
import sys
import tokenize

source = sys.stdin.read()
tree = ast.parse(source)
issues = []
for token in tokenize.generate_tokens(io.StringIO(source).readline):
    if token.type == tokenize.COMMENT:
        issues.append(f"line {token.start[0]}: prohibited code comment")
for node in ast.walk(tree):
    if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and not re.fullmatch(r"[a-z_][a-z0-9_]*", node.name):
        issues.append(f"line {node.lineno}: invalid function {node.name}")
    if isinstance(node, ast.ClassDef) and not re.fullmatch(r"[A-Z][A-Za-z0-9]*", node.name):
        issues.append(f"line {node.lineno}: invalid class {node.name}")
    if isinstance(node, ast.Name) and isinstance(node.ctx, ast.Store) and not re.fullmatch(r"(?:[a-z_][a-z0-9_]*|[A-Z][A-Z0-9_]*)", node.id):
        issues.append(f"line {node.lineno}: invalid identifier {node.id}")
    if isinstance(node, ast.Expr) and isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
        issues.append(f"line {node.lineno}: prohibited code docstring")
    if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in {"eval", "exec"}:
        issues.append(f"line {node.lineno}: prohibited {node.func.id}")
print(json.dumps(issues))
