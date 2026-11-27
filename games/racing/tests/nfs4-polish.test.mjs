import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync } from "node:fs";
import { classicTrackNames } from "../web/classic-catalog.ts";
import { classicTrackFromRoute } from "../web/classic-road.ts";
import { classicSeatColor, minimapMarkerColors } from "../web/classic-minimap.ts";
import { collectClassicSceneAssets } from "../web/classic-loading.ts";
import {
  classicSkidAssets,
  classicSkidPatches,
  classicSkidPatchForSurface,
  classicBrakeAudioVoice,
  ClassicBrakePlayback,
} from "../web/classic-brake-audio.ts";
import { formatPlayerCounts } from "../../../web/src/game-api/default-players.ts";

function relativeLuminance(hex) {
  const channels = [1, 3, 5]
    .map((offset) => parseInt(hex.slice(offset, offset + 2), 16) / 255)
    .map((value) => (value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4));
  return channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
}

function contrastRatio(hexA, hexB) {
  const lumA = relativeLuminance(hexA);
  const lumB = relativeLuminance(hexB);
  const lighter = Math.max(lumA, lumB);
  const darker = Math.min(lumA, lumB);
  return (lighter + 0.05) / (darker + 0.05);
}

test("all five tracks share identical canonical names across catalog and parsed routes", () => {
  const expected = {
    france: "法国乡间公路",
    coastal: "海豚湾",
    uk: "凯尔特遗迹",
    park: "金迪亚克公园",
    germany: "德国乡间公路",
  };
  assert.deepEqual(classicTrackNames, expected);
  for (const [id, name] of Object.entries(expected)) {
    const raw = JSON.parse(
      readFileSync(new URL(`../assets/public/racing/v3/tracks/${id}.route.json`, import.meta.url), "utf8"),
    );
    const parsed = classicTrackFromRoute(raw);
    assert.equal(parsed.name, name, `${id} route name mismatch`);
  }
});

