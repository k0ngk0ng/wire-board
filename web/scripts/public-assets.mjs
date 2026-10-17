import { cpSync, existsSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import path from "node:path";

// Materialize only explicitly public assets. Source code and private game data
// never enter Vite's public directory. All generated files stay in the repo.
export function preparePublicAssets(root, generatedSources = []) {
  const destination = path.join(root, ".local/web-public");
  const sources = [path.join(root, "web/public"), ...generatedSources];
  for (const entry of readdirSync(path.join(root, "games"), {
    withFileTypes: true,
  })) {
    if (entry.isDirectory())
      sources.push(path.join(root, "games", entry.name, "assets/public"));
  }
  const owners = new Map();
  function collect(directory, prefix = "") {
    for (const entry of readdirSync(directory, { withFileTypes: true })) {
      const relative = path.join(prefix, entry.name);
      const source = path.join(directory, entry.name);
      if (entry.isDirectory()) collect(source, relative);
      else {
        if (!entry.isFile())
          throw new Error(`Public assets must be regular files: ${source}`);
        if (owners.has(relative))
          throw new Error(`Duplicate public asset: ${relative}`);
        owners.set(relative, source);
      }
    }
  }
  for (const source of sources) if (existsSync(source)) collect(source);
  rmSync(destination, { recursive: true, force: true });
  mkdirSync(destination, { recursive: true });
  for (const [relative, source] of owners) {
    const target = path.join(destination, relative);
    mkdirSync(path.dirname(target), { recursive: true });
    cpSync(source, target);
  }
  return destination;
}
