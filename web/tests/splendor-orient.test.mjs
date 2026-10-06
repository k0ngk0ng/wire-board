import { test } from "node:test";
import assert from "node:assert/strict";
import {
  gemBonus,
  gemCanBuy,
  gemCanAcquire,
  gemCardDescription,
  gemDefaultPayment,
  gemGoldNeeded,
  gemOrientChoices,
  gemPaymentError,
} from "../src/splendor-orient-state.ts";
const card = (id, orient, color = -1, cost = [0, 0, 0, 0, 0]) => ({
  id,
  orient,
  color,
  cost,
  tier: 1,
  points: 0,
});
const player = () => ({
  cards: [],
  bonus: [0, 0, 0, 0, 0],
  tokens: [0, 0, 0, 0, 0, 0],
});

test("virtual gold cannot become physical tokens or split one doubled gold across two colors", () => {
  const p = player();
  p.cards = [card(101, "gold"), card(102, "gold")];
  const c = card(1, undefined, 2, [1, 1, 1, 0, 0]);
  for (const posts of [[], [4]]) {
    p.tradingPosts = posts;
    assert.equal(gemGoldNeeded(p, c.cost), 3);
    assert.notEqual(
      gemPaymentError(p, c, gemDefaultPayment(p, c, [101]), [101]),
      "",
    );
    assert.equal(
      gemPaymentError(p, c, gemDefaultPayment(p, c, [101, 102]), [101, 102]),
      "",
    );
  }
  c.cost = [3, 0, 0, 0, 0];
  assert.equal(
    gemPaymentError(p, c, gemDefaultPayment(p, c, [101]), [101]),
    "",
  );
  c.cost = [1, 0, 0, 0, 0];
  assert.match(
    gemPaymentError(p, c, gemDefaultPayment(p, c, [101, 102]), [101, 102]),
    /至少/,
  );
});

test("physical gold and both halves of virtual gold can be selected without silently spending a card", () => {
  const p = player();
  p.tokens = [2, 0, 0, 0, 0, 2];
  p.cards = [card(101, "gold")];
  const c = card(1, undefined, 2, [3, 1, 1, 0, 0]);
  assert.deepEqual(gemDefaultPayment(p, c), [2, 0, 0, 0, 0, 3]);
  assert.notEqual(gemPaymentError(p, c, gemDefaultPayment(p, c), []), "");
  assert.equal(gemCanBuy(p, c), true);
  assert.deepEqual(gemDefaultPayment(p, c, [101]), [2, 0, 0, 0, 0, 1]);
  for (const n of [1, 2])
    assert.equal(gemPaymentError(p, c, [2, 0, 0, 0, 0, n], [101]), "");
  assert.notEqual(gemPaymentError(p, c, [2, 0, 0, 0, 0, 0], [101]), "");
  p.tokens[5] = 3;
  assert.match(gemPaymentError(p, c, [2, 0, 0, 0, 0, 3], [101]), /至少/);
  assert.equal(gemPaymentError(p, c, [2, 0, 0, 0, 0, 3], []), "");
});

test("sacrifice uses two physical cards, prioritizes copies, and accepts omitted green color zero", () => {
  const p = player();
  const copy = card(101, "copy", 0);
  copy.bonusCount = 2;
  p.cards = [
    copy,
    card(102, "double", 0),
    card(103, undefined, 0),
    card(104, undefined, 1),
  ];
  const c = card(1001, "sacrifice", 2); // sacrificeColor omitted by Go omitempty for green
  assert.equal(gemCanBuy(p, c), true);
  const zero = gemDefaultPayment(p, c);
  assert.match(gemPaymentError(p, c, zero, [102, 103]), /优先/);
  assert.equal(gemPaymentError(p, c, zero, [101, 102]), "");
  assert.notEqual(gemPaymentError(p, c, zero, [101, 104]), "");
  assert.notEqual(gemPaymentError(p, c, zero, [101]), "");
  assert.notEqual(gemPaymentError(p, c, zero, [101, 101]), "");
  assert.notEqual(gemPaymentError(p, c, [1, 0, 0, 0, 0, 0], [101, 102]), "");
  p.cards = [card(101, "double", 0)];
  assert.equal(gemCanBuy(p, c), false);
});

test("unpaired copies require an existing colored bonus and retain the copied bonus count", () => {
  const p = player(),
    c = card(1001, "copy");
  p.cards = [card(1002, "gold")];
  assert.equal(gemCanAcquire(p, c), false);
  assert.equal(gemCanBuy(p, c), false);
  p.cards.push(card(1003, "double", 1));
  assert.equal(gemCanAcquire(p, c), true);
  c.color = 1;
  c.bonusCount = 2;
  assert.equal(gemBonus(c), 2);
  assert.match(gemCardDescription(c), /2 枚钻石/);
  assert.equal(gemBonus(card(1, "gold")), 0);
});

test("payment rejects stale, duplicate, foreign cards, negative, fractional and excess tokens", () => {
  const p = player();
  p.tokens = [2, 0, 0, 0, 0, 2];
  p.cards = [card(101, "gold")];
  const c = card(1, undefined, 0, [1, 1, 0, 0, 0]);
  for (const ids of [[102], [101, 101]])
    assert.notEqual(gemPaymentError(p, c, [0, 0, 0, 0, 0, 0], ids), "");
  for (const tokens of [
    [-1, 0, 0, 0, 0, 0],
    [0.5, 0, 0, 0, 0, 0],
    [2, 0, 0, 0, 0, 0],
    [0, 0, 0, 0, 0, 3],
    [0],
  ])
    assert.notEqual(gemPaymentError(p, c, tokens, [101]), "");
  assert.equal(gemPaymentError(p, c, [1, 0, 0, 0, 0, 1], []), "");
  assert.notEqual(gemPaymentError(p, c, [1, 0, 0, 0, 0, 2], []), "");
});

test("Orient responses follow current effect, public server choices, viewer and autoplay ownership", () => {
  const p = player();
  p.cards = [card(101, "double", 1), card(102, "copy"), card(103, "gold")];
  const room = {
    status: "playing",
    you: 0,
    seats: [{}],
    game: {
      turn: 0,
      phase: "gem_copy",
      splendor: { players: [p], effects: [{ kind: "copy", card: 102 }] },
    },
  };
  assert.deepEqual(
    gemOrientChoices(room).map((c) => c.id),
    [101],
  );
  room.spectating = true;
  assert.deepEqual(gemOrientChoices(room), []);
  room.spectating = false;
  room.seats[0].autoPlay = true;
  assert.deepEqual(gemOrientChoices(room), []);
  room.seats[0].autoPlay = false;
  room.game.phase = "gem_free_card";
  room.game.splendor.effects = [{ kind: "free_card", tier: 1 }];
  room.game.splendor.freeCardChoices = [card(1001, "gold")];
  assert.deepEqual(
    gemOrientChoices(room).map((c) => c.id),
    [1001],
  );
  room.game.turn = 1;
  assert.deepEqual(gemOrientChoices(room), []);
  room.game.turn = 0;
  room.game.finished = true;
  assert.deepEqual(gemOrientChoices(room), []);
});
