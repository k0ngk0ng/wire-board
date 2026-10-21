// Failure inventory written before implementation:
// - A dropped item is mistaken for a pickup, even when its event seat matches.
// - Enemy or teammate pickups leak to another player; seat 0 is treated as false.
// - Observers hear pickups, including a stale nonnegative seat after spectating.
// - Invalid/anonymous seat values accidentally compare equal.
// - Other event types are mistaken for pickups.
// Board integration must retain its existing seen/scope and muted/hidden gates.
import test from "node:test";
import assert from "node:assert/strict";
import { shouldPlayPickup } from "../web/pickup-audio.ts";

test("only the pickup owner hears items, including seat zero", () => {
  for (let seat = 0; seat < 8; seat++) {
    for (let you = 0; you < 8; you++) {
      assert.equal(
        shouldPlayPickup({ type: "item", seat }, you, false),
        seat === you,
      );
    }
  }
});
test("observers never hear pickups even with a retained seat", () => {
  for (let you = -1; you < 8; you++)
    assert.equal(
      shouldPlayPickup({ type: "item", seat: you }, you, true),
      false,
    );
});
test("drop and all other events cannot produce a pickup sound", () => {
  for (const type of [
    "drop",
    "place",
    "explode",
    "trap",
    "rescue",
    "eliminate",
    "shield",
    "finish",
  ])
    assert.equal(shouldPlayPickup({ type, seat: 0 }, 0, false), false);
});
test("anonymous and malformed seat values cannot qualify as the owner", () => {
  for (const seat of [-1, undefined, null, NaN, 0.5, "0"])
    assert.equal(shouldPlayPickup({ type: "item", seat }, seat, false), false);
});
