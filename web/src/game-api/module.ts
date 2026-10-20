import type { ComponentType } from "react";

export type GameRoom = {
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
  music: (sources: readonly string[]) => Promise<boolean>;
  stopMusic: () => void;
  // Independently keyed continuous effects; sources are fallback alternatives.
  // Repeated calls adjust loudness/pitch without restarting the media.
  loop: (
    key: string,
    sources: readonly string[],
    options?: { volume?: number; rate?: number },
  ) => Promise<boolean>;
  stopLoop: (key: string) => void;
};
export type GameClock = {
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
export type GameModule<Data = unknown, Options = unknown> = {
  id: string;
  name: string;
  description: string;
  category?: "board" | "poker" | "realtime" | "sports" | "puzzle";
  requiresFullTable?: boolean;
  minPlayers: number;
  maxPlayers: number;
  playerCounts?: readonly number[];
  supportsBots?: boolean;
  ownsSounds?: boolean;
  // Continuous controls stay responsive while an earlier request is in flight.
  // The module must coalesce input and validate its own sequence/epoch on server.
  realtime?: boolean;
  defaultOptions?: Options;
  // Optional game-owned visual timeline. Preserve host room metadata; never
  // modify authoritative snapshots or decide outcomes in this adapter.
  createPresentation?: () => RoomPresentation;
  Board: ComponentType<BoardProps<Data>>;
  Options?: ComponentType<OptionsProps<Options>>;
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
