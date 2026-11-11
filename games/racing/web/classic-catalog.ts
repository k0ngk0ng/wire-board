import vehicles from "./classic-vehicles.json";
import type { ClassicPalette } from "./classic-paint";

export type ClassicVehicleDefinition = {
  id: string;
  name: string;
  sourceName: string;
  dimensions: number[];
  colors: ClassicPalette[];
  maxSpeedKmh: number;
  mass: number;
  groundOffset: number;
};
export const classicVehicles = vehicles as ClassicVehicleDefinition[];
export const classicVehicleByID = (id: string) =>
  classicVehicles.find((vehicle) => vehicle.id === id) ?? classicVehicles[0];
export const classicTrackNames = {
  france: "法国乡间公路",
  coastal: "海豚湾",
  uk: "凯尔特遗迹",
  park: "金迪亚克公园",
} as const;
