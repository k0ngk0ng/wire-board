import type { Car, Track } from "./types";
import type { ClassicPropWorld } from "./classic-road.ts";
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
  props?: ClassicPropWorld;
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
        brakeAmount: current.brakeAmount,
        drift: current.drift,
        protected: current.protected,
        extrapolated: true,
      },
      track,
      visualAt - currentAt,
      visualAt,
      sample.props,
    );
  const span = currentAt - previousAt;
  return span > 0
    ? interpolateCar(older, current, (visualAt - previousAt) / span)
    : { ...current };
}
