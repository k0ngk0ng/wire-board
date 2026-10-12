import type { GameHeader, RoomBase } from "../../../web/src/types";
export type DotaOrder = {
  card: number;
  lane: number;
  target: number;
  buy: string;
};
export type DotaPlayer = {
  hero: string;
  team: number;
  lane: number;
  hp: number;
  gold: number;
  charge: number;
  spent: number;
  gear: string[];
  wounded: boolean;
};
export type DotaCardSpec = { name: string; art: string; text: string };
export type DotaHeroSpec = {
  id: string;
  name: string;
  hp: number;
  passive: string;
  skills: DotaCardSpec[];
};
export type DotaItemSpec = {
  id: string;
  name: string;
  price: number;
  text: string;
  solo: boolean;
};
export type DotaState = {
  rules: string;
  n: number;
  sequence: number;
  captain: number;
  confirmed: boolean[];
  players: DotaPlayer[];
  first: number;
  core: number[];
  towers: number[][];
  track: number[];
  kills: number[];
  locked: boolean[];
  plans: Record<string, DotaOrder>;
  actors: number[] | null;
  legal: DotaOrder[];
  maximum: number[];
  draft: number[];
  draftStep: number;
  history: {
    round: number;
    orders: DotaOrder[];
    before: DotaPlayer[];
    after: DotaPlayer[];
    messages: string[];
    core: number[];
    towers: number[][];
  }[];
  catalog: {
    heroes: DotaHeroSpec[];
    items: DotaItemSpec[];
    common: DotaCardSpec[];
  };
};
export type RoomOptions = {};
export type Game = GameHeader & { dota?: DotaState };
export type Room = RoomBase & RoomOptions & { game?: Game };
