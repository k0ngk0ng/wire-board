import test from "node:test";
import assert from "node:assert/strict";
import {
  caravanGeometry,
  caravanPick,
  caravanSelected,
  caravanResponder,
  caravanCanRespond,
  caravanBonus,
  caravanAdded,
  caravanPlacementStep,
  caravanPrompt,
} from "../src/catan-caravans-state.ts";
import { catanSavedVictoryTarget } from "../src/catan-rule-context.ts";

test("south-facing original wagon rotates to all six actual travel directions", () => {
  const labels = ["朝东南", "朝南", "朝西南", "朝西北", "朝北", "朝东北"];
  for (let i = 0; i < 6; i++) {
    const angle = ((30 + i * 60) * Math.PI) / 180;
    const g = {
      vertices: [
        { x: 100, y: 200 },
        { x: 100 + 46 * Math.cos(angle), y: 200 + 46 * Math.sin(angle) },
      ],
      edges: [{ a: 0, b: 1 }],
    };
    const p = caravanGeometry(g, { edge: 0, from: 0 }),
      r = caravanGeometry(g, { edge: 0, from: 1 });
    assert.equal(p.direction, labels[i]);
    assert.equal(r.direction, labels[(i + 3) % 6]);
    assert.equal(p.x, r.x);
    assert.equal(p.y, r.y);
    const rotation = (p.rotation * Math.PI) / 180;
    assert.ok(Math.abs(-Math.sin(rotation) - p.dx) < 1e-10);
    assert.ok(Math.abs(Math.cos(rotation) - p.dy) < 1e-10);
    assert.ok(Math.abs(p.dx + r.dx) < 1e-10 && Math.abs(p.dy + r.dy) < 1e-10);
  }
  assert.equal(
    caravanGeometry({ edges: [], vertices: [] }, { edge: 0, from: 0 }),
    null,
  );
  assert.equal(
    caravanGeometry(
      {
        edges: [{ a: 0, b: 1 }],
        vertices: [
          { x: 0, y: 0 },
          { x: 0, y: 0 },
        ],
      },
      { edge: 0, from: 0 },
    ),
    null,
  );
});

function placement() {
  const before = {
    id: "table",
    version: 10,
    status: "playing",
    you: 1,
    game: {
      catan: {
        setupStep: 6,
        setupLimit: 6,
        rollId: 9,
        players: [{}, {}, {}],
        vertices: [
          { id: 0, x: 100, y: 100 },
          { id: 1, x: 100, y: 162 },
        ],
        edges: [{ a: 0, b: 1 }],
        caravans: {
          sequence: 2,
          wagons: [],
          pending: { kind: "vote" },
          choices: [{ edge: 0, from: 0 }],
        },
      },
    },
  };
  const after = structuredClone(before);
  after.version++;
  delete after.game.catan.caravans.pending;
  after.game.catan.caravans.wagons.push({ edge: 0, from: 0 });
  return { before, after };
}

test("only a newly confirmed public wagon animates, including the winning placement", () => {
  const { before, after } = placement();
  assert.deepEqual(caravanAdded(before, after), { edge: 0, from: 0 });
  before.game.catan.caravans.pending.kind = "place";
  after.status = "finished";
  assert.deepEqual(caravanAdded(before, after), { edge: 0, from: 0 });
  before.spectating = after.spectating = true;
  before.you = after.you = -1;
  assert.deepEqual(caravanAdded(before, after), { edge: 0, from: 0 });
  assert.equal(caravanAdded(after, after), null);
});

test("reconnect gaps, rematches and viewer switches never replay wagon placements", () => {
  for (const change of [
    (r) => {
      r.version += 1;
    },
    (r) => {
      r.version -= 1;
    },
    (r) => {
      r.id = "another";
    },
    (r) => {
      r.you = 2;
    },
    (r) => {
      r.spectating = true;
    },
    (r) => {
      r.status = "closed";
    },
    (r) => {
      r.game.catan.setupStep = 0;
    },
    (r) => {
      r.game.catan.rollId = 0;
    },
    (r) => {
      r.game.catan.caravans.sequence++;
    },
    (r) => {
      r.game.catan.caravans.wagons.push({ edge: 1, from: 2 });
    },
    (r) => {
      r.game.catan.caravans.pending = { kind: "bid" };
    },
  ]) {
    const { before, after } = placement();
    change(after);
    assert.equal(caravanAdded(before, after), null);
  }
});

test("placement motion rejects unconfirmed directions, changed history and invalid geometry", () => {
  const { before, after } = placement();
  after.game.catan.caravans.wagons[0].from = 1;
  assert.equal(caravanAdded(before, after), null);
  after.game.catan.caravans.wagons[0].from = 0;
  after.game.catan.vertices[1].y = 100;
  assert.equal(caravanAdded(before, after), null);
  after.game.catan.vertices[1].y = 162;
  before.game.catan.caravans.wagons.unshift({ edge: 2, from: 3 });
  after.game.catan.caravans.wagons.unshift({ edge: 2, from: 4 });
  assert.equal(caravanAdded(before, after), null);
});

test("a shared edge never silently picks one of two legal directions", () => {
  const g = {
    caravans: {
      choices: [
        { edge: 4, from: 2 },
        { edge: 4, from: 3 },
        { edge: 8, from: 9 },
      ],
    },
  };
  const pick = caravanPick(g, 4);
  assert.deepEqual(pick, { edge: 4, from: null });
  assert.equal(caravanSelected(g, pick), undefined);
  assert.deepEqual(caravanSelected(g, { edge: 4, from: 3 }), {
    edge: 4,
    from: 3,
  });
  assert.deepEqual(caravanPick(g, 8), { edge: 8, from: 9 });
  assert.equal(caravanPick(g, 99), null);
  assert.equal(caravanSelected(g, { edge: 4, from: 99 }), undefined);
});