test("eight unified seat colors are unique and meet dark HUD contrast requirements", () => {
  assert.equal(minimapMarkerColors.length, 8);
  assert.equal(new Set(minimapMarkerColors).size, 8);
  for (let seat = 0; seat < 8; seat++) {
    const color = classicSeatColor(seat);
    assert.match(color, /^#[0-9a-f]{6}$/i);
    assert.ok(
      contrastRatio(color, "#08131f") >= 4.5,
      `seat ${seat} color ${color} contrast on #08131f is below 4.5`,
    );
  }
  assert.equal(classicSeatColor(8), classicSeatColor(0));
  assert.equal(classicSeatColor(-1), classicSeatColor(7));
});

test("player count formatter collapses contiguous ranges and preserves sparse sets", () => {
  assert.equal(
    formatPlayerCounts({ minPlayers: 2, maxPlayers: 8, playerCounts: [2, 3, 4, 5, 6, 7, 8] }),
    "2～8",
  );
  assert.equal(
    formatPlayerCounts({ minPlayers: 2, maxPlayers: 6, playerCounts: [2, 3, 4, 5, 6] }),
    "2～6",
  );
  assert.equal(
    formatPlayerCounts({ minPlayers: 2, maxPlayers: 8, playerCounts: [2, 4, 6, 8] }),
    "2 / 4 / 6 / 8",
  );
  assert.equal(
    formatPlayerCounts({ minPlayers: 1, maxPlayers: 1, playerCounts: [1] }),
    "1",
  );
  assert.equal(
    formatPlayerCounts({ minPlayers: 2, maxPlayers: 4 }),
    "2～4",
  );
});

test("classic scene asset collection always includes engine and brake skid audio", () => {
  const manifest = JSON.parse(
    readFileSync(new URL("../assets/public/racing/v3/manifest.json", import.meta.url), "utf8"),
  );
  const skidSpecs = classicSkidAssets(manifest);
  assert.equal(skidSpecs.length, classicSkidPatches.length + 1);
  const specs = collectClassicSceneAssets(manifest, "park", "day", ["p911", "fm50"]);
  const paths = new Set(specs.map((item) => item.path));
  assert.ok(paths.has("audio/p911/engine.json"), "p911 engine metadata must always preload");
  assert.ok(paths.has("audio/fm50/engine.json"), "fm50 engine metadata must always preload");
  assert.ok(paths.has("audio/sfx/skid.json"), "skid metadata must always preload");
  for (const patch of classicSkidPatches) {
    assert.ok(paths.has(`audio/sfx/skid-${patch}.flac`), `skid patch ${patch} must preload`);
  }
  assert.ok(paths.has("audio/sfx/collision.json"), "collision metadata must always preload");
  assert.ok(paths.has("audio/horn/horn.json"), "horn metadata must always preload");
});

test("brake and tire screech audio maps NFS4 surfaces and responds to braking, handbrake, slip, and stop states", () => {
  assert.equal(classicSkidPatchForSurface(1), 42);
  assert.equal(classicSkidPatchForSurface(7), 41);
  assert.equal(classicSkidPatchForSurface(3), 45);
  assert.equal(classicSkidPatchForSurface(2), 47);
  assert.equal(classicSkidPatchForSurface(4), 46);

  const stopped = classicBrakeAudioVoice(
    { vx: 0, vy: 0, angle: 0, speed: 0, brake: true, brakeAmount: 1, drift: false, offroad: false, finishAt: 0, resetUntil: 0 },
    1000,
    1,
  );
  assert.equal(stopped, null);

  const airborne = classicBrakeAudioVoice(
    { airborne: true, vx: 260, vy: 0, angle: 0, speed: 260, brake: true, brakeAmount: 1, drift: true, offroad: false, finishAt: 0, resetUntil: 0 },
    1000,
    1,
  );
  assert.equal(airborne, null);

  const braking = classicBrakeAudioVoice(
    { vx: 220, vy: 0, angle: 0, speed: 220, brake: true, brakeAmount: 1, drift: false, offroad: false, finishAt: 0, resetUntil: 0, motion: { longAccel: -10, brake: 1, handbrake: 0 } },
    1000,
    1,
  );
  assert.ok(braking !== null, "active braking at speed should produce brake screech voice");
  assert.equal(braking.patch, 42);
  assert.ok(braking.volume > 0.08 && braking.volume <= 0.32);
  assert.ok(braking.rate >= 0.72 && braking.rate <= 1.4);

  const driftingOffroad = classicBrakeAudioVoice(
    { vx: 180, vy: 45, angle: 0, speed: 185, brake: false, drift: true, offroad: true, finishAt: 0, resetUntil: 0 },
    1000,
    2,
  );
  assert.ok(driftingOffroad !== null);
  assert.equal(driftingOffroad.patch, 47);

  const calls = [];
  const stops = [];
  const playback = new ClassicBrakePlayback({
    loop: async (key, sources, options) => {
      calls.push({ key, sources, options });
      return true;
    },
    stopLoop: (key) => {
      stops.push(key);
    },
  });
  const loaded = new Map(classicSkidPatches.map((patch) => [patch, `data:audio/flac;base64,ZmFrZQ==`]));
  playback.update(loaded, braking, true);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].key, "racing-skid-42");
  assert.equal(calls[0].options.continuous, true);
  playback.update(loaded, driftingOffroad, true);
  assert.deepEqual(stops, ["racing-skid-42"]);
  assert.equal(calls[1].key, "racing-skid-47");
  playback.update(loaded, null, true);
  assert.deepEqual(stops, ["racing-skid-42", "racing-skid-47"]);
});

test("tablet touch media query enlarges steering arrows, brake, and throttle buttons", () => {
  const css = readFileSync(new URL("../web/style.css", import.meta.url), "utf8");
  const tabletBlock = css.match(/@media\s*\(min-width:\s*700px\)\s*and\s*\(min-height:\s*600px\)\s*\{[\s\S]*?\n\}/);
  assert.ok(tabletBlock, "tablet touch media query should exist");
  assert.match(tabletBlock[0], /\.is-classic\.has-touch\s+\.racing-steering\s+button[\s\S]*?width:\s*(10[4-9]|11\d|12\d)px/);
  assert.match(tabletBlock[0], /\.is-classic\.has-touch\s+\.racing-pedals\s+\.racing-brake[\s\S]*?width:\s*(9[6-9]|10\d|11\d)px/);
  assert.match(tabletBlock[0], /\.is-classic\.has-touch\s+\.racing-pedals\s+\.racing-throttle[\s\S]*?width:\s*(10[8-9]|11\d|12\d)px/);
});
