import assert from "node:assert/strict";
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { predictCar } from "../web/prediction.ts";
import { classicTrackFromRoute, classicPropWorld } from "../web/classic-road.ts";
import { classicTerrainFromCatalog } from "../web/classic-terrain.ts";

const [fixture, report] = process.argv.slice(2);
const rows = JSON.parse(readFileSync(fixture, "utf8"));
const tracks = new Map();
const catalogs = new Map();
let airborne = 0, landings = 0, contactBoundaryDifferences = 0, maxPosition = 0, maxHeight = 0, maxSpeed = 0;
for (const row of rows) {
  if (!tracks.has(row.track)) {
    const base = new URL(`../assets/public/racing/v3/tracks/${row.track}`, import.meta.url);
    const track = classicTrackFromRoute(JSON.parse(readFileSync(`${base.pathname}.route.json`, "utf8")));
    const collision = readFileSync(`${base.pathname}.collision.json`);
    track.terrain = classicTerrainFromCatalog({ ...JSON.parse(collision), unit: "metres", geometry: track.geometry, sourceSha256: createHash("sha256").update(collision).digest("hex") }, track);
    tracks.set(row.track, track);
    catalogs.set(row.track, JSON.parse(readFileSync(new URL(`../assets/public/racing/v3/props/${row.track}.json`, import.meta.url), "utf8")));
  }
  const props = classicPropWorld(catalogs.get(row.track), row.props, tracks.get(row.track));
  const predicted = predictCar(row.before, row.input, tracks.get(row.track), row.ms, row.at, props);
  const position = Math.hypot(predicted.x - row.after.x, predicted.y - row.after.y);
  const height = Math.abs(predicted.height - (row.after.height ?? 0));
  const speed = Math.abs(predicted.speed - row.after.speed);
  maxPosition = Math.max(maxPosition, position);
  maxHeight = Math.max(maxHeight, height);
  maxSpeed = Math.max(maxSpeed, speed);
  if (row.before.airborne || row.after.airborne) airborne++;
  if (row.before.airborne && !row.after.airborne) landings++;
  assert(position < .04 && height < .02 && speed < .35,
    `${row.track} seq=${row.before.seq} airborne=${row.before.airborne} position=${position} height=${height} speed=${speed}`);
  if (!!predicted.airborne !== !!row.after.airborne) contactBoundaryDifferences++;
  assert.equal(!!predicted.airborne, !!row.after.airborne);
}
assert(tracks.size === 4 && airborne > 10 && landings > 4);
const result = { passed: true, steps: rows.length, tracks: tracks.size, airborne, landings, contactBoundaryDifferences, maxPositionDM: maxPosition, maxHeightDM: maxHeight, maxSpeedDMPS: maxSpeed };
writeFileSync(report, `${JSON.stringify(result, null, 2)}\n`);
console.log(JSON.stringify(result));
