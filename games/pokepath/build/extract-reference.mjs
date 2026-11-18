import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { mediaResourceFacts } from "./media-resources.mjs";
import { generateRulesRevision } from "./rules-revision.mjs";

const require = createRequire(new URL("../../../web/package.json", import.meta.url));
const ts = require("typescript");
const root = fileURLToPath(new URL("../../../", import.meta.url));
const sourceRoot = path.join(root, ".local/pokepath/research/source/src/js/game/data");
const gameRoot = path.join(root, "games/pokepath");
const sourceURL = "https://pokepath-game.pages.dev/";
const sha256 = (value) => createHash("sha256").update(value).digest("hex");
const forbidden = new Set(["__proto__", "constructor", "prototype"]);
const counts = { species: 517, enemies: 347, routes: 47, items: 197, abilities: 254, passives: 85, achievements: 39, weather: 11, projectiles: 62 };

export function parseLiteralDeclarations(source, filename, resolveImport) {
  const ast = ts.createSourceFile(filename, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
  if (ast.parseDiagnostics.length) throw Error(`Invalid reference data syntax: ${filename}`);
  const declarations = new Map();
  const imports = new Map();
  const values = new Map();
  const resolving = new Set();
  for (const statement of ast.statements) {
    if (ts.isImportDeclaration(statement) && statement.importClause?.namedBindings && ts.isNamedImports(statement.importClause.namedBindings)) {
      for (const binding of statement.importClause.namedBindings.elements) imports.set(binding.name.text, { source: statement.moduleSpecifier.text, name: binding.propertyName?.text ?? binding.name.text });
    }
    if (ts.isVariableStatement(statement)) for (const declaration of statement.declarationList.declarations) {
      if (ts.isIdentifier(declaration.name) && declaration.initializer) declarations.set(declaration.name.text, declaration.initializer);
    }
  }
  function fail(node) {
    throw Error(`Unsupported nonliteral reference data ${ts.SyntaxKind[node.kind]} in ${filename}`);
  }
  function key(node) {
    if (ts.isIdentifier(node) || ts.isStringLiteral(node) || ts.isNumericLiteral(node)) return node.text;
    if (ts.isComputedPropertyName(node)) return String(evaluate(node.expression));
    return fail(node);
  }
  function evaluate(node) {
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) return node.text;
    if (ts.isNumericLiteral(node)) {
      const value = Number(node.text);
      if (!Number.isFinite(value)) throw Error(`Nonfinite reference number in ${filename}`);
      return value;
    }
    if (node.kind === ts.SyntaxKind.TrueKeyword) return true;
    if (node.kind === ts.SyntaxKind.FalseKeyword) return false;
    if (node.kind === ts.SyntaxKind.NullKeyword || ts.isOmittedExpression(node)) return null;
    if (ts.isParenthesizedExpression(node)) return evaluate(node.expression);
    if (ts.isIdentifier(node)) return get(node.text);
    if (ts.isPrefixUnaryExpression(node) && [ts.SyntaxKind.MinusToken, ts.SyntaxKind.PlusToken].includes(node.operator)) {
      const value = evaluate(node.operand);
      if (typeof value !== "number") return fail(node);
      return node.operator === ts.SyntaxKind.MinusToken ? -value : value;
    }
    if (ts.isArrayLiteralExpression(node)) {
      const output = [];
      for (const element of node.elements) {
        if (ts.isSpreadElement(element)) {
          const spread = evaluate(element.expression);
          if (!Array.isArray(spread)) return fail(element);
          output.push(...spread);
        } else output.push(evaluate(element));
      }
      return output;
    }
    if (ts.isObjectLiteralExpression(node)) {
      const output = {};
      for (const property of node.properties) {
        if (!ts.isPropertyAssignment(property)) return fail(property);
        const name = key(property.name);
        if (forbidden.has(name)) throw Error(`Unsafe reference field in ${filename}`);
        output[name] = evaluate(property.initializer);
      }
      return output;
    }
    if (ts.isElementAccessExpression(node) || ts.isPropertyAccessExpression(node)) {
      const target = evaluate(node.expression);
      const name = ts.isPropertyAccessExpression(node) ? node.name.text : String(evaluate(node.argumentExpression));
      if (!target || typeof target !== "object" || forbidden.has(name) || !Object.hasOwn(target, name)) throw Error(`Unknown static table reference ${name} in ${filename}`);
      return target[name];
    }
    if (ts.isNewExpression(node) && ts.isIdentifier(node.expression) && node.expression.text === "Set" && node.arguments?.length === 1) {
      const value = evaluate(node.arguments[0]);
      if (!Array.isArray(value)) return fail(node);
      return [...new Set(value)];
    }
    return fail(node);
  }
  function get(name) {
    if (name === "undefined") return undefined;
    if (values.has(name)) return values.get(name);
    if (resolving.has(name)) throw Error(`Cyclic static table reference ${name} in ${filename}`);
    resolving.add(name);
    try {
      let value;
      if (declarations.has(name)) value = evaluate(declarations.get(name));
      else if (imports.has(name) && resolveImport) {
        const imported = imports.get(name);
        value = resolveImport(imported.source, imported.name);
      } else throw Error(`Unknown static reference ${name} in ${filename}`);
      values.set(name, value);
      return value;
    } finally {
      resolving.delete(name);
    }
  }
  return { get };
}

