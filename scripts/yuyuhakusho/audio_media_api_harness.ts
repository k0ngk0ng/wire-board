import { GameAudio } from "../../web/src/audio";

type Row = { kind: string; id: number; at: number; time: number; duration: number | null; paused: boolean; ended: boolean; src: string; loop: boolean; rate: number };
const elements: HTMLAudioElement[] = [];
const events: Row[] = [];
const platform = new GameAudio(() => {
  const media = new Audio();
  const id = elements.push(media) - 1;
  for (const kind of ["loadstart", "loadedmetadata", "play", "playing", "pause", "ended", "seeking", "seeked", "waiting", "stalled", "error", "timeupdate"])
    media.addEventListener(kind, () => events.push({ kind, id, at: performance.now(), time: media.currentTime, duration: Number.isFinite(media.duration) ? media.duration : null, paused: media.paused, ended: media.ended, src: media.currentSrc, loop: media.loop, rate: media.playbackRate }), { capture: true });
  return media;
});
platform.mount();
const port = platform.channel("yuyu-media-verifier");
const other = platform.channel("other-room-verifier");
const h = {
  ready: true, running: false, complete: false, error: "", events,
  checks: [] as unknown[], overlap: undefined as unknown,
  visibility: [] as { hidden: boolean; at: number }[],
  captureSupported: false, captureError: "", recorder: undefined as MediaRecorder | undefined,
  chunks: [] as Blob[], recording: undefined as unknown,
  platform, port, other,
  snapshot() {
    return elements.map((media, id) => ({ id, connected: media.isConnected, src: media.currentSrc, attribute: media.getAttribute("src"), time: media.currentTime, duration: Number.isFinite(media.duration) ? media.duration : null, paused: media.paused, muted: media.muted, ended: media.ended, loop: media.loop, rate: media.playbackRate, platform: media.hasAttribute("data-platform-audio"), music: media.hasAttribute("data-platform-music"), loopKey: media.dataset.platformLoop ?? null }));
  },
};
Object.assign(window, { yuyuMediaAPI: h });
const wait = async (predicate: () => boolean, limit = 18000) => {
  const started = performance.now();
  while (!predicate()) {
    if (performance.now() - started > limit) throw Error("native media predicate timed out");
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
};
const assert = (condition: unknown, message: string) => { if (!condition) throw Error(message); };
const music = () => elements.find((media) => media.hasAttribute("data-platform-music"))!;
const active = () => h.snapshot().filter((media) => media.attribute && !media.paused);
const check = (kind: string, value: object) => h.checks.push({ kind, ...value });
async function recordStart(media: HTMLAudioElement) {
  h.captureError = ""; h.chunks = [];
  try {
    const source = media as HTMLAudioElement & { captureStream?: () => MediaStream };
    if (!source.captureStream || !MediaRecorder.isTypeSupported("audio/webm;codecs=opus")) return false;
    const stream = source.captureStream();
    h.recorder = new MediaRecorder(stream, { mimeType: "audio/webm;codecs=opus" });
    h.recorder.ondataavailable = (event) => { if (event.data.size) h.chunks.push(event.data); };
    h.recorder.start(); h.captureSupported = true; return true;
  } catch (error) { h.captureError = String(error); return false; }
}
async function recordStop() {
  const recorder = h.recorder;
  if (!recorder) return null;
  if (recorder.state !== "inactive") await new Promise<void>((resolve) => { recorder.addEventListener("stop", () => resolve(), { once: true }); recorder.stop(); });
  const blob = new Blob(h.chunks, { type: recorder.mimeType });
  const bytes = new Uint8Array(await blob.arrayBuffer());
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 16384) binary += String.fromCharCode(...bytes.subarray(offset, offset + 16384));
  return { mime: recorder.mimeType, bytes: blob.size, data: btoa(binary), error: h.captureError };
}

document.addEventListener("visibilitychange", () => {
  h.visibility.push({ hidden: document.hidden, at: performance.now() });
  if (document.hidden) platform.stop();
});

