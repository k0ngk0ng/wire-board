import test from "node:test";
import assert from "node:assert/strict";
import { mergeNetworkStream } from "../web/network-stream.ts";

const windowFor = (first, values) => {
  const runs = [];
  for (const value of values) {
    const last = runs.at(-1);
    if (last && last[2] === value) last[0]++;
    else runs.push([1, 0, value, 0, 0, 0]);
  }
  const bytes = runs.flatMap(([count, automated, ...held]) => [
    count,
    automated,
    ...held.flatMap((value) => [value >> 8, value & 255]),
  ]);
  return {
    version: 1,
    firstFrame: values.length ? String(first) : "0",
    frameCount: values.length,
    data: Buffer.from(bytes).toString("base64"),
  };
};
const view = (frame, first, values, extra = {}) => ({
  protocol: 1,
  epoch: "match-a",
  resourceDescriptorSHA256: "resource-a",
  localSeat: 0,
  options: { arena: 10 },
  frame: String(frame),
  replay: windowFor(first, values),
  ...extra,
});
const expand = (window) => {
  const raw = Buffer.from(window.data, "base64");
  const values = [];
  for (let at = 0; at < raw.length; at += 10)
    for (let count = 0; count < raw[at]; count++)
      values.push(raw.readUInt16BE(at + 2));
  return values;
};

test("same-frame empty ACK keeps the preceding authority input window", () => {
  const first = view(103, 101, [64, 0, 64]);
  const ack = view(103, 0, [], { serverAt: 5000 });
  const before = structuredClone(first);
  const result = mergeNetworkStream(first, ack);
  assert.deepEqual(result.replay, first.replay);
  assert.equal(result.serverAt, 5000);
  assert.deepEqual(first, before);
  assert.notEqual(result, ack);
});

test("coalesced continuous packets preserve input edges and canonical runs", () => {
  let result = view(3, 1, [64, 0, 64]);
  result = mergeNetworkStream(result, view(5, 3, [64, 0, 96]));
  result = mergeNetworkStream(result, view(7, 6, [96, 0]));
  assert.equal(result.replay.firstFrame, "1");
  assert.equal(result.replay.frameCount, 7);
  assert.deepEqual(expand(result.replay), [64, 0, 64, 0, 96, 96, 0]);
  assert.equal(
    result.replay.data,
    windowFor(1, [64, 0, 64, 0, 96, 96, 0]).data,
  );
});

test("retained inputs stay within 120 frames and preserve uint64 precision", () => {
  const first = 18446744073709551400n;
  let result = view(first + 99n, first, Array(100).fill(64));
  result = mergeNetworkStream(
    result,
    view(first + 159n, first + 100n, Array(60).fill(0)),
  );
  assert.equal(result.replay.firstFrame, String(first + 40n));
  assert.equal(result.replay.frameCount, 120);
  assert.deepEqual(expand(result.replay), [
    ...Array(60).fill(64),
    ...Array(60).fill(0),
  ]);
});

test("gaps and new identities never invent missing input", () => {
  const previous = view(3, 1, [64, 0, 64]);
  for (const incoming of [
    view(8, 7, [0, 64]),
    view(8, 0, []),
    view(5, 4, [0, 64], { epoch: "match-b" }),
    view(5, 4, [0, 64], { localSeat: 1 }),
    view(5, 4, [0, 64], { resourceDescriptorSHA256: "resource-b" }),
    view(5, 4, [0, 64], { options: { arena: 2 } }),
    view(5, 4, [0, 64], { resync: true }),
  ]) {
    assert.deepEqual(mergeNetworkStream(previous, incoming), incoming);
  }
  assert.deepEqual(mergeNetworkStream(undefined, previous), previous);
});

test("overlap conflicts and malformed RLE fail before authority reaches the scene", () => {
  const previous = view(3, 1, [64, 0, 64]);
  assert.throws(() => mergeNetworkStream(previous, view(5, 3, [0, 64, 0])));
  for (const replay of [
    {
      version: 2,
      firstFrame: "1",
      frameCount: 1,
      data: windowFor(1, [0]).data,
    },
    { ...windowFor(1, [0]), frameCount: 121 },
    { ...windowFor(1, [0]), firstFrame: "01" },
    { ...windowFor(1, [0]), firstFrame: "18446744073709551616" },
    { ...windowFor(1, [0]), data: "AA==" },
    { ...windowFor(1, []), data: "AA==" },
  ]) {
    assert.throws(() =>
      mergeNetworkStream(previous, { ...view(4, 4, [0]), replay }),
    );
  }
});

test("boundary runs and maximum frame retain every original seat mask", () => {
  for (const count of [119, 120]) {
    const end = 18446744073709551615n;
    const packet = view(end, end - BigInt(count) + 1n, Array(count).fill(4095));
    const result = mergeNetworkStream(undefined, packet);
    assert.deepEqual(result.replay, packet.replay);
  }
  const empty = view(0, 0, []);
  assert.deepEqual(mergeNetworkStream(empty, empty), empty);
});
