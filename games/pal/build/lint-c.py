import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[3]
payload = json.load(sys.stdin)
compiler = next((ROOT / '.local/pal/toolchains').glob('wasi-sdk-34.0-*/bin/clang'))
common = [str(compiler), '-x', 'c', '-fsyntax-only', '-DPAL_CLASSIC=1', '-DPAL_HAS_MP3=0', '-DPAL_HAS_OGG=0', '-DPAL_HAS_OPUS=0', '-DPAL_HAS_GLSL=0', '-I' + str(ROOT / 'games/pal/native/include'), '-I' + str(ROOT / 'games/pal/native/upstream')]
syntax = subprocess.run([*common, '-'], input=payload['Source'], capture_output=True, text=True)
tokens = subprocess.run([*common, '-Xclang', '-dump-raw-tokens', '-'], input=payload['Source'], capture_output=True, text=True)
issues = []
if syntax.returncode:
    issues.append(payload['Name'] + ': ' + syntax.stderr.strip())
if tokens.returncode:
    issues.append(payload['Name'] + ': C tokenization failed')
for match in re.finditer(r"(?m)^comment\s+'", tokens.stderr):
    issues.append(payload['Name'] + ': prohibited code comment')
print(json.dumps(issues))
