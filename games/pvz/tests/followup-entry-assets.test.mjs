import test from "node:test";
import assert from "node:assert/strict";
import { neededUnits } from "../web/entry-assets.ts";

test("opening does not preload an unavailable boss and its entire army", () => {
  const view = {
    tick: 0,
    players: [{ deck: ["zombie", "zomboss"], imitaterKind: "peashooter" }],
    plants: [],
    zombies: [],
    events: [],
    catalog: {
      plants: [],
      zombies: [
        { kind: "zombie" },
        { kind: "zomboss", unlockAt: 2400 },
        { kind: "gargantuar" },
      ],
    },
  };
  const opening = neededUnits(view);
  assert.ok(opening.includes("zombie"));
  assert.ok(!opening.includes("zomboss"));
  assert.ok(!opening.includes("gargantuar"));
  assert.ok(!opening.includes("peashooter"));
  const later = neededUnits({ ...view, tick: 2200 });
  assert.ok(later.includes("zomboss"));
  assert.ok(later.includes("gargantuar"));
});
test("visible actors and their attachments always load irrespective of staged card availability", () => {
  const view = {
    tick: 0,
    players: [],
    plants: [{ kind: "imitater", targetKind: "tallnut" }],
    zombies: [{ kind: "gargantuar" }],
    events: [],
    catalog: { plants: [], zombies: [] },
  };
  const kinds = neededUnits(view);
  for (const name of ["imitater", "tallnut", "gargantuar", "imp"])
    assert.ok(kinds.includes(name));
});
