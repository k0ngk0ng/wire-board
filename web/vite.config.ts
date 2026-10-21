import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";
import { preparePublicAssets } from "./scripts/public-assets.mjs";
import { prepareBasketballDelivery } from "../games/basketball/build/runtime-delivery.mjs";
const root = fileURLToPath(new URL("../", import.meta.url));
export default defineConfig({
  publicDir: preparePublicAssets(root, [
    prepareBasketballDelivery(root),
  ]),
  plugins: [react()],
  resolve: { dedupe: ["react", "react-dom", "lucide-react", "three"] },
  server: {
    proxy: {
      "/api": { target: "http://127.0.0.1:8080", ws: true },
      "/healthz": "http://127.0.0.1:8080",
    },
  },
  build: { target: "es2022" },
});
