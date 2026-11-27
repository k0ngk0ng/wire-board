import { useEffect, useRef, useState } from "react";
import type { GameAudioPort } from "../../../web/src/game-api/module";
import type { ClassicScenePack } from "./classic-loading";
import { classicFLACDataURL } from "./classic-media";
import {
  collisionSpatialListener,
  loadClassicCollisionSounds,
  resolveCollisionSpatialSound,
  type ClassicCollisionListener,
} from "./classic-collision-audio";
import type { RaceEvent } from "./types";

export function useClassicCollisionAudio(
  pack: ClassicScenePack | undefined,
  listener: ClassicCollisionListener | undefined,
  audio: GameAudioPort,
  running: boolean,
) {
  const [sources, setSources] = useState<{
    pack: ClassicScenePack;
    byPatch: Map<number, string>;
  }>();
  const lastPatch = useRef(0);
  const listenerRef = useRef(listener);
  listenerRef.current = listener;
  const activeSources = sources && sources.pack === pack ? sources.byPatch : undefined;

  useEffect(() => {
    setSources(undefined);
    if (!pack) return;
    const controller = new AbortController();
    void loadClassicCollisionSounds(pack, controller.signal)
      .then(({ buffers }) => {
        controller.signal.throwIfAborted();
        const byPatch = new Map<number, string>();
        for (const [patch, bytes] of buffers) {
          byPatch.set(patch, classicFLACDataURL(bytes));
        }
        setSources({ pack, byPatch });
      })
      .catch(() => {});
    return () => {
      controller.abort();
    };
  }, [pack]);

  useEffect(() => {
    if (!running || !listener) return;
    audio.spatialListener(collisionSpatialListener(listener));
  }, [audio, running, listener?.x, listener?.y, listener?.angle, listener?.height]);

  useEffect(() => {
    if (!running) audio.stopSpatial();
  }, [audio, running]);

  useEffect(() => () => audio.stopSpatial(), [audio]);

  return {
    ready: !!activeSources,
    playEvent(event: RaceEvent) {
      if (!activeSources) return false;
      const resolved = resolveCollisionSpatialSound(
        event,
        listenerRef.current,
        lastPatch.current,
      );
      if (!resolved) return false;
      const url = activeSources.get(resolved.patch);
      if (!url) return false;
      lastPatch.current = resolved.patch;
      void audio.spatial(
        `racing-collision-${event.seat}-${event.id % 6}`,
        [url],
        {
          ...resolved.spatial,
          playbackID: `racing-col-${event.id}`,
        },
      );
      return true;
    },
  };
}
