import ast
import io
import json
import sys
import tokenize

payload = json.load(sys.stdin)
issues = []
try:
    tree = ast.parse(payload['Source'], filename=payload['Name'])
    for token in tokenize.generate_tokens(io.StringIO(payload['Source']).readline):
        if token.type == tokenize.COMMENT:
            issues.append(f"{payload['Name']}:{token.start[0]}:{token.start[1]+1}: prohibited code comment")
    for node in ast.walk(tree):
        if isinstance(node, (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)) and ast.get_docstring(node) is not None:
            issues.append(f"{payload['Name']}:{getattr(node, 'lineno', 1)}: prohibited documentation string")
except (SyntaxError, tokenize.TokenError, IndentationError) as error:
    issues.append(str(error))
print(json.dumps(issues))
