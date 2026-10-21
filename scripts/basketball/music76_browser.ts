import { GameAudio } from "../../web/src/audio";
import {
  BasketballAudio,
  type BasketballMusicState,
} from "../../games/basketball/web/audio";
import playback from "../../games/basketball/assets/audio-playback.json";
const entry = playback.music["76"];
const platform = new GameAudio();
platform.mount();
const port = platform.channel("music76-e2e");
const adapter = new BasketballAudio(port, {
  localBase: "/audio",
  assetsBase: new URLSearchParams(location.search).has("fallback")
    ? "/cdn-unavailable"
    : undefined,
});
type EventRow = {
  kind: string;
  at: number;
  time: number;
  paused: boolean;
  ended: boolean;
  src: string;
};
const h = {
  platform,
  port,
  adapter,
  entry,
  started: false,
  error: "",
  media: undefined as HTMLAudioElement | undefined,
  events: [] as EventRow[],
  visibility: [] as { hidden: boolean; at: number }[],
  sequence: 1,
  loopStart: entry.loop_start_sample / entry.sample_rate,
  duration: entry.decoded_samples / entry.sample_rate,
  recorder: undefined as MediaRecorder | undefined,
  chunks: [] as Blob[],
  captureSupported: false,
  captureError: "",
};
Object.assign(window, { music76: h });
function observe(m: HTMLAudioElement) {
  for (const kind of [
    "loadedmetadata",
    "play",
    "playing",
    "pause",
    "ended",
    "seeking",
    "seeked",
    "waiting",
    "stalled",
    "error",
    "timeupdate",
  ]) {
    m.addEventListener(
      kind,
      () => {
        h.events.push({
          kind,
          at: performance.now(),
          time: m.currentTime,
          paused: m.paused,
          ended: m.ended,
          src: m.currentSrc,
        });
        document.querySelector("pre")!.textContent = JSON.stringify(
          {
            time: m.currentTime,
            duration: m.duration,
            loopStart: h.loopStart,
            ended: m.ended,
            paused: m.paused,
            events: h.events.slice(-4),
          },
          null,
          2,
        );
      },
      { capture: true },
    );
  }
}
document.addEventListener("visibilitychange", () => {
  h.visibility.push({ hidden: document.hidden, at: performance.now() });
  if (document.hidden) platform.stop();
});
document.querySelector<HTMLButtonElement>("#start")!.onclick = async () => {
  try {
    await platform.unlock();
    const r = await adapter.consume([76], h.sequence);
    if (r.failed.length || r.unsupported.length) throw Error(JSON.stringify(r));
    h.media = document.querySelector<HTMLAudioElement>(
      "audio[data-platform-music]",
    )!;
    observe(h.media);
    h.started = true;
  } catch (e) {
    h.error = String(e);
  }
};
Object.assign(h, {
  snapshot() {
    const m = h.media!;
    return {
      src: m.currentSrc,
      attribute: m.getAttribute("src"),
      duration: m.duration,
      time: m.currentTime,
      paused: m.paused,
      ended: m.ended,
      loop: m.loop,
      rate: m.playbackRate,
      mediaCount: document.querySelectorAll("audio").length,
      hidden: document.hidden,
    };
  },
  async command(ids: number[]) {
    return adapter.consume(ids, ++h.sequence);
  },
  async restore(positionMS: number, paused = false, elapsedMS = 0) {
    adapter.reset(++h.sequence);
    const s: BasketballMusicState = {
      epoch: 7,
      trackID: 76,
      assetSHA256: entry.distribution_sha256,
      controlSequence: h.sequence,
      positionMS,
      at: 1000,
      paused,
    };
    return adapter.restoreMusic(s, 1000 + elapsedMS);
  },
  recordStart() {
    h.captureError = "";
    h.chunks = [];
    try {
      const m = h.media as HTMLAudioElement & {
        captureStream?: () => MediaStream;
      };
      if (
        !m.captureStream ||
        !MediaRecorder.isTypeSupported("audio/webm;codecs=opus")
      )
        return false;
      const stream = m.captureStream();
      h.recorder = new MediaRecorder(stream, {
        mimeType: "audio/webm;codecs=opus",
      });
      h.recorder.ondataavailable = (e) => {
        if (e.data.size) h.chunks.push(e.data);
      };
      h.recorder.start();
      h.captureSupported = true;
      return true;
    } catch (e) {
      h.captureError = String(e);
      return false;
    }
  },
  async recordStop() {
    const r = h.recorder;
    if (!r) return null;
    if (r.state !== "inactive")
      await new Promise<void>((resolve) => {
        r.addEventListener("stop", () => resolve(), { once: true });
        r.stop();
      });
    const b = new Blob(h.chunks, { type: r.mimeType });
    const bytes = new Uint8Array(await b.arrayBuffer());
    let binary = "";
    for (let i = 0; i < bytes.length; i += 16384)
      binary += String.fromCharCode(...bytes.subarray(i, i + 16384));
    return {
      mime: r.mimeType,
      bytes: b.size,
      data: btoa(binary),
      error: h.captureError,
    };
  },
});
