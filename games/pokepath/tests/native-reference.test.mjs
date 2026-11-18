import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { parseLiteralDeclarations, validateCatalog } from "../build/extract-reference.mjs";

test("静态事实读取器只接受字面量与已知表引用", () => {
  const result = parseLiteralDeclarations("const colors = ['red', 'blue']; export const fact = { power: 13, perLevel: 2, terrain: [1, 2], color: colors[0], text: `规则事实`, negative: -1 };", "facts.js");
  assert.deepEqual(result.get("fact"), { power: 13, perLevel: 2, terrain: [1, 2], color: "red", text: "规则事实", negative: -1 });
  for (const source of ["export const fact = fetch('https://invalid.test')", "export const fact = (()=>1)()", "export const fact = { run(){ return 1 } }", "export const fact = Math.random()", "export const fact = process.env"]) {
    const parsed = parseLiteralDeclarations(source, "unsafe.js");
    assert.throws(() => parsed.get("fact"), /static|literal|reference|unsupported|unknown/i);
  }
});

test("独立事实目录保留完整数值、网格、图集和稳定引用", async () => {
  const catalog = JSON.parse(await readFile(new URL("../data/catalog.json", import.meta.url), "utf8"));
  assert.doesNotThrow(() => validateCatalog(catalog));
  assert.equal(catalog.version, "1.7.3");
  for (const [key, expected] of Object.entries({ species: 517, enemies: 347, routes: 47, items: 197, abilities: 254, passives: 85, achievements: 39, weather: 11, projectiles: 62 })) assert.equal(Object.keys(catalog[key]).length, expected, key);
  assert.equal(catalog.collection.length, 257);
  assert.equal(new Set(catalog.collection.map((key) => catalog.species[key].id)).size, 257);
  assert.equal(Object.values(catalog.routes).reduce((count, route) => count + Object.keys(route.waves).length, 0), 4007);
  const starter = catalog.species.charmander;
  assert.equal(starter.powerBase, 13);
  assert.equal(starter.powerPerLevel, 2);
  assert.equal(starter.attackMsBase, 1100);
  assert.equal(starter.abilityId, "burn");
  assert.equal(starter.evolutionKey, "charmeleon");
  assert.deepEqual([starter.sprite.width, starter.sprite.height, starter.sprite.frames, starter.sprite.rows, starter.sprite.frameWidth, starter.sprite.frameHeight], [128, 320, 4, 8, 32, 40]);
  assert.equal(catalog.species.megaAmpharos.sprite.fractionalFrames, true);
  assert.equal(catalog.species.megaAmpharos.sprite.frameHeight, 58.5);
  assert.equal(catalog.enemies.rattata.speed, 60);
  assert.equal(catalog.enemies.shaymin.health, 120000);
  assert.equal(catalog.enemies.shaymin.regeneration, 1000);
  assert.equal(catalog.enemies.shaymin.passiveId, "naturalCure");
  assert.deepEqual(catalog.routes[0].paths[0][0], { x: 358.5, y: -60 });
  assert.deepEqual(catalog.routes[0].placements[0], { id: 0, x: 48, y: 96, terrain: 2 });
  assert.deepEqual(catalog.routes[0].waves[1].sequence, ["rattata", "rattata", "rattata"]);
  assert.deepEqual([catalog.routes[21].width, catalog.routes[21].height], [1440, 624]);
  assert.deepEqual([catalog.routes[26].width, catalog.routes[26].height], [720, 1872]);
  assert.deepEqual([catalog.routes[45].width, catalog.routes[45].height], [2160, 624]);
  assert.equal(catalog.items.poisonBarb.price, 10000);
  assert.equal(catalog.upgradeCosts.low[1], 20);
  assert.equal(catalog.upgradeCosts.mid[1], 40);
  assert.equal(catalog.upgradeCosts.mid[2], 49);
  assert.equal(catalog.upgradeCosts.high[1], 60);
  assert.equal(catalog.upgradeCosts.veryHigh[1], 60);
  for (const [scale, prices] of Object.entries(catalog.upgradeCosts)) {
    assert.equal(prices.length, 101, scale);
    assert.equal(prices[0], 0);
    assert.ok(prices.every((price, level) => Number.isSafeInteger(price) && price >= 0 && (!level || price >= prices[level - 1])));
    assert.equal(prices[100], scale === "veryHigh" ? 150000 : 100000);
  }
  assert.match(catalog.abilities.burn.descriptionEn, /burn/i);
  const broken = structuredClone(catalog);
  broken.species.charmander.abilityId = "unknown-ability";
  assert.throws(() => validateCatalog(broken), /ability|reference/i);
  broken.species.charmander.abilityId = "burn";
  broken.routes[0].waves[1].sequence[0] = "unknown-enemy";
  assert.throws(() => validateCatalog(broken), /enemy|reference/i);
});

