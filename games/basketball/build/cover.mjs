import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import {
  existsSync,
  lstatSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  readdirSync,
  realpathSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
const filename = fileURLToPath(import.meta.url);
const hash = (b) => createHash("sha256").update(b).digest("hex");
const requireValue = (value, message) => {
  if (!value) throw Error(message);
};
function inside(root, relative) {
  const file = path.resolve(root, relative),
    rel = path.relative(root, file);
  requireValue(
    rel && !rel.startsWith("../") && !path.isAbsolute(rel),
    "Cover path must stay inside checkout",
  );
  let part = root;
  for (const bit of rel.split(path.sep)) {
    part = path.join(part, bit);
    const st = lstatSync(part, { throwIfNoEntry: false });
    requireValue(!st?.isSymbolicLink(), "Cover path contains a symlink");
  }
  return file;
}
export function loadCoverPin(directory) {
  const root = realpathSync(directory),
    pin = JSON.parse(
      readFileSync(
        inside(root, "games/basketball/assets/cover-manifest.json"),
        "utf8",
      ),
    );
  requireValue(
    pin.schema === 1 &&
      pin.game === "basketball" &&
      /^[a-f0-9]{64}$/.test(pin.sha256),
    "Invalid cover identity",
  );
  requireValue(
    pin.localPath === "/basketball/v1/cover/heroes-" + pin.sha256 + ".png" &&
      pin.url ===
        "https://cdn.ichenj.com/bga/basketball/cover-" +
          pin.sha256 +
          "/heroes.png",
    "Invalid cover destination",
  );
  requireValue(
    pin.mime === "image/png" &&
      Number.isSafeInteger(pin.bytes) &&
      pin.bytes >= 33 &&
      pin.bytes <= 2 * 1024 * 1024,
    "Invalid cover length/type",
  );
  requireValue(
    [pin.width, pin.height].every(
      (n) => Number.isInteger(n) && n > 0 && n <= 4096,
    ),
    "Invalid cover dimensions",
  );
  return pin;
}
function context(directory) {
  const root = realpathSync(directory),
    pin = loadCoverPin(root),
    parent = inside(root, ".local/basketball-cover-cache"),
    cache = inside(root, path.join(parent, pin.sha256));
  return { root, pin, parent, cache, publicRoot: path.join(cache, "public") };
}
function verifyBytes(bytes, pin) {
  requireValue(
    bytes.length === pin.bytes && hash(bytes) === pin.sha256,
    "Cover checksum/length differs",
  );
  requireValue(
    bytes.subarray(0, 8).equals(Buffer.from("89504e470d0a1a0a", "hex")) &&
      bytes.toString("ascii", 12, 16) === "IHDR",
    "Cover is not PNG",
  );
  requireValue(
    bytes.readUInt32BE(16) === pin.width &&
      bytes.readUInt32BE(20) === pin.height,
    "Cover dimensions differ",
  );
}
function inventory(directory, prefix = "") {
  const files = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const name = prefix + entry.name;
    if (entry.isDirectory())
      files.push(...inventory(path.join(directory, entry.name), name + "/"));
    else {
      requireValue(entry.isFile(), "Unexpected cover cache entry");
      files.push(name);
    }
  }
  return files.sort();
}
export function verifyCover(directory, publicDirectory) {
  const c = context(directory),
    base = publicDirectory ? inside(c.root, publicDirectory) : c.publicRoot;
  verifyBytes(
    readFileSync(inside(c.root, path.join(base, c.pin.localPath.slice(1)))),
    c.pin,
  );
  if (base === c.publicRoot)
    requireValue(
      JSON.stringify(inventory(base)) ===
        JSON.stringify([c.pin.localPath.slice(1)]),
      "Unlisted file in cover cache",
    );
  return base;
}
function valid(c) {
  try {
    return verifyCover(c.root);
  } catch {
    return false;
  }
}
export async function materializeCover(
  directory,
  { fetchImpl = fetch, localFile } = {},
) {
  const c = context(directory);
  if (valid(c)) return c.publicRoot;
  mkdirSync(c.parent, { recursive: true });
  const stage = mkdtempSync(path.join(c.parent, "prepare-"));
  try {
    let bytes;
    if (localFile) bytes = readFileSync(inside(c.root, localFile));
    else {
      const response = await fetchImpl(c.pin.url, {
        redirect: "error",
        signal: AbortSignal.timeout(30000),
      });
      requireValue(
        response.ok && response.status === 200 && response.body,
        "Cover download failed",
      );
      requireValue(
        (response.headers.get("content-type") ?? "")
          .split(";")[0]
          .trim()
          .toLowerCase() === c.pin.mime,
        "Cover MIME differs",
      );
      const chunks = [],
        reader = response.body.getReader();
      let count = 0;
      try {
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          count += value.byteLength;
          requireValue(count <= c.pin.bytes, "Cover exceeds pinned length");
          chunks.push(Buffer.from(value));
        }
      } finally {
        await reader.cancel().catch(() => {});
      }
      bytes = Buffer.concat(chunks);
    }
    verifyBytes(bytes, c.pin);
    const target = inside(
      c.root,
      path.join(stage, "public", c.pin.localPath.slice(1)),
    );
    mkdirSync(path.dirname(target), { recursive: true });
    writeFileSync(target, bytes, { flag: "wx" });
    if (valid(c)) return c.publicRoot;
    inside(c.root, c.cache);
    if (existsSync(c.cache)) rmSync(c.cache, { recursive: true, force: true });
    renameSync(stage, c.cache);
    return verifyCover(c.root);
  } finally {
    rmSync(stage, { recursive: true, force: true });
  }
}
export function prepareBasketballCover(directory) {
  const c = context(directory);
  if (valid(c)) return c.publicRoot;
  execFileSync(process.execPath, [filename, "--prepare", c.root], {
    stdio: ["ignore", "pipe", "pipe"],
    maxBuffer: 1 << 20,
  });
  return verifyCover(c.root);
}
if (process.argv[1] && path.resolve(process.argv[1]) === filename) {
  try {
    requireValue(
      process.argv[2] === "--prepare" && process.argv[3],
      "Use --prepare CHECKOUT",
    );
    console.log(
      await materializeCover(process.argv[3], {
        localFile: process.env.BASKETBALL_COVER_FILE,
      }),
    );
  } catch (error) {
    console.error("Basketball cover preparation failed:", error.message);
    process.exitCode = 1;
  }
}
