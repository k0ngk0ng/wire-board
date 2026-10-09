import test from "node:test";
import assert from "node:assert/strict";
import {
  transportCanAct,
  transportEdges,
  transportSelectedAction,
  transportArrivalMotion,
  transportWagonPosition,
} from "../src/catan-transport-state.ts";
import {
  catanSavedVictoryTarget,
  catanRuleContext,
} from "../src/catan-rule-context.ts";
const room = () => ({
  status: "playing",
  you: 0,
  seats: [{}],
  game: {
    turn: 0,
    phase: "catan_transport_move",
    catan: {
      players: [{}],
      transport: {
        canAct: true,
        barbarianPending: false,
        barbarianSequence: 9,
        state: { sequence: 14, travel: { pending: -1 } },
        choices: {
          steps: [{ edge: 3, mp: 2, toll: 1, pay: 1 }],
          relocate: [5, 6],
        },
      },
    },
  },
});

function arrivalRooms() {
  const before = room();
  before.id = "transport-animation";
  before.version = 40;
  before.game.catan.transport.map = {
    sites: [{ kind: "glassworks", center: 4, tile: 2 }],
  };
  before.game.catan.transport.state = {
    sequence: 14,
    arrivalResolved: false,
    travel: { player: 0, ended: true, arrived: 0 },
    supply: [10, 11, 12],
    wagons: [
      {
        position: 4,
        level: 2,
        delivered: 1,
        gold: 5,
        cargo: { id: 7, cargo: "sand", origin: "quarry" },
      },
    ],
  };
  const after = structuredClone(before);
  after.version++;
  after.game.phase = "catan_roll";
  after.game.turn = 1;
  const state = after.game.catan.transport.state;
  delete state.travel;
  state.supply[1]--;
  state.wagons[0] = {
    position: 4,
    level: 2,
    delivered: 2,
    gold: 8,
    cargo: { id: 13, cargo: "glass", origin: "glassworks" },
  };
  return { before, after };
}
test("cargo shares the wagon's map anchor, including overlapping players and map scale", () => {
  const game = {
    hexSize: 31,
    vertices: [{ x: 100, y: 200 }],
    transport: {
      state: { wagons: [{ position: 0 }, { position: -1 }, { position: 0 }] },
    },
  };
  assert.deepEqual(transportWagonPosition(game, 0), { x: 94.75, y: 200 });
  assert.deepEqual(transportWagonPosition(game, 2), { x: 105.25, y: 200 });
  assert.equal(transportWagonPosition(game, 1), null);
  game.transport.state.wagons[2].position = -1;
  assert.deepEqual(transportWagonPosition(game, 0), { x: 100, y: 200 });
});
test("arrival animates public cargo for actor, opponent and observer across turn handoff", () => {
  for (const [you, spectating] of [
    [0, false],
    [1, false],
    [-1, true],
  ]) {
    const { before, after } = arrivalRooms();
    before.you = after.you = you;
    before.spectating = after.spectating = spectating;
    assert.deepEqual(transportArrivalMotion(before, after), {
      id: "transport-animation:14:0",
      player: 0,
      site: 0,
      delivered: "sand",
      loaded: "glass",
      gold: 3,
    });
    assert.equal(transportArrivalMotion(after, after), null);
  }
});
test("arrival distinguishes first loading, keeping cargo, winning delivery and Swift Journey", () => {
  {
    const { before, after } = arrivalRooms();
    delete before.game.catan.transport.state.wagons[0].cargo;
    const wagon = after.game.catan.transport.state.wagons[0];
    wagon.delivered = 1;
    wagon.gold = 5;
    assert.equal(transportArrivalMotion(before, after).delivered, undefined);
    assert.equal(transportArrivalMotion(before, after).loaded, "glass");
  }
  {
    const { before, after } = arrivalRooms();
    after.game.catan.transport.state.wagons = structuredClone(
      before.game.catan.transport.state.wagons,
    );
    after.game.catan.transport.state.supply = [
      ...before.game.catan.transport.state.supply,
    ];
    assert.equal(transportArrivalMotion(before, after), null);
  }
  {
    const { before, after } = arrivalRooms();
    after.status = "finished";
    after.game.finished = true;
    delete after.game.catan.transport.state.wagons[0].cargo;
    after.game.catan.transport.state.supply = [
      ...before.game.catan.transport.state.supply,
    ];
    const event = transportArrivalMotion(before, after);
    assert.equal(event.delivered, "sand");
    assert.equal(event.loaded, undefined);
    assert.equal(event.gold, 3);
  }
  {
    const { before, after } = arrivalRooms();
    after.game.phase = "catan_transport_move";
    after.game.turn = 0;
    after.game.catan.transport.state.sequence++;
    assert.equal(transportArrivalMotion(before, after).loaded, "glass");
  }
});
test("arrival never replays a reconnect, changed viewer, reset or unrelated inventory change", () => {
  const mutations = [
    (b, a) => a.version++,
    (b, a) => (a.version = b.version),
    (b, a) => (a.id = "another"),
    (b, a) => (a.you = -1),
    (b, a) => (a.spectating = true),
    (b, a) => (a.status = "waiting"),
    (b) => (b.game.phase = "catan_action"),
    (b) => (b.game.catan.transport.state.arrivalResolved = true),
    (b) => (b.game.catan.transport.state.travel.ended = false),
    (b) => (b.game.catan.transport.state.travel.arrived = -1),
    (b, a) => (a.game.catan.transport.state.sequence += 2),
    (b, a) => a.game.catan.transport.state.wagons[0].position++,
    (b, a) => a.game.catan.transport.state.wagons[0].gold++,
    (b, a) => a.game.catan.transport.state.wagons[0].delivered++,
    (b, a) => (a.game.catan.transport.state.wagons[0].cargo.origin = "castle"),
    (b, a) => a.game.catan.transport.state.supply[0]--,
  ];
  for (const mutate of mutations) {
    const { before, after } = arrivalRooms();
    mutate(before, after);
    assert.equal(transportArrivalMotion(before, after), null, String(mutate));
  }
});
test("transport movement only uses server legal edges and correct persisted response sequence", () => {
  const r = room();
  assert.deepEqual(transportEdges(r), [3]);
  assert.deepEqual(transportSelectedAction(r, { edge: 3, piece: null }), {
    type: "catan_transport_step",
    offer: 14,
    edge: 3,
  });
  assert.equal(transportSelectedAction(r, { edge: 5, piece: null }), null);
  r.game.catan.transport.state.travel.pending = 2;
  assert.deepEqual(transportEdges(r), [5, 6]);
  assert.deepEqual(transportSelectedAction(r, { edge: 6, piece: null }), {
    type: "catan_transport_relocate",
    offer: 14,
    edge: 6,
  });
  r.game.catan.transport.barbarianPending = true;
  assert.equal(transportSelectedAction(r, { edge: 6, piece: null }), null);
  assert.equal(transportSelectedAction(r, { edge: 6, piece: 3 }), null);
  assert.deepEqual(transportSelectedAction(r, { edge: 5, piece: 1 }), {
    type: "catan_transport_barbarian",
    offer: 9,
    edge: 5,
    card: 1,
  });
});
test("observers, other seats, eliminated players, autoplay and finished rooms cannot operate transport", () => {
  for (const change of [
    { spectating: true },
    { you: -1 },
    { you: 1 },
    { status: "finished" },
    { seats: [{ autoPlay: true }] },
  ]) {
    const r = { ...room(), ...change };
    assert.equal(transportCanAct(r), false);
    assert.deepEqual(transportEdges(r), []);
    assert.equal(transportSelectedAction(r, { edge: 3, piece: null }), null);
  }
  const r = room();
  r.game.catan.players[0].eliminated = true;
  assert.equal(transportCanAct(r), false);
  r.game.catan.players[0].eliminated = false;
  r.game.finished = true;
  assert.equal(transportCanAct(r), false);
});
test("saved transport rules override ordinary ten-point fallback", () => {
  const r = room();
  assert.equal(catanSavedVictoryTarget(r.game.catan), 13);
  assert.equal(catanRuleContext(r).transport, true);
  assert.equal(catanRuleContext(r).target, 13);
});

