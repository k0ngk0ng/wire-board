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
  stop: () => void;
};
export type BoardProps<Data = unknown> = {
  room: GameRoom;
  data: Data;
  act: (action: Record<string, unknown>) => Promise<void>;
  command: (type: string, extra?: Record<string, unknown>) => Promise<void>;
  busy: boolean;
  assets: string;
  sound: boolean;
  audio: GameAudioPort;
  clock: { serverNow: number; receivedAt: number };
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
  category?: "board" | "poker" | "realtime";
  requiresFullTable?: boolean;
  minPlayers: number;
  maxPlayers: number;
  playerCounts?: readonly number[];
  supportsBots?: boolean;
  ownsSounds?: boolean;
  defaultOptions?: Options;
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
