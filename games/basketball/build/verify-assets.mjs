import { fileURLToPath } from "node:url";
import path from "node:path";
import { verifyBasketballPublicFiles } from "./assets.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const destination = path.resolve(root, process.argv[2] ?? "web/dist");
const pin = verifyBasketballPublicFiles(root, destination, { exact: false });
console.log(JSON.stringify({ status: "verified", objects: pin.entries.length, contentSHA256: pin.contentSHA256, localFallback: true }));