test("movement quote names bank or opponent without inferring neutral rounding in the browser", async () => {
  const { transportStepDescription } =
    await import("../src/catan-transport-state.ts");
  const r = room();
  r.seats = [{ name: "自己" }, { name: "对手甲" }];
  r.game.catan.players = [{}, {}];
  const step = { edge: 5, mp: 1, toll: 1, pay: -1, bank: 1, neutral: true };
  assert.equal(
    transportStepDescription(r, step),
    "1移动点 · 1金币付给银行（中立道路）",
  );
  assert.equal(
    transportStepDescription(r, { ...step, pay: 1, bank: 0 }),
    "1移动点 · 1金币付给对手甲（中立道路）",
  );
  assert.equal(
    transportStepDescription(r, { ...step, pay: 1, bank: 0, neutral: false }),
    "1移动点 · 1金币付给对手甲",
  );
  r.game.catan.players[1].eliminated = true;
  assert.equal(
    transportStepDescription(r, { ...step, pay: 1, bank: 0, neutral: false }),
    "1移动点 · 1金币付给银行",
  );
  assert.equal(
    transportStepDescription(r, {
      ...step,
      mp: 2,
      toll: 0,
      pay: -1,
      bank: 0,
      neutral: false,
    }),
    "2移动点 · 无过路费",
  );
});

