import ast
import io
import json
import re
import sys
import tokenize

data = json.load(sys.stdin)
issues = []
try:
    tree = ast.parse(data['Source'], filename=data['Name'])
    for token in tokenize.generate_tokens(io.StringIO(data['Source']).readline):
        if token.type == tokenize.COMMENT:
            issues.append(f"{data['Name']}:{token.start[0]}: prohibited code comment")
    for node in ast.walk(tree):
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name not in data.get('FrameworkMethods', []) and not re.fullmatch(r'_*[a-z][a-z0-9]*(?:_[a-z0-9]+)*_*', node.name):
            issues.append(f"{data['Name']}:{node.lineno}: invalid Python function name {node.name}")
        if isinstance(node, ast.ClassDef) and not re.fullmatch(r'[A-Z][A-Za-z0-9]*', node.name):
            issues.append(f"{data['Name']}:{node.lineno}: invalid Python class name {node.name}")
except (SyntaxError, tokenize.TokenError, IndentationError) as error:
    issues.append(str(error))
json.dump(issues, sys.stdout)
