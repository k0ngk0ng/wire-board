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
  explorerScenarioLabel,
  explorerLairTotal,
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
test("full mission labels and lair total distinguish it from the fish scenario", () => {
  const g = room().game.catan;
  g.explorer.board = { scenario: "fish-for-catan", target: 15 };
  g.explorer.lairs = {};
  g.explorer.fish = {};
  assert.equal(explorerLairTotal(g), 5);
  assert.equal(explorerScenarioLabel(g), "鱼群任务");
  g.explorer.spice = {};
  g.explorer.board = { scenario: "explorers-and-pirates", target: 17 };
  g.victoryTarget = 17;
  assert.equal(explorerLairTotal(g), 6);
  assert.equal(explorerScenarioLabel(g), "完整三任务");
  const result = catanResultDescription(g);
  for (const text of ["完整三任务", "17分", "巢穴", "香料", "鱼群"])
    assert.ok(result.includes(text));
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
test("combined city responses follow the server actor without giving the turn owner their choices", () => {
  const r = room(),
    x = r.game.catan.explorer;
  r.game.turn = 1;
  r.game.phase = "catan_pillage";
  x.canRespond = true;
  x.actor = 0;
  x.choices = [{ type: "catan_pillage", vertex: 0, prompt: 5 }];
  assert.deepEqual(explorerChoices(r), x.choices);
  x.actor = 1;
  assert.deepEqual(explorerChoices(r), []);
  x.actor = 0;
  x.canRespond = false;
  assert.deepEqual(explorerChoices(r), []);
  x.canRespond = true;
  x.sequence++;
  assert.deepEqual(explorerChoices(r), []);
});
test("combined seven discards accept exactly the eight-card hand shape, including commodities", () => {
  const r = room();
  r.game.turn = 1;
  r.game.phase = "catan_discard";
  r.game.catan.players[0].resources = [0, 0, 0, 0, 0, 0, 0, 8];
  r.game.catan.discardDue[0] = 4;
  assert.deepEqual(explorerDiscardAction(r, [0, 0, 0, 0, 0, 0, 0, 4]), {
    type: "catan_discard",
    prompt: 5,
    tokens: [0, 0, 0, 0, 0, 0, 0, 4],
  });
  assert.equal(explorerDiscardAction(r, [0, 0, 0, 0, 4]), null);
  assert.equal(explorerDiscardAction(r, [0, 0, 0, 0, 0, 4, 0, 0]), null);
});
test("city placement and knight moves target vertices; skipped replies never highlight vertex or edge zero", () => {
  const g = room().game.catan;
  assert.deepEqual(
    explorerTarget(g, {
      type: "catan_explorer_setup",
      choice: "city",
      target: 0,
    }),
    { kind: "vertex", id: 0 },
  );
  assert.deepEqual(
    explorerTarget(g, { type: "catan_knight_move", vertex: 1, target: 2 }),
    { kind: "vertex", id: 2 },
  );
  assert.equal(
    explorerTarget(g, {
      type: "catan_treason_place",
      vertex: 0,
      color: 0,
      choice: "skip",
    }),
    null,
  );
  assert.equal(
    explorerTarget(g, { type: "catan_diplomacy", edge: 0, choice: "skip" }),
    null,
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

test("spice confirmations keep cargo identities, farm bonuses and six-step mission", async () => {
  const { explorerFarmAbilities, explorerScenarioLabel, explorerFreightLabel } =
    await import("../src/catan-explorer-state.ts");
  const r = room(),
    g = r.game.catan;
  g.explorer.board = {
    council: { tile: 0 },
    farms: [
      { tile: 0, ability: "swift" },
      { tile: 1, ability: "gold" },
      { tile: 2, ability: "pirate", pirateDie: 4 },
    ],
  };
  g.explorer.spice = { progress: [1, 0], scores: [2, 0], leader: 0 };
  g.explorer.cargo.spice = [
    { origin: 0, owner: 0, at: { kind: "supply", index: -1 } },
    { origin: 1, owner: 0, at: { kind: "harbor", index: 0 } },
    { origin: 2, owner: 0, at: { kind: "ship", index: 0 } },
  ];
  assert.deepEqual(explorerFarmAbilities(g, 0), {
    swift: 1,
    gold: 1,
    pirate: [4],
    count: 3,
  });
  assert.equal(explorerScenarioLabel(g), "香料与鱼群");
  const a = {
    type: "catan_explorer_spice_land",
    prompt: 5,
    target: 0,
    slot: 0,
    card: 2,
  };
  assert.deepEqual(explorerTarget(g, a), { kind: "tile", id: 0 });
  assert.match(explorerActionDescription(g, a), /永久派驻.*不能召回.*航速＋1/);
  assert.match(
    explorerActionDescription(g, { type: "catan_explorer_chase", target: 0 }),
    /4、6点成功/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_spice_gold",
      card: 0,
    }),
    /支付1木材.*独立/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_transfer",
      slot: 0,
      vertex: 0,
      spiceLoad: [0, 1],
      take: [0],
    }),
    /装入2袋香料并卸下1枚移民/,
  );
  assert.equal(explorerFreightLabel([2], [], [0]), "1名船员、1袋香料");
  r.game.catan.explorer.choices = [a];
  assert.ok(explorerSelectedAction(r, { room: r.id, action: a }));
  r.game.catan.explorer.choices = [];
  assert.equal(explorerSelectedAction(r, { room: r.id, action: a }), null);
  assert.match(catanResultDescription(g), /香料任务进度/);
});

test("mixed spice and crew use separate map slots, including sailing and permanent farms", async () => {
  const { explorerCargoPoint, explorerSpicePoint, explorerSpiceFlight } =
    await import("../src/catan-explorer-state.ts");
  const before = room().game.catan;
  before.tiles = [
    { x: 300, y: 400 },
    { x: 500, y: 500 },
  ];
  before.explorer.board = { council: { tile: 1 } };
  before.explorer.cargo.units = [{ kind: "ship", index: 0 }];
  before.explorer.cargo.spice = [
    { origin: 0, owner: 0, at: { kind: "ship", index: 0 } },
  ];
  assert.deepEqual(explorerCargoPoint(before, 0, { kind: "ship", index: 0 }), {
    x: 42,
    y: -23,
  });
  assert.deepEqual(explorerSpicePoint(before, 0), { x: 58, y: -23 });
  const after = structuredClone(before);
  after.explorer.cargo.spice[0].at = { kind: "supply", index: -1 };
  const motion = {
    kind: "catan_explorer_spice_deliver",
    spice: [
      {
        sack: 0,
        from: { kind: "ship", index: 0 },
        to: { kind: "supply", index: -1 },
      },
    ],
  };
  assert.deepEqual(explorerSpiceFlight(before, after, motion, 0), {
    points: [
      { x: 58, y: -23 },
      { x: 500, y: 500 },
    ],
    appear: false,
    retire: true,
    delivered: true,
  });
  assert.equal(
    explorerSpiceFlight(
      before,
      after,
      { ...motion, kind: "catan_explorer_ship" },
      0,
    ).delivered,
    false,
  );
  after.explorer.cargo.units[0] = { kind: "farm", index: 0 };
  assert.deepEqual(explorerCargoPoint(after, 0, { kind: "farm", index: 0 }), {
    x: 300,
    y: 434,
  });
  const spawn = structuredClone(after);
  spawn.explorer.cargo.spice[0].at = { kind: "farm", index: 0 };
  assert.equal(
    explorerSpiceFlight(
      after,
      spawn,
      { ...motion, kind: "catan_explorer_sail" },
      0,
    ).appear,
    true,
  );
  assert.equal(explorerSpicePoint(after, 0), null);
});

test("expanded mission counts and trade limits retain the opening player count", async () => {
  const { explorerSupplyLimits } =
    await import("../src/catan-explorer-state.ts");
  for (const n of [2, 3, 4, 5, 6]) {
    const g = room().game.catan;
    g.players = Array.from({ length: n }, () => ({ eliminated: true }));
    g.players[0].eliminated = false;
    g.explorer.board = { scenario: "fish-for-catan" };
    assert.equal(explorerLairTotal(g), n > 4 ? 8 : 5);
    g.explorer.board.scenario = "explorers-and-pirates";
    assert.equal(explorerLairTotal(g), n > 4 ? 8 : 6);
    assert.deepEqual(
      explorerSupplyLimits(g),
      n > 4 ? { resource: 24, gold: 172 } : { resource: 19, gold: 148 },
    );
  }
});
test("second paired player cannot open domestic trades while first player can", async () => {
  const { explorerCanOffer } = await import("../src/catan-explorer-state.ts");
  const r = room();
  r.game.phase = "catan_turn";
  assert.equal(explorerCanOffer(r), true);
  r.game.catan.paired = { primary: 0, secondary: 1, second: false };
  assert.equal(explorerCanOffer(r), true);
  r.game.catan.paired.second = true;
  r.game.turn = r.you = 1;
  assert.equal(explorerCanOffer(r), false);
  r.game.catan.paired.second = false;
  assert.equal(explorerCanOffer({ ...r, spectating: true }), false);
  r.seats[1].autoPlay = true;
  assert.equal(explorerCanOffer(r), false);
});

test("recruitment explicitly confirms fish/spice returns and retires them without delivery", async () => {
  const { explorerFishFlight, explorerSpiceFlight } =
    await import("../src/catan-explorer-state.ts");
  const r = room();
  r.game.phase = "catan_turn";
  const g = r.game.catan;
  const base = {
    type: "catan_explorer_unit",
    prompt: 5,
    card: 2,
    choice: "ship",
    target: 0,
  };
  const fish = { ...base, targets: [0] };
  const spice = { ...base, spiceUnload: [0] };
  g.explorer.choices = [fish, spice];
  assert.notEqual(explorerActionKey(fish), explorerActionKey(spice));
  assert.deepEqual(explorerTarget(g, fish), { kind: "edge", id: 0 });
  assert.match(
    explorerActionDescription(g, fish),
    /先归还1群鱼，不推进鱼群任务/,
  );
  assert.match(
    explorerActionDescription(g, spice),
    /先归还1袋香料，不推进香料任务.*能力保留.*不能再次领取/,
  );
  for (const kind of ["fish", "spice"]) {
    const before = structuredClone(g);
    before.explorer.board = { council: { tile: 0 } };
    before.tiles = [{ x: 500, y: 500 }];
    before.explorer.cargo.units =
      kind === "fish"
        ? [
            { kind: "supply", index: -1 },
            { kind: "harbor", index: 0 },
          ]
        : [
            { kind: "supply", index: -1 },
            { kind: "harbor", index: 0 },
            { kind: "ship", index: 0 },
          ];
    before.explorer.cargo.fish =
      kind === "fish" ? [{ kind: "ship", index: 0 }] : [];
    before.explorer.cargo.spice =
      kind === "spice"
        ? [{ owner: 0, origin: 1, at: { kind: "ship", index: 0 } }]
        : [];
    const after = structuredClone(before);
    const motion = { kind: "catan_explorer_unit" };
    let flight;
    if (kind === "fish") {
      after.explorer.cargo.fish[0] = { kind: "supply", index: -1 };
      motion.fish = [
        {
          fish: 0,
          from: before.explorer.cargo.fish[0],
          to: after.explorer.cargo.fish[0],
        },
      ];
      flight = explorerFishFlight(before, after, motion, 0);
    } else {
      after.explorer.cargo.spice[0].at = { kind: "supply", index: -1 };
      motion.spice = [
        {
          sack: 0,
          from: before.explorer.cargo.spice[0].at,
          to: after.explorer.cargo.spice[0].at,
        },
      ];
      flight = explorerSpiceFlight(before, after, motion, 0);
    }
    assert.equal(flight.retire, true);
    assert.equal(flight.appear, false);
    assert.equal(flight.delivered, false);
    assert.deepEqual(flight.points[1], {
      x: flight.points[0].x,
      y: flight.points[0].y - 24,
    });
  }
});

test("combined bank descriptions use the responding player's rates and all eight card names", () => {
  const g = room().game.catan;
  g.explorer.actor = 1;
  g.players[1].rates = [3, 3, 3, 3, 3, 2, 4, 4];
  assert.equal(
    explorerActionDescription(g, {
      type: "catan_explorer_bank",
      color: 5,
      target: 7,
    }),
    "支付2纸张，领取1钱币。",
  );
  assert.equal(
    explorerActionDescription(g, {
      type: "catan_explorer_bank",
      color: 6,
      target: 3,
    }),
    "支付4布料，领取1粮食。",
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_bank",
      color: -1,
      target: 2,
    }),
    /2金币.*羊毛/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_explorer_bank",
      color: 0,
      target: -1,
    }),
    /支付3木材.*金币/,
  );
});

