import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { lstat, mkdir, readFile, readdir, realpath, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const workspace = fileURLToPath(new URL("../../../", import.meta.url));
const generatorPath = fileURLToPath(import.meta.url);
const revisionName = "games/pokepath/engine/rules_revision.go";
const revisionManifest = "games/pokepath/data/rules-revision.json";
const publicName = "rules-revision.json";
const digest = (bytes) => createHash("sha256").update(bytes).digest("hex");
const environment = { ...process.env, TMPDIR: path.join(workspace, ".local/tmp") };

async function baseRoot(value) {
  const candidate = path.resolve(value ?? workspace);
  const local = path.relative(workspace, candidate);
  if (local === ".." || local.startsWith(`..${path.sep}`) || path.isAbsolute(local)) throw Error("Rules revision root is outside workspace");
  const resolved = await realpath(candidate);
  const realLocal = path.relative(workspace, resolved);
  if (realLocal === ".." || realLocal.startsWith(`..${path.sep}`) || path.isAbsolute(realLocal)) throw Error("Rules revision root is outside workspace through symlink");
  return resolved;
}

async function filesWithin(directory) {
  const output = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (entry.isSymbolicLink()) throw Error(`Symbolic link is not a rules input: ${entry.name}`);
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) output.push(...await filesWithin(filename));
    else if (entry.isFile()) output.push(filename);
  }
  return output.sort();
}

async function engineInputs(root) {
  const engine = path.join(root, "games/pokepath/engine");
  const files = [];
  for (const filename of await filesWithin(engine)) {
    const local = path.relative(root, filename).split(path.sep).join("/");
    if (!filename.endsWith(".go") || filename.endsWith("_test.go") || local === revisionName) continue;
    const formatted = execFileSync("gofmt", [], { env: environment, input: await readFile(filename), maxBuffer: 32 << 20, stdio: ["pipe", "pipe", "pipe"] });
    files.push({ path: local, bytes: formatted.length, sha256: digest(formatted) });
  }
  if (!files.length) throw Error("No product Go rules files were found");
  return files;
}

async function changed(filename, expected) {
  try {
    return !(await readFile(filename)).equals(expected);
  } catch (error) {
    if (error.code === "ENOENT") return true;
    throw error;
  }
}

async function outputWithin(root, filename) {
	let current = filename;
	for (;;) {
		try {
			if ((await lstat(current)).isSymbolicLink()) throw Error(`Rules revision output is a symlink: ${filename}`);
			const resolved = await realpath(current);
			if (resolved !== root && !resolved.startsWith(root + path.sep)) throw Error(`Rules revision output is outside workspace: ${filename}`);
			return;
		} catch (error) {
			if (error.code !== "ENOENT") throw error;
			const parent = path.dirname(current);
			if (parent === current) throw Error(`Rules revision output is outside workspace: ${filename}`);
			current = parent;
		}
	}
}

export async function generateRulesRevision(options = {}) {
  const root = await baseRoot(options.root);
  const publicRoot = path.join(root, "games/pokepath/assets/public/pokepath/v1");
  const inputs = await engineInputs(root);
  const engine = { format: 1, normalization: "gofmt-go-v1", files: inputs };
  const rulesRevision = digest(JSON.stringify(engine));
  const catalogBytes = await readFile(path.join(root, "games/pokepath/data/catalog.json"));
  const catalog = JSON.parse(catalogBytes);
  if (!/^[a-f0-9]{64}$/.test(catalog.source?.sourceSha256)) throw Error("Rules facts identity is missing");
  const clientFiles = [path.join(publicRoot, "catalog.json"), ...(await filesWithin(path.join(publicRoot, "routes"))).filter((filename) => /^\d+\.json$/.test(path.basename(filename)))];
  const clientDataFiles = [];
  for (const filename of clientFiles.sort()) {
    const bytes = await readFile(filename);
    clientDataFiles.push({ path: path.relative(publicRoot, filename).split(path.sep).join("/"), bytes: bytes.length, sha256: digest(bytes) });
  }
  const revisionCode = Buffer.from(`package engine\n\nconst NativeRulesRevision = "${rulesRevision}"\n\nfunc RulesVersion() string {\n\treturn "pokepath-native-1/" + NativeRulesRevision + "/" + CatalogData().Source.SourceSHA256\n}\n`);
  const manifest = { format: 1, version: catalog.version, rulesRevision, rulesVersion: `pokepath-native-1/${rulesRevision}/${catalog.source.sourceSha256}`, factsSHA256: catalog.source.sourceSha256, engine, catalog: { path: "games/pokepath/data/catalog.json", bytes: catalogBytes.length, sha256: digest(catalogBytes) }, clientData: { sha256: digest(JSON.stringify(clientDataFiles)), files: clientDataFiles }, generatorSHA256: digest(await readFile(generatorPath)), revisionFileSHA256: digest(revisionCode) };
  const manifestBytes = Buffer.from(`${JSON.stringify(manifest, null, 2)}\n`);
  const resourcePath = path.join(publicRoot, "resources.json");
  const resources = JSON.parse(await readFile(resourcePath, "utf8"));
  const clientFacts = new Map(clientDataFiles.map((file) => [file.path, file]));
  resources.files = resources.files.filter((file) => file.path !== publicName).map((file) => clientFacts.has(file.path) ? { ...file, ...clientFacts.get(file.path) } : file);
  resources.files.push({ path: publicName, bytes: manifestBytes.length, sha256: digest(manifestBytes), mime: "application/json" });
  resources.files.sort((left, right) => left.path.localeCompare(right.path));
  const outputs = [[path.join(root, revisionName), revisionCode], [path.join(root, revisionManifest), manifestBytes], [path.join(publicRoot, publicName), manifestBytes], [resourcePath, Buffer.from(`${JSON.stringify(resources)}\n`)]];
  const differences = [];
  for (const [filename, bytes] of outputs) {
    await outputWithin(root, filename);
    if (await changed(filename, bytes)) differences.push([filename, bytes]);
  }
  if (JSON.stringify(await engineInputs(root)) !== JSON.stringify(inputs)) throw Error("Rules source changed during revision generation; retry after writes finish");
  if (options.publicOnly) {
    const tracked = new Set([path.join(root, revisionName), path.join(root, revisionManifest)]);
    const stale = differences.filter(([filename]) => tracked.has(filename));
    if (stale.length) throw Error(`Stale tracked rules identity: ${stale.map(([filename]) => path.relative(root, filename)).join(", ")}`);
  }
  if (options.check && differences.length) throw Error(`Stale generated rules revision: ${differences.map(([filename]) => path.relative(root, filename)).join(", ")}`);
  if (!options.check) for (const [filename, bytes] of differences) {
    await mkdir(path.dirname(filename), { recursive: true });
    await writeFile(filename, bytes);
  }
  return manifest;
}

if (process.argv[1] && path.resolve(process.argv[1]) === generatorPath) generateRulesRevision({ check: process.argv.includes("--check"), publicOnly: process.argv.includes("--public-only") }).then((manifest) => process.stdout.write(`${JSON.stringify({ checked: process.argv.includes("--check"), rulesRevision: manifest.rulesRevision, factsSHA256: manifest.factsSHA256, clientDataSHA256: manifest.clientData.sha256, engineFiles: manifest.engine.files.length, clientDataFiles: manifest.clientData.files.length })}\n`)).catch((error) => { process.stderr.write(`${error.stack}\n`); process.exitCode = 1; });
