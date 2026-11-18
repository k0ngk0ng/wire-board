import { readFile, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { fileURLToPath, pathToFileURL } from "node:url";
import { resolve } from "node:path";
import { parseLiteralDeclarations } from "./extract-reference.mjs";

export async function extractUIText() {
  const source = await readFile(new URL("../../../.local/pokepath/research/source/src/js/file/text.js", import.meta.url), "utf8");
  const text = parseLiteralDeclarations(source, "text.js").get("text");
  const paths = ["src/js/game/UI.js", "src/js/game/scenes/MenuScene.js", "src/js/game/scenes/DexScene.js"];
  const sources = await Promise.all(paths.map(path => readFile(new URL(`../../../.local/pokepath/research/source/${path}`, import.meta.url), "utf8")));
  const ui = parseLiteralDeclarations(sources[0], "UI.js");
  const menu = parseLiteralDeclarations(sources[1], "MenuScene.js");
  const dex = parseLiteralDeclarations(sources[2], "DexScene.js");
  text.interfaceOptions = { targetModes: ui.get("TARGET_MODES_TRADUCTIONS"), effectOpacityLabels: menu.get("effectOpacityLabels"), muteLabels: menu.get("MUTE_LABELS"), shortcuts: menu.get("SHORTCUTS"), unownDialogue: dex.get("UNOWN_DIALOGUE") };
  const files = [{ path: "src/js/file/text.js", bytes: Buffer.byteLength(source), sha256: createHash("sha256").update(source).digest("hex") }, ...sources.map((value, index) => ({ path: paths[index], bytes: Buffer.byteLength(value), sha256: createHash("sha256").update(value).digest("hex") }))];
  const target = new URL("../assets/public/pokepath/v1/ui-text.json", import.meta.url);
  const bytes = Buffer.from(JSON.stringify({ format: "pokepath-ui-text-1", source: { url: "https://pokepath-game.pages.dev/src/js/file/text.js", version: "1.7.3", sha256: files[0].sha256, files }, text }) + "\n");
  await writeFile(target, bytes);
  return { path: "ui-text.json", bytes: bytes.length, sha256: createHash("sha256").update(bytes).digest("hex"), mime: "application/json", file: fileURLToPath(target) };
}

export async function registerUIText() {
  const result = await extractUIText();
  const { file, ...record } = result;
  const target = new URL("../assets/public/pokepath/v1/resources.json", import.meta.url);
  const resources = JSON.parse(await readFile(target, "utf8"));
  resources.files = [...resources.files.filter(value => value.path !== record.path), record].sort((first, second) => first.path.localeCompare(second.path));
  await writeFile(target, JSON.stringify(resources) + "\n");
  return { ...record, file };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) process.stdout.write(JSON.stringify(await registerUIText()) + "\n");
