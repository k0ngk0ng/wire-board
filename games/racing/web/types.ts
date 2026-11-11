import { CAR_MODELS } from "./car-definitions";
import atlas from "../assets/public/racing/v1/atlas.json";
import type { ClassicMotionState } from "./classic-motion";
import type { ClassicBarrier, ClassicRoadSample } from "./classic-road";
export type Point = { x: number; y: number };
export type Options = {
  track: "france" | "coastal" | "uk" | "park" | "metro" | "harbor" | "woodland" | "coast" | "canyon";
  laps: 1 | 3 | 5;
  timeOfDay?: "day" | "night";
  vehicles?: string[];
};
export type Scenery = {
  x: number;
  y: number;
  radius: number;
  height: number;
  kind: "building" | "container" | "tree" | "palm" | "rock";
};
export type Track = {
  version?: number;
  geometry?: string;
  length?: number;
  road?: ClassicRoadSample[];
  barriers?: ClassicBarrier[];
  props?: Scenery[];
  id: string;
  name: string;
  width: number;
  height: number;
  roadWidth: number;
  points: Point[];
};
export type Car = Point & {
  vehicle?: string;
  motion?: ClassicMotionState;
  rpm?: number;
  gear?: number;
  height?: number;
  pitch?: number;
  roll?: number;
  routeIndex?: number;
  protected?: boolean;
  horn?: boolean;
  seat: number;
  vx: number;
  vy: number;
  angle: number;
  speed: number;
  steer: number;
  throttle: number;
  brake: boolean;
  brakeAmount?: number;
  drift: boolean;
  skin: number;
  seq: number;
  lap: number;
  checkpoint: number;
  progress: number;
  rank: number;
  finishAt: number;
  offroad: boolean;
  lastInputAt: number;
  idleAt: number;
  resetUntil: number;
};
export type RaceEvent = Point & {
  id: number;
  type: "start" | "lap" | "finish" | "collision" | "reset";
  at: number;
  seat: number;
  intensity: number;
};
export type View = {
  physics?: number;
  options: Options;
  phase: "countdown" | "racing" | "finishing" | "finished";
  epoch: string;
  serverAt: number;
  startAt: number;
  finishDeadline: number;
  maxRaceAt: number;
  track: Track;
  cars: Car[];
  events: RaceEvent[];
  controls: { canDrive: boolean; canReset: boolean; canSkin: boolean };
};
export const seatColors = atlas.cars.seatColors;
export const skinNames = CAR_MODELS.map((car) => car.name);
export const trackNames = {
  metro: "霓虹城区",
  harbor: "工业港湾",
  woodland: "林间山道",
  coast: "滨海公路",
  canyon: "赤岩峡谷",
};
export const assetURL = (base: string, file: string) =>
  `${base.replace(/\/$/, "")}/racing/v1/${file}`;
export const sources = (base: string, file: string) =>
  base ? [assetURL(base, file), assetURL("", file)] : [assetURL("", file)];
