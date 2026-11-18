import type { Route, View } from "./types";

const secretArt = {
  gengarite: "src/assets/images/maps/3-1bis.png",
  klefki: "src/assets/images/maps/8-1bis.png",
  beedrillite: "src/assets/images/maps/9-2bis.png"
} as const;

export function mapArtwork(view: View, route?: Route) {
  const progression = view.progression;
  if (route?.id === 6 && progression?.secretItems?.gengarite) return secretArt.gengarite;
  if (route?.id === 29 && progression?.secrets?.klefki) return secretArt.klefki;
  if (route?.id === 39 && progression?.secretItems?.beedrillite) return secretArt.beedrillite;
  return route?.background;
}

export type MapEffectClock = { frame: number; physical: number; phase: number; mode: number; alpha?: number };

export function mapEffectClock(now: number, physical: number): MapEffectClock {
  return { frame: Math.floor(now * .06 + 1e-6), physical, phase: 0, mode: 0 };
}

export function updateMapEffect(clock: MapEffectClock, mode: number, now: number, physical: number) {
  const frame = Math.floor(now * .06 + 1e-6);
  if (mode === 0) {
    clock.frame = frame;
    clock.physical = physical;
    clock.mode = 0;
    return 0;
  }
  const count = Math.min(24, Math.max(frame - clock.frame, clock.mode !== mode || clock.alpha === undefined ? 1 : 0));
  if (!count) return clock.alpha ?? .65;
  const scaled = Math.max(0, physical - clock.physical) / count;
  for (let index = 0; index < count; index++) {
    clock.phase += scaled;
    const target = mode === 2 ? 1 : .65 + .01 * Math.sin(clock.phase * .001);
    const alpha = clock.alpha ?? target;
    clock.alpha = alpha + (target - alpha) * .05;
  }
  clock.frame = frame;
  clock.physical = physical;
  clock.mode = mode;
  return clock.alpha ?? .65;
}
