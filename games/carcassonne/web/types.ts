import type { GameHeader, RoomBase } from "../../../web/src/types";
export type CarFeature = {
  kind: "city" | "road" | "field" | "monastery";
  ports: number[] | null;
  cities?: number[];
  shields?: number;
  x: number;
  y: number;
};
export type CarDefinition = {
  name: string;
  arts: number[];
  features: CarFeature[];
};
export type CarTile = {
  x: number;
  y: number;
  kind: number;
  art: number;
  rotation: number;
  owner: number;
  feature: number;
};
export type CarPlacement = { x: number; y: number; rotation: number };
export type CarcassonneState = {
  players: { score: number; meeples: number; eliminated?: boolean }[];
  tiles: CarTile[];
  current: number;
  last: number;
  remaining: number;
  discarded: number[];
  catalog: CarDefinition[];
  legal: CarPlacement[];
  meepleChoices: number[];
};
export type RoomOptions = {};
export type Game = GameHeader & { carcassonne?: CarcassonneState };
export type Room = RoomBase & RoomOptions & { game?: Game };