test("response permissions follow the bidder without replacing the action owner", () => {
  const room = {
    status: "playing",
    you: 2,
    game: {
      turn: 0,
      catan: {
        players: [{}, {}, {}],
        caravans: { actor: 2, pending: { kind: "vote" }, canAct: true },
      },
    },
  };
  assert.equal(caravanResponder(room), 2);
  assert.equal(caravanCanRespond(room), true);
  assert.equal(caravanCanRespond({ ...room, you: 0 }), false);
  assert.equal(caravanCanRespond({ ...room, spectating: true }), false);
  assert.equal(caravanCanRespond({ ...room, you: -1 }), false);
  assert.equal(caravanResponder({ ...room, status: "finished" }), undefined);
  assert.equal(
    caravanCanRespond({ ...room, game: { ...room.game, finished: true } }),
    false,
  );
  const after = structuredClone(room);
  delete after.game.catan.caravans.pending;
  assert.equal(caravanResponder(after), undefined);
  assert.equal(caravanCanRespond(after), false);
  assert.equal(catanSavedVictoryTarget(room.game.catan), 12);
});

test("the public building badge marks one bonus for two or three incident wagons", () => {
  const g = {
    edges: [
      { a: 0, b: 1 },
      { a: 1, b: 2 },
      { a: 1, b: 3 },
    ],
    caravans: { wagons: [{ edge: 0, from: 0 }] },
  };
  assert.equal(caravanBonus(g, 1), false);
  g.caravans.wagons.push({ edge: 1, from: 1 });
  assert.equal(caravanBonus(g, 1), true);
  assert.equal(caravanBonus(g, 0), false);
  g.caravans.wagons.push({ edge: 2, from: 3 });
  assert.equal(caravanBonus(g, 1), true);
});

function twoPlacements(tie = false) {
  const { before } = placement();
  const g = before.game.catan;
  g.players = [{}, {}];
  g.setupStep = g.setupLimit = 4;
  g.two = {};
  g.vertices.push({ id: 2, x: 160, y: 100 }, { id: 3, x: 160, y: 162 });
  g.edges.push({ a: 2, b: 3 });
  Object.assign(g.caravans, {
    actor: 0,
    pending: { kind: "place", active: 0, chooser: 0, two: { start: 0 } },
    choices: [
      { edge: 0, from: 0 },
      { edge: 1, from: 2 },
    ],
  });
  const first = structuredClone(before);
  first.version++;
  const c = first.game.catan.caravans;
  c.wagons.push({ edge: 0, from: 0 });
  c.pending.two.first = { edge: 0, from: 0 };
  c.actor = c.pending.chooser = tie ? 1 : 0;
  c.choices = [{ edge: 1, from: 2 }];
  const second = structuredClone(first);
  second.version++;
  second.game.catan.caravans.wagons.push({ edge: 1, from: 2 });
  delete second.game.catan.caravans.pending;
  return { before, first, second };
}

test("both two-player placements animate when the same bidder continues or a tie changes players", () => {
  for (const tie of [false, true]) {
    const { before, first, second } = twoPlacements(tie);
    assert.equal(caravanPlacementStep(before.game.catan), 1);
    assert.equal(caravanPlacementStep(first.game.catan), 2);
    assert.equal(caravanPlacementStep(second.game.catan), undefined);
    assert.equal(caravanPrompt(before.game.catan), "请放置第1辆马车");
    assert.equal(caravanPrompt(first.game.catan), "请放置第2辆马车");
    assert.deepEqual(caravanAdded(before, first), { edge: 0, from: 0 });
    assert.deepEqual(caravanAdded(first, second), { edge: 1, from: 2 });
    assert.equal(caravanAdded(first, first), null);
    for (const r of [before, first, second]) {
      r.you = -1;
      r.spectating = true;
    }
    assert.deepEqual(caravanAdded(before, first), { edge: 0, from: 0 });
    assert.deepEqual(caravanAdded(first, second), { edge: 1, from: 2 });
    first.status = "finished";
    delete first.game.catan.caravans.pending;
    assert.deepEqual(caravanAdded(before, first), { edge: 0, from: 0 });
  }
});

test("unfinished wagon animation requires matching persisted progress and consecutive snapshots", () => {
  for (const mutate of [
    (b, a) => delete a.game.catan.two,
    (b, a) => delete b.game.catan.two,
    (b, a) => delete b.game.catan.caravans.pending.two,
    (b, a) => delete a.game.catan.caravans.pending.two.first,
    (b, a) => a.game.catan.caravans.pending.two.start++,
    (b, a) => b.game.catan.caravans.pending.two.start++,
    (b, a) => a.game.catan.caravans.pending.two.first.from++,
    (b, a) => a.game.catan.caravans.pending.active++,
    (b, a) => (a.game.catan.caravans.pending.kind = "bid"),
    (b, a) => (b.game.catan.caravans.pending.kind = "bid"),
    (b, a) => a.game.catan.caravans.sequence++,
    (b, a) => (a.game.finished = true),
    (b, a) => a.version++,
    (b, a) => a.you++,
    (b, a) => (a.spectating = true),
  ]) {
    const { before, first } = twoPlacements();
    mutate(before, first);
    assert.equal(caravanAdded(before, first), null);
  }
  const { before } = twoPlacements();
  before.game.catan.caravans.pending.kind = "bid";
  assert.equal(caravanPlacementStep(before.game.catan), undefined);
  assert.equal(caravanPrompt(before.game.catan), "请完成商队投票");
});