test("前端按图目录与资源摘要不携带原程序", async () => {
  const publicRoot = new URL("../assets/public/pokepath/v1/", import.meta.url);
  const core = JSON.parse(await readFile(new URL("catalog.json", publicRoot), "utf8"));
  assert.equal(Object.keys(core.routes).length, 47);
  assert.equal(core.routes[0].waves, undefined);
  assert.equal(core.routes[0].paths, undefined);
  assert.equal(core.routes[0].placements, undefined);
  const route = JSON.parse(await readFile(new URL("routes/0.json", publicRoot), "utf8"));
  assert.equal(Object.keys(route.waves).length, 100);
  const resources = JSON.parse(await readFile(new URL("resources.json", publicRoot), "utf8"));
  const files = resources.files;
  assert.ok(files.some((file) => file.path === "catalog.json"));
  assert.equal(files.filter((file) => /^routes\/\d+\.json$/.test(file.path)).length, 47);
  assert.equal(files.some((file) => /\.(?:js|mjs|css)$/.test(file.path)), false);
  for (const file of files.filter((entry) => entry.path === "catalog.json" || entry.path.startsWith("routes/"))) {
    const bytes = await readFile(new URL(file.path, publicRoot));
    assert.equal(bytes.length, file.bytes);
    assert.equal(createHash("sha256").update(bytes).digest("hex"), file.sha256);
  }
});

test("规则来源摘要覆盖独立价格事实", async () => {
  const catalog = JSON.parse(await readFile(new URL("../data/catalog.json", import.meta.url), "utf8"));
  const manifest = JSON.parse(await readFile(new URL("../data/manifest.json", import.meta.url), "utf8"));
  const bytes = await readFile(new URL("../data/upgrade-costs.json", import.meta.url));
  const themeBytes = await readFile(new URL("../data/theme-colors.json", import.meta.url));
  assert.deepEqual(manifest.factFiles, [{ path: "data/upgrade-costs.json", bytes: bytes.length, sha256: createHash("sha256").update(bytes).digest("hex") }, { path: "data/theme-colors.json", bytes: themeBytes.length, sha256: createHash("sha256").update(themeBytes).digest("hex") }]);
  const inputs = [...manifest.sourceFiles, ...manifest.factFiles].sort((left, right) => left.path.localeCompare(right.path));
  const digest = createHash("sha256").update(JSON.stringify(inputs)).digest("hex");
  assert.equal(catalog.source.sourceSha256, digest);
  const changed = structuredClone(inputs);
  changed.find((entry) => entry.path === "data/upgrade-costs.json").sha256 = "0".repeat(64);
  assert.notEqual(createHash("sha256").update(JSON.stringify(changed)).digest("hex"), digest);
});

test("音乐事实保留曲序、名称和地图音源关联", async () => {
  const catalog = JSON.parse(await readFile(new URL("../data/catalog.json", import.meta.url), "utf8"));
  const core = JSON.parse(await readFile(new URL("../assets/public/pokepath/v1/catalog.json", import.meta.url), "utf8"));
  const resources = JSON.parse(await readFile(new URL("../assets/public/pokepath/v1/resources.json", import.meta.url), "utf8"));
  assert.equal(Object.keys(catalog.songs).length, 41);
  assert.equal(catalog.playlist.length, 41);
  assert.equal(new Set(catalog.playlist).size, 41);
  assert.equal(catalog.songs.route10.id, 3);
  assert.equal(catalog.songs.route10.nameEn, "Route 1-4");
  assert.equal(catalog.songs.route10.audio, "src/assets/sounds/music/route10.mp3");
  assert.equal(catalog.playlist[3], "route10");
  assert.equal(catalog.playlist[40], "halloween");
  assert.equal(catalog.routes[9].musicKey, "route10");
  assert.equal(catalog.routes[20].musicKey, "route30");
  assert.equal(catalog.routes[30].musicKey, "route8");
  assert.equal(catalog.routes[46].musicKey, "halloween");
  assert.deepEqual(core.songs, catalog.songs);
  assert.deepEqual(core.playlist, catalog.playlist);
  for (const [order, key] of catalog.playlist.entries()) {
    const song = catalog.songs[key];
    assert.equal(song.key, key);
    assert.equal(song.id, order);
    assert.equal(song.names.length, 10);
    assert.ok(resources.files.some((file) => file.path === song.audio && file.mime === "audio/mpeg"), key);
  }
  for (const [id, route] of Object.entries(catalog.routes)) {
    assert.equal(catalog.songs[route.musicKey].id, route.musicId, id);
    assert.equal(core.routes[id].musicKey, route.musicKey, id);
  }
  const broken = structuredClone(catalog);
  broken.routes[0].musicKey = "unknown-song";
  assert.throws(() => validateCatalog(broken), /song|music|reference/i);
  broken.routes[0].musicKey = "route1";
  broken.playlist[1] = "route1";
  assert.throws(() => validateCatalog(broken), /song|music|playlist/i);
});

