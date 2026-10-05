import test from "node:test";
import assert from "node:assert/strict";
import {
  twoResponder,
  twoCanRespond,
  twoPick,
  twoChoices,
  twoSelected,
  twoReturnValid,
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
