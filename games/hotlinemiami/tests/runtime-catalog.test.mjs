import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { validateHotlineAssets, validateHotlineNativeSources } from "../build/runtime-delivery.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/hotlinemiami/verification/large-map-quality");
mkdirSync(output, { recursive: true });
const definitions = [["no-talk", 17], ["full-house", 31], ["push-it", 33], ["overdose", 20], ["tension", 49], ["neighbors", 35], ["highball", 65], ["exposed", 148], ["hot-heavy", 59]];
const results = [];

function fixture() {
  const nativeMaps = definitions.map(([id, sourceRoomId], index) => ({ id, sourceRoomId, sourceRoomName: `native-room-${sourceRoomId}`, sourceRoomOffset: index * 100, width: index < 6 ? 640 : 1024, height: index < 6 ? 480 : 768, coordinateScale: 1 }));
  for (let index = 0; index < nativeMaps.length; index++) {
    const reachable = index < 6 ? 1000 + index * 10 : 2000 + (index - 6) * 100;
    const walkable = index < 6 ? 1100 : 2600;
    nativeMaps[index].arenaAudit = { gridStep: 16, radius: 9, floorCells: index < 6 ? 1150 : 2800, walkableFloorCells: walkable, reachableFloorCells: reachable, reachableAreaPx2: reachable * 256, reachablePermille: Math.floor(reachable * 1000 / walkable), minSpawnPathPx: 128, baselineReachableFloorCells: 1050, baselineRatioPermille: Math.floor(reachable * 1000 / 1050), allSpawnAndWeaponsReachable: true, sourceFloorSHA256: "a".repeat(64), sourceMetadataSHA256: "b".repeat(64) };
  }
  const manifest = { version: 1, images: {}, maps: [], characters: Array.from({ length: 8 }, (_, index) => ({ id: `character-${index}` })), weapons: Array.from({ length: 12 }, (_, index) => ({ id: `weapon-${index}` })) };
  for (const source of nativeMaps) {
    const image = `map-${source.id}`;
    const preview = `preview-${source.id}`;
    manifest.images[image] = { file: `images/${image}.png`, width: source.width, height: source.height, bytes: 100, sha256: "a".repeat(64) };
    manifest.images[preview] = { file: `images/${preview}.png`, width: 320, height: 240, bytes: 100, sha256: "b".repeat(64) };
    manifest.maps.push({ id: source.id, image, preview, width: source.width, height: source.height, rules: { ...source, coordinateScale: 256, width: source.width * 256, height: source.height * 256 } });
  }
  return { manifest, nativeMaps };
}

function floorProof(nativeMaps) {
  return { sourceMetadataSHA256: "b".repeat(64), maps: nativeMaps.map(source => {
    const floor = Array.from({ length: source.arenaAudit.floorCells }, (_, index) => ({ x: 8 + index % 40 * 16, y: 8 + Math.floor(index / 40) * 16 }));
    source.arenaAudit.sourceFloorSHA256 = createHash("sha256").update(JSON.stringify(floor)).digest("hex");
    return { id: source.id, sourceRoomId: source.sourceRoomId, floor };
  }) };
}

test("the catalog accepts nine identified native scenes with eight characters and twelve weapons", () => {
  const { manifest, nativeMaps } = fixture();
  assert.doesNotThrow(() => validateHotlineAssets(manifest, nativeMaps));
  results.push({ scenario: "nine-native-scenes", passed: true });
});

test("each exact content count rejects missing and extra choices", () => {
  for (const [key, expected] of [["maps", 9], ["characters", 8], ["weapons", 12]]) {
    for (const offset of [-1, 1]) {
      const { manifest, nativeMaps } = fixture();
      manifest[key] = Array.from({ length: expected + offset }, (_, index) => manifest[key][index % expected]);
      assert.throws(() => validateHotlineAssets(manifest, nativeMaps), new RegExp(`${key}.*${expected}`));
    }
  }
  const { manifest, nativeMaps } = fixture();
  manifest.maps = manifest.maps.slice(0, 6);
  assert.throws(() => validateHotlineAssets(manifest, nativeMaps), /maps.*9/);
  results.push({ scenario: "exact-counts", passed: true });
});

