import { useEffect, useMemo, useState } from "react";
import type { GameAudioPort } from "../../../web/src/game-api/module";
import {
  ClassicBrakePlayback,
  classicBrakeAudioVoice,
  loadClassicSkidSounds,
} from "./classic-brake-audio";
import type { ClassicScenePack } from "./classic-loading";
import { classicFLACDataURL } from "./classic-media";
import type { Car } from "./types";

export function useClassicBrakeAudio(
  pack: ClassicScenePack | undefined,
  car: Car | undefined,
  serverNow: number,
  localInput: { brake: boolean; brakeAmount?: number; drift: boolean } | undefined,
  audio: GameAudioPort,
  running: boolean,
) {
  const playback = useMemo(() => new ClassicBrakePlayback(audio), [audio]);
  const [loaded, setLoaded] = useState<{
    pack: ClassicScenePack;
    byPatch: Map<number, string>;
  }>();
  const sources = loaded && loaded.pack === pack ? loaded.byPatch : undefined;

  useEffect(() => {
    setLoaded(undefined);
    playback.stop();
    if (!pack) return;
    const controller = new AbortController();
    void loadClassicSkidSounds(pack, controller.signal)
      .then(({ buffers }) => {
        controller.signal.throwIfAborted();
        const byPatch = new Map<number, string>();
        for (const [patch, bytes] of buffers) {
          byPatch.set(patch, classicFLACDataURL(bytes));
        }
        setLoaded({ pack, byPatch });
      })
      .catch(() => {});
    return () => {
      controller.abort();
      playback.stop();
    };
  }, [pack, playback]);

  const effectiveCar = useMemo(() => {
    if (!car) return undefined;
    if (!localInput) return car;
    const brakeActive = localInput.brake || car.brake;
    const brakeAmount = localInput.brake
      ? (localInput.brakeAmount ?? 1)
      : (car.brakeAmount ?? (car.brake ? 1 : 0));
    const driftActive = localInput.drift || car.drift;
    return {
      ...car,
      brake: brakeActive,
      brakeAmount,
      drift: driftActive,
      motion: car.motion
        ? {
            ...car.motion,
            brake: Math.max(car.motion.brake ?? 0, brakeAmount),
            handbrake: Math.max(car.motion.handbrake ?? 0, driftActive ? 1 : 0),
          }
        : undefined,
    };
  }, [car, localInput?.brake, localInput?.brakeAmount, localInput?.drift]);

  const surface = useMemo(() => {
    if (!car || !pack) return undefined;
    if (car.offroad) {
      const roadSample =
        car.routeIndex !== undefined
          ? pack.track.road?.[car.routeIndex]
          : undefined;
      if (roadSample && (roadSample.surface === 4 || roadSample.surface === 12)) {
        return roadSample.surface;
      }
      return 2;
    }
    const roadSample =
      car.routeIndex !== undefined
        ? pack.track.road?.[car.routeIndex]
        : undefined;
    return roadSample?.surface ?? 1;
  }, [car?.offroad, car?.routeIndex, pack]);

  useEffect(() => {
    const voice =
      running && effectiveCar
        ? classicBrakeAudioVoice(effectiveCar, serverNow, surface)
        : null;
    playback.update(sources, voice, running);
  }, [sources, effectiveCar, serverNow, surface, running, playback]);

  useEffect(() => playback.stop, [playback]);
  return playback.stop;
}
