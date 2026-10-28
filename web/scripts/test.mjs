import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { discoverGameTests } from "./game-test-discovery.mjs";

const root = fileURLToPath(new URL("../../", import.meta.url));
const selected = process.argv[2];
const tests = discoverGameTests(root, selected);
if (!tests.length) throw new Error("No frontend tests selected");
const result = spawnSync(
  process.execPath,
  ["--experimental-strip-types", "--test", ...tests],
  { cwd: root, stdio: "inherit" },
);
if (result.error) throw result.error;
process.exit(result.status ?? 1);
