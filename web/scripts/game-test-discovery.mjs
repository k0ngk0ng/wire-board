import { readdirSync } from "node:fs";
import path from "node:path";

export function discoverGameTests(root, selected) {
  const games = readdirSync(path.join(root, "games"), { withFileTypes: true })
    .filter((entry) => entry.isDirectory() && !entry.name.startsWith("_") && !entry.name.startsWith("."))
    .map((entry) => entry.name);
  if (selected && !games.includes(selected))
    throw new Error(`Unknown game: ${selected}`);
  const directories = (selected ? [selected] : games)
    .flatMap((game) => [`games/${game}/tests`, `games/${game}/web`]);
  if (!selected) directories.unshift("web/tests");
  return directories.flatMap((directory) => {
    try {
      return readdirSync(path.join(root, directory))
        .filter((name) => name.endsWith(".test.mjs"))
        .map((name) => path.join(root, directory, name));
    } catch (error) {
      if (error.code === "ENOENT") return [];
      throw error;
    }
  }).sort();
}
