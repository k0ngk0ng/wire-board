import type { GameAudioPort } from "../../../web/src/game-api/module";
import catalog from "../assets/audio-catalog.json" with { type: "json" };
import playback from "../assets/audio-playback.json" with { type: "json" };

export type BasketballAudioOptions = {
  /** CDN root; the game appends its immutable versioned asset path. */
  assetsBase?: string;
  /** Explicit local preview directory, or the future packaged fallback path. */
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
// The same integer sample metadata is embedded by the server. Never infer a
// loop region from a filename, the preview, or the source scene's video clock.
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

/** Original event selection only; playback, lifecycle and mute belong to the platform. */
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
    const local = `${(this.options.localBase ?? assetPath).replace(/\/+$/, "")}/${file}`;
    const root = this.options.assetsBase?.replace(/\/+$/, "");
    return root ? [`${root}${assetPath}/${file}`, local] : [local];
  }

  /** Call once for each new engine event batch, not for snapshot rendering.
   * Supplying a monotonically increasing sequence also rejects repeated/old
   * batches. Media promises never delay issuing later controls in this batch.
   */
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
          // Only the separately verified original-PCM derivative loops. The
          // untouched 133-second research recording is never hard-looped.
          const track = music.get(id)!;
          // Original $80E7 clears pause bit $40 for a new music command.
          // The same ID is a fresh request too, not a deduplicated UI refresh.
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

  /** Call after establishing the snapshot watermark. Only current background
   * state is restored; past effects never pass through consume(). Positions
   * use the authoritative server media wall clock, at playbackRate=1. */
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

  /** Stop the previous session and establish the restored event watermark. */
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