document.querySelector<HTMLButtonElement>("#start")!.onclick = async () => {
  if (h.running) return;
  h.running = true;
  try {
    assert(await platform.unlock(), "trusted gesture did not unlock native media");
    const base = performance.now();
    assert(await port.playWithFallback(["/audio/1f.wav"]), "first actual finite effect did not play");
    await wait(() => elements[0].currentTime > 0.12);
    const first = h.snapshot();
    assert(await port.playWithFallback(["/audio/3f.wav"]), "second actual finite effect did not play");
    await wait(() => elements[0].currentTime > 0.08 && elements[0].currentSrc.endsWith("/3f.wav"));
    const second = h.snapshot();
    h.overlap = { expectedIndependentFiniteVoices: 2, actualIndependentFiniteVoices: active().filter((media) => !media.music).length, first, second, events: events.filter((row) => row.at >= base) };
    check("finite-overlap-capability", { supported: active().filter((media) => !media.music).length === 2, evidence: h.overlap });
    port.stop();

    assert(await port.music(["/audio/8f.wav"], { loop: false, playbackID: "finite-1" }), "finite original music did not play");
    assert(await port.playWithFallback(["/audio/1f.wav"]), "finite effect beside music did not play");
    await wait(() => active().length === 2 && music().currentTime > 0.1);
    const simultaneous = h.snapshot();
    port.stopMusic();
    assert(!music().getAttribute("src") && elements[0].getAttribute("src")?.endsWith("/1f.wav") && !elements[0].paused, "stopMusic affected finite effect or retained music");
    check("music-and-finite-effect", { simultaneous, afterStopMusic: h.snapshot() });
    port.stop();

    assert(await port.music(["/audio/8f.wav"], { loop: false, startOffset: 1.25, playbackID: "offset-1" }), "actual offset finite music did not play");
    await wait(() => music().currentTime >= 1.25);
    const offset = h.snapshot();
    assert(!music().loop && Math.abs(music().duration - 4.172040816326531) < 1 / 44100, "finite original duration or loop flag changed");
    const plays = events.filter((row) => row.kind === "play" && row.id === elements.indexOf(music())).length;
    const beforeRefresh = music().currentTime;
    assert(await port.music(["/audio/8f.wav"], { loop: false, startOffset: 0, playbackID: "offset-1" }), "stable original music refresh failed");
    assert(events.filter((row) => row.kind === "play" && row.id === elements.indexOf(music())).length === plays && music().currentTime >= beforeRefresh, "stable music instance restarted");
    assert(await port.music(["/audio/8f.wav"], { loop: false, startOffset: 0, playbackID: "offset-2" }), "new original music instance did not restart");
    assert(music().currentTime < 0.3, "new music instance retained old offset");
    check("finite-offset-and-instance", { initialOffset: offset, afterNewInstance: h.snapshot() });
    assert(await port.playWithFallback(["/audio/1f.wav"]), "pause fixture effect failed");
    await wait(() => music().currentTime > 0.15 && elements[0].currentTime > 0.1);
    port.pause(); const paused = h.snapshot();
    await new Promise((resolve) => setTimeout(resolve, 250));
    const still = h.snapshot();
    for (const prior of paused.filter((row) => row.attribute)) {
      const after = still.find((row) => row.id === prior.id)!;
      assert(after.paused && Math.abs(after.time - prior.time) < 0.001, "paused native media position advanced");
    }
    assert(await port.resume(), "paused owner failed native resume");
    const resumed = h.snapshot();
    for (const prior of paused.filter((row) => row.attribute)) {
      const after = resumed.find((row) => row.id === prior.id)!;
      assert(!after.paused && after.attribute === prior.attribute && Math.abs(after.time - prior.time) < 0.15, "resume reloaded or rewound native source");
    }
    check("native-pause-resume", { paused, still, resumed });
    port.stop();

    const expired = await port.music(["/audio/8f.wav"], { loop: false, startOffset: 90, playbackID: "expired" });
    assert(!expired && !music().getAttribute("src") && music().paused, "expired finite restore replayed from start");
    check("expired-finite-restore", { returned: expired, state: h.snapshot() });
    assert(await port.music(["/missing/8f.wav", "/audio/8f.wav"], { loop: false, startOffset: 0.75, playbackID: "fallback" }), "real missing source did not fallback");
    assert(music().currentSrc.endsWith("/audio/8f.wav") && music().currentTime >= 0.75, "real fallback lost original offset");
    check("real-fallback-offset", { state: h.snapshot() });
    port.stop();

    const loopStarted = performance.now();
    assert(await port.music(["/diagnostic-loop.wav"], { loop: true, loopStartSeconds: 1, playbackID: "diagnostic-suffix" }), "diagnostic suffix fixture did not start");
    assert(!music().loop && music().playbackRate === 1 && music().duration === 3, "diagnostic natural suffix fixture was shortened or accelerated");
    await recordStart(music());
    await wait(() => events.filter((row) => row.kind === "ended" && row.at >= loopStarted).length >= 2, 14000);
    await wait(() => music().currentTime > 1.15 && music().currentTime < 2);
    h.recording = await recordStop();
    const loopEvents = events.filter((row) => row.at >= loopStarted);
    const ended = loopEvents.filter((row) => row.kind === "ended");
    const boundaries = ended.slice(0, 2).map((row) => {
      const playing = loopEvents.find((candidate) => candidate.kind === "playing" && candidate.at >= row.at)!;
      const seeked = loopEvents.find((candidate) => candidate.kind === "seeked" && candidate.at >= row.at)!;
      assert(playing && seeked && playing.time >= 1 && playing.time < 1.15, "natural suffix returned to intro or failed to resume");
      return { ended: row, seeked, playing, endedToPlayingMS: playing.at - row.at, endedToSeekedMS: seeked.at - row.at };
    });
    assert(ended[1].at - ended[0].at > 1900 && ended[1].at - ended[0].at < 2600, "natural suffix did not traverse the full two-second body");
    check("two-natural-suffix-boundaries", { diagnostic: true, sampleRate: 44100, prefixFrames: 44100, bodyFrames: 88200, boundaries, events: loopEvents, state: h.snapshot() });

    const stale = music().onended;
    platform.setEnabled(false); stale?.call(music(), new Event("ended"));
    assert(h.snapshot().every((row) => !row.attribute && row.paused), "mute retained or restarted native source");
    platform.setEnabled(true); stale?.call(music(), new Event("ended"));
    assert(h.snapshot().every((row) => !row.attribute && row.paused), "unmute replayed stale native source");
    check("native-mute-stale-ended", { state: h.snapshot() });
    assert(await port.music(["/audio/8f.wav"], { loop: false, playbackID: "old-owner" }), "old owner source did not start");
    const oldEnded = music().onended;
    assert(await other.music(["/audio/1f.wav"], { loop: false, startOffset: 0.25, playbackID: "other-owner" }), "other owner source did not start");
    oldEnded?.call(music(), new Event("ended")); port.stop();
    assert(music().getAttribute("src")?.endsWith("/audio/1f.wav") && !music().paused && music().currentTime >= 0.25, "old owner or callback cancelled successor");
    check("native-owner-replacement", { state: h.snapshot() });
    other.stop();
    assert(await port.music(["/diagnostic-loop.wav"], { playbackID: "visibility-fixture" }), "visibility fixture did not start");
    h.complete = true;
    document.querySelector("pre")!.textContent = JSON.stringify({ complete: true, checks: h.checks.length, finiteOverlapSupported: false, sampleGaplessVerified: false }, null, 2);
  } catch (error) {
    h.error = String(error);
    document.querySelector("pre")!.textContent = h.error;
  } finally { h.running = false; }
};
