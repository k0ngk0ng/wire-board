import { GameAudio } from "../../web/src/audio";
import { BasketballAudio } from "../../games/basketball/web/audio";

const platform = new GameAudio();
platform.mount();
const port = platform.channel("basketball-audio-loop-acceptance");
const adapter = new BasketballAudio(port, { localBase: "/audio", assetsBase: new URLSearchParams(location.search).has("fallback") ? "/cdn-unavailable" : undefined });
const musicID = Number(new URLSearchParams(location.search).get("track") ?? "66");
const codecProbe = new URLSearchParams(location.search).get("probe");
const status = document.querySelector<HTMLElement>("#status")!;
const harness = {
  platform, adapter, started: false, error: "", loops: 0,
  samples: [] as { at: number; time: number; paused: boolean; ended: boolean }[],
  media: undefined as HTMLAudioElement | undefined,
};
Object.assign(window, { basketballLoopAcceptance: harness });
document.querySelector<HTMLButtonElement>("#start")!.onclick = async () => {
  try {
    await platform.unlock();
    if (codecProbe) {
      if (!(await port.music([`/audio/${encodeURIComponent(codecProbe)}`], {loop: true}))) throw new Error("Codec probe failed to start");
    } else {
      const result = await adapter.consume([musicID], 1);
      if (result.failed.length || result.unsupported.length) throw new Error("Original music failed to start");
    }
    const media = document.querySelector<HTMLAudioElement>("audio[data-platform-music]")!;
    harness.media = media;
    let previous = 0;
    media.addEventListener("timeupdate", () => {
      if (previous > media.duration - 1 && media.currentTime < 1) harness.loops++;
      previous = media.currentTime;
      harness.samples.push({ at: performance.now(), time: media.currentTime, paused: media.paused, ended: media.ended });
      status.textContent = JSON.stringify({ loops: harness.loops, currentTime: media.currentTime, duration: media.duration, paused: media.paused, ended: media.ended });
    });
    harness.started = true;
  } catch (error) { harness.error = String(error); status.textContent = harness.error; }
};
