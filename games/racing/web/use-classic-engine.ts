import { useEffect, useMemo, useState } from "react";
import type { GameAudioPort } from "../../../web/src/game-api/module";
import { ClassicEnginePlayback, type ClassicEngineAudio, type ClassicLoadedEngine } from "./classic-audio";
import { classicEngineAssets, type ClassicScenePack } from "./classic-loading";
import { fetchClassicFiles } from "./classic-assets";
import { classicFLACDataURL } from "./classic-media";
import type { Car } from "./types";

export function useClassicEngine(pack: ClassicScenePack | undefined, car: Car | undefined, base: string, audio: GameAudioPort, running: boolean) {
  const playback = useMemo(() => new ClassicEnginePlayback(audio), [audio]);
  const [loaded, setLoaded] = useState<{ pack: ClassicScenePack; id: string; playback: ClassicEnginePlayback; engine: ClassicLoadedEngine }>();
  const id = car?.vehicle;
  const engine = loaded && loaded.pack === pack && loaded.id === id && loaded.playback === playback ? loaded.engine : undefined;
  useEffect(() => {
    setLoaded(undefined);
    playback.stop();
    if (!pack || !id || !running) return;
    const controller = new AbortController();
    const specs = classicEngineAssets(pack.manifest, id);
    void (async () => {
      const files = specs.every((file) => pack.files.has(file.path)) ? pack.files
        : await fetchClassicFiles(specs, { base: pack.base, signal: controller.signal });
      controller.signal.throwIfAborted();
      const metadata = JSON.parse(new TextDecoder().decode(files.get(`audio/${id}/engine.json`)!)) as ClassicEngineAudio;
      const sources = new Map<string, string>();
      for (const sample of metadata.samples) {
        const bytes = files.get(sample.path);
        if (!bytes) throw Error("原车发动机循环缺失");
        sources.set(sample.path, classicFLACDataURL(bytes));
      }
      if (!controller.signal.aborted) setLoaded({ pack, id, playback, engine: { metadata, sources } });
    })().catch(() => { /* Audio errors never interrupt driving. */ });
    return () => {
      controller.abort();
      playback.stop();
    };
  }, [pack, id, base, playback, running]);
  useEffect(() => {
    playback.update(engine, car?.rpm ?? engine?.metadata.idleRPM ?? 1000, car?.motion?.throttle ?? car?.throttle ?? 0, running && !!car);
  }, [engine, car?.rpm, car?.motion?.throttle, car?.throttle, running, playback]);
  useEffect(() => playback.stop, [playback]);
  return playback.stop;
}
