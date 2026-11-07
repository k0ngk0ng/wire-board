// Private output prevents controller acceptance from replacing another task's
// web/dist. Public media is a snapshot of an existing verified local build.
import { cpSync, existsSync, mkdirSync, readdirSync, rmSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "../../../web/node_modules/@vitejs/plugin-react/dist/index.js";
import { gameLoadManifest } from "../../../web/scripts/game-load-manifest.mjs";
const root = fileURLToPath(new URL("../../../", import.meta.url));
const source = path.join(root, "web/dist"),
  work = path.join(root, ".local/gamepad");
if (!existsSync(path.join(source, "index.html")))
  throw Error(
    "Build the regular web frontend once before the private controller acceptance build.",
  );
const publicDir = path.join(work, "public");
rmSync(publicDir, { recursive: true, force: true });
mkdirSync(publicDir, { recursive: true });
for (const entry of readdirSync(source)) {
  if (["assets", "index.html", "game-load-manifest.json"].includes(entry))
    continue;
  cpSync(path.join(source, entry), path.join(publicDir, entry), {
    recursive: true,
  });
}
export default {
  root: path.join(root, "web"),
  publicDir,
  plugins: [react(), gameLoadManifest()],
  resolve: { dedupe: ["react", "react-dom", "lucide-react", "three"] },
  build: {
    target: "es2022",
    outDir: path.join(work, "web-dist"),
    emptyOutDir: true,
  },
};
