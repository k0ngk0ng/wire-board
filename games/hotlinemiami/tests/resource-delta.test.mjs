import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { validateHotlineResourceDelta } from "../build/resource-delta.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const output = path.join(root, ".local/hotlinemiami/verification/large-map-quality");
mkdirSync(output, { recursive: true });
const prefix = "games/hotlinemiami/assets/public/hotlinemiami/";
const sourceCommit = "a".repeat(40);
const results = [];

function record(content, owner = "hotlinemiami") {
  const bytes = Buffer.from(content);
  const sha256 = createHash("sha256").update(bytes).digest("hex");
  return { key: `bga/${owner}/${sha256}.png`, bytes: bytes.length, sha256, mime: "image/png" };
}

function fixture() {
  const before = { version: 1, files: { [prefix + "runtime/character.png"]: record("unchanged original character"), [prefix + "runtime/map-no-talk.png"]: record("old map with baked actor"), "games/tetris/assets/public/tetris/image.png": record("unrelated", "tetris") } };
  const history = { version: 1, files: [{ path: "games/tetris/assets/public/tetris/old.png", ...record("previous unrelated history", "tetris"), sourceCommit: "b".repeat(40) }], embedded: [{ path: "unrelated-source.ts", sha256: "b".repeat(64) }] };
  const after = structuredClone(before);
  after.files[prefix + "runtime/map-no-talk.png"] = record("same native layout without baked actors");
  after.files[prefix + "runtime/map-highball.png"] = record("new original highball scene");
  after.files[prefix + "runtime/character-alias.png"] = structuredClone(before.files[prefix + "runtime/character.png"]);
  const afterHistory = structuredClone(history);
  afterHistory.files.push({ path: prefix + "runtime/map-no-talk.png", ...before.files[prefix + "runtime/map-no-talk.png"], sourceCommit });
  return { before, history, after, afterHistory };
}

test("new scenes and necessary old-map corrections preserve unchanged addresses and append exact history", () => {
  const { before, history, after, afterHistory } = fixture();
  const result = validateHotlineResourceDelta(before, history, after, afterHistory, sourceCommit);
  assert.equal(result.passed, true);
  assert.deepEqual(result.changedFiles, [prefix + "runtime/map-no-talk.png"]);
  assert.deepEqual(result.reusedFiles, [prefix + "runtime/character.png"]);
  assert.equal(result.newObjects.length, 2);
  assert.equal(result.archivedRecords, 1);
  results.push({ scenario: "minimal-increment", passed: true, ...result });
});

test("unchanged byte identities cannot get new keys or be removed", () => {
  for (const mutation of [
    after => { after.files[prefix + "runtime/character.png"].key = after.files[prefix + "runtime/character.png"].key.replace(".png", ".webp"); },
    after => { delete after.files[prefix + "runtime/character.png"]; },
  ]) {
    const { before, history, after, afterHistory } = fixture();
    mutation(after);
    assert.throws(() => validateHotlineResourceDelta(before, history, after, afterHistory, sourceCommit), /unchanged|removed|key/i);
  }
  results.push({ scenario: "stable-identities", passed: true });
});

test("changed bytes cannot overwrite immutable keys or omit the prior exact history", () => {
  for (const mutation of [
    (before, after) => { after.files[prefix + "runtime/map-no-talk.png"].key = before.files[prefix + "runtime/map-no-talk.png"].key; },
    (before, after, history) => { history.files = []; },
    (before, after, history) => { history.files[0].sha256 = "0".repeat(64); },
  ]) {
    const { before, history, after, afterHistory } = fixture();
    mutation(before, after, afterHistory);
    assert.throws(() => validateHotlineResourceDelta(before, history, after, afterHistory, sourceCommit), /history|immutable|key|record/i);
  }
  results.push({ scenario: "immutable-history", passed: true });
});

test("other games and pre-existing history stay exactly unchanged", () => {
  for (const mutation of [
    (after, history) => { after.files["games/tetris/assets/public/tetris/image.png"] = record("unrelated edit", "tetris"); },
    (after, history) => { history.embedded[0].sha256 = "0".repeat(64); },
    (after, history) => { history.files.push({ path: "games/tetris/assets/public/tetris/new.png", ...record("unrelated", "tetris"), sourceCommit }); },
  ]) {
    const { before, history, after, afterHistory } = fixture();
    mutation(after, afterHistory);
    assert.throws(() => validateHotlineResourceDelta(before, history, after, afterHistory, sourceCommit), /unrelated|history|scope/i);
  }
  results.push({ scenario: "scope-preserved", passed: true });
});

test("duplicating one archive entry cannot hide a missing replacement record", () => {
  const { before, history, after, afterHistory } = fixture();
  after.files[prefix + "runtime/character.png"] = record("another genuinely changed byte payload");
  afterHistory.files.push(structuredClone(afterHistory.files.at(-1)));
  assert.throws(() => validateHotlineResourceDelta(before, history, after, afterHistory, sourceCommit), /history|replaced|duplicate/i);
  results.push({ scenario: "one-history-per-replaced-record", passed: true });
});

test.after(() => {
  writeFileSync(path.join(output, "resource-delta.json"), JSON.stringify({ passed: results.length === 5, cases: results }, null, 2) + "\n");
});
