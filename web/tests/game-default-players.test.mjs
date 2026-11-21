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

test("人数边界和枚举先校验整数，不将浮点元数据带入比较", () => {
  for (const minPlayers of [1.5, Number.NaN, Number.POSITIVE_INFINITY, Number.MAX_SAFE_INTEGER + 1]) {
    assert.equal(gameDefaultPlayers({ minPlayers, maxPlayers: 10, defaultPlayers: 6 }), 1);
  }
  for (const maxPlayers of [10.5, Number.NaN, Number.POSITIVE_INFINITY, Number.MAX_SAFE_INTEGER + 1]) {
    assert.equal(gameDefaultPlayers({ minPlayers: 2, maxPlayers, defaultPlayers: 6 }), 2);
  }
  for (const playerCounts of [[2, 4.5, 6], [2, Number.NaN, 6], [2, Number.MAX_SAFE_INTEGER + 1, 6]]) {
    assert.equal(gameDefaultPlayers({ minPlayers: 2, maxPlayers: 10, playerCounts, defaultPlayers: 6 }), 2);
  }
  assert.equal(gameDefaultPlayers({ minPlayers: 2, maxPlayers: 10, playerCounts: [2, 4, 6], defaultPlayers: 6 }), 6);
});
