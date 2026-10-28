import type { GameAudioPort } from "../../../web/src/game-api/module";

export type ContraAudioVoice = {
  key: string;
  id: number;
  channel: string;
  playbackID: string;
  positionTicks: number;
  paused: boolean;
  muted: boolean;
  restore: boolean;
  continuous: boolean;
};
export type ContraAudioEvent = {
  sequence: number;
  tick: number;
  action: string;
  key?: string;
  value: boolean;
  voice?: ContraAudioVoice | null;
};
export type ContraAudioView = {
  epoch: string;
  sequence: number;
  tick: number;
  paused: boolean;
  voices?: readonly ContraAudioVoice[] | null;
  events?: readonly ContraAudioEvent[] | null;
};
export type ContraAudioResource = {
  sources: readonly string[];
  loop: boolean;
  loopStartSeconds?: number;
  tickSeconds: number;
};
export type ContraAudioResources = {
  resolve: (voice: ContraAudioVoice) => ContraAudioResource | undefined;
  // One proven terminal carry: full original $6E, never generic SFX recovery.
  resolveTerminalCue?: (
    voice: ContraAudioVoice,
  ) => ContraAudioResource | undefined;
};
type Stem = {
  channel: string;
  file: string;
  sha256: string;
  researchDirectory?: string;
};
type SourceCatalog = {
  sounds: readonly {
    id: number;
    stems: readonly Stem[];
    file?: string;
    researchDirectory?: string;
    sha256?: string;
  }[];
};
type LoopCatalog = {
  sampleRate: number;
  songs: readonly {
    id: number;
    pcmSeamReady: boolean;
    nativeBoundaryReady: boolean;
    loopReady: boolean;
    loopStartSeconds: number;
    periodTicks: number;
    periodSamples: number;
    stems: readonly Stem[];
  }[];
};
// A caller maps a retained path + digest to a verified local or immutable CDN
// resource. Research filesystem paths are never passed directly to the browser.
export function createContraAudioResources(
  source: SourceCatalog,
  loops: LoopCatalog,
  resolve: (path: string, sha256: string) => readonly string[] | undefined,
): ContraAudioResources {
  return {
    resolveTerminalCue(voice) {
      if (voice.id !== 0x6e || voice.continuous) return;
      const cue = source.sounds.find((s) => s.id === 0x6e);
      if (
        !cue?.file ||
        !cue.researchDirectory ||
        !cue.sha256 ||
        !/^[a-f0-9]{64}$/i.test(cue.sha256)
      )
        return;
      const sources = resolve(
        `${cue.researchDirectory}/${cue.file}`,
        cue.sha256,
      );
      if (!sources?.length) return;
      return { sources, loop: false, tickSeconds: 0.016639 };
    },
    resolve(voice) {
      let stem: Stem | undefined;
      let loopStartSeconds: number | undefined;
      let tickSeconds = 0.016639;
      if (voice.continuous) {
        const song = loops.songs.find((s) => s.id === voice.id);
        if (!song?.pcmSeamReady || !song.nativeBoundaryReady || !song.loopReady)
          return;
        if (!["pulse1", "pulse2", "triangle"].includes(voice.channel)) return;
        stem = song.stems.find((s) => s.channel === voice.channel);
        loopStartSeconds = song.loopStartSeconds;
        tickSeconds = song.periodSamples / loops.sampleRate / song.periodTicks;
      } else {
        stem = source.sounds
          .find((s) => s.id === voice.id)
          ?.stems.find((s) => s.channel === voice.channel);
      }
      if (!stem || !/^[a-f0-9]{64}$/i.test(stem.sha256)) return;
      if (!Number.isFinite(tickSeconds) || tickSeconds <= 0) return;
      const path = stem.researchDirectory
        ? `${stem.researchDirectory}/${stem.file}`
        : stem.file;
      const sources = resolve(path, stem.sha256);
      if (!sources?.length) return;
      return { sources, loop: voice.continuous, loopStartSeconds, tickSeconds };
    },
  };
}

type Port = Pick<
  GameAudioPort,
  | "track"
  | "stopTrack"
  | "muteTrack"
  | "pauseTrack"
  | "resumeTrack"
  | "seekTrack"
  | "restoreTrack"
>;
type Lane = {
  voice: ContraAudioVoice;
  resource?: ContraAudioResource;
  stopped: boolean;
  settled: boolean;
};

// All audible work stays in BoardProps.audio. This class owns only ordered game
// event identities and semantic lanes; it never creates or inspects media nodes.
export class ContraAudioAdapter {
  private port: Port;
  private resources: ContraAudioResources;
  private epoch: string;
  private sequence?: number;
  private tick = -1;
  private lifecycle = 0;
  private disposed = false;
  private lanes = new Map<string, Lane>();
  private missing = new Set<string>();
  private visible = new Map<string, ContraAudioVoice>();
  private suspended = false;
  private finalized = false;

