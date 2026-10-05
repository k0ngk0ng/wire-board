import { test } from "node:test";
import assert from "node:assert/strict";
import {
  fishAction,
  fishActionsFor,
  fishMapMode,
  fishGroundBlocked,
  fishGroundGeometry,
  fishResponder,
  fishTurn,
  fishValue,
} from "../src/catan-fishing-state.ts";
const selection = (s = {}) => ({
  kind: "catan_fish_robber",
  ids: [11],
  color: null,
  target: null,
  edge: null,
  ...s,
});
function fixture() {
  return {
    status: "playing",
    you: 0,
    spectating: false,
    game: {
      turn: 0,
      phase: "catan_turn",
      finished: false,
      catan: {
        players: [{}, {}, {}],
        bank: [19, 0, 19, 19, 19],
        fishing: {
          canReplace: false,
          tokens: {
            players: [
              {
                count: 4,
                tokens: [
                  { id: 0, fish: 1 },
                  { id: 11, fish: 2 },
                  { id: 21, fish: 3 },
                  { id: 22, fish: 3 },
                ],
              },
              { count: 7 },
              { count: 2 },
            ],
            bootOwner: 0,
          },
          legal: {
            costs: {
              catan_fish_robber: 2,
              catan_fish_steal: 3,
              catan_fish_resource: 4,
              catan_fish_road: 5,
              catan_fish_dev: 7,
            },
            actions: [
              "catan_fish_robber",
              "catan_fish_steal",
              "catan_fish_resource",
              "catan_fish_road",
              "catan_fish_dev",
            ],
            resources: [0, 2, 3, 4],
            targets: [1],
            roads: [8, 9],
            bootTargets: [2],
          },
        },
      },
    },
  };
}
test("fish actions permit pre-roll and normal action, reject interrupted and inactive viewers", () => {
  const r = fixture();
  for (const phase of ["catan_roll", "catan_turn"]) {
    r.game.phase = phase;
    assert.ok(fishTurn(r));
    assert.deepEqual(fishAction(r, selection()), {
      type: "catan_fish_robber",
      tokens: [11],
    });
  }
  for (const phase of [
    "catan_discard",
    "catan_robber",
    "catan_roads",
    "catan_gold",
    "catan_fish_replace",
    "catan_setup_road",
  ]) {
    r.game.phase = phase;
    assert.equal(fishAction(r, selection()), null);
  }
  r.game.phase = "catan_turn";
  for (const delta of [
    { spectating: true },
    { you: -1 },
    { you: 1 },
    { status: "closed" },
  ])
    assert.equal(fishAction({ ...r, ...delta }, selection()), null);
  r.game.finished = true;
  assert.equal(fishAction(r, selection()), null);
  r.game.finished = false;
  r.game.catan.players[0].eliminated = true;
  assert.equal(fishAction(r, selection()), null);
});
test("payment uses only own faces, rejects duplicate/stale IDs and insufficient amounts; permits overpay", () => {
  const r = fixture();
  assert.equal(fishValue(r, [0, 11, 21]), 6);
  for (const ids of [[11, 11], [99], [29], [-1], [0.5]]) {
    assert.equal(fishValue(r, ids), null);
    assert.equal(fishAction(r, selection({ ids })), null);
  }
  for (const ids of [[], [0]])
    assert.equal(fishAction(r, selection({ ids })), null);
  assert.deepEqual(fishAction(r, selection({ ids: [21] })), {
    type: "catan_fish_robber",
    tokens: [21],
  });
  assert.equal(fishValue({ ...r, you: 1 }, [11]), null);
  r.game.catan.fishing.tokens.players[0].tokens = [];
  assert.equal(fishAction(r, selection()), null);
});
test("all five effects recheck legal choices and bank; boot requires no payment", () => {
  const r = fixture(),
    f = r.game.catan.fishing;
  const cases = [
    [selection(), { type: "catan_fish_robber", tokens: [11] }],
    [
      selection({ kind: "catan_fish_steal", ids: [21], target: 1 }),
      { type: "catan_fish_steal", tokens: [21], target: 1 },
    ],
    [
      selection({ kind: "catan_fish_resource", ids: [0, 21], color: 2 }),
      { type: "catan_fish_resource", tokens: [0, 21], color: 2 },
    ],
    [
      selection({ kind: "catan_fish_road", ids: [11, 21], edge: 8 }),
      { type: "catan_fish_road", tokens: [11, 21], edge: 8 },
    ],
    [
      selection({ kind: "catan_fish_dev", ids: [0, 21, 22] }),
      { type: "catan_fish_dev", tokens: [0, 21, 22] },
    ],
  ];
  for (const [s, want] of cases) {
    assert.deepEqual(fishAction(r, s), want);
    f.legal.actions = f.legal.actions.filter((k) => k !== s.kind);
    assert.equal(fishAction(r, s), null);
    f.legal.actions.push(s.kind);
  }
  for (const [kind, field] of [
    ["catan_fish_resource", "color"],
    ["catan_fish_steal", "target"],
    ["catan_fish_road", "edge"],
  ])
    for (const id of [null, -1, 99, 0.5])
      assert.equal(
        fishAction(r, selection({ kind, ids: [0, 11, 21, 22], [field]: id })),
        null,
      );
  const bank = selection({
    kind: "catan_fish_resource",
    ids: [0, 21],
    color: 2,
  });
  r.game.catan.bank[2] = 0;
  assert.equal(fishAction(r, bank), null);
  const boot = selection({ kind: "catan_fish_boot", ids: [], target: 2 });
  assert.deepEqual(fishAction(r, boot), { type: "catan_fish_boot", target: 2 });
  assert.equal(fishAction(r, { ...boot, ids: [0] }), null);
  f.legal.bootTargets = [];
  assert.equal(fishAction(r, boot), null);
  for (const cost of [0, -1, NaN, 2.5, undefined]) {
    f.legal.costs.catan_fish_robber = cost;
    assert.equal(fishAction(r, selection()), null);
  }
});
test("replacement targets actual responder including seat zero, allows one owned token or keep", () => {
  const r = fixture(),
    f = r.game.catan.fishing;
  r.game.turn = 2;
  r.game.phase = "catan_fish_replace";
  f.tokens.responder = 0;
  f.canReplace = true;
  assert.equal(fishResponder(r), 0);
  assert.equal(fishTurn(r), false);
  const s = selection({ kind: "catan_fish_replace" });
  assert.deepEqual(fishAction(r, s), { type: "catan_fish_replace", card: 11 });
  assert.deepEqual(
    fishAction(r, selection({ kind: "catan_fish_keep", ids: [] })),
    { type: "catan_fish_keep" },
  );
  for (const ids of [[], [0, 11], [99]])
    assert.equal(fishAction(r, { ...s, ids }), null);
  assert.equal(fishAction(r, selection({ kind: "catan_fish_keep" })), null);
  assert.equal(fishAction(r, selection()), null);
  f.tokens.responder = 1;
  assert.equal(fishAction(r, s), null);
  f.tokens.responder = 0;
  f.canReplace = false;
  assert.equal(fishAction(r, s), null);
  f.canReplace = true;
  r.game.phase = "catan_turn";
  assert.equal(fishResponder(r), undefined);
  assert.equal(fishAction(r, s), null);
});
test("coastal art anchors the original tip and faces outwards in every rotation independent of endpoint order", () => {
  for (let degrees = 0; degrees < 360; degrees += 30) {
    const angle = (degrees * Math.PI) / 180,
      c = Math.cos(angle),
      s = Math.sin(angle);
    const rotate = ([x, y]) => ({
      x: 100 + x * c - y * s,
      y: 200 + x * s + y * c,
    });
    const g = {
      hexSize: 62,
      vertices: [
        rotate([-31, (-62 * Math.sqrt(3)) / 2]),
        rotate([0, 0]),
        rotate([-31, (62 * Math.sqrt(3)) / 2]),
      ],
    };
    const p = fishGroundGeometry(g, [0, 1, 2]);
    assert.deepEqual(p, fishGroundGeometry(g, [2, 1, 0]));
    assert.equal(p.x, 100);
    assert.equal(p.y, 200);
    assert.ok(Math.abs(p.labelX - (100 - 62 * 0.62 * c)) < 1e-8);
    assert.ok(Math.abs(p.labelY - (200 - 62 * 0.62 * s)) < 1e-8);
    assert.ok(Math.abs(p.scale * Math.hypot(66, 116.5) - 62) < 1e-8);
    const rad = (p.angle * Math.PI) / 180;
    assert.ok(Math.abs(Math.cos(rad) - c) < 1e-8);
    assert.ok(Math.abs(Math.sin(rad) - s) < 1e-8);
  }
  assert.equal(fishGroundGeometry({ vertices: [] }, [0, 1, 2]), null);
  assert.equal(
    fishGroundGeometry(
      {
        vertices: [
          { x: 0, y: 0 },
          { x: 0, y: 0 },
          { x: 0, y: 0 },
        ],
      },
      [0, 1, 2],
    ),
    null,
  );
});

