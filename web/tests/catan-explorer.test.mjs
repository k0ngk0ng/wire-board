import test from "node:test";
import assert from "node:assert/strict";
import {
  explorerChoices,
  explorerCanRespond,
  explorerSelectedAction,
  explorerDiscardAction,
  explorerActionKey,
  explorerTarget,
  explorerShipPosition,
  explorerContents,
  explorerActionDescription,
} from "../src/catan-explorer-state.ts";
import { catanColorIndex } from "../src/catan-player-colors.ts";
import { catanResultDescription } from "../src/catan-results.ts";
const room = () => ({
  id: "initial-voyage",
  status: "playing",
  you: 0,
  seats: [{}, {}],
  game: {
    turn: 0,
    phase: "catan_explorer_move",
    catan: {
      hexSize: 62,
      players: [{ resources: [2, 3, 4, 5, 6] }, {}],
      discardDue: [0, 0],
      vertices: [
        { x: 0, y: 0 },
        { x: 100, y: 0 },
        { x: 100, y: 100 },
      ],
      edges: [
        { a: 0, b: 1 },
        { a: 1, b: 2 },
      ],
      tiles: [],
      victoryTarget: 8,
      explorer: {
        sequence: 5,
        choices: [
          { type: "catan_explorer_sail", prompt: 5, slot: 0, targets: [1] },
        ],
        fleet: { positions: [0, -1, -1, 0, -1, -1], turn: { current: -1 } },
        cargo: {
          units: [
            { kind: "ship", index: 0 },
            { kind: "harbor", index: 0 },
          ],
        },
      },
    },
  },
});
test("only present humans can act; observers and autoplay have no controls", () => {
  for (const change of [
    (r) => (r.spectating = true),
    (r) => (r.you = -1),
    (r) => (r.seats[0].autoPlay = true),
    (r) => (r.game.catan.players[0].eliminated = true),
    (r) => (r.game.finished = true),
    (r) => (r.status = "closed"),
  ]) {
    const r = room();
    change(r);
    assert.equal(explorerCanRespond(r), false);
    assert.deepEqual(explorerChoices(r), []);
  }
  const r = room();
  r.game.turn = 1;
  assert.deepEqual(explorerChoices(r), []);
  assert.equal(explorerCanRespond(r), true);
});
test("confirmation revalidates room, prompt, route and live legal choices", () => {
  const r = room(),
    a = r.game.catan.explorer.choices[0],
    pick = { room: r.id, action: structuredClone(a) };
  assert.deepEqual(explorerSelectedAction(r, pick), a);
  assert.equal(explorerSelectedAction({ ...r, id: "different" }, pick), null);
  r.game.catan.explorer.sequence++;
  assert.equal(explorerSelectedAction(r, pick), null);
  r.game.catan.explorer.sequence--;
  r.game.catan.explorer.choices[0].targets = [0, 1];
  assert.equal(explorerSelectedAction(r, pick), null);
  assert.equal(
    explorerActionKey({ type: "catan_road", edge: 0, prompt: 5 }),
    explorerActionKey({ prompt: 5, edge: 0, type: "catan_road" }),
  );
});
test("parallel discard works off turn, checks exact five resources, and carries prompt", () => {
  const r = room();
  r.game.turn = 1;
  r.game.phase = "catan_discard";
  r.game.catan.discardDue[0] = 3;
  assert.deepEqual(explorerDiscardAction(r, [1, 2, 0, 0, 0]), {
    type: "catan_discard",
    prompt: 5,
    tokens: [1, 2, 0, 0, 0],
  });
  for (const bad of [
    [3, 0, 0, 0, 0],
    [0, 0, 0, 0, 0],
    [1.5, 1.5, 0, 0, 0],
    [-1, 4, 0, 0, 0],
    [1, 2],
  ])
    assert.equal(explorerDiscardAction(r, bad), null);
  r.seats[0].autoPlay = true;
  assert.equal(explorerDiscardAction(r, [1, 2, 0, 0, 0]), null);
});
test("same-edge ships have distinct perpendicular offsets; supplies stay off map", () => {
  const g = room().game.catan;
  assert.deepEqual(explorerShipPosition(g, 0), { x: 50, y: -11 });
  assert.deepEqual(explorerShipPosition(g, 3), { x: 50, y: 11 });
  assert.equal(explorerShipPosition(g, 1), null);
  assert.deepEqual(explorerContents(g, "ship", 0), [0]);
  assert.deepEqual(explorerContents(g, "harbor", 0), [1]);
});
test("map targets retain zero IDs and refer to the actual ship or harbor", () => {
  const g = room().game.catan;
  assert.deepEqual(
    explorerTarget(g, {
      type: "catan_explorer_unit",
      choice: "ship",
      target: 0,
    }),
    { kind: "edge", id: 0 },
  );
  assert.deepEqual(
    explorerTarget(g, {
      type: "catan_explorer_unit",
      choice: "harbor",
      target: 0,
    }),
    { kind: "vertex", id: 0 },
  );
  assert.deepEqual(
    explorerTarget(g, { type: "catan_explorer_sail", targets: [0, 1] }),
    { kind: "edge", id: 1 },
  );
  assert.equal(
    explorerTarget(g, { type: "catan_explorer_bank", color: 0, target: 1 }),
    null,
  );
});
test("destructive rebuilding, full cargo replacement and switching ships are explicit", () => {
  const g = room().game.catan;
  assert.match(
    explorerActionDescription(g, { type: "catan_explorer_ship", slot: 0 }),
    /拆回原船及全部货物/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_unit",
      choice: "harbor",
      target: 0,
      cards: [0],
    }),
    /归还1枚移民/,
  );
  g.explorer.fleet.turn.current = 1;
  g.tiles = [{ resource: 8, vertices: [2] }];
  const text = explorerActionDescription(g, {
    type: "catan_explorer_sail",
    slot: 0,
    targets: [1],
  });
  assert.match(text, /发现迷雾后本船停止/);
  assert.match(text, /上一艘船不能继续/);
  assert.match(catanResultDescription(g), /村庄1分、港口2分/);
  assert.equal(catanColorIndex(g, -2), 2);
  assert.equal(catanColorIndex(g, -3), 3);
});

