import { useEffect, useMemo, useState } from "react";
import type { GameAudioPort } from "../../../web/src/game-api/module";
import type { ClassicScenePack } from "./classic-loading";
import type { Car } from "./types";
import { ClassicHornPlayback, loadClassicHorn } from "./classic-horn";
import { classicFLACDataURL } from "./classic-media";

export function useClassicHorn(
  pack: ClassicScenePack | undefined,
  cars: readonly Car[],
  localSeat: number | null | undefined,
  localHorn: boolean,
  audio: GameAudioPort,
  running: boolean,
): () => void {
  const playback = useMemo(() => new ClassicHornPlayback(audio), [audio]);
  const [loaded, setLoaded] = useState<{ pack: ClassicScenePack; playback: ClassicHornPlayback; source: string }>();
  const source = loaded && loaded.pack === pack && loaded.playback === playback ? loaded.source : undefined;
  useEffect(() => {
    setLoaded(undefined);playback.stop();
    if (!pack) return;
    const controller = new AbortController();
    void loadClassicHorn(pack, controller.signal).then(({ bytes }) => {
      controller.signal.throwIfAborted();
      setLoaded({ pack, playback, source: classicFLACDataURL(bytes) });
    }).catch(() => { /* The verified scene loader owns loading/retry UI. */ });
    return () => {
      controller.abort();playback.stop();
    };
  }, [pack, playback]);
  useEffect(() => {
    playback.update(source, cars, localSeat, localHorn, running);
  }, [source, cars, localSeat, localHorn, running, playback]);
  useEffect(() => playback.stop, [playback]);
  return playback.stop;
}