test("duplicate or relabelled original rooms cannot stand in for the three new scenes", () => {
  for (const mutation of [
    manifest => { manifest.maps[6].id = manifest.maps[0].id; },
    manifest => { manifest.maps[6].rules.sourceRoomId = manifest.maps[0].rules.sourceRoomId; },
    manifest => { manifest.maps[6].rules.sourceRoomId = 999; },
    manifest => { manifest.maps[6].rules.sourceRoomName = "other-native-room"; },
  ]) {
    const { manifest, nativeMaps } = fixture();
    mutation(manifest);
    assert.throws(() => validateHotlineAssets(manifest, nativeMaps), /scene|source|native|duplicate/i);
  }
  results.push({ scenario: "scene-identity", passed: true });
});

test("padding, image scaling and scaled collision bounds cannot replace original playable dimensions", () => {
  for (const mutation of [
    manifest => { manifest.maps[6].width += 256; },
    manifest => { manifest.images[manifest.maps[6].image].width += 256; },
    manifest => { manifest.maps[6].rules.width += 256 * 256; },
    manifest => { manifest.maps[6].rules.coordinateScale = 1; },
    manifest => { delete manifest.images[manifest.maps[6].preview]; },
  ]) {
    const { manifest, nativeMaps } = fixture();
    mutation(manifest);
    assert.throws(() => validateHotlineAssets(manifest, nativeMaps), /dimension|scale|image|preview|native/i);
  }
  results.push({ scenario: "native-dimensions", passed: true });
});

test("native floor and reachable metrics must be internally consistent and source bound", () => {
  for (const mutation of [
    nativeMaps => { delete nativeMaps[6].arenaAudit; },
    nativeMaps => { nativeMaps[6].arenaAudit.reachableAreaPx2++; },
    nativeMaps => { nativeMaps[6].arenaAudit.reachableFloorCells = 0; },
    nativeMaps => { nativeMaps[6].arenaAudit.walkableFloorCells = 1; },
    nativeMaps => { nativeMaps[6].arenaAudit.sourceFloorSHA256 = "missing"; },
    nativeMaps => { nativeMaps[6].arenaAudit.sourceMetadataSHA256 = "missing"; },
    nativeMaps => { nativeMaps[6].arenaAudit.allSpawnAndWeaponsReachable = false; },
    nativeMaps => { nativeMaps[6].arenaAudit.baselineReachableFloorCells++; },
  ]) {
    const { manifest, nativeMaps } = fixture();
    mutation(nativeMaps);
    assert.throws(() => validateHotlineAssets(manifest, nativeMaps), /floor|reachable|audit|source|baseline/i);
  }
  results.push({ scenario: "native-floor-audit", passed: true });
});

test("actual original floor point bytes are bound to each scene audit", () => {
  const { nativeMaps } = fixture();
  const proof = floorProof(nativeMaps);
  assert.doesNotThrow(() => validateHotlineNativeSources(nativeMaps, proof));
  for (const mutation of [
    value => { value.maps[6].floor[0].x++; },
    value => { value.maps[6].sourceRoomId = 17; },
    value => { value.sourceMetadataSHA256 = "c".repeat(64); },
  ]) {
    const changed = structuredClone(proof);
    mutation(changed);
    assert.throws(() => validateHotlineNativeSources(nativeMaps, changed), /source|floor|scene/i);
  }
  results.push({ scenario: "original-floor-byte-binding", passed: true });
});

test.after(() => {
  writeFileSync(path.join(output, "catalog.json"), JSON.stringify({ passed: results.length === 6, cases: results }, null, 2) + "\n");
});
