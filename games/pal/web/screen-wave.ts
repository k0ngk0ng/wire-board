/*!
 * SPDX-License-Identifier: GPL-3.0-only
 * Copyright (c) 2009-2011, Wei Mingzhi <whistler_wmz@users.sf.net>.
 * Copyright (c) 2011-2026, SDLPAL development team.
 * Portions based on PALx Project by palxex.
 * Copyright (c) 2006, Pal Lockheart <palxex@gmail.com>.
 * See games/pal/native/upstream/LICENSE for the GNU GPL version 3.
 */
// Port of pinned SDLPAL PAL_ApplyWave. The caller chooses the original stage:
// scene maps before actors/UI, or the battle's immutable offscreen surface.
export function applyScreenWave(
  pixels: Uint8Array,
  level: number,
  phase: number,
): void {
  if (
    pixels.length !== 64000 ||
    !Number.isInteger(level) ||
    level < 1 ||
    level > 255 ||
    !Number.isInteger(phase) ||
    phase < 0 ||
    phase > 31
  )
    throw Error("仙剑波动画面状态无效");
  const wave = new Int32Array(32),
    row = new Uint8Array(320);
  let value = 0,
    delta = 68;
  for (let i = 0; i < 16; i++) {
    delta -= 8;
    value += delta;
    wave[i] = Math.trunc((value * level) / 256);
    wave[i + 16] = 320 - wave[i];
  }
  for (let y = 0; y < 200; y++) {
    const shift = wave[(phase + y) % 32],
      offset = y * 320;
    if (!shift || shift === 320) continue;
    row.set(pixels.subarray(offset, offset + 320));
    pixels.set(row.subarray(shift), offset);
    pixels.set(row.subarray(0, shift), offset + 320 - shift);
  }
}