test("combined progress purchase selects a nonempty public stack, revalidates supply and keeps resources separate", () => {
  const r = fixture(),
    g = r.game.catan,
    f = g.fishing;
  g.citiesKnights = { progressRemaining: [18, 0, 17] };
  f.legal.costs.catan_fish_progress = 7;
  f.legal.actions.push("catan_fish_progress");
  f.legal.progressTracks = [0, 2];
  const s = selection({
    kind: "catan_fish_progress",
    ids: [0, 21, 22],
    color: 2,
  });
  assert.deepEqual(fishAction(r, s), {
    type: "catan_fish_progress",
    tokens: [0, 21, 22],
    color: 2,
  });
  for (const color of [null, -1, 1, 3, 2.5])
    assert.equal(fishAction(r, { ...s, color }), null);
  assert.equal(fishAction(r, { ...s, ids: [0, 21] }), null);
  g.citiesKnights.progressRemaining[2] = 0;
  assert.equal(fishAction(r, s), null);
  g.citiesKnights.progressRemaining[2] = 17;
  f.legal.progressTracks = [0];
  assert.equal(fishAction(r, s), null);
  f.legal.progressTracks = [0, 2];
  delete g.citiesKnights;
  assert.equal(fishAction(r, s), null);
  f.legal.resources.push(5);
  g.bank[5] = 12;
  // Even a stale or malformed legal list cannot turn the four-fish action into a commodity purchase.
  assert.equal(
    fishAction(r, { ...s, kind: "catan_fish_resource", color: 5 }),
    null,
  );
});

