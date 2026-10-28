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
  turnDeadline?: number;
  options?: unknown;
  seats: {
    id: string;
    name: string;
    ready: boolean;
    bot?: boolean;
    autoPlay?: boolean;
    left?: boolean;
  }[];
  game?: {
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
  play: (sources: readonly string[]) => Promise<boolean>;
  // Alternatives for one clip; unlike play(), these are not a playlist.
  playWithFallback: (sources: readonly string[]) => Promise<boolean>;
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
    options?: { volume?: number; rate?: number },
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

export type BoardProps<Data = unknown> = {
  onReady?: () => void;
  onLoadProgress?: (percent: number, label: string) => void;
  onLoadError?: (error: string) => void;
  room: GameRoom;
  data: Data;
  act: (action: Record<string, unknown>) => Promise<void>;
  // Bounded reliable input pipeline; HTTP fallback remains one request.
  inputWindow?: number;
  command: (type: string, extra?: Record<string, unknown>) => Promise<void>;
  busy: boolean;
  assets: string;
  sound: boolean;
  audio: GameAudioPort;
  clock: GameClock;
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
  playerCounts?: readonly number[];
  supportsBots?: boolean;
  mobilePlayNotice?: boolean;
  ownsSounds?: boolean;
  // Continuous controls stay responsive while an earlier request is in flight.
  // The module must coalesce input and validate its own sequence/epoch on server.
  realtime?: boolean;
  // Allow only input cleanup during manual autoplay, never a gameplay action.
  // The server must independently declare and validate the same capability.
  isInputRelease?: (action: Record<string, unknown>) => boolean;
  defaultOptions?: Options;
  // Optional game-owned visual timeline. Preserve host room metadata; never
  // modify authoritative snapshots or decide outcomes in this adapter.
  createPresentation?: () => RoomPresentation;
  resources?: (room: GameRoom, media: Record<string, number>) => string[];
  asyncBoard?: boolean;
  Board: ComponentType<BoardProps<Data>>;
  Options?: ComponentType<OptionsProps<Options>>;
  // Waiting-room extension inside the shared shell; never part of creation.
  Preparation?: ComponentType<PreparationProps<Options>>;
  Rules?: ComponentType;
  Cover?: ComponentType;
  PlayerSummary?: ComponentType<{
    room: GameRoom;
    data: Data;
    seat: number;
    assets: string;
  }>;
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
