import test from "node:test";
import assert from "node:assert/strict";
import { gameDefaultPlayers } from "../src/game-api/default-players.ts";

test("十席团队游戏默认创建5对5", () => {
  assert.equal(gameDefaultPlayers({ minPlayers: 2, maxPlayers: 10, playerCounts: [2, 4, 6, 8, 10], defaultPlayers: 10 }), 10);
});

test("既有模块沿用最少人数，默认人数须是合法整数选项", () => {
  assert.equal(gameDefaultPlayers({ minPlayers: 2, maxPlayers: 10 }), 2);
  for (const defaultPlayers of [0, 1, 3, 11, 4.5, Number.NaN, Number.POSITIVE_INFINITY]) {
    assert.equal(gameDefaultPlayers({ minPlayers: 2, maxPlayers: 10, playerCounts: [2, 4, 6, 8, 10], defaultPlayers }), 2);
  }
  assert.equal(gameDefaultPlayers({ minPlayers: 2, maxPlayers: 10, defaultPlayers: 6 }), 6);
});
