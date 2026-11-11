import { useEffect, useRef, useState } from "react";
import type { GameAudioPort } from "../../../web/src/game-api/module";
import { classicEngineVoices, type ClassicEngineAudio } from "./classic-audio";
import { classicEngineAssets, type ClassicScenePack } from "./classic-loading";
import { fetchClassicFiles } from "./classic-assets";
import { classicFLACDataURL } from "./classic-media";
import type { Car } from "./types";

type LoadedEngine = { metadata: ClassicEngineAudio; sources: Map<string, string> };

export function useClassicEngine(pack: ClassicScenePack | undefined, car: Car | undefined, base: string, audio: GameAudioPort, running: boolean) {
  const [engine, setEngine] = useState<LoadedEngine | null>(null);
  const active = useRef(new Set<string>());
  const stop = () => {
    for (const key of active.current) audio.stopLoop(key);
    active.current.clear();
  };
  const id = car?.vehicle;
  useEffect(() => {
    setEngine(null);
    stop();
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
      if (!controller.signal.aborted) setEngine({ metadata, sources });
    })().catch(() => { /* Audio errors never interrupt driving. */ });
    return () => {
      controller.abort();
      stop();
    };
  }, [pack, id, base, audio, running]);
  useEffect(() => {
    if (!running || !engine || !car) {
      stop();
      return;
    }
    const voices = classicEngineVoices(engine.metadata, car.rpm ?? engine.metadata.idleRPM, car.motion?.throttle ?? car.throttle);
    for (const voice of voices) {
      const source = engine.sources.get(voice.path);
      if (voice.volume < .001 || !source) {
        audio.stopLoop(voice.key);
        active.current.delete(voice.key);
      } else {
        active.current.add(voice.key);
        void audio.loop(voice.key, [source], { volume: voice.volume, rate: voice.rate });
      }
    }
  }, [engine, car?.rpm, car?.motion?.throttle, car?.throttle, running, audio]);
  useEffect(() => stop, [audio]);
  return stop;
}