test("棋盘房棋子保留稳定标识、物种和原始网格坐标", async () => {
  const catalog = JSON.parse(await readFile(new URL("../data/catalog.json", import.meta.url), "utf8"));
  const core = JSON.parse(await readFile(new URL("../assets/public/pokepath/v1/catalog.json", import.meta.url), "utf8"));
  const route = JSON.parse(await readFile(new URL("../assets/public/pokepath/v1/routes/35.json", import.meta.url), "utf8"));
  assert.equal(route.chessPlacements.length, 16);
  assert.equal(route.facts.chessTiles.length, 780);
  assert.deepEqual(route.chessPlacements[0], { key: "p1", species: "pawniard", x: 456, y: 144 });
  assert.deepEqual(route.chessPlacements.find((piece) => piece.key === "h2"), { key: "h2", species: "kingdra", x: 216, y: 432 });
  assert.deepEqual(route.chessPlacements.find((piece) => piece.key === "p"), { key: "p", species: "pawniard", x: 504, y: 432 });
  assert.equal(new Set(route.chessPlacements.map((piece) => piece.key)).size, 16);
  assert.deepEqual(route.chessPlacements, catalog.routes[35].chessPlacements);
  assert.equal(core.routes[35].chessPlacements, undefined);
  const broken = structuredClone(catalog);
  broken.routes[35].chessPlacements[0].species = "unknown-species";
  assert.throws(() => validateCatalog(broken), /chess|reference|species/i);
  broken.routes[35].chessPlacements[0].species = "pawniard";
  broken.routes[35].chessPlacements[1].key = "p1";
  assert.throws(() => validateCatalog(broken), /chess|duplicate/i);
});

test("公共媒体路径遵守CDN规范并保留重命名素材来源", async () => {
  const publicRoot = new URL("../assets/public/pokepath/v1/", import.meta.url);
  const media = JSON.parse(await readFile(new URL("../assets/manifest.json", import.meta.url), "utf8"));
  const resources = JSON.parse(await readFile(new URL("resources.json", publicRoot), "utf8"));
  const canonical = /^(?!.*(?:^|\/)\.\.?(?:\/|$))[A-Za-z0-9][A-Za-z0-9._/-]*$/;
  for (const file of [...media.files, ...resources.files]) assert.match(file.path, canonical, file.path);
  const oldPath = "src/assets/images/pokemon/normal/marshadow-walk (2).png";
  const newPath = "src/assets/images/pokemon/normal/marshadow-walk-2.png";
  const renamed = media.files.find((file) => file.path === newPath);
  assert.equal(renamed.sourcePath, oldPath);
  assert.equal(renamed.url, "https://pokepath-game.pages.dev/src/assets/images/pokemon/normal/marshadow-walk%20(2).png");
  const bytes = await readFile(new URL(newPath, publicRoot));
  assert.equal(bytes.length, renamed.bytes);
  assert.equal(createHash("sha256").update(bytes).digest("hex"), renamed.sha256);
  assert.deepEqual(bytes, await readFile(new URL("src/assets/images/pokemon/normal/marshadow-walk.png", publicRoot)));
  await assert.rejects(readFile(new URL(oldPath, publicRoot)), /ENOENT/);
  assert.equal(resources.files.some((file) => file.path === oldPath), false);
  assert.ok(resources.files.some((file) => file.path === newPath && file.sha256 === renamed.sha256));
});
