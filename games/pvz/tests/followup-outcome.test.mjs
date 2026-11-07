import assert from "node:assert/strict";
import test from "node:test";
import { describeOutcome, formatElapsed, roundRow } from "../web/outcome.ts";

test("both successful attacks explain the authoritative winner with sub-second precision", () => {
  const rounds = [
    { attackTeam: 1, breached: true, elapsedMS: 80050 },
    { attackTeam: 0, breached: true, elapsedMS: 80000 },
  ];
  assert.equal(
    describeOutcome(rounds, 0),
    "A队获胜：双方均攻破，A队用时 1:20.00，比B队快 0.05 秒。",
  );
  assert.equal(formatElapsed(80050), "1:20.05");
});
test("only successful attacking team wins regardless of round order", () => {
  assert.equal(
    describeOutcome(
      [
        { attackTeam: 1, breached: false, elapsedMS: 180000 },
        { attackTeam: 0, breached: true, elapsedMS: 150000 },
      ],
      0,
    ),
    "A队获胜：A队成功攻破，B队未能攻破。",
  );
  assert.equal(
    describeOutcome(
      [
        { attackTeam: 1, breached: true, elapsedMS: 150000 },
        { attackTeam: 0, breached: false, elapsedMS: 180000 },
      ],
      1,
    ),
    "B队获胜：B队成功攻破，A队未能攻破。",
  );
});
test("two defenses or equal breakthrough times explicitly explain a draw", () => {
  assert.equal(
    describeOutcome(
      [
        { attackTeam: 1, breached: false, elapsedMS: 180000 },
        { attackTeam: 0, breached: false, elapsedMS: 180000 },
      ],
      -1,
    ),
    "和局：双方进攻均未突破防线。",
  );
  assert.equal(
    describeOutcome(
      [
        { attackTeam: 1, breached: true, elapsedMS: 80050 },
        { attackTeam: 0, breached: true, elapsedMS: 80050 },
      ],
      -1,
    ),
    "和局：双方均攻破，突破用时同为 1:20.05。",
  );
});
test("an attack failure must never be labelled as the attacking team's successful defense", () => {
  assert.deepEqual(
    roundRow({ attackTeam: 1, breached: false, elapsedMS: 180000 }, 0),
    {
      round: "第1回合",
      attack: "B队",
      defense: "A队",
      result: "未攻破",
      time: "3:00.00",
    },
  );
});
test("incomplete rounds do not fabricate a winner", () => {
  assert.equal(
    describeOutcome(
      [{ attackTeam: 1, breached: true, elapsedMS: 80000 }],
      undefined,
    ),
    "",
  );
});
