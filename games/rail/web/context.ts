import { createContext } from "react";
import type { RailMapSpec } from "./types";
export const RailMapsContext = createContext<RailMapSpec[]>([]);
export const playerColors = [
  "#337ab3",
  "#cf5347",
  "#42956b",
  "#d4a833",
  "#414753",
  "#8a5a9e",
  "#527f8d",
  "#a06a4f",
  "#6f8f4f",
  "#8a6f9e",
];
