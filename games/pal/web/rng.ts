// SPDX-License-Identifier: GPL-3.0-only
// Original RNG delta commands follow SDLPAL rngplay.c.
// Copyright (c) 2009-2011 Wei Mingzhi and 2011-2026 SDLPAL development team.
// See ../resources/LICENSE.SDLPAL and ../native/reference/rngplay.c.
function apply(data: Uint8Array, pixels?: Uint8Array) {
  let ptr = 0,
    dst = 0;
  const read = (n: number) => {
    if (n < 0 || n > data.length - ptr) throw Error("仙剑演出数据不完整");
    const bytes = data.subarray(ptr, ptr + n);
    ptr += n;
    return bytes;
  };
  while (ptr < data.length) {
    const op = data[ptr++];
    if (op === 0 || op === 19) return;
    if (op === 2) {
      dst += 2;
      continue;
    }
    const literal = op >= 6 && op <= 12,
      repeat = op >= 13 && op <= 18,
      skip = op === 3 || op === 4;
    if (!literal && !repeat && !skip) continue;
    let count = op - (repeat ? 11 : 5);
    if (skip || op === 11 || op === 12 || op === 17 || op === 18) {
      const n = op === 4 || op === 12 || op === 18 ? 2 : 1,
        b = read(n);
      count = b[0] + 1 + (n === 2 ? b[1] * 256 : 0);
    }
    if (skip) {
      dst += count * 2;
      continue;
    }
    if (count < 1 || count > 32000 || dst < 0 || dst > 64000 - count * 2)
      throw Error("仙剑演出画面范围无效");
    const bytes = read(repeat ? 2 : count * 2);
    if (pixels) {
      if (repeat)
        for (let i = 0; i < count; i++) pixels.set(bytes, dst + i * 2);
      else pixels.set(bytes, dst);
    }
    dst += count * 2;
  }
}

export function applyRNGFrame(data: Uint8Array, pixels: Uint8Array) {
  if (pixels.length !== 64000 || data.length > 65000)
    throw Error("仙剑演出画面尺寸无效");
  data = data.slice();
  apply(data);
  apply(data, pixels);
}

export function readRNGArchive(data: Uint8Array): Uint8Array[] {
  if (data.length < 8) throw Error("仙剑演出资源不完整");
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength),
    first = view.getUint32(0, true);
  if (first < 8 || first % 4 || first > data.length || first / 4 - 1 > 4096)
    throw Error("仙剑演出索引无效");
  const frames: Uint8Array[] = [];
  for (let p = 0; p < first - 4; p += 4) {
    const start = view.getUint32(p, true),
      end = view.getUint32(p + 4, true);
    if (
      start < first ||
      end < start ||
      end > data.length ||
      end - start > 65000
    )
      throw Error("仙剑演出帧范围无效");
    const bytes = data.subarray(start, end);
    apply(bytes);
    frames.push(bytes);
  }
  if (view.getUint32(first - 4, true) !== data.length)
    throw Error("仙剑演出资源长度无效");
  return frames;
}