test("transport city knight chase uses server targets and clears stale choices", () => {
  const r = room();
  r.game.phase = "catan_turn";
  const t = r.game.catan.transport;
  t.choices.knightChases = [{ vertex: 7, barbarians: [1] }];
  t.choices.knightTargets = [5, 6];
  const pick = { knight: 7, piece: 1, edge: 6 };
  assert.deepEqual(transportEdges(r, pick), [5, 6]);
  assert.deepEqual(transportSelectedAction(r, pick), {
    type: "catan_transport_knight_chase",
    vertex: 7,
    card: 1,
    edge: 6,
  });
  assert.equal(transportSelectedAction(r, { ...pick, piece: 0 }), null);
  assert.equal(transportSelectedAction(r, { ...pick, edge: 9 }), null);
  t.choices.knightChases = [];
  assert.equal(transportSelectedAction(r, pick), null);
  r.spectating = true;
  assert.deepEqual(transportEdges(r, pick), []);
});

test("transport city draft and restored target stay fifteen, base stays thirteen", () => {
  assert.equal(
    catanRuleContext({
      kind: "catan",
      capacity: 3,
      catanScenario: "transport",
      catanCitiesKnights: {},
    }).target,
    15,
  );
  assert.equal(
    catanSavedVictoryTarget({ transport: {}, citiesKnights: {} }),
    15,
  );
  assert.equal(catanSavedVictoryTarget({ transport: {} }), 13);
});

test("neutral bridge toll shows both bank and opponent shares", async () => {
  const { transportStepDescription } =
    await import("../src/catan-transport-state.ts");
  const r = room();
  r.seats.push({ name: "对手甲" });
  assert.equal(
    transportStepDescription(r, {
      mp: 1,
      toll: 2,
      pay: 1,
      bank: 1,
      neutral: true,
    }),
    "1移动点 · 1金币给银行、1金币给对手甲（中立桥梁）",
  );
});

test("river transport depot wagons leave production numbers visible without moving ordinary wagons", () => {
  const g = {
    hexSize: 62,
    vertices: [{ x: 100, y: 200 }],
    rivers: { transport: "catan-rivers-transport-2025" },
    transport: {
      map: { sites: [{ center: 0 }] },
      state: { wagons: [{ position: 0 }] },
    },
  };
  assert.deepEqual(transportWagonPosition(g, 0), { x: 100, y: 252 });
  delete g.rivers;
  assert.deepEqual(transportWagonPosition(g, 0), { x: 100, y: 200 });
});

test("shared barbarian relocation requires a legal hex and its matching edge", () => {
 const r = room(); const t = r.game.catan.transport;
 t.attack = "catan-attack-transport-2025";
 t.state.travel.pending = 41;
 t.choices.relocateHexes = [{tile:7,edges:[5,6]},{tile:8,edges:[6,9]}];
 assert.deepEqual(transportEdges(r,{edge:null,piece:null}),[]);
 assert.equal(transportSelectedAction(r,{edge:5,piece:null,tile:8}),null);
 assert.deepEqual(transportSelectedAction(r,{edge:6,piece:null,tile:8}),{type:"catan_transport_relocate",offer:14,edge:6,tile:8});
 r.spectating = true;
 assert.equal(transportSelectedAction(r,{edge:6,piece:null,tile:8}),null);
});
test("attack transport keeps 14 points in waiting and saved games with either knight recipe", () => {
 for(const knights of [false,true]) {
  const draft={capacity:6,catanScenario:"attack-transport",catanCitiesKnights:knights?{}:undefined};
  const context=catanRuleContext(draft);
  assert.equal(context.attack,true); assert.equal(context.transport,true); assert.equal(context.target,14);
  assert.equal(catanSavedVictoryTarget({transport:{attack:"recipe"},citiesKnights:knights?{}:undefined}),14);
 }
});
