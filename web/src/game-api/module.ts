import type { ComponentType } from "react";

export type GameRoom = {
  ratingPolicy?: string;
  ratingStarted?: boolean;
  ratingEligible?: boolean;
  matchId?: string;
  loading?: {
    token: string;
    build: string;
    frozenAt: number;
    ready: boolean[];
  } | null;
  clockOffsetMs?: number;
  id: string;
  name: string;
  kind: string;
  host: string;
  capacity: number;
  status: string;
  version: number;
  you: number;
  spectating?: boolean;
  supportsSave?: boolean;
  supportsCheckpoint?: boolean;
  checkpointSavedAt?: number;
  saveVotes?: string[];
  turnDeadline?: number;
  options?: unknown;
  seats: {
    network?: import("../network-quality").NetworkInfo;
    id: string;
    name: string;
    ready: boolean;
    bot?: boolean;
    autoPlay?: boolean;
    left?: boolean;
  }[];
  game?: {
    restartable?: boolean;
    paused?: boolean;
    pauseExpiresAt?: number;
    data?: unknown;
    actors?: number[];
    attention?: number[];
    eliminated?: number[];
    systemDeadline?: number;
    finished: boolean;
    winners: number[];
    log: string[];
  };
};
export type GameAudioPort = {
  prepare: (sources: readonly string[], options?: { signal?: AbortSignal }) => Promise<boolean>;
  play: (sources: readonly string[], options?: { volume?: number }) => Promise<boolean>;
  // Alternatives for one clip; unlike play(), these are not a playlist.
  playWithFallback: (sources: readonly string[]) => Promise<boolean>;
  // Finite native voices: at most16 stable lanes/owner and32/platform.
  // IDs identify instances, not lanes; identical updates never restart a clip.
  // reuseLoaded opts repeated short effects into native source/decoder reuse.
  effect: (
    lane: string,
    sources: readonly string[],
    options: {
      playbackID: string;
      startOffset?: number;
      reuseLoaded?: boolean;
    },
  ) => Promise<boolean>;
  stopEffect: (lane: string, expectedPlaybackID?: string) => void;
  transient: (key: string, sources: readonly string[], options: { playbackID: string }) => Promise<boolean>;
  spatial: (key: string, sources: readonly string[], options: import("../audio-spatial").SpatialOptions) => Promise<boolean>;
  spatialListener: (listener: import("../audio-spatial").SpatialListener) => void;
  stopSpatial: (key?: string) => void;
  openStream: (
    options: import("./audio-stream").GameAudioStreamConfig,
  ) => import("./audio-stream").GameAudioStreamHandle | undefined;
  openPCMStream: (
    options: import("./audio-pcm-stream").GameAudioPCMStreamConfig,
  ) => import("./audio-pcm-stream").GameAudioPCMStreamHandle | undefined;
  stop: () => void;
  // Pause preserves this owner's media positions; stop/mute/cleanup cancels them.
  pause: () => void;
  resume: () => Promise<boolean>;
  music: (
    sources: readonly string[],
    // Offset is elapsed media seconds. With loopStartSeconds, the prefix plays
    // once and elapsed positions wrap only within the suffix. A stable
    // playbackID prevents snapshot refreshes from restarting the same track.
    options?: {
      volume?: number;
      loop?: boolean;
      startOffset?: number;
      playbackID?: string;
      loopStartSeconds?: number;
    },
  ) => Promise<boolean>;
  stopMusic: () => void;
  // Independently keyed continuous effects; sources are fallback alternatives.
  // Repeated calls adjust loudness/pitch without restarting the media.
  loop: (
    key: string,
    sources: readonly string[],
    // Opt-in short-loop phase buffers still output through one platform-owned
    // native HTMLAudio MediaStream; other games retain the file-loop path.
    options?: {
      volume?: number;
      rate?: number;
      continuous?: boolean;
      seamOverlapMS?: number;
    },
  ) => Promise<boolean>;
  stopLoop: (key: string) => void;
  // Owner-scoped native media lanes; default is finite playback. Sources are
  // primary/fallback alternatives. Supply an epoch/event playbackID to prevent
  // snapshot replay of that lane's latest event after end, stop or mute. The game
  // still filters older/out-of-order events. Use stable keys (max 64 lanes).
  track: (
    key: string,
    sources: readonly string[],
    options?: {
      loop?: boolean;
      startOffset?: number;
      loopStartSeconds?: number;
      playbackID?: string;
      muted?: boolean;
      // Native playback rate, bound to this event identity; offsets remain media seconds.
      rate?: number;
    },
  ) => Promise<boolean>;
  stopTrack: (key: string) => void;
  // Soft mute keeps the lane's native time advancing. Pause freezes its position.
  muteTrack: (key: string, muted: boolean) => void;
  pauseTrack: (key: string) => void;
  resumeTrack: (key: string) => Promise<boolean>;
  // Reposition only a live owner track; never play/resume/recreate it. True also
  // accepts a pending metadata seek. Loop offsets use elapsed prefix/suffix time.
  seekTrack: (key: string, offsetSeconds: number) => boolean;
  // Explicit authority resync of a retained continuous track only. Reuse this
  // owner's exact playbackID/source/loop; never revive finite effects or muted
  // global output. Paused restores make no native play call until resumed.
  restoreTrack: (
    key: string,
    playbackID: string,
    options: { startOffset: number; muted?: boolean; paused?: boolean },
  ) => Promise<boolean>;
};
export type GameClock = {
  paused?: boolean;
  serverNow: number;
  receivedAt: number;
  epoch?: number;
};
export type PresentationSnapshot = { room: GameRoom; clock: GameClock };
export type PresentationFrame = PresentationSnapshot & {
  blocked: boolean;
  wakeAt?: number;
};
export type RoomPresentation = {
  update: (
    snapshot: PresentationSnapshot,
    now: number,
    reset: boolean,
  ) => PresentationFrame;
  sample: (now: number) => PresentationFrame;
};

