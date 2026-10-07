import json
import subprocess
import unittest
from unittest.mock import patch

import test_go_shard as shard


class GoShardTests(unittest.TestCase):
    def test_all_names_appear_in_exactly_one_shard(self):
        names = [f"TestScenario{i}" for i in range(1000)] + ["Example", "FuzzActions", "Test中文"]
        groups = [shard.partition(names, index, 8) for index in range(8)]
        flattened = [name for group in groups for name in group]
        self.assertCountEqual(flattened, names)
        self.assertEqual(len(flattened), len(set(flattened)))
        # Adding another test must not reshuffle existing tests.
        for index, group in enumerate(groups):
            self.assertEqual(group, [name for name in shard.partition(names + ["TestNew"], index, 8) if name != "TestNew"])

    def test_discovery_includes_examples_and_fuzz_seeds_not_benchmarks(self):
        lines = [("first", "TestOne"), ("second", "TestOne"), ("first", "Example"),
                 ("first", "ExampleThing"), ("first", "FuzzActions"), ("first", "BenchmarkSpeed"),
                 ("first", "ok example/module 0.1s"), ("first", "Test中文")]
        output = "\n".join(json.dumps({"Action": "output", "Package": package, "Output": name + "\n"}) for package, name in lines)
        with patch("test_go_shard.subprocess.run", return_value=subprocess.CompletedProcess([], 0, output, "")):
            self.assertEqual(shard.discover(["./..."]), ["Example", "ExampleThing", "FuzzActions", "TestOne", "Test中文"])

    def test_failed_discovery_cannot_pass_verification(self):
        with patch("test_go_shard.subprocess.run", return_value=subprocess.CompletedProcess([], 2, "", "compile failed")), patch("sys.stderr"):
            with self.assertRaises(SystemExit) as error:
                shard.discover(["./..."])
            self.assertEqual(error.exception.code, 2)

    def test_invalid_or_empty_shards_fail(self):
        for index, count in [(-1, 8), (8, 8), (0, 0)]:
            with self.assertRaises(ValueError):
                shard.partition([], index, count)
        with patch("test_go_shard.subprocess.run", return_value=subprocess.CompletedProcess([], 0, "", "")):
            with self.assertRaises(SystemExit):
                shard.discover(["./..."])

    def test_run_failure_is_returned_and_pattern_is_exact(self):
        with patch("sys.argv", ["test_go_shard.py", "--index", "0", "--count", "1", "--race"]), patch("test_go_shard.discover", return_value=["TestOne", "Example"]), patch("test_go_shard.subprocess.run", return_value=subprocess.CompletedProcess([], 1)) as run:
            self.assertEqual(shard.main(), 1)
            command = run.call_args.args[0]
            self.assertIn("-race", command)
            self.assertIn("-count=1", command)
            self.assertEqual(command[command.index("-run") + 1], "^(?:TestOne|Example)$")
            self.assertEqual(command[-1], "./...")


if __name__ == "__main__":
    unittest.main()