test("free roads and city responses explain costs, losses and optional continuations", () => {
  const g = room().game.catan;
  g.freeRoads = 2;
  assert.match(
    explorerActionDescription(g, { type: "catan_road", edge: 0 }),
    /免费.*不支付/,
  );
  g.freeRoads = 0;
  assert.match(
    explorerActionDescription(g, { type: "catan_road", edge: 0 }),
    /支付1木材、1砖块/,
  );
  assert.match(
    explorerActionDescription(g, { type: "catan_pillage", vertex: 0 }),
    /横置.*失去1分.*港口不受/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_treason_place",
      choice: "skip",
    }),
    /放弃/,
  );
  assert.match(
    explorerActionDescription(g, {
      type: "catan_knight_move",
      vertex: 0,
      target: 2,
    }),
    /交点1移到3/,
  );
});

test("combined progress instructions distinguish harbor medicine and pirate taxation from base rules", async () => {
  const { catanProgressDescription } =
    await import("../src/catan-progress-names.ts");
  const { explorerPhaseLabel } = await import("../src/catan-explorer-state.ts");
  assert.match(
    catanProgressDescription(5, false, true),
    /城市（1粮食＋2矿石）.*港口（1粮食＋1矿石）/,
  );
  assert.match(
    catanProgressDescription(21, false, true),
    /探险海盗.*返回建设阶段/,
  );
  assert.doesNotMatch(
    catanProgressDescription(21, false, true),
    /每位相邻建筑/,
  );
  assert.match(catanProgressDescription(21, false), /每位相邻建筑/);
  assert.match(
    catanProgressDescription(21, false, true, false, true),
    /本站初航.*不产生移动或偷牌效果/,
  );
  assert.match(catanProgressDescription(7, false, true), /两条道路/);
  assert.doesNotMatch(catanProgressDescription(7, false, true), /船只/);
  assert.match(catanProgressDescription(7, true), /船只/);
  assert.match(explorerPhaseLabel("catan_progress_end"), /航行前/);
  assert.match(explorerPhaseLabel("catan_commercial_harbor"), /商业港/);
});