  constructor(
    port: Port,
    resources: ContraAudioResources,
    expectedEpoch: string,
  ) {
    this.port = port;
    this.resources = resources;
    this.epoch = expectedEpoch;
  }
  bindEpoch(expectedEpoch: string) {
    if (this.disposed || this.epoch === expectedEpoch) return;
    this.stop();
    this.lanes.clear();
    this.missing.clear();
    this.epoch = expectedEpoch;
    this.sequence = undefined;
    this.tick = -1;
    this.finalized = false;
  }
  get missingResources(): readonly string[] {
    return [...this.missing].sort();
  }
  private key(key: string) {
    return `contra/${key}`;
  }
  private stopLane(key: string) {
    const lane = this.lanes.get(key);
    if (!lane || lane.stopped) return;
    this.port.stopTrack(this.key(key));
    lane.stopped = true;
  }
  stop() {
    this.lifecycle++;
    this.suspended = false;
    this.visible.clear();
    for (const key of this.lanes.keys()) this.stopLane(key);
  }
  // Visibility suspension differs from permanent stop/failure/disposal. Only
  // the next explicit authority resync may restore a persistent music loop.
  suspend() {
    if (this.disposed) return;
    this.stop();
    this.suspended = true;
  }
  dispose() {
    if (!this.disposed) this.stop();
    this.disposed = true;
  }

