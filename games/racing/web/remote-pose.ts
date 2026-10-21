import type { Car, Track } from "./types";
import { interpolateCar } from "./presentation.ts";
import { predictCar } from "./prediction.ts";
export function remotePose(sample: {
  older: Car;
  current: Car;
  track: Track;
  previousAt: number;
  currentAt: number;
  visualAt: number;
  driving: boolean;
}): Car {
  const { older, current, track, previousAt, currentAt, visualAt, driving } =
    sample;
  if (current.finishAt || current.resetUntil > visualAt) return { ...current };
  if (driving)
    return predictCar(
      current,
      {
        steer: current.steer,
        throttle: current.throttle,
        brake: current.brake,
        drift: current.drift,
      },
      track,
      visualAt - currentAt,
      visualAt,
    );
  const span = currentAt - previousAt;
  return span > 0
    ? interpolateCar(older, current, (visualAt - previousAt) / span)
    : { ...current };
}
