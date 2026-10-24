// Specified before implementation. Failure cases: weak randomness/fallback,
// modulo bias, server byte limits, and generation sharing mutable output.
import test from "node:test";
import assert from "node:assert/strict";
import { generateResetPassword } from "../src/admin/password.ts";

test("uses Web Crypto and returns a 20-byte copyable password", () => {
  const calls = [];
  const random = {
    getRandomValues(a) {
      calls.push(a.length);
      a.forEach((_, i) => (a[i] = i));
      return a;
    },
  };
  assert.equal(generateResetPassword(random), "ABCDEFGHIJKLMNOPQRST");
  assert.deepEqual(calls, [20]);
});
test("every byte maps uniformly to the 64-character alphabet", () => {
  const counts = new Map();
  for (let n = 0; n < 256; n++) {
    const out = generateResetPassword({
      getRandomValues(a) {
        return a.fill(n);
      },
    });
    assert.match(out, /^[A-Za-z0-9_-]{20}$/);
    counts.set(out[0], (counts.get(out[0]) || 0) + 1);
  }
  assert.equal(counts.size, 64);
  assert.ok([...counts.values()].every((n) => n === 4));
});
test("entropy failure does not fall back to Math.random", () => {
  assert.throws(
    () =>
      generateResetPassword({
        getRandomValues() {
          throw Error("unavailable");
        },
      }),
    /unavailable/,
  );
});
test("independent generations meet server byte limits", () => {
  const seen = new Set();
  for (let i = 0; i < 100; i++) {
    const p = generateResetPassword();
    assert.equal(new TextEncoder().encode(p).length, 20);
    assert.match(p, /^[A-Za-z0-9_-]{20}$/);
    seen.add(p);
  }
  assert.equal(seen.size, 100);
});
