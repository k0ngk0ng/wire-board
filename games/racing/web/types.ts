import atlas from "../assets/public/racing/v1/atlas.json";
export type Point = { x: number; y: number };
export type Options = {
  track: "woodland" | "coast" | "canyon";
  laps: 1 | 3 | 5;
};
export type Track = {
  id: string;
  name: string;
  width: number;
  height: number;
  roadWidth: number;
  points: Point[];
};
export type Car = Point & {
  seat: number;
  vx: number;
  vy: number;
  angle: number;
  speed: number;
  steer: number;
  throttle: number;
  brake: boolean;
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
export const skinNames = atlas.cars.bodyNames;
export const trackNames = {
  woodland: "林间环道",
  coast: "海岸曲线",
  canyon: "赤岩回廊",
};
export const assetURL = (base: string, file: string) =>
  `${base.replace(/\/$/, "")}/racing/v1/${file}`;
export const sources = (base: string, file: string) =>
  base ? [assetURL(base, file), assetURL("", file)] : [assetURL("", file)];