test("sea fish actions use private payment and current ship targets, never road targets", () => {
  const r = fixture(),
    g = r.game.catan,
    f = g.fishing;
  g.seafarers = { pirate: 0, scenario: "islands" };
  f.legal.costs.catan_fish_ship = 5;
  f.legal.costs.catan_fish_pirate = 2;
  f.legal.actions.push("catan_fish_ship", "catan_fish_pirate");
  f.legal.ships = [12];
  const ship = selection({ kind: "catan_fish_ship", ids: [11, 21], edge: 12 });
  const pirate = selection({ kind: "catan_fish_pirate", ids: [21] });
  for (const phase of ["catan_roll", "catan_turn"]) {
    r.game.phase = phase;
    assert.deepEqual(fishAction(r, ship), {
      type: "catan_fish_ship",
      tokens: [11, 21],
      edge: 12,
    });
    assert.deepEqual(fishAction(r, pirate), {
      type: "catan_fish_pirate",
      tokens: [21],
    });
  }
  for (const edge of [null, -1, 8, 9, 12.5])
    assert.equal(fishAction(r, { ...ship, edge }), null);
  assert.equal(fishAction(r, { ...ship, ids: [21] }), null);
  assert.equal(fishAction(r, { ...ship, ids: [21, 21] }), null);
  f.legal.ships = [];
  assert.equal(fishAction(r, ship), null);
  delete f.legal.ships;
  assert.equal(fishAction(r, ship), null);
  f.legal.ships = [12];
  g.seafarers.pirate = -1;
  assert.equal(fishAction(r, pirate), null);
  g.seafarers.pirate = 0;
  f.legal.actions = f.legal.actions.filter((k) => k !== "catan_fish_pirate");
  assert.equal(fishAction(r, pirate), null);
  f.legal.actions.push("catan_fish_pirate");
  for (const delta of [
    { you: 1 },
    { you: -1 },
    { spectating: true },
    { status: "finished" },
  ]) {
    assert.equal(fishAction({ ...r, ...delta }, ship), null);
    assert.equal(fishAction({ ...r, ...delta }, pirate), null);
  }
  delete g.seafarers;
  assert.equal(fishAction(r, ship), null);
  assert.equal(fishAction(r, pirate), null);
});
test("fish menus distinguish variants and ship/road selection modes", () => {
  const g = fixture().game.catan;
  const kinds = () => fishActionsFor(g).map(([kind]) => kind);
  assert.ok(!kinds().includes("catan_fish_ship"));
  assert.ok(!kinds().includes("catan_fish_pirate"));
  assert.ok(kinds().includes("catan_fish_dev"));
  assert.ok(!kinds().includes("catan_fish_progress"));
  g.seafarers = { pirate: -1 };
  assert.ok(kinds().includes("catan_fish_ship"));
  assert.ok(kinds().includes("catan_fish_pirate"));
  g.seafarers.wonders = {};
  assert.ok(!kinds().includes("catan_fish_pirate"));
  g.citiesKnights = {};
  assert.ok(!kinds().includes("catan_fish_dev"));
  assert.ok(kinds().includes("catan_fish_progress"));
  assert.equal(fishMapMode("catan_fish_ship"), "fish_ship");
  assert.equal(fishMapMode("catan_fish_road"), "fish_road");
  assert.equal(fishMapMode("catan_fish_pirate"), "");
  assert.equal(fishMapMode("catan_fish_keep"), "");
});
test("pirate stops only its own sea-hex ground, including hex zero; frame and base grounds keep producing", () => {
  const g = { seafarers: { pirate: 0 } };
  assert.equal(fishGroundBlocked(g, { seaTile: 0 }), true);
  for (const seaTile of [undefined, null, -1, 1, NaN, 0.5])
    assert.equal(fishGroundBlocked(g, { seaTile }), false);
  g.seafarers.pirate = -1;
  assert.equal(fishGroundBlocked(g, { seaTile: 0 }), false);
  assert.equal(fishGroundBlocked(g, {}), false);
  assert.equal(fishGroundBlocked({}, { seaTile: 0 }), false);
});