function textRecord(key, raw) {
  const names = Array.isArray(raw.name) ? raw.name : raw.name ? [String(raw.name)] : [];
  const descriptions = Array.isArray(raw.description) ? raw.description : raw.description ? [String(raw.description)] : [];
  return { key, id: raw.id ?? key, name: names[8] ?? names[0] ?? key, nameEn: names[0] ?? key, names, description: descriptions[8] ?? descriptions[0] ?? "", descriptionEn: descriptions[0] ?? "", descriptions };
}

function assetPath(value) {
  if (!value) return "";
  const normalized = value.replace(/^\.\//, "");
  if (!normalized.startsWith("src/assets/") || normalized.includes("..") || normalized.includes("\\")) throw Error(`Invalid reference asset path: ${value}`);
  return normalized;
}

export function normalizeTutorialFacts(raw) {
  if (!Array.isArray(raw)) throw Error("Invalid tutorial fact table");
  const result = {};
  for (const topic of raw) {
    if (!topic || !/^[a-z][A-Za-z0-9]*$/.test(topic.key) || Object.hasOwn(result, topic.key) || !Array.isArray(topic.title) || !Array.isArray(topic.text) || topic.title.some(value => typeof value !== "string") || topic.text.some(value => typeof value !== "string")) throw Error("Invalid or duplicate tutorial fact");
    result[topic.key] = { key: topic.key, name: topic.title[8] || topic.title[0] || topic.key, names: topic.title, description: topic.text[8] || topic.text[0] || "", descriptions: topic.text, sprite: assetPath(topic.image) };
  }
  return result;
}

export function parseThemeColors(source) {
  const names = ["original", "classic", "halloween", "monochrome"];
  const tables = { root: {} };
  const clean = source.replace(/\/\*[\s\S]*?\*\//g, "");
  const color = /^(?:#[a-fA-F0-9]{3,8}|(?:rgb|hsl)a?\([^)]*\)|transparent|var\(--[A-Za-z0-9-]+\))$/;
  for (const block of clean.matchAll(/(:root|\[data-theme="(?:original|classic|halloween|monochrome)"\])\s*\{([^{}]*)\}/g)) {
    const name = block[1] === ":root" ? "root" : block[1].match(/"([a-z]+)"/)[1];
    tables[name] ??= {};
    for (const declaration of block[2].matchAll(/--([A-Za-z0-9-]+)\s*:\s*([^;{}]+);/g)) if (color.test(declaration[2].trim())) tables[name][declaration[1]] = declaration[2].trim();
  }
  const result = {};
  for (const name of names) {
    if (!tables[name]) throw Error(`Missing original theme palette: ${name}`);
    const variables = { ...tables.root, ...tables[name] };
    const resolved = {};
    const resolve = (key, trail = new Set()) => {
      if (resolved[key]) return resolved[key];
      if (trail.has(key) || !variables[key]) throw Error(`Invalid theme color reference: ${name}.${key}`);
      trail.add(key);
      const value = variables[key].replace(/var\(--([A-Za-z0-9-]+)\)/g, (_, dependency) => resolve(dependency, new Set(trail)));
      if (!color.test(value) || value.includes("var(")) throw Error(`Invalid resolved theme color: ${name}.${key}`);
      resolved[key] = value;
      return value;
    };
    for (const key of Object.keys(variables)) resolve(key);
    result[name] = resolved;
  }
  return result;
}

export function validateCatalog(catalog) {
  for (const [key, count] of Object.entries(counts)) if (!catalog[key] || Object.keys(catalog[key]).length !== count) throw Error(`Incomplete ${key} reference table`);
  if (catalog.collection?.length !== 257 || new Set(catalog.collection.map((key) => catalog.species[key]?.id)).size !== 257) throw Error("Incomplete collection identifiers");
  const known = (table, key, label) => { if (key && !Object.hasOwn(table, key)) throw Error(`Unknown ${label} reference: ${key}`); };
  if (Object.keys(catalog.songs ?? {}).length !== 41 || catalog.playlist?.length !== 41 || new Set(catalog.playlist).size !== 41) throw Error("Incomplete music playlist facts");
  for (const [order, key] of catalog.playlist.entries()) {
    const song = catalog.songs[key];
    if (!song || song.key !== key || song.id !== order || song.names?.length !== 10 || !/^[a-z][a-z0-9]*$/.test(key) || song.audio !== `src/assets/sounds/music/${key}.mp3`) throw Error(`Invalid song facts: ${key}`);
  }
  let waveCount = 0;
  for (const [key, pokemon] of Object.entries(catalog.species)) {
    known(catalog.abilities, pokemon.abilityId, "ability");
    known(catalog.projectiles, pokemon.projectileId, "projectile");
    known(catalog.species, pokemon.evolutionKey, "evolution");
    if (pokemon.key !== key || !Array.isArray(pokemon.terrains) || !pokemon.terrains.length) throw Error(`Invalid species facts ${key}`);
  }
  for (const enemy of Object.values(catalog.enemies)) {
    known(catalog.passives, enemy.passiveId, "passive");
    if (!Number.isFinite(enemy.health) || enemy.health <= 0 || !Number.isFinite(enemy.speed) || enemy.speed < 0) throw Error(`Invalid enemy facts ${enemy.key}`);
  }
  for (const route of Object.values(catalog.routes)) {
    known(catalog.weather, route.weatherId, "weather");
    known(catalog.songs, route.musicKey, "music");
    if (catalog.songs[route.musicKey]?.id !== route.musicId) throw Error(`Invalid route music reference: ${route.id}`);
    if (!route.paths?.length || !route.placements?.length || !Number.isFinite(route.width) || !Number.isFinite(route.height)) throw Error(`Invalid route facts ${route.id}`);
    if (route.placementGrid.length !== route.gridColumns * route.gridRows) throw Error(`Invalid route grid ${route.id}`);
    if (new Set(route.placements.map((tile) => tile.id)).size !== route.placements.length) throw Error(`Duplicate placement ID ${route.id}`);
    const chessSpecies = { h: "kingdra", q: "vespiquen", b: "bisharp", k: "kingambit", p: "pawniard", t: "stakataka" };
    const pieces = route.chessPlacements ?? [];
    if (pieces.length !== (route.id === 35 ? 16 : 0) || new Set(pieces.map((piece) => piece.key)).size !== pieces.length) throw Error(`Incomplete or duplicate chess facts ${route.id}`);
    for (const piece of pieces) {
      known(catalog.species, piece.species, "chess species");
      if (chessSpecies[piece.key?.[0]] !== piece.species || !Number.isFinite(piece.x) || !Number.isFinite(piece.y) || piece.x < 0 || piece.y < 0 || piece.x >= route.width || piece.y >= route.height || piece.x % 24 || piece.y % 24) throw Error(`Invalid chess placement ${piece.key}`);
    }
    for (const wave of Object.values(route.waves)) {
      waveCount++;
      if (!Array.isArray(wave.sequence) || !wave.sequence.some(Boolean)) throw Error(`Empty wave on route ${route.id}`);
      for (const enemy of [...wave.preview, ...wave.sequence]) {
        if (enemy !== null && typeof enemy !== "string") throw Error("Wave sequence must contain enemy IDs or null");
        known(catalog.enemies, enemy, "enemy");
      }
    }
  }
  if (waveCount !== 4007) throw Error(`Incomplete waves reference table: ${waveCount}`);
  for (const pool of [catalog.collection, catalog.eggPool, catalog.forcedInitialPool, catalog.forcedSecondaryPool]) for (const key of pool ?? []) known(catalog.species, key, "pool species");
  for (const pool of Object.values(catalog.itemPools ?? {})) for (const key of pool) known(catalog.items, key, "pool item");
  for (const [scale, prices] of Object.entries(catalog.upgradeCosts ?? {})) {
    if (!Array.isArray(prices) || prices.length !== 101 || prices[0] !== 0 || prices.some((price) => !Number.isSafeInteger(price) || price < 0)) throw Error(`Incomplete upgrade price facts: ${scale}`);
  }
  for (const pokemon of Object.values(catalog.species)) if (!catalog.upgradeCosts?.[pokemon.costScale]) throw Error(`Unknown upgrade price scale: ${pokemon.costScale}`);
  const inspect = (value) => {
    if (typeof value === "function" || typeof value === "number" && !Number.isFinite(value)) throw Error("Nonfact value in normalized catalog");
    if (value && typeof value === "object") for (const entry of Object.values(value)) inspect(entry);
  };
  inspect(catalog);
  return true;
}

export async function extractReference() {
  const sourceFiles = new Map();
  const modules = new Map();
  async function load(filename) {
    if (modules.has(filename)) return modules.get(filename);
    const source = await readFile(path.join(sourceRoot, filename), "utf8");
    sourceFiles.set(filename, { path: `src/js/game/data/${filename}`, bytes: Buffer.byteLength(source), sha256: sha256(source) });
    const record = { source };
    modules.set(filename, record);
    const ast = ts.createSourceFile(filename, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.JS);
    for (const statement of ast.statements) if (ts.isImportDeclaration(statement)) {
      const specifier = statement.moduleSpecifier.text;
      if (!specifier.startsWith("./") || !/^[.\/a-zA-Z0-9-]+\.js$/.test(specifier)) throw Error(`Reference import leaves the data directory: ${specifier}`);
      const target = path.posix.normalize(path.posix.join(path.posix.dirname(filename), specifier));
      if (target.startsWith("..")) throw Error("Reference import escapes the data directory");
      await load(target);
    }
    record.parser = parseLiteralDeclarations(source, filename, (specifier, name) => {
      const target = path.posix.normalize(path.posix.join(path.posix.dirname(filename), specifier));
      const imported = modules.get(target);
      if (!imported?.parser) throw Error(`Unavailable static import ${target}`);
      if (target === "songData.js" && name === "songData") {
        const songs = imported.parser.get("songs");
        const rows = [];
        for (const song of Object.values(songs)) rows[song.order] = { id: song.order, song: song.song, name: song.name };
        return rows;
      }
      return imported.parser.get(name);
    });
    return record;
  }
  const declarations = { species: ["pokemonData.js", "pokemonData"], enemies: ["enemyData.js", "enemyData"], routes: ["routeData.js", "routeData"], items: ["itemData.js", "itemData"], abilities: ["abilityData.js", "abilityData"], passives: ["passiveData.js", "passiveData"], achievements: ["achievementReworkData.js", "achievementReworkData"], weather: ["weatherData.js", "weatherData"], projectiles: ["projectileData.js", "projectileData"] };
  const raw = {};
  for (const [key, [filename, name]] of Object.entries(declarations)) raw[key] = (await load(filename)).parser.get(name);
  const mediaManifest = JSON.parse(await readFile(path.join(gameRoot, "assets/manifest.json"), "utf8"));
  for (const file of mediaManifest.files) {
    if (!/^(?!.*(?:^|\/)\.\.?(?:\/|$))[A-Za-z0-9][A-Za-z0-9._/-]*$/.test(file.path)) throw Error(`Noncanonical public asset path: ${file.path}`);
  }
  const media = new Map(mediaManifest.files.map((file) => [file.path, file]));
  const identities = new Map();
  for (const [table, records] of Object.entries(raw)) for (const [key, record] of Object.entries(records)) identities.set(record, `${table}/${key}`);
  const fact = (value, seen = new Set()) => {
    if (value === undefined) return undefined;
    if (value === null || typeof value === "boolean" || typeof value === "number") return value;
    if (typeof value === "string") return value.startsWith("./src/assets/") ? assetPath(value) : value;
    if (identities.has(value)) return identities.get(value);
    if (typeof value !== "object" || seen.has(value)) throw Error("Nonliteral or cyclic reference facts");
    const next = new Set(seen).add(value);
    if (Array.isArray(value)) return value.map((entry) => fact(entry, next));
    return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, fact(entry, next)]));
  };
  const extra = (record, excluded) => fact(Object.fromEntries(Object.entries(record).filter(([key]) => !excluded.includes(key))));
  const image = (value, rows = 8) => {
    if (!value?.image) return {};
    const filename = assetPath(value.image);
    const file = media.get(filename);
    if (!file?.width || !file?.height) throw Error(`Missing precise sprite dimensions: ${filename}`);
    const frames = value.frames ?? 1;
    if (!Number.isInteger(frames) || frames <= 0) throw Error(`Invalid sprite grid ${filename}: ${file.width}x${file.height}, ${frames}x${rows}`);
    return { base: assetPath(value.base), image: filename, frames, rows, hold: value.hold ?? 5, width: file.width, height: file.height, frameWidth: file.width / frames, frameHeight: file.height / rows, fractionalFrames: Boolean(file.width % frames || file.height % rows), facts: extra(value, ["base", "image", "frames", "hold"]) };
  };
  const catalog = { version: "1.7.3", source: { url: sourceURL, version: "1.7.3", method: "static-literal-facts-only" }, species: {}, enemies: {}, routes: {}, songs: {}, playlist: [], items: {}, abilities: {}, passives: {}, achievements: [], weather: {}, projectiles: {} };
  const songs = modules.get("songData.js").parser.get("songs");
  for (const value of Object.values(songs)) {
    const key = value.song;
    const audio = assetPath(`src/assets/sounds/music/${key}.mp3`);
    if (!media.has(audio) || catalog.songs[key] || !Number.isInteger(value.order) || value.order < 0 || value.order >= 41 || catalog.playlist[value.order]) throw Error(`Invalid song source facts: ${key}`);
    catalog.songs[key] = { id: value.order, key, name: value.name[8] ?? value.name[0], nameEn: value.name[0], names: value.name, audio };
    catalog.playlist[value.order] = key;
  }
  for (const [key, value] of Object.entries(raw.species)) {
    const projectileId = identities.get(value.projectile)?.split("/")[1] ?? "";
    catalog.species[key] = { ...textRecord(key, value), id: value.id, color: value.color, powerBase: value.power.base, powerPerLevel: value.power.scale, attackMsBase: value.speed.base, attackMsPerLevel: value.speed.scale, rangeBase: value.range.base, rangePerLevel: value.range.scale, innerRange: value.range.inner ?? 0, criticalBase: value.critical.base, criticalPerLevel: value.critical.scale, abilityId: value.ability.id, terrains: value.tiles, evolutionKey: value.evolution?.pokemon ?? "", evolutionLevel: value.evolution?.level ?? 0, projectileId, projectileSpeed: value.projectileSpeed, attackType: value.attackType, rangeType: value.rangeType, costScale: value.costScale, sprite: image(value.sprite), facts: extra(value, ["id", "key", "name", "color", "power", "speed", "range", "critical", "ability", "tiles", "evolution", "projectile", "projectileSpeed", "attackType", "rangeType", "costScale", "sprite"]) };
  }
  for (const [key, value] of Object.entries(raw.enemies)) catalog.enemies[key] = { ...textRecord(key, value), id: key, health: value.hp, armor: value.armor ?? 0, speed: value.speed * 60, speedPerFrame: value.speed, power: value.power, regeneration: value.regeneration ?? 0, canStun: value.canStun, canSlow: value.canSlow, canBurn: value.canBurn, canPoison: value.canPoison, invisible: value.invisible, passiveId: value.passive?.id ?? "", isBoss: value.isBoss, isElite: value.isElite, gold: value.gold, sprite: image(value.sprite), facts: {} };
  for (const [key, value] of Object.entries(raw.routes)) {
    const width = value.xlh ? 2160 : value.xl ? 1440 : 720;
    const height = value.xlv ? 1872 : 624;
    const columns = width / 24;
    const placements = [];
    value.placementTile.forEach((terrain, index) => { if (terrain) placements.push({ id: placements.length, x: index % columns * 24, y: Math.floor(index / columns) * 24, terrain }); });
    const chessPlacements = [];
    const chessSpecies = { h: "kingdra", q: "vespiquen", b: "bisharp", k: "kingambit", p: "pawniard", t: "stakataka" };
    (value.chessTiles ?? []).forEach((piece, index) => { if (piece) chessPlacements.push({ key: piece, species: chessSpecies[piece[0]], x: index % columns * 24, y: Math.floor(index / columns) * 24 }); });
    const waves = {};
    for (const [number, wave] of Object.entries(value.waves)) waves[number] = { preview: wave.preview.map((enemy) => enemy.id), sequence: wave.wave.map((enemy) => enemy === null ? null : enemy.id), facts: extra(wave, ["preview", "wave"]) };
    catalog.routes[key] = { ...textRecord(key, value), id: value.id, background: assetPath(value.background), effect: assetPath(value.effect), width, height, tileSize: 24, gridColumns: columns, gridRows: height / 24, placementGrid: value.placementTile, placements, ...(chessPlacements.length ? { chessPlacements } : {}), paths: value.waypoints, mirrorPaths: value.waypointsMirror, offset: value.offset ?? { x: 0, y: 0 }, mirrorOffset: value.offsetMirror ?? { x: 0, y: 0 }, unlockStars: value.unlock ?? 0, isSecret: value.isSecret ?? false, secretKey: value.secretKey ?? "", weatherId: value.weather ?? "", challengeRewards: value.challengeReward ?? [], musicId: value.music?.id ?? 0, musicKey: value.music?.song ?? "", waves, facts: extra(value, ["id", "name", "background", "effect", "placementTile", "waypoints", "waypointsMirror", "offset", "offsetMirror", "unlock", "isSecret", "secretKey", "weather", "challengeReward", "music", "waves"]) };
  }
  for (const [key, value] of Object.entries(raw.items)) catalog.items[key] = { ...textRecord(key, value), id: key, price: value.price ?? 0, tier: value.tier ?? "", equipable: value.isEquipable ?? false, retirable: value.isRetirable ?? false, sprite: assetPath(value.sprite), tags: value.tags ?? [], condition: fact(value.condition ?? {}), restriction: fact(value.restriction ?? {}), megaStone: value.megaStone ?? false, megaPos: value.megaPos ?? 0, facts: extra(value, ["id", "name", "description", "price", "tier", "isEquipable", "isRetirable", "sprite", "tags", "condition", "restriction", "megaStone", "megaPos"]) };
  for (const table of ["abilities", "passives", "weather"]) for (const [key, value] of Object.entries(raw[table])) catalog[table][key] = { ...textRecord(key, value), id: key, ...(value.sprite ? { sprite: assetPath(value.sprite) } : {}), facts: extra(value, ["id", "name", "description", "sprite"]) };
  for (const [key, value] of Object.entries(raw.projectiles)) catalog.projectiles[key] = { key, id: key, sprite: image(value.sprite, 1), facts: extra(value, ["sprite"]) };
  for (const value of raw.achievements) catalog.achievements.push({ ...textRecord(String(value.id), value), id: value.id, image: assetPath(value.image), facts: extra(value, ["id", "name", "description", "image", "status"]) });
  const pokemon = modules.get("pokemonData.js").parser;
  catalog.collection = pokemon.get("allPokemon");
  catalog.eggPool = pokemon.get("eggListData");
  catalog.forcedInitialPool = pokemon.get("forcedInitialPool");
  catalog.forcedSecondaryPool = pokemon.get("forcedSecondaryPool");
  const items = modules.get("itemData.js").parser;
  catalog.itemPools = { normal: items.get("normalPool"), luxury: items.get("luxuryPool"), admin: items.get("adminPool") };
  catalog.removedItemIds = items.get("removedItemIds");
  catalog.tutorials = normalizeTutorialFacts((await load("tutorialData.js")).parser.get("tutorials"));
  for (const topic of Object.values(catalog.tutorials)) if (topic.sprite && !media.has(topic.sprite)) throw Error(`Missing tutorial illustration: ${topic.sprite}`);
  const themeBytes = await readFile(path.join(gameRoot, "data/theme-colors.json"));
  const themeFacts = JSON.parse(themeBytes);
  if (themeFacts.version !== catalog.version || themeFacts.method !== "literal-color-facts-only" || Object.keys(themeFacts.colors).length !== 4) throw Error("Incompatible theme color facts");
  catalog.themes = themeFacts.colors;
  const priceBytes = await readFile(path.join(gameRoot, "data/upgrade-costs.json"));
  const priceFacts = JSON.parse(priceBytes);
  if (priceFacts.version !== catalog.version || priceFacts.method !== "offline-arithmetic-fact-table") throw Error("Incompatible upgrade price facts");
  catalog.upgradeCosts = priceFacts.costs;
  const sourceManifest = [...sourceFiles.values()].sort((left, right) => left.path.localeCompare(right.path));
  const factFiles = [{ path: "data/upgrade-costs.json", bytes: priceBytes.length, sha256: sha256(priceBytes) }, { path: "data/theme-colors.json", bytes: themeBytes.length, sha256: sha256(themeBytes) }];
  catalog.source.sourceSha256 = sha256(JSON.stringify([...sourceManifest, ...factFiles].sort((left, right) => left.path.localeCompare(right.path))));
  validateCatalog(catalog);
  const dataRoot = path.join(gameRoot, "data");
  const publicRoot = path.join(gameRoot, "assets/public/pokepath/v1");
  await mkdir(dataRoot, { recursive: true });
  await mkdir(path.join(publicRoot, "routes"), { recursive: true });
  const serialized = `${JSON.stringify(catalog, null, 2)}\n`;
  await writeFile(path.join(dataRoot, "catalog.json"), serialized);
  const goLiteral = "`" + serialized.replaceAll("`", '` + "`" + `') + "`";
  await writeFile(path.join(gameRoot, "engine/resources.go"), `package engine\n\nimport "encoding/json"\n\nconst catalogJSON = ${goLiteral}\n\nfunc loadBuiltinCatalog() *Catalog {\n\tvar catalog Catalog\n\tif err := json.Unmarshal([]byte(catalogJSON), &catalog); err != nil {\n\t\tpanic(err)\n\t}\n\treturn &catalog\n}\n`);
  const generated = [];
  const publish = async (filename, value) => {
    const bytes = Buffer.from(`${JSON.stringify(value)}\n`);
    await writeFile(path.join(publicRoot, filename), bytes);
    generated.push({ path: filename, bytes: bytes.length, sha256: sha256(bytes), mime: "application/json" });
  };
  const core = { ...catalog, routes: Object.fromEntries(Object.entries(catalog.routes).map(([key, route]) => [key, { id: route.id, key: route.key, name: route.name, nameEn: route.nameEn, names: route.names, background: route.background, effect: route.effect, width: route.width, height: route.height, unlockStars: route.unlockStars, isSecret: route.isSecret, secretKey: route.secretKey, weatherId: route.weatherId, challengeRewards: route.challengeRewards, musicId: route.musicId, musicKey: route.musicKey, order: route.facts.order, position: route.facts.pos, waveCount: Object.keys(route.waves).length }])) };
  await publish("catalog.json", core);
  for (const [key, route] of Object.entries(catalog.routes)) await publish(`routes/${key}.json`, route);
  const { extractUIText } = await import("./ui-text.mjs");
  const uiText = await extractUIText();
  generated.push({ path: uiText.path, bytes: uiText.bytes, sha256: uiText.sha256, mime: uiText.mime });
  const mediaFacts = mediaResourceFacts(mediaManifest);
  await writeFile(path.join(publicRoot, "resources.json"), `${JSON.stringify({ version: catalog.version, files: [...mediaFacts.files, ...generated].sort((left, right) => left.path.localeCompare(right.path)), aliases: mediaFacts.aliases })}\n`);
  const manifest = { version: catalog.version, method: "static-literal-facts-only", sourceFiles: sourceManifest, factFiles, priceFacts: priceFacts.reference, catalog: { path: "data/catalog.json", bytes: Buffer.byteLength(serialized), sha256: sha256(serialized) }, counts: { ...counts, collection: catalog.collection.length, waves: 4007, songs: catalog.playlist.length, tutorials: Object.keys(catalog.tutorials).length, themes: Object.keys(catalog.themes).length }, publicFiles: generated };
  await writeFile(path.join(dataRoot, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
  const verificationRoot = path.join(root, ".local/pokepath/verification/native-reference");
  await mkdir(verificationRoot, { recursive: true });
  await writeFile(path.join(verificationRoot, "result.json"), `${JSON.stringify({ command: "node games/pokepath/build/extract-reference.mjs", ...manifest }, null, 2)}\n`);
  await generateRulesRevision();
  process.stdout.write(`${JSON.stringify({ counts: manifest.counts, bytes: manifest.catalog.bytes, sha256: manifest.catalog.sha256, publicFiles: generated.length })}\n`);
  return catalog;
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) extractReference().catch((error) => { process.stderr.write(`${error.stack}\n`); process.exitCode = 1; });
