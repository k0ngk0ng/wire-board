import test from "node:test";
import assert from "node:assert/strict";
import {
  twoResponder,
  twoCanRespond,
  twoPick,
  twoChoices,
  twoSelected,
  twoReturnValid,
  twoChoiceName,
  twoRetreatTargets,
} from "../src/catan-two-state.ts";
import { catanColorIndex } from "../src/catan-player-colors.ts";
const choice = (owner, edge, vertex = -1) => ({ owner, edge, vertex });
function fixture() {
  return {
    status: "playing",
    you: 0,
    game: {
      turn: 0,
      catan: {
        players: [{}, {}],
        two: {
          actor: 0,
          canAct: true,
          sequence: 3,
          pending: { kind: "road" },
          choices: [choice(-2, 4), choice(-3, 4), choice(-2, 7)],
        },
      },
    },
  };
}
test("neutral selection never guesses a color or reuses a prior response", () => {
  const g = fixture().game.catan,
    s = twoPick(g, 4, -1);
  assert.equal(s.owner, null);
  assert.equal(twoSelected(g, s), undefined);
  assert.equal(twoChoices(g, s).length, 2);
  assert.deepEqual(twoSelected(g, { ...s, owner: -3 }), choice(-3, 4));
  assert.equal(twoPick(g, 7, -1).owner, -2);
  g.two.sequence++;
  assert.equal(twoSelected(g, { ...s, owner: -3 }), undefined);
  assert.deepEqual(twoChoices(g, s), []);
  assert.equal(twoPick(g, 8, -1), null);
});
test("neutral villages and fallback roads preserve their exact target", () => {
  const g = fixture().game.catan;
  g.two.pending.kind = "settlement";
  g.two.choices = [choice(-3, -1, 9)];
  assert.deepEqual(twoSelected(g, twoPick(g, -1, 9)), choice(-3, -1, 9));
  assert.equal(twoPick(g, 9, -1), null);
  g.two.choices = [choice(-2, 9)];
  assert.deepEqual(twoSelected(g, twoPick(g, 9, -1)), choice(-2, 9));
});
test("only the actual response owner gets controls, never observers or finished tables", () => {
  const r = fixture();
  assert.equal(twoResponder(r), 0);
  assert.equal(twoCanRespond(r), true);
  r.you = 1;
  assert.equal(twoCanRespond(r), false);
  r.you = 0;
  r.spectating = true;
  assert.equal(twoCanRespond(r), false);
  r.spectating = false;
  r.game.catan.two.canAct = false;
  assert.equal(twoCanRespond(r), false);
  r.game.catan.two.canAct = true;
  r.game.catan.two.pending = undefined;
  r.game.catan.two.trade = {};
  assert.equal(twoCanRespond(r), true);
  r.game.finished = true;
  assert.equal(twoCanRespond(r), false);
  assert.equal(twoResponder(r), undefined);
});
test("return exactly two held cards and use distinct neutral colors without changing ordinary setup", () => {
  assert.equal(twoReturnValid([0, 0, 2, 0, 0], [0, 0, 2, 0, 0]), true);
  for (const give of [
    [0, 0, 1, 0, 0],
    [2, 0, 0, 0, 0],
    [0, 0, 3, 0, 0],
    [0, 0, 2.5, 0, -0.5],
    [2],
  ])
    assert.equal(twoReturnValid([0, 0, 2, 0, 0], give), false);
  const g = fixture().game.catan;
  assert.equal(catanColorIndex(g, -2), 2);
  assert.equal(catanColorIndex(g, -3), 3);
  assert.equal(catanColorIndex({ baseSetup: { neutralColor: 5 } }, -2), 5);
});

test("two-player rules follow the actual save and retain ten-point victory", async () => {
  const { catanRuleContext } = await import("../src/catan-rule-context.ts");
  const { catanResultDescription } = await import("../src/catan-results.ts");
  const room = fixture();
  room.capacity = 6;
  room.catanOptions = { fiveSix: true };
  assert.equal(catanRuleContext(room).two, true);
  assert.equal(catanRuleContext(room).players, 2);
  assert.equal(catanRuleContext(room).fiveSix, false);
  assert.equal(catanRuleContext(room).target, 10);
  assert.match(catanResultDescription(room.game.catan), /中立势力/);
  room.game.catan.two = undefined;
  room.catanTwoRules = "catan-for-two-2025";
  assert.equal(catanRuleContext(room).two, false);
});