import {
  explorerMotionBetween,
  explorerMotionPath,
} from "../src/catan-explorer-state.ts";
test("motion requires consecutive action and room versions, but accepts opponents and spectators", () => {
  const a = room();
  a.version = 8;
  a.game.catan.explorer.actionId = 3;
  const b = structuredClone(a);
  b.version++;
  b.game.catan.explorer.actionId = 4;
  b.game.catan.explorer.motion = {
    id: 4,
    player: 1,
    kind: "catan_explorer_sail",
    ship: 3,
    path: [0, 1],
    vertex: -1,
  };
  assert.equal(explorerMotionBetween(a, b)?.player, 1);
  a.spectating = b.spectating = true;
  a.you = b.you = -1;
  assert.equal(explorerMotionBetween(a, b)?.player, 1);
  assert.equal(explorerMotionBetween(b, b), null);
  for (const change of [
    (r) => r.version++,
    (r) => r.version--,
    (r) => (r.id = "new"),
    (r) => (r.you = 0),
    (r) => (r.spectating = false),
    (r) => (r.status = "closed"),
    (r) => r.game.catan.explorer.actionId++,
    (r) => r.game.catan.explorer.motion.id--,
    (r) => (r.game.catan.explorer.motion = null),
  ]) {
    const invalid = structuredClone(b);
    change(invalid);
    assert.equal(explorerMotionBetween(a, invalid), null);
  }
  b.status = "finished";
  assert.ok(explorerMotionBetween(a, b));
});
test("sailing uses all actual path edges and shared-edge offsets, never a guessed route", () => {
  const before = room().game.catan,
    after = structuredClone(before);
  after.explorer.fleet.positions[0] = 1;
  const path = explorerMotionPath(before, after, {
    ship: 0,
    path: [0, 1, 0, 1],
  });
  assert.deepEqual(path, [
    { x: 50, y: -11 },
    { x: 100, y: 50 },
    { x: 50, y: 0 },
    { x: 100, y: 50 },
  ]);
  assert.deepEqual(
    explorerMotionPath(before, after, { ship: 0, path: [0, 99, 1] }),
    [],
  );
});

import { explorerPathPoint } from "../src/catan-explorer-state.ts";
test("motion progresses continuously by distance through bends, including zero-length edges", () => {
  const points = [
    { x: 0, y: 0 },
    { x: 10, y: 0 },
    { x: 10, y: 30 },
  ];
  assert.deepEqual(explorerPathPoint(points, 0.125), { x: 5, y: 0 });
  assert.deepEqual(explorerPathPoint(points, 0.5), { x: 10, y: 10 });
  assert.deepEqual(explorerPathPoint(points, 1), points[2]);
  assert.deepEqual(explorerPathPoint([points[0], points[0], points[1]], 0.5), {
    x: 5,
    y: 0,
  });
  assert.deepEqual(explorerPathPoint([points[0], points[0]], 1), points[0]);
});

