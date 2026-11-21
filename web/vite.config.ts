import { assetBase } from "./scripts/cdn-base.mjs";
import { gameLoadManifest } from "./scripts/game-load-manifest.mjs";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";
import { preparePublicAssets } from "./scripts/public-assets.mjs";
import { prepareBasketballDelivery } from "../games/basketball/build/runtime-delivery.mjs";
import { prepareContraDelivery } from "../games/contra/build/runtime-delivery.mjs";
import { prepareYuYuDelivery } from "../games/yuyuhakusho/build/runtime-delivery.mjs";
import { preparePvZRuntime } from "../games/pvz/build/runtime.mjs";
import { preparePokepathNative } from "../games/pokepath/build/browser-runtime.mjs";
import { prepareHotlineDelivery } from "../games/hotlinemiami/build/runtime-delivery.mjs";
import { prepareCS16Delivery } from "../games/cs16/build/runtime-delivery.mjs";
const root = fileURLToPath(new URL("../", import.meta.url));
export default defineConfig({
  base: assetBase(process.env.WIRE_BOARD_ASSET_BASE),
  define: { "import.meta.env.VITE_CDN_ORIGIN": JSON.stringify(process.env.CDN_ORIGIN || "") },
  publicDir: preparePublicAssets(root, [prepareBasketballDelivery(root), prepareContraDelivery(root), preparePvZRuntime(root), prepareYuYuDelivery(root), preparePokepathNative(), prepareHotlineDelivery(root), prepareCS16Delivery(root)], ["hotlinemiami", "cs16"]),
  plugins: [react(), gameLoadManifest()],
  resolve: { dedupe: ["react", "react-dom", "lucide-react", "three", "fflate"] },
  worker: { format: "es" },
  server: {
    proxy: {
      "/api": { target: "http://127.0.0.1:8080", ws: true },
      "/healthz": "http://127.0.0.1:8080",
    },
  },
  build: { target: "es2022" },
});