test("neutral placement animation uses one confirmed public change for players and observers", async () => {
  const { twoNeutralAdded } = await import("../src/catan-two-state.ts");
  const before = fixture();
  Object.assign(before, { id: "two", version: 8 });
  Object.assign(before.game.catan, {
    rollId: 4,
    setupStep: 4,
    edges: [
      { id: 0, owner: -1 },
      { id: 1, owner: -2 },
    ],
    vertices: [
      { id: 0, owner: -1, level: 0 },
      { id: 1, owner: -3, level: 1 },
    ],
  });
  const after = structuredClone(before);
  after.version++;
  delete after.game.catan.two.pending;
  after.game.catan.edges[0].owner = -3;
  assert.deepEqual(twoNeutralAdded(before, after), choice(-3, 0));
  after.game.catan.edges[0].bridge = true;
  assert.deepEqual(twoNeutralAdded(before, after), choice(-3, 0));
  // Observer has no legal choice list, but still sees the public placed piece.
  for (const r of [before, after]) {
    r.you = -1;
    r.spectating = true;
    delete r.game.catan.two.choices;
  }
  assert.deepEqual(twoNeutralAdded(before, after), choice(-3, 0));
  after.game.catan.edges[0].owner = -1;
  Object.assign(after.game.catan.vertices[0], { owner: -2, level: 1 });
  assert.deepEqual(twoNeutralAdded(before, after), choice(-2, -1, 0));
  after.status = "finished";
  assert.deepEqual(twoNeutralAdded(before, after), choice(-2, -1, 0));
  const reject = (modify) => {
    const b = structuredClone(before),
      a = structuredClone(after);
    modify(b, a);
    assert.equal(twoNeutralAdded(b, a), null);
  };
  reject((b, a) => a.version++);
  reject((b, a) => (a.version = b.version));
  reject((b, a) => (a.id = "rematch"));
  reject((b, a) => (a.you = 0));
  reject((b, a) => (a.spectating = false));
  reject((b, a) => a.game.catan.two.sequence++);
  reject((b, a) => a.game.catan.rollId++);
  reject((b) => (b.game.catan.setupStep = 0));
  reject((b, a) => (a.game.catan.two.pending = { kind: "road" }));
  reject((b) => delete b.game.catan.two.pending);
  reject((b, a) => (a.game.catan.edges[0].owner = -3)); // Multiple pieces.
  reject((b, a) => (a.game.catan.vertices[0].owner = 0)); // Not neutral.
  reject((b, a) => (a.game.catan.vertices[0].level = 2)); // Not a village.
  reject((b, a) => (a.game.catan.vertices = b.game.catan.vertices)); // No change.
});

test("river response identifies bridges and retains explicit neutral owners and fallback roads", () => {
  const g = fixture().game.catan;
  g.rivers = { map: { bridges: [4], swamps: [1, 2] } };
  g.two.pending.kind = "bridge";
  assert.equal(twoChoiceName(g, choice(-2, 4)), "桥梁");
  assert.equal(twoChoiceName(g, choice(-2, 7)), "道路");
  assert.equal(twoChoiceName(g, choice(-2, -1, 4)), "村庄");
  assert.equal(twoSelected(g, twoPick(g, 4, -1)), undefined);
  assert.deepEqual(
    twoSelected(g, { ...twoPick(g, 4, -1), owner: -3 }),
    choice(-3, 4),
  );
});

test("retreat uses only live legal swamp targets and cannot leak controls to an observer", () => {
  const r = fixture(),
    g = r.game.catan;
  g.tiles = [0, 1, 2, 3].map((id) => ({ id, resource: id === 3 ? 5 : 0 }));
  g.rivers = { map: { bridges: [], swamps: [1, 2] } };
  g.robber = 0;
  Object.assign(g.two, {
    pending: undefined,
    tokenWindow: true,
    tokens: [5, 5],
    cost: 2,
    retreatTiles: [1, 2],
  });
  assert.deepEqual(twoRetreatTargets(r), [1, 2]);
  g.robber = 1;
  assert.deepEqual(twoRetreatTargets(r), [2]);
  g.two.retreatTiles = [0, 1, 2, 2, 3, 90];
  assert.deepEqual(twoRetreatTargets(r), [2]);
  const deny = (modify) => {
    const copy = structuredClone(r);
    modify(copy, copy.game.catan.two);
    assert.deepEqual(twoRetreatTargets(copy), []);
  };
  deny((r) => (r.spectating = true));
  deny((r) => (r.you = -1));
  deny((r) => (r.status = "finished"));
  deny((r) => (r.game.finished = true));
  deny((r, q) => (q.spent = true));
  deny((r, q) => (q.tokenWindow = false));
  deny((r, q) => (q.tokens[0] = 1));
  deny((r, q) => delete q.retreatTiles);
  deny((r) => (r.game.catan.players[0].eliminated = true));
  g.rivers = undefined;
  delete g.two.retreatTiles;
  assert.deepEqual(twoRetreatTargets(r), [3]);
  g.two.retreatTiles = [];
  assert.deepEqual(twoRetreatTargets(r), []);
});

