import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from collections import Counter

ROOT=Path(__file__).resolve().parents[2]
WORK=ROOT/'.local/catan/ci-partitions'
WORK.mkdir(parents=True,exist_ok=True)
spec=importlib.util.spec_from_file_location('scope',Path(__file__).with_name('scope.py'))
scope=importlib.util.module_from_spec(spec)
spec.loader.exec_module(scope)


def selections(commands):
    result=Counter()
    for command in commands:
        pattern=command[command.index('-run')+1]
        names=pattern[4:-2].split('|') if pattern.startswith('^(?:') else [pattern]
        result.update((command[-1],name) for name in names)
    return result


class PartitionE2E(unittest.TestCase):
    def test_all_real_catan_selections_partition_exactly_once(self):
        for race in [False,True]:
            original=selections(scope.go_commands(ROOT,'catan',race=race,extended=True))
            divided=Counter()
            for index in range(20):
                commands=scope.go_commands(ROOT,'catan',index,20,race=race,extended=True)
                self.assertTrue(all('-timeout=45m' in command for command in commands))
                divided.update(selections(commands))
            self.assertEqual(divided,original)
            self.assertTrue(all(value==1 for value in divided.values()))

    def replay(self, fail=False):
        with tempfile.TemporaryDirectory(dir=WORK) as name:
            root=Path(name)
            source=root/'games/catan/server/rules_test.go'
            source.parent.mkdir(parents=True)
            names=[]
            for bucket in range(20):
                index=0
                while True:
                    candidate='TestCatanConcurrentBucket'+str(bucket)+'Case'+str(index)
                    if int.from_bytes(hashlib.sha256(candidate.encode()).digest()[:8],'big')%20==bucket:
                        names.append(candidate)
                        break
                    index+=1
            source.write_text('package catan\n'+'\n'.join('func '+item+'() {}' for item in names))
            frontend=root/'games/catan/tests/input.test.mjs'
            frontend.parent.mkdir(parents=True)
            frontend.write_text('')
            logs=root/'calls'
            logs.mkdir()
            binary=root/'bin'
            binary.mkdir()
            driver='''#!/usr/bin/env python3
import json,os,sys,time,uuid
from pathlib import Path
start=time.monotonic_ns()
time.sleep(0.02)
row={'tool':Path(sys.argv[0]).name,'args':sys.argv[1:],'start':start,'end':time.monotonic_ns()}
(Path(os.environ['PARTITION_CALLS'])/(uuid.uuid4().hex+'.json')).write_text(json.dumps(row))
if os.environ.get('PARTITION_FAIL') and os.environ['PARTITION_FAIL'] in ' '.join(sys.argv[1:]):sys.exit(1)
'''
            for tool in ['go','node']:
                target=binary/tool
                target.write_text(driver)
                target.chmod(0o755)
            env={**os.environ,'PATH':str(binary)+os.pathsep+os.environ['PATH'],'PARTITION_CALLS':str(logs),'PARTITION_FAIL':names[0] if fail else ''}
            args=SimpleNamespace(game='catan',backend=True,shard=0,count=1,catan_extended=True)
            with patch.object(scope,'ROOT',root),patch.dict(os.environ,env,clear=True):
                if fail:
                    with self.assertRaises(subprocess.CalledProcessError):scope.run(args)
                else:scope.run(args)
            summary=json.loads((root/'.local/ci/catan/partitions.json').read_text())
            self.assertEqual(summary['passed'],not fail)
            rows=[json.loads(path.read_text()) for path in logs.glob('*.json')]
            calls=[row for row in rows if row['tool']=='go']
            if not fail:
                self.assertEqual(sorted(summary['completed']),list(range(20)))
                self.assertEqual(sum(row['tool']=='node' for row in rows),1)
                self.assertEqual(len(calls),40)
                for race in [False,True]:
                    commands=[['go',*row['args']] for row in calls if ('-race' in row['args'])==race]
                    self.assertEqual(selections(commands),Counter({('./games/catan/server',item):1 for item in names}))
                endpoints=sorted([(row['start'],1) for row in calls]+[(row['end'],-1) for row in calls])
                active=maximum=0
                for moment,delta in endpoints:
                    active+=delta
                    maximum=max(maximum,active)
                self.assertGreater(maximum,1)
                self.assertLessEqual(maximum,2)
            else:
                self.assertTrue(summary['failed'])
                self.assertTrue(any(not json.loads(path.read_text())['passed'] for path in (root/'.local/ci/catan').glob('result-*.json')))

    def test_real_processes_cover_every_partition_with_bounded_parallelism(self):
        self.replay()

    def test_failed_partition_keeps_the_aggregate_failed(self):
        self.replay(fail=True)


if __name__=='__main__':
    result=unittest.TextTestRunner(verbosity=2).run(unittest.defaultTestLoader.loadTestsFromTestCase(PartitionE2E))
    (WORK/'result.json').write_text(json.dumps({'passed':result.wasSuccessful(),'tests':result.testsRun,'failures':len(result.failures),'errors':len(result.errors)},indent=2)+'\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
