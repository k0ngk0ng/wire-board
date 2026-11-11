export const CLASSIC_CAMERA_MODES = ["近追车", "远追车", "引擎盖", "驾驶舱"] as const;
/** F3 cycles these world drawing ranges; the last entry preserves full range. */
export const CLASSIC_DRAW_DISTANCES = [1200, 1800, 2400] as const;

type Common = { mode: number; direction: 1 | -1; fov: number; near: number };
export type ClassicCameraPlan = Common & (
  | { kind: "chase"; distance: number; lift: number; hideExterior: false }
  | { kind: "hood" | "cockpit"; eye: [number, number, number]; hideExterior: boolean }
);

/** Positions and near clipping are metres in the normalized original car. */
export function classicCameraPlan(
  selected: number | undefined,
  rear: boolean,
  portrait: boolean,
  dimensions: readonly number[],
  cockpitEye?: readonly number[],
): ClassicCameraPlan {
  const mode = Number.isInteger(selected) && selected! >= 0 && selected! < CLASSIC_CAMERA_MODES.length ? selected! : 0;
  const direction = rear ? -1 : 1;
  if (mode < 2) return {
    mode, direction, kind: "chase", distance: mode === 0 ? (portrait ? 6.6 : 6.9) : 10.2,
    lift: mode === 0 ? 2.7 : 4.2, fov: portrait ? 72 : 62, near: .1, hideExterior: false,
  };
  if (mode === 2) return {
    mode, direction, kind: "hood", fov: portrait ? 78 : 72, near: .05, hideExterior: false,
    eye: [0, dimensions[1] * .75, rear ? -dimensions[2] * .5 - .15 : dimensions[2] * .32],
  };
  if (!cockpitEye || cockpitEye.length !== 3 || cockpitEye.some(n => !Number.isFinite(n)))
    throw Error("原车驾驶舱视角资源不完整");
  return { mode, direction, kind: "cockpit", fov: portrait ? 82 : 76, near: .05,
    eye: [cockpitEye[0], cockpitEye[1], cockpitEye[2]], hideExterior: true };
}

/** CSS-pixel rectangle, measured from the canvas's top left. */
export function classicMirrorViewport(width: number, height: number) {
  if (!(width > 0 && height > 0)) return null;
  const w = Math.min(420, width * .42), h = Math.min(100, w / 3.5, height * .15);
  return { x: (width - w) / 2, y: Math.min(12, height * .03), width: w, height: h };
}
