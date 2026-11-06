import { useEffect, useRef, useState } from "react";
import type { BoardProps } from "../../../web/src/game-api/module";
import type { Art } from "./art";
import { SceneMusic, type MusicFrame } from "./music-state";
import { mapFor } from "./scene";
import type { View } from "./types";

export function useSceneMusic(props: BoardProps<View>, view: View, art: Art | null, ready: boolean) {
  const [visible, setVisible] = useState(() => typeof document === "undefined" || !document.hidden);
  const driver = useRef<SceneMusic | null>(null);
  const latest = useRef({ props, view, ready }); latest.current = { props, view, ready };
  const scene = mapFor(view).id;
  const sample = (): MusicFrame => {
    const current = latest.current;
    const authority = current.view.snapshot as { tick?: number; zombies?: View["zombies"] } | undefined;
    return {
      enabled: current.props.sound,
      visible: !document.hidden,
      ready: current.ready,
      loading: !!current.props.room.loading,
      paused: !!current.props.clock.paused,
      finished: current.view.phase === "finished",
      elapsed: Math.max(0, authority?.tick ?? current.view.tick) * .05,
      // A predicted spawn or concealed enemy must never trigger a music cue.
      pressure: (authority?.zombies ?? []).filter(zombie => zombie.hp > 0 && zombie.allegiance !== "plants").length,
      clockEpoch: current.props.clock.epoch,
    };
  };
  useEffect(() => {
    const changed = () => { setVisible(!document.hidden); void driver.current?.sync(sample()); };
    document.addEventListener("visibilitychange", changed);
    return () => document.removeEventListener("visibilitychange", changed);
  }, []);
  useEffect(() => {
    if (!art || !art.manifest.music?.[scene]) return;
    // Immutable CDN music is streamed by the platform's native media channel.
    // Full-file integrity is verified by delivery; bootstrap never fetches a
    // complete FLAC merely to hand its URL to that channel a second time.
    const music = new SceneMusic(props.audio, art.base, art.manifest.music[scene], `${props.room.id}:${scene}:${art.manifest.version}`);
    driver.current = music;
    void music.sync(sample());
    const gesture = () => { music.retry(); void music.sync(sample()); };
    window.addEventListener("pointerdown", gesture, { passive: true });
    window.addEventListener("keydown", gesture);
    return () => { music.dispose(); driver.current = null; window.removeEventListener("pointerdown", gesture); window.removeEventListener("keydown", gesture); };
  }, [art, scene, props.room.id, props.audio]);
  useEffect(() => { void driver.current?.sync(sample()); }, [view, ready, props.sound, props.clock.paused, props.clock.epoch, props.room.loading, visible]);
}