test("mission targets and descriptions distinguish setup, crew and mandatory responses", () => {
  const g = room().game.catan;
  g.explorer.setupPlacement = { player: 0, owner: -2, kind: "harbor" };
  for (const choice of ["harbor", "settlement", "road", "ship"]) {
    const a = { type: "catan_explorer_setup", prompt: 1, target: 0, choice };
    assert.deepEqual(explorerTarget(g, a), {
      kind: ["harbor", "settlement"].includes(choice) ? "vertex" : "edge",
      id: 0,
    });
    assert.match(explorerActionDescription(g, a), /中立方/);
  }
  for (const type of [
    "catan_explorer_pirate_place",
    "catan_explorer_land",
    "catan_explorer_pickup",
    "catan_explorer_resolve",
    "catan_explorer_battle",
  ]) {
    const a = { type, prompt: 5, target: 0, slot: 0, cards: [2, 3] };
    assert.deepEqual(explorerTarget(g, a), { kind: "tile", id: 0 });
    assert.notEqual(explorerActionDescription(g, a), "掷骰并按点数生产资源。");
  }
  assert.deepEqual(
    explorerTarget(g, { type: "catan_explorer_chase", prompt: 5, target: 0 }),
    { kind: "edge", id: 0 },
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_unit",
      prompt: 5,
      card: 2,
      choice: "ship",
      target: 0,
    }),
    /1羊毛、1矿石.*船员1，占1格/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_transfer",
      prompt: 5,
      slot: 0,
      vertex: 0,
      give: [0],
      take: [2, 3],
    }),
    /装入1枚移民并卸下2名船员/,
  );
});

test("same-actor setup advances its own prompt and invalidates the prior confirmation", () => {
  const r = room();
  r.game.phase = "catan_explorer_setup";
  const x = r.game.catan.explorer;
  x.sequence = 7;
  x.choices = [
    { type: "catan_explorer_setup", prompt: 7, choice: "road", target: 0 },
  ];
  const pick = { room: r.id, action: structuredClone(x.choices[0]) };
  assert.deepEqual(explorerSelectedAction(r, pick), x.choices[0]);
  x.sequence = 8;
  x.choices = [
    { type: "catan_explorer_setup", prompt: 8, choice: "ship", target: 0 },
  ];
  assert.equal(explorerSelectedAction(r, pick), null);
  assert.equal(explorerChoices(r)[0].choice, "ship");
});

test("cargo flights anchor each unit to its actual map row without using viewport coordinates", async () => {
  const { explorerCargoPoint } = await import("../src/catan-explorer-state.ts");
  const g = room().game.catan;
  g.tiles = [{ x: 300, y: 400 }];
  g.explorer.cargo.units = Array.from({ length: 15 }, () => ({
    kind: "supply",
    index: -1,
  }));
  g.explorer.cargo.units[2] = { kind: "ship", index: 0 };
  g.explorer.cargo.units[3] = { kind: "ship", index: 0 };
  assert.deepEqual(explorerCargoPoint(g, 2, { kind: "ship", index: 0 }), {
    x: 42,
    y: -23,
  });
  assert.deepEqual(explorerCargoPoint(g, 3, { kind: "ship", index: 0 }), {
    x: 58,
    y: -23,
  });
  g.explorer.cargo.units[2] = { kind: "lair", index: 0 };
  g.explorer.cargo.units[3] = { kind: "lair", index: 0 };
  g.explorer.cargo.units[13] = { kind: "lair", index: 0 };
  assert.deepEqual(explorerCargoPoint(g, 2, { kind: "lair", index: 0 }), {
    x: 282,
    y: 434,
  });
  assert.deepEqual(explorerCargoPoint(g, 3, { kind: "lair", index: 0 }), {
    x: 300,
    y: 434,
  });
  assert.deepEqual(explorerCargoPoint(g, 13, { kind: "lair", index: 0 }), {
    x: 318,
    y: 434,
  });
  assert.equal(explorerCargoPoint(g, 4, { kind: "lair", index: 0 }), null);
  assert.equal(explorerCargoPoint(g, 2, { kind: "supply", index: -1 }), null);
});

