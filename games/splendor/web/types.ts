import type { GameHeader, RoomBase } from "../../../web/src/types";
export type Card = {
  orient?:
    "gold" | "copy" | "copy_cascade" | "double" | "cascade" | "sacrifice";
  bonusCount?: number;
  sacrificeColor?: number;
  id: number;
  tier: number;
  points: number;
  color: number;
  cost: number[];
};
export type GemCity = {
  tile: number;
  side: number;
  name: string;
  points: number;
  cost: number[];
  any?: number;
};
export type Noble = { id: number; cost: number[] };
export type SplendorCardEvent = {
  orient?: boolean;
  id: number;
  player: number;
  action: "buy" | "reserve" | "noble" | "free";
  source: "market" | "deck" | "reserved" | "nobles";
  tier: number;
  slot: number;
  card?: Card;
  noble?: Noble;
};
export type SplendorTokenEvent = {
  id: number;
  player: number;
  action: "take" | "gold" | "pay" | "return";
  tokens: number[];
};
export type GemPlayer = {
  tradingPosts?: number[];
  eliminated?: boolean;
  tokens: number[];
  bonus: number[];
  reserved?: Card[];
  reservedCount?: number;
  cards: Card[];
  nobles: Noble[];
  score: number;
};
export type SplendorOptions = {
  extraNobles?: boolean;
  cities?: boolean;
  orient?: boolean;
  rules?: string;
  tradingPosts?: boolean;
  strongholds?: boolean;
};
export type SplendorState = {
  catalog?: string;
  cities?: GemCity[];
  cityEligibility?: number[][];
  options?: SplendorOptions;
  tradingPostRules?: {
    id: number;
    name: string;
    description: string;
    cost: number[];
  }[];
  postChoices?: number[];
  reserveChoice?: Card[];
  effects?: { kind: string; exclude: number; card?: number; tier?: number }[];
  freeCardChoices?: Card[];
  strongholds?: Record<string, { player: number; count: number }>;
  strongholdActions?: {
    type: string;
    choice: string;
    card: number;
    target: number;
  }[];
  conquestCard?: number;
  startPlayer?: number;
  cardEventId?: number;
  cardEvents?: SplendorCardEvent[];
  tokenEventId?: number;
  tokenEvents?: SplendorTokenEvent[];
  bank: number[];
  market: Card[][];
  nobles: Noble[];
  players: GemPlayer[];
  remaining: number[];
  lastRound: boolean;
};
export type RoomOptions = { splendorOptions?: SplendorOptions };
export type Game = GameHeader & { splendor?: SplendorState };
export type Room = RoomBase & RoomOptions & { game?: Game };