export type GameControllerBindings = {
  keyboard: Readonly<Record<string, string>>;
  keyboardAlt?: Readonly<Record<string, readonly string[]>>;
};

export type BoardProps<Data = unknown> = {
  onReady?: () => void;
  onLoadProgress?: (
    percent: number,
    label: string,
    bytes?: { loaded: number; total: number },
  ) => void;
  loadSizes?: Readonly<Record<string, number>>;
  onLoadError?: (error: string) => void;
  room: GameRoom;
  data: Data;
  act: (action: Record<string, unknown>) => Promise<void>;
  inputWindow?: number;
  command: (type: string, extra?: Record<string, unknown>) => Promise<void>;
  busy: boolean;
  assets: string;
  sound: boolean;
  audioEnabled?: boolean;
  audio: GameAudioPort;
  clock: GameClock;
  controllerBindings?: GameControllerBindings;
};
export type OptionsProps<Options = unknown> = {
  value: Options;
  onChange: (value: Options) => void;
  capacity: number;
  disabled: boolean;
};
export type PreparationProps<Options = unknown> = {
  room: GameRoom;
  options: Options;
  host: boolean;
  busy: boolean;
  assets: string;
  prepare: (action: Record<string, unknown>) => void;
};
export type PlayerGroup = {
  id: string;
  label: string;
  seats: readonly number[];
};
export type GameModule<Data = unknown, Options = unknown> = {
  ratingPolicy?: "coop_campaign_v1";
  id: string;
  name: string;
  description: string;
  category?: "board" | "poker" | "realtime" | "sports" | "puzzle" | "action";
  // Presentation is independent of the catalogue category.
  tableLayout?: "poker";
  requiresFullTable?: boolean;
  minPlayers: number;
  maxPlayers: number;
  defaultPlayers?: number;
  playerCounts?: readonly number[];
  supportsBots?: boolean;
  mobilePlayNotice?: boolean;
  ownsSounds?: boolean;
  // Continuous controls stay responsive while an earlier request is in flight.
  // The module must coalesce input and validate its own sequence/epoch on server.
  realtime?: boolean;
  streamInputOnly?: boolean;
  mergeStreamData?: (previous: unknown, incoming: unknown) => unknown;
  // Allow only input cleanup during manual autoplay, never a gameplay action.
  // The server must independently declare and validate the same capability.
  isInputRelease?: (action: Record<string, unknown>) => boolean;
  defaultOptions?: Options;
  resizeOptions?: (options: Options, capacity: number) => Options;
  // Optional game-owned visual timeline. Preserve host room metadata; never
  // modify authoritative snapshots or decide outcomes in this adapter.
  createPresentation?: () => RoomPresentation;
  resources?: (room: GameRoom, media: Record<string, number>) => string[];
  residentResources?: (room: GameRoom, media: Record<string, number>, assets: string) => readonly import("./resident-assets").ResidentResource[] | Promise<readonly import("./resident-assets").ResidentResource[]>;
  assetLoading?: "board";
  asyncBoard?: boolean;
  Board: ComponentType<BoardProps<Data>>;
  Options?: ComponentType<OptionsProps<Options>>;
  // Waiting-room extension inside the shared shell; never part of creation.
  Preparation?: ComponentType<PreparationProps<Options>>;
  // Creation keeps Options; this waiting-room preparation owns its controls.
  preparationOwnsOptions?: boolean;
  playerLayout?: {
    flow?: "horizontal";
    density?: "compact";
    groups?: (room: GameRoom, data: Data | undefined) => readonly PlayerGroup[];
  };
  // Reserve only the measured visible remainder for the playable surface.
  viewportFit?: boolean;
  // Reserve a game's full-width aspect in ordinary pages instead of reducing
  // its playable surface to accommodate the surrounding page controls.
  viewportAspect?: number;
  // Overlay fullscreen platform actions without reserving playable space.
  fullscreenToolbarOverlay?: boolean;
  Rules?: ComponentType;
  Cover?: ComponentType;
  PlayerSummary?: ComponentType<{
    room: GameRoom;
    data: Data;
    seat: number;
    assets: string;
  }>;
  ResultDetails?: ComponentType<{ room: GameRoom; data: Data }>;
  // Only the six migrated games use the existing room-protocol adapter.
  legacy?: boolean;
};

// Type erasure happens once at the registry boundary. Inside a game, its view,
// options and components keep the game's own types.
export function defineGame<Data, Options = Record<string, never>>(
  module: GameModule<Data, Options>,
): GameModule {
  return module as unknown as GameModule;
}
