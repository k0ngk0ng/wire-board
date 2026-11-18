import importlib.util
import json
from pathlib import Path
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]
TOOL = ROOT / 'games/pal/build/check-ci.py'
result = subprocess.run([sys.executable, '-B', TOOL, '--plan'], cwd=ROOT, capture_output=True, text=True)
assert result.returncode == 0, result.stderr
plan = json.loads(result.stdout)
assert plan['profile'] == 'original-98' and plan['preparedOriginalCount'] >= 15
assert plan['nativeOutputDirectory'] == '.local/pal/dense-battle-candidate/campaign'
assert plan['validateProductionBundle'] is True
assert plan['requiredRuntime'] == 'verified-server-publication'
checks = {n: group['package'] for group in plan['checks'] for n in group['tests']}
assert checks['TestOriginalHiddenSearchEventInventoryPreserved'] == './games/pal/tests'
assert checks['TestPALOriginalSoloPrivateSaveRestartHTTP'] == checks['TestPALOriginalManualCheckpointReloadAndOldPageHTTP'] == './internal/server'
assert checks['TestNativeOriginalOpeningSaveRestore'] == './games/pal/engine'
assert all('Reference' not in name and 'RetainedRuntimeCatalog' not in name for name in checks)
assert len(checks) == sum(len(group['tests']) for group in plan['checks'])
assert plan['historicalArtifactMode'] == 'local-research'
assert all(tool['sha256'] and tool['bytes'] > 0 for target in plan['toolchains'].values() for tool in target)
workflow = (ROOT / '.github/workflows/build.yml').read_text()
assert "if: matrix.game == 'pal' && matrix.backend" in workflow
assert 'python3 -B games/pal/build/check-ci.py' in workflow
dependency_step = workflow.split('      - name: Install dependencies for game rendering checks',1)[1].split('      - name:',1)[0]
assert "matrix.game == 'pal'" in dependency_step
assert 'runtime-candidates/audio' not in workflow and 'publish-server.py' not in workflow
assert 'name: pal-clean-ci' in workflow
pal_upload = workflow.split('      - name: Validate PAL from clean source and original resources',1)[1].split('      - name: Verify changed Contra',1)[0]
generic_upload = workflow.split('name: game-${{ matrix.game }}-acceptance',1)[0].rsplit('      - uses: actions/upload-artifact@v4',1)[1]
assert "if: always() && matrix.game == 'pal' && matrix.backend" in pal_upload
assert "if: always() && (matrix.game != 'pal' || !matrix.backend)" in generic_upload
spec = importlib.util.spec_from_file_location('ci_scope', ROOT / 'scripts/ci/scope.py')
scope = importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)
for change in ['games/pal/native/bridge.c', 'games/pal/build/native.py', 'games/pal/build/check-ci.py', 'games/pal/assets/source-manifest.json', 'games/pal/assets/authority-release.json', 'games/pal/assets/server-release.json']:
    rows = scope.plan([change], ['pal', 'rail'])['matrix']['include']
    assert rows == [{'game':'pal','backend':True,'shard':0,'count':1}], (change, rows)
assert scope.plan(['games/pal/web/board.css'],['pal','rail'])['matrix']['include'] == [{'game':'pal','backend':False,'shard':0,'count':1}]
print(json.dumps({'passed': True, 'requiredGoTests': len(checks), 'historicalLanePreserved': True}))