test("fish zero ID targets its public shoal and mixed cargo keeps distinct IDs", async () => {
  const { explorerFishContents, explorerFreightLabel, explorerFishPoint } =
    await import("../src/catan-explorer-state.ts");
  const g = room().game.catan;
  g.tiles = [{ x: 40, y: 30 }];
  g.explorer.board = {
    scenario: "fish-for-catan",
    council: { tile: 0, anchors: [0, 2] },
  };
  g.explorer.fish = { progress: [1, 0], scores: [2, 0], leader: 0 };
  g.explorer.cargo.fish = [
    { kind: "shoal", index: 0 },
    { kind: "ship", index: 3 },
  ];
  assert.deepEqual(
    explorerTarget(g, { type: "catan_explorer_fish_load", card: 0, slot: 0 }),
    { kind: "tile", id: 0 },
  );
  assert.deepEqual(
    explorerTarget(g, {
      type: "catan_explorer_fish_deliver",
      card: 1,
      slot: 3,
    }),
    { kind: "tile", id: 0 },
  );
  assert.deepEqual(explorerFishContents(g, "ship", 3), [1]);
  assert.deepEqual(explorerFishPoint(g, 1), { x: 50, y: -1 });
  assert.equal(explorerFreightLabel([], [0]), "1群鱼");
  const description = explorerActionDescription(g, {
    type: "catan_explorer_transfer",
    slot: 0,
    vertex: 0,
    cards: [0],
    take: [0],
  });
  assert.match(description, /装入1群鱼并卸下1枚移民/);
  g.explorer.cargo.fish[0] = { kind: "supply", index: -1 };
  assert.equal(
    explorerTarget(g, { type: "catan_explorer_fish_load", card: 0 }),
    null,
  );
  assert.equal(explorerFishPoint(g, 0), null);
  assert.match(catanResultDescription(g), /鱼群任务.*鱼群任务进度/);
});

test("fish confirmations expire when the fish is collected or the roll is consumed", () => {
  const r = room();
  for (const action of [
    { type: "catan_explorer_fish_load", prompt: 5, slot: 0, card: 0 },
    { type: "catan_explorer_fish_roll", prompt: 5 },
  ]) {
    r.game.catan.explorer.choices = [action];
    const pick = { room: r.id, action };
    assert.deepEqual(explorerSelectedAction(r, pick), action);
    r.game.catan.explorer.choices = [];
    assert.equal(explorerSelectedAction(r, pick), null);
  }
});

test("fish flights use real berths, survive map scrolling and distinguish delivery from removal", async () => {
  const { explorerFishFlight, explorerFishPoint } =
    await import("../src/catan-explorer-state.ts");
  const before = room().game.catan;
  before.tiles = [
    { x: 40, y: 30 },
    { x: 180, y: 220 },
  ];
  before.explorer.board = { council: { tile: 1, anchors: [0, 2] } };
  before.explorer.cargo.fish = [{ kind: "shoal", index: 0 }];
  const after = structuredClone(before);
  after.explorer.cargo.fish[0] = { kind: "ship", index: 3 };
  const load = {
    kind: "catan_explorer_fish_load",
    fish: [
      {
        fish: 0,
        from: before.explorer.cargo.fish[0],
        to: after.explorer.cargo.fish[0],
      },
    ],
  };
  const f = explorerFishFlight(before, after, load, 0);
  assert.deepEqual(f.points, [{ x: 40, y: 50 }, explorerFishPoint(after, 0)]);
  assert.equal(f.retire, false);
  assert.equal(f.appear, false);
  // Return supply on delivery flies to the council; pirate/removal fades locally.
  const supply = structuredClone(after);
  supply.explorer.cargo.fish[0] = { kind: "supply", index: -1 };
  const retire = {
    ...load,
    kind: "catan_explorer_fish_deliver",
    fish: [
      {
        fish: 0,
        from: after.explorer.cargo.fish[0],
        to: supply.explorer.cargo.fish[0],
      },
    ],
  };
  assert.deepEqual(explorerFishFlight(after, supply, retire, 0).points[1], {
    x: 180,
    y: 220,
  });
  assert.equal(
    explorerFishFlight(
      after,
      supply,
      { ...retire, kind: "catan_explorer_ship" },
      0,
    ).delivered,
    false,
  );
  const spawn = {
    kind: "catan_explorer_fish_roll",
    fish: [
      {
        fish: 0,
        from: supply.explorer.cargo.fish[0],
        to: before.explorer.cargo.fish[0],
      },
    ],
  };
  assert.deepEqual(explorerFishFlight(supply, before, spawn, 0), {
    points: [
      { x: 40, y: 32 },
      { x: 40, y: 50 },
    ],
    appear: true,
    retire: false,
    delivered: false,
    width: 34,
  });
  assert.equal(explorerFishFlight(supply, before, spawn, 1), null);
});