test("river rules and wealth result follow the actual game rather than a waiting draft", async () => {
  const { catanRuleContext } = await import("../src/catan-rule-context.ts");
  const { catanResultDescription } = await import("../src/catan-results.ts");
  const r = fixture();
  r.catanTwoScenario = "rivers";
  assert.equal(catanRuleContext(r).rivers, false);
  r.game.catan.rivers = { map: { bridges: [], swamps: [1, 2] } };
  assert.equal(catanRuleContext(r).rivers, true);
  assert.equal(catanRuleContext(r).scenario, "");
  assert.match(catanResultDescription(r.game.catan), /河流.*财富/);
  delete r.game;
  r.catanTwoRules = "catan-for-two-2025";
  r.capacity = 2;
  assert.equal(catanRuleContext(r).rivers, true);
  assert.equal(catanRuleContext(r).two, true);
  assert.equal(catanRuleContext(r).target, 10);
});

test("two-player merchant-train room shows twelve points while the running game remains authoritative", async () => {
  const { catanRuleContext } = await import("../src/catan-rule-context.ts");
  const r = fixture();
  r.capacity = 2;
  r.catanTwoRules = "catan-for-two-2025";
  r.catanTwoScenario = "caravans";
  assert.equal(catanRuleContext(r).caravans, false);
  assert.equal(catanRuleContext(r).target, 10);
  r.game.catan.caravans = { rules: "catan-caravans-2025" };
  r.catanTwoScenario = "rivers";
  assert.equal(catanRuleContext(r).caravans, true);
  assert.equal(catanRuleContext(r).rivers, false);
  assert.equal(catanRuleContext(r).scenario, "");
  assert.equal(catanRuleContext(r).target, 12);
  delete r.game;
  r.catanTwoScenario = "caravans";
  assert.equal(catanRuleContext(r).two, true);
  assert.equal(catanRuleContext(r).caravans, true);
  assert.equal(catanRuleContext(r).target, 12);
});

test("merchant-train retreat explicitly moves offboard, never to a waterhole or invented desert", () => {
  const r = fixture(),
    g = r.game.catan;
  g.caravans = {};
  g.tiles = [
    { id: 0, resource: 0 },
    { id: 1, resource: 5 },
  ];
  g.robber = 0;
  Object.assign(g.two, {
    pending: undefined,
    tokenWindow: true,
    tokens: [5, 5],
    cost: 2,
    retreatTiles: [-1],
  });
  assert.deepEqual(twoRetreatTargets(r), [-1]);
  for (const mutate of [
    (r) => (r.game.catan.robber = -1),
    (r) => delete r.game.catan.two.retreatTiles,
    (r) => (r.game.catan.two.retreatTiles = [1]),
    (r) => (r.game.catan.two.spent = true),
    (r) => (r.game.catan.two.tokenWindow = false),
    (r) => (r.game.catan.two.tokens[0] = 1),
    (r) => (r.spectating = true),
    (r) => (r.you = -1),
    (r) => (r.game.finished = true),
    (r) => (r.status = "closed"),
  ]) {
    const copy = structuredClone(r);
    mutate(copy);
    assert.deepEqual(twoRetreatTargets(copy), []);
  }
});

test("two-player merchant-train results include actual wagon building bonuses", async () => {
  const { catanResultDescription } = await import("../src/catan-results.ts");
  const r = fixture();
  r.game.catan.caravans = {};
  assert.match(
    catanResultDescription(r.game.catan),
    /商队.*12分.*马车.*每座＋1/,
  );
});
