import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import { classicVehicle, stepClassicMotion } from "../web/classic-motion.ts";

const [fixturePath, trajectoryPath, reportPath] = process.argv.slice(2);
const fixtures = JSON.parse(await readFile(fixturePath, "utf8"));
const expected = new Map(
  JSON.parse(await readFile(trajectoryPath, "utf8")).map((row) => [
    row.name,
    row.states,
  ]),
);
const numericFields = [
  "x",
  "z",
  "vx",
  "vz",
  "angle",
  "yawRate",
  "steering",
  "longAccel",
  "lateralAccel",
  "rpm",
  "shiftTimer",
  "throttle",
  "brake",
  "handbrake",
  "reverseTimer",
];
const empty = () =>
  Object.fromEntries([...numericFields, "gear"].map((key) => [key, 0]));
const maximumErrors = Object.fromEntries(numericFields.map((key) => [key, 0]));
for (const id of ["", "missing", "__proto__", "constructor"]) {
  assert.equal(classicVehicle(id).id, "p911", `unknown vehicle ${id}`);
}
let samples = 0;
for (const fixture of fixtures) {
  let state = { ...empty(), ...fixture.initial };
  const vehicle = classicVehicle(fixture.vehicle);
  let cursor = 1;
  for (const segment of fixture.segments) {
    for (let i = 0; i < Math.round(segment.seconds / 0.02); i++) {
      state = stepClassicMotion(
        state,
        segment.input,
        vehicle,
        segment.surface ?? {},
        0.02,
      );
      const go = expected.get(fixture.name)[cursor++];
      assert.equal(state.gear, go.gear, `${fixture.name} ${cursor} gear`);
      for (const key of numericFields) {
        assert.ok(
          Number.isFinite(state[key]),
          `${fixture.name} ${key} is finite`,
        );
        const error = Math.abs(state[key] - go[key]);
        maximumErrors[key] = Math.max(maximumErrors[key], error);
        assert.ok(
          error < (key === "rpm" ? 0.0001 : 0.000001),
          `${fixture.name} ${cursor} ${key}: ${state[key]} vs ${go[key]}`,
        );
      }
      samples++;
    }
  }
  assert.equal(cursor, expected.get(fixture.name).length);
}
for (const dt of [0.02, 0, -1, NaN, Infinity]) {
  const state = stepClassicMotion(
    {
      ...empty(),
      x: NaN,
      vx: Infinity,
      yawRate: NaN,
      steering: -Infinity,
      rpm: NaN,
      gear: 999,
    },
    { steer: NaN, throttle: Infinity, brakeAmount: NaN },
    {},
    { grip: NaN, grade: Infinity },
    dt,
  );
  for (const key of numericFields)
    assert.ok(Number.isFinite(state[key]), `invalid ${key} dt=${dt}`);
}
for (const bad of [
  { mass: NaN },
  { lateralGrip: NaN },
  { redlineRPM: Infinity },
  { gearRatios: [NaN, 1, 1, 1, 1, 1] },
]) {
  const state = stepClassicMotion(
    empty(),
    { throttle: 1 },
    { ...classicVehicle("p911"), ...bad },
    {},
    0.02,
  );
  for (const key of numericFields)
    assert.ok(Number.isFinite(state[key]), `invalid vehicle ${key}`);
}
await writeFile(
  reportPath,
  JSON.stringify(
    { status: "passed", fixtures: fixtures.length, samples, maximumErrors },
    null,
    2,
  ) + "\n",
);
console.log(
  `Compared ${samples} fixed 20 ms Go/TS states across ${fixtures.length} scenarios.`,
);
