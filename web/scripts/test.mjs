import { readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import path from "node:path";

const root = fileURLToPath(new URL("../../", import.meta.url));
const selected = process.argv[2];
const games = readdirSync(path.join(root, "games"), { withFileTypes: true })
  .filter(
    (entry) =>
      entry.isDirectory() &&
      !entry.name.startsWith("_") &&
      !entry.name.startsWith("."),
  )
  .map((entry) => entry.name);
if (selected && !games.includes(selected))
  throw new Error(`Unknown game: ${selected}`);
const directories = selected
  ? [`games/${selected}/tests`]
  : ["web/tests", ...games.map((game) => `games/${game}/tests`)];
const tests = directories
  .flatMap((directory) => {
    try {
      return readdirSync(path.join(root, directory))
        .filter((name) => name.endsWith(".test.mjs"))
        .map((name) => path.join(root, directory, name));
    } catch (error) {
      if (error.code === "ENOENT") return [];
      throw error;
    }
  })
  .sort();
if (!tests.length) throw new Error("No frontend tests selected");
const result = spawnSync(
  process.execPath,
  ["--experimental-strip-types", "--test", ...tests],
  { cwd: root, stdio: "inherit" },
);
if (result.error) throw result.error;
process.exit(result.status ?? 1);
