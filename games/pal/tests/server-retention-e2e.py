import argparse
import json
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]
parser = argparse.ArgumentParser()
parser.add_argument('--publish', action='store_true')
args = parser.parse_args()
if not args.publish:
    parser.error('explicit --publish is required for real immutable server distribution acceptance')
folder = ROOT / '.local/pal/verification'
baseline_id = 'a9234deb9d91a84e0d273f0d0ef3e90e81a7fbdf052c41ed4fcb0c91a35566ac'
baseline = json.loads((ROOT / 'games/pal/assets/server-release.json').read_text())['catalogs'][baseline_id]


def run(command, log):
    with (folder / log).open('w') as output:
        subprocess.run(command, cwd=ROOT, check=True, stdout=output, stderr=subprocess.STDOUT)


run(['python3', '-B', 'games/pal/build/server-package.py', '--retain', '.local/pal/native/rules.wasm'], 'server-retention-package.json')
package = json.loads((folder / 'server-retention-package.json').read_text())['package']
run(['python3', '-B', 'games/pal/assets/publish-server.py', '--package', package, '--publish', '--report', str(folder / 'server-publication-retained.json')], 'server-publication-retained-run.txt')
published = json.loads((folder / 'server-publication-retained.json').read_text())
assert published['passed'] and published['pin']['retainedCores'] == ['7ca83b5bcf13e71c1020ef287d9817e5675adf2cbdf7c627644f4cb3566dd03a']
assert all(published['pin']['files'][name]['url'] == row['url'] for name, row in baseline['files'].items() if name != 'manifest.json')
assert published['pin']['newObjects'] in [0, 2]
first = folder / 'server-publication-retained-first.json'
if not first.exists():
    first.write_bytes((folder / 'server-publication-retained.json').read_bytes())
run(['python3', '-B', 'games/pal/assets/publish-server.py', '--publish', '--report', str(folder / 'server-publication-retained-repeat.json')], 'server-publication-retained-repeat-run.txt')
repeat = json.loads((folder / 'server-publication-retained-repeat.json').read_text())
assert repeat['passed'] and repeat['pin']['assetID'] == published['pin']['assetID'] and repeat['pin']['newObjects'] == repeat['pin']['uploadBytes'] == 0
run(['python3', '-B', 'games/pal/build/prepare-server.py', '--out', '.local/pal/retained-server/pal'], 'prepared-retained-server.json')
run(['python3', '-B', 'games/pal/build/prepare-server.py', '--out', '.local/pal/retained-server/pal'], 'prepared-retained-server-repeat.json')
assert json.loads((folder / 'prepared-retained-server.json').read_text())['passed']
assert json.loads((folder / 'prepared-retained-server-repeat.json').read_text())['reused']
report = {'passed': True, 'scope': 'real original-profile server distribution, retained legacy core addition, unchanged authority/current-core URLs, repeated publish zero upload and CDN-only transactional preparation; not application image acceptance', 'assetID': published['pin']['assetID'], 'unchangedObjectsReused': 2, 'newObjects': published['pin']['newObjects'], 'repeatNewObjects': 0, 'repeatUploadBytes': 0, 'retainedCores': published['pin']['retainedCores']}
(folder / 'server-retention-e2e.json').write_text(json.dumps(report, indent=2) + '\n')
print(json.dumps(report))
