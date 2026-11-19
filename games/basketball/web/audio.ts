import release from "../assets/release-manifest.json" with { type: "json" };
import type { GameAudioPort } from "../../../web/src/game-api/module";
import catalog from "../assets/audio-catalog.json" with { type: "json" };
import playback from "../assets/audio-playback.json" with { type: "json" };

export type BasketballAudioOptions = {

  assetsBase?: string;

  localBase?: string;
};

export type BasketballAudioReport = {
  ignored: boolean;
  unsupported: number[];
  failed: number[];
};

const assetPath = "/basketball/v1/audio";
const effects = new Map(catalog.effects.map((entry) => [entry.id, entry.wav]));
for (const [id, rendition] of Object.entries(playback.effects)) {
  effects.set(Number(id), rendition.wav);
}
type MusicRendition = {
  file: string;
  loop: boolean;
  revision: string;
  loopStartSeconds?: number;
};
const music = new Map<number, MusicRendition>();


for (const [id, rendition] of Object.entries(playback.music)) {
  const entry = rendition as Record<string, unknown>;
  if (
    typeof entry.file !== "string" ||
    entry.mime_type !== "audio/mpeg" ||
    typeof entry.distribution_sha256 !== "string" ||
    !/^[a-f0-9]{64}$/.test(entry.distribution_sha256)
  )
    continue;
  let loopStartSeconds: number | undefined;
  if (Number(id) === 76 || entry.loop_start_sample !== undefined) {
    const fields = [
      entry.sample_rate,
      entry.loop_start_sample,
      entry.loop_end_sample,
      entry.loop_samples,
      entry.decoded_samples,
    ];
    if (
      !fields.every(
        (v) => typeof v === "number" && Number.isSafeInteger(v) && v > 0,
      )
    )
      continue;
    const [rate, start, end, period, count] = fields as number[];
    if (
      entry.loop !== true ||
      rate !== 44100 ||
      start >= end ||
      end !== count ||
      end - start !== period
    )
      continue;
    loopStartSeconds = start / rate;
  }
  music.set(Number(id), {
    file: entry.file,
    loop: entry.loop === true,
    revision: entry.distribution_sha256,
    loopStartSeconds,
  });
}

export type BasketballMusicState = {
  epoch: number;
  trackID: number;
  assetSHA256: string;
  controlSequence: number;
  positionMS: number;
  at: number;
  paused: boolean;
};


export class BasketballAudio {
  private readonly port: GameAudioPort;
  private readonly options: BasketballAudioOptions;
  private lastSequence = -1;
  private disposed = false;
  private paused = false;
  private currentMusic?: number;

  constructor(port: GameAudioPort, options: BasketballAudioOptions = {}) {
    this.port = port;
    this.options = { ...options };
  }

  private sources(file: string): string[] {
    if (this.options.localBase && !this.options.assetsBase)
      return [`${this.options.localBase.replace(/\/+$/, "")}/${file}`];
    if (!this.options.assetsBase) {
      const files: Record<string, { url: string }> = release.files;
      const published = files[`${assetPath}/${file}`];
      if (!published) throw Error("篮球音频发布记录不完整");
      return [published.url];
    }
    const root = (this.options.assetsBase || release.assetsBaseURL).replace(/\/+$/, "");
    return [`${root}${assetPath}/${file}`];
  }





  async consume(
    soundIDs: readonly number[],
    sequence?: number,
  ): Promise<BasketballAudioReport> {
    const report: BasketballAudioReport = {
      ignored: false,
      unsupported: [],
      failed: [],
    };
    if (
      this.disposed ||
      (sequence !== undefined &&
        (!Number.isSafeInteger(sequence) ||
          sequence < 0 ||
          sequence <= this.lastSequence))
    ) {
      report.ignored = true;
      return report;
    }
    if (sequence !== undefined) this.lastSequence = sequence;
    const pending: Promise<number | undefined>[] = [];
    for (const id of soundIDs) {
      try {
        let played: Promise<boolean> | undefined;
        if (id === 0) {
          this.port.stop();
          this.paused = false;
          this.currentMusic = undefined;
        } else if (id === 253) {
          this.port.pause();
          this.paused = true;
        } else if (id === 255) {
          const hadMusic = this.currentMusic !== undefined;
          played = Promise.resolve(this.port.resume()).then(
            (resumed) => resumed || !hadMusic,
          );
          this.paused = false;
        } else if (effects.has(id))
          played = this.port.playWithFallback(this.sources(effects.get(id)!));
        else if (music.has(id)) {


          const track = music.get(id)!;


          if (this.paused) {
            void this.port.resume();
            this.paused = false;
          }
          if (this.currentMusic === id) this.port.stopMusic();
          this.currentMusic = id;
          played = this.port.music(this.sources(track.file), {
            loop: track.loop,
            ...(track.loopStartSeconds === undefined
              ? {}
              : { loopStartSeconds: track.loopStartSeconds }),
          });
        } else {
          report.unsupported.push(id);
        }
        if (played)
          pending.push(
            Promise.resolve(played).then(
              (ok) => (ok ? undefined : id),
              () => id,
            ),
          );
      } catch {
        pending.push(Promise.resolve(id));
      }
    }
    report.failed = (await Promise.all(pending)).filter(
      (id): id is number => id !== undefined,
    );
    return report;
  }




  async restoreMusic(
    state: BasketballMusicState,
    serverNow = state.at,
  ): Promise<boolean> {
    if (
      this.disposed ||
      !Number.isSafeInteger(state.epoch) ||
      state.epoch < 0 ||
      !Number.isSafeInteger(state.controlSequence) ||
      state.controlSequence < 0 ||
      !Number.isFinite(state.positionMS) ||
      state.positionMS < 0 ||
      !Number.isFinite(state.at) ||
      !Number.isFinite(serverNow)
    )
      return false;
    if (state.controlSequence > this.lastSequence) return false;
    try {
      if (state.trackID === 0) {
        this.port.stopMusic();
        this.currentMusic = undefined;
        this.paused = state.paused;
        if (state.paused) this.port.pause();
        return true;
      }
      const track = music.get(state.trackID);
      if (!track || track.revision !== state.assetSHA256) return false;
      const offset =
        (state.positionMS +
          (state.paused ? 0 : Math.max(0, serverNow - state.at))) /
        1000;
      if (this.paused) {
        void this.port.resume();
        this.paused = false;
      }
      this.currentMusic = state.trackID;
      const played = this.port.music(this.sources(track.file), {
        loop: track.loop,
        ...(track.loopStartSeconds === undefined
          ? {}
          : { loopStartSeconds: track.loopStartSeconds }),
        startOffset: offset,
        playbackID: `${state.epoch}:${state.assetSHA256}`,
      });
      if (state.paused) {
        this.port.pause();
        this.paused = true;
      }
      return await played;
    } catch {
      return false;
    }
  }


  reset(lastSequence = -1): void {
    if (this.disposed) return;
    this.lastSequence =
      Number.isSafeInteger(lastSequence) && lastSequence >= -1
        ? lastSequence
        : -1;
    this.currentMusic = undefined;
    this.paused = false;
    this.port.stop();
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    this.port.stop();
  }
}
