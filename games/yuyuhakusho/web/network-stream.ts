import type { InputWindow, NetworkView } from "./types";

type WindowFrames = { first: bigint; values: string[] };
const maxFrames = 120;
const maxFrame = 0xffffffffffffffffn;
function decimal(value: string) {
  if (typeof value !== "string" || !/^(0|[1-9][0-9]*)$/.test(value))
    throw Error("规则流帧号无效");
  const frame = BigInt(value);
  if (frame > maxFrame) throw Error("规则流帧号越界");
  return frame;
}
function decode(window: InputWindow, target: bigint): WindowFrames {
  if (
    !window ||
    window.version !== 1 ||
    !Number.isInteger(window.frameCount) ||
    window.frameCount < 0 ||
    window.frameCount > maxFrames ||
    typeof window.data !== "string" ||
    window.data.length > 1600
  )
    throw Error("规则流窗口无效");
  const first = decimal(window.firstFrame);
  if (!window.frameCount) {
    if (first !== 0n || window.data !== "") throw Error("规则流空窗口无效");
    return { first, values: [] };
  }
  if (!first || first + BigInt(window.frameCount) - 1n !== target)
    throw Error("规则流窗口边界不同");
  const raw = atob(window.data);
  if (!raw.length || raw.length % 10 || btoa(raw) !== window.data)
    throw Error("规则流编码无效");
  const values: string[] = [];
  let previous = "";
  for (let at = 0; at < raw.length; at += 10) {
    const count = raw.charCodeAt(at),
      value = raw.slice(at + 1, at + 10);
    if (
      !count ||
      values.length + count > window.frameCount ||
      raw.charCodeAt(at + 1) > 15 ||
      value === previous
    )
      throw Error("规则流记录无效");
    for (let seat = 0; seat < 4; seat++)
      if (raw.charCodeAt(at + 2 + seat * 2) > 15) throw Error("规则流按键无效");
    values.push(...Array<string>(count).fill(value));
    previous = value;
  }
  if (values.length !== window.frameCount) throw Error("规则流长度不同");
  return { first, values };
}
function encode(first: bigint, values: string[]): InputWindow {
  let raw = "";
  for (let at = 0; at < values.length;) {
    let count = 1;
    while (at + count < values.length && values[at + count] === values[at])
      count++;
    raw += String.fromCharCode(count) + values[at];
    at += count;
  }
  return {
    version: 1,
    firstFrame: String(first),
    frameCount: values.length,
    data: btoa(raw),
  };
}
export function mergeNetworkStream(
  previous: unknown,
  incoming: unknown,
): unknown {
  const next = incoming as NetworkView & { resync?: boolean };
  if (!next || next.protocol !== 1) throw Error("规则流协议无效");
  const target = decimal(next.frame),
    added = decode(next.replay, target);
  const old = previous as NetworkView | undefined;
  if (
    !old ||
    next.resync ||
    old.epoch !== next.epoch ||
    old.resourceDescriptorSHA256 !== next.resourceDescriptorSHA256 ||
    old.localSeat !== next.localSeat ||
    old.options.arena !== next.options.arena ||
    decimal(old.frame) > target
  )
    return incoming;
  const retained = decode(old.replay, decimal(old.frame));
  if (!added.values.length)
    return old.frame === next.frame && retained.values.length
      ? { ...next, replay: old.replay }
      : incoming;
  const end = retained.first + BigInt(retained.values.length);
  if (
    !retained.values.length ||
    added.first > end ||
    added.first < retained.first
  )
    return incoming;
  const offset = Number(added.first - retained.first);
  for (let at = offset; at < retained.values.length; at++)
    if (added.values[at - offset] !== retained.values[at])
      throw Error("规则流重叠记录不同");
  const values = [...retained.values.slice(0, offset), ...added.values];
  const drop = Math.max(0, values.length - maxFrames);
  return {
    ...next,
    replay: encode(retained.first + BigInt(drop), values.slice(drop)),
  };
}
