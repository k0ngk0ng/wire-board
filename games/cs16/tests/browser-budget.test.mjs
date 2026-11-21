import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';

test('the authorized one millisecond touch allowance agrees across both browser gates and report metadata', () => {
  const source = readFileSync(new URL('./native-input.mjs', import.meta.url), 'utf8');
  const match = source.match(/responseP95MS <= \(options\.touch \? (\d+) : (\d+)\)/);
  assert.ok(match);
  assert.equal(Number(match[1]), 51);
  assert.equal(Number(match[2]), 33);
  const cases = [[49, true], [50, true], [50.8, true], [51, true], [51.001, false], [52, false]];
  for (const [value, accepted] of cases) assert.equal(value <= Number(match[1]), accepted);
  const script = `import ast,json,sys
from pathlib import Path
path=Path(sys.argv[1])
tree=ast.parse(path.read_text())
function=next(node for node in tree.body if isinstance(node,ast.FunctionDef) and node.name=='check_sample_budgets')
namespace={}
exec(compile(ast.Module(body=[function],type_ignores=[]),str(path),'exec'),namespace)
report=next(node.value for node in tree.body if isinstance(node,ast.Assign) and any(isinstance(target,ast.Name) and target.id=='REPORT' for target in node.targets))
budgets=ast.literal_eval(next(value for key,value in zip(report.keys,report.values) if isinstance(key,ast.Constant) and key.value=='budgets'))
results=[]
for value in [49,50,50.8,51,51.001,52]:
 summary={'keyboardP95MS':None,'mouseP95MS':None,'touchP95MS':value,'frameP95MS':16.8,'frameP99MS':16.8,'visibleFPS':60}
 try:
  namespace['check_sample_budgets'](summary)
  results.append(True)
 except AssertionError:
  results.append(False)
print(json.dumps({'budgets':budgets,'accepted':results}))`;
  const result = spawnSync('python3', ['-B', '-c', script, fileURLToPath(new URL('./accept_browser.py', import.meta.url))], { encoding: 'utf8' });
  assert.equal(result.status, 0, result.stderr);
  const output = JSON.parse(result.stdout);
  assert.equal(output.budgets.touchP95MS, 51);
  assert.equal(output.budgets.keyboardP95MS, 33);
  assert.deepEqual(output.accepted, cases.map(value => value[1]));
});