test("two-piece recruitment confirms exact cargo and keeps discard choices distinct", async () => {
  const { explorerActionOptionLabel } =
    await import("../src/catan-explorer-state.ts");
  const g = room().game.catan;
  const base = {
    type: "catan_explorer_unit",
    prompt: 5,
    card: 1,
    choice: "harbor",
    target: 0,
  };
  const crew = { ...base, cards: [2, 3] },
    mixed = { ...base, cards: [2], spiceUnload: [0] },
    spice = { ...base, spiceUnload: [0, 1] };
  assert.match(explorerActionDescription(g, crew), /归还2名船员/);
  assert.match(explorerActionDescription(g, mixed), /归还1袋香料/);
  assert.match(explorerActionDescription(g, spice), /归还2袋香料/);
  for (const a of [crew, mixed, spice]) {
    assert.match(explorerActionDescription(g, a), /本站补充规则/);
    assert.match(explorerActionDescription(g, a), /木、砖、羊、粮各1/);
    assert.match(explorerActionOptionLabel(g, a), /归还/);
  }
  assert.notEqual(explorerActionKey(crew), explorerActionKey(mixed));
  assert.notEqual(explorerActionKey(mixed), explorerActionKey(spice));
});

test("helper construction keeps its payment and personnel identity through map confirmation", () => {
  const r = room(),
    g = r.game.catan;
  const ordinary = { type: "catan_road", edge: 1, prompt: 5 };
  const helper = { ...ordinary, skill: "helper", tokens: [0, 1, 1, 0, 0] };
  const otherPayment = { ...helper, tokens: [1, 0, 0, 1, 0] };
  g.explorer.choices = [ordinary, helper, otherPayment];
  assert.notEqual(explorerActionKey(ordinary), explorerActionKey(helper));
  assert.notEqual(explorerActionKey(helper), explorerActionKey(otherPayment));
  assert.deepEqual(
    explorerSelectedAction(r, { room: r.id, action: helper }),
    helper,
  );
  g.explorer.choices = [ordinary, otherPayment];
  assert.equal(explorerSelectedAction(r, { room: r.id, action: helper }), null);
  assert.match(explorerActionDescription(g, helper), /砖块×1、羊毛×1/);
  const ship = {
    type: "catan_explorer_ship",
    skill: "helper",
    tokens: [0, 0, 1, 1, 0],
    card: 0,
    slot: 0,
    edge: 1,
    prompt: 5,
  };
  assert.match(explorerActionDescription(g, ship), /拆回原船及全部货物/);
  assert.doesNotMatch(explorerActionDescription(g, ship), /归还移民/);
  const build = {
    type: "catan_settlement",
    skill: "helper",
    card: 1,
    vertex: 2,
    prompt: 5,
  };
  assert.match(
    explorerActionDescription(g, build),
    /归还.*港口1.*木材×1、砖块×1/,
  );
  assert.deepEqual(explorerTarget(g, build), { kind: "vertex", id: 2 });
  assert.deepEqual(
    explorerTarget(g, { type: "catan_helper", edge: 0, target: 1, prompt: 5 }),
    { kind: "edge", id: 1 },
  );
});

test("helper response belongs to the responder and cannot retain a stale private choice", () => {
  const r = room(),
    x = r.game.catan.explorer;
  r.game.phase = "catan_helper";
  r.game.turn = 1;
  x.actor = 0;
  x.canRespond = true;
  const choice = { type: "catan_helper_choice", prompt: 5, color: 3 };
  x.choices = [choice];
  assert.deepEqual(explorerChoices(r), [choice]);
  const pick = { room: r.id, action: choice };
  x.choices = [
    { type: "catan_helper_choice", prompt: 5, choice: "exchange", card: 7 },
  ];
  assert.equal(explorerSelectedAction(r, pick), null);
  x.actor = 1;
  assert.deepEqual(explorerChoices(r), []);
  x.actor = 0;
  r.seats[0].autoPlay = true;
  assert.deepEqual(explorerChoices(r), []);
});