  private start(voice: ContraAudioVoice, override?: ContraAudioResource) {
    const current = this.visible.get(voice.key);
    // A terminal publication can contain several audio updates. Start the
    // surviving voice at the age of the displayed state, not its earlier event.
    if (current?.playbackID === voice.playbackID) voice = current;
    const before = this.lanes.get(voice.key);
    // A same-ID tombstone is never retried after mute, hidden, end or stop.
    if (before?.voice.playbackID === voice.playbackID) return;
    this.stopLane(voice.key);
    const resource = override ?? this.resources.resolve(voice);
    const lane: Lane = {
      voice: { ...voice },
      resource,
      stopped: false,
      settled: false,
    };
    this.lanes.set(voice.key, lane);
    if (!resource) {
      this.missing.add(`${voice.id}:${voice.channel}`);
      lane.stopped = true;
      return;
    }
    const key = this.key(voice.key);
    // Paused restore must be silent from the very first native play() call.
    const pending = this.port.track(key, resource.sources, {
      playbackID: voice.playbackID,
      loop: resource.loop,
      loopStartSeconds: resource.loopStartSeconds,
      startOffset: voice.positionTicks * resource.tickSeconds,
      // The authority/presentation clock is 60 Hz; media was captured at the
      // original NTSC update interval. Offsets remain in source media seconds.
      rate: resource.tickSeconds * 60,
      muted: voice.muted || voice.paused,
    });
    if (voice.paused) {
      this.port.pauseTrack(key);
      this.port.muteTrack(key, voice.muted);
    }
    this.observePlayback(voice.key, lane, pending, voice.positionTicks);
  }
  private align(key: string, lane: Lane) {
    const visible = this.visible.get(key);
    if (
      lane.stopped ||
      !lane.resource ||
      visible?.playbackID !== lane.voice.playbackID
    )
      return;
    this.port.seekTrack(
      this.key(key),
      visible.positionTicks * lane.resource.tickSeconds,
    );
  }
  private observePlayback(
    key: string,
    lane: Lane,
    pending: Promise<boolean>,
    startedAt: number,
  ) {
    const lifecycle = this.lifecycle;
    void pending.then(
      (ok) => {
        if (
          this.disposed ||
          this.lifecycle !== lifecycle ||
          this.lanes.get(key) !== lane ||
          lane.stopped
        )
          return;
        lane.settled = true;
        // Slow metadata/fallback must not permanently lag the displayed state.
        // Active-only seek cannot replay a stopped/ended/muted native voice.
        if (ok && this.visible.get(key)?.positionTicks !== startedAt)
          this.align(key, lane);
      },
      () => {
        if (
          !this.disposed &&
          this.lifecycle === lifecycle &&
          this.lanes.get(key) === lane
        )
          lane.settled = true;
      },
    );
  }
  private apply(event: ContraAudioEvent) {
    if (event.action === "reset") {
      this.lifecycle++;
      this.suspended = false;
      for (const key of this.lanes.keys()) this.stopLane(key);
      return;
    }
    if (!event.key) return;
    if (event.action === "start" && event.voice) {
      this.start(event.voice);
      return;
    }
    const lane = this.lanes.get(event.key);
    if (!lane || lane.stopped) return;
    const key = this.key(event.key);
    switch (event.action) {
      case "stop":
        this.stopLane(event.key);
        break;
      case "mute":
        lane.voice.muted = event.value;
        if (!event.value) this.align(event.key, lane);
        this.port.muteTrack(key, event.value);
        break;
      case "pause":
        lane.voice.paused = true;
        this.port.pauseTrack(key);
        this.align(event.key, lane);
        break;
      case "resume":
        lane.voice.paused = false;
        this.align(event.key, lane);
        void this.port.resumeTrack(key).catch(() => false);
        break;
    }
  }
  private resync(view: ContraAudioView) {
    const restoreSuspended = this.suspended;
    this.suspended = false;
    const music = new Map(
      (view.voices ?? []).filter((v) => v.restore).map((v) => [v.key, v]),
    );
    for (const [key, lane] of this.lanes) {
      if (
        !music.has(key) ||
        music.get(key)?.playbackID !== lane.voice.playbackID
      )
        this.stopLane(key);
    }
    for (const voice of music.values()) {
      const lane = this.lanes.get(voice.key);
      if (!lane || lane.voice.playbackID !== voice.playbackID) {
        this.start(voice);
        continue;
      }
      if (!lane.resource) continue;
      if (lane.stopped) {
        if (
          restoreSuspended &&
          voice.restore &&
          voice.continuous &&
          lane.resource.loop
        ) {
          lane.voice = { ...voice };
          lane.stopped = false;
          lane.settled = false;
          const pending = this.port.restoreTrack(
            this.key(voice.key),
            voice.playbackID,
            {
              startOffset: voice.positionTicks * lane.resource.tickSeconds,
              muted: voice.muted,
              paused: voice.paused,
            },
          );
          this.observePlayback(voice.key, lane, pending, voice.positionTicks);
        }
        continue;
      }
      const key = this.key(voice.key);
      // Soft mute first; resuming a pulse overridden by slot4 must not blip.
      this.port.muteTrack(key, voice.muted);
      if (voice.paused && !lane.voice.paused) this.port.pauseTrack(key);
      this.port.seekTrack(key, voice.positionTicks * lane.resource.tickSeconds);
      if (!voice.paused && lane.voice.paused)
        void this.port.resumeTrack(key).catch(() => false);
      lane.voice = { ...voice };
    }
  }
  private carryTerminalCue(view: ContraAudioView): boolean {
    const voices = view.voices ?? [];
    const base = voices.find((v) => v.key === "slot-0");
    // These three same-age tonal lanes are the original $6E entry. No other
    // active effect may be silently folded into an unrelated complete mix.
    if (
      !base ||
      voices.length !== 3 ||
      !["slot-0", "slot-1", "slot-2"].every((k) =>
        voices.some((v) => v.key === k),
      ) ||
      voices.some(
        (v) =>
          v.id !== 0x6e ||
          v.continuous ||
          v.paused ||
          v.positionTicks !== base.positionTicks,
      )
    )
      return false;
    const resource = this.resources.resolveTerminalCue?.(base);
    if (!resource || resource.loop) {
      this.missing.add("110:terminal-cue");
      return false;
    }
    this.lifecycle++;
    for (const key of this.lanes.keys()) this.stopLane(key);
    this.start(
      {
        ...base,
        key: "terminal-cue",
        channel: "full",
        restore: false,
        muted: false,
      },
      resource,
    );
    return true;
  }
  update(
    view: ContraAudioView,
    options: { resync?: boolean; finalized?: boolean } = {},
  ) {
    if (
      this.disposed ||
      view.epoch !== this.epoch ||
      view.tick < this.tick ||
      (this.sequence !== undefined && view.sequence < this.sequence)
    )
      return;
    this.visible = new Map((view.voices ?? []).map((v) => [v.key, { ...v }]));
    const first = this.sequence === undefined;
    const events = (view.events ?? []).filter(
      (e) => e.sequence > (this.sequence ?? view.sequence),
    );
    let next = this.sequence ?? view.sequence;
    const continuous =
      events.every((e) => e.sequence === ++next) && next === view.sequence;
    const terminalEdge = options.finalized === true && !this.finalized;
    this.finalized ||= options.finalized === true;
    if (first || options.resync || !continuous) this.resync(view);
    else if (!(terminalEdge && this.carryTerminalCue(view))) {
      for (const event of events) this.apply(event);
    }
    for (const [key, voice] of this.visible) {
      const lane = this.lanes.get(key);
      if (lane?.voice.playbackID === voice.playbackID)
        lane.voice = { ...voice };
    }
    this.sequence = view.sequence;
    this.tick = view.tick;
  }
}
