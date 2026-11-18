import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { cpSync, mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { preparePokepathNative } from "./browser-runtime.mjs";
import { gameLoadManifest } from "../../../web/scripts/game-load-manifest.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const { defineConfig } = require("vite");
const reactModule = require("@vitejs/plugin-react");
const react = reactModule.default ?? reactModule;
const destination = join(root, ".local/pokepath/native-public");
const runtime = preparePokepathNative();
rmSync(destination, { recursive: true, force: true });
mkdirSync(destination, { recursive: true });
for (const source of [join(root, "web/public"), join(root, "games/pokepath/assets/public"), runtime]) cpSync(source, destination, { recursive: true });

export default defineConfig({
  root: join(root, "web"),
  base: "/",
  publicDir: destination,
  plugins: [react(), gameLoadManifest()],
  resolve: { dedupe: ["react", "react-dom", "lucide-react", "three"] },
  build: { target: "es2022", outDir: join(root, ".local/pokepath/native-web"), emptyOutDir: true },
});
