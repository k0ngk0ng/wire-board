import type { Art } from "./art";
import type { MapSpec, Plant, Spec, View } from "./types";

export const SCENES = [
  ["day", "白天草坪"], ["night", "夜晚草坪"], ["pool", "泳池"], ["fog", "浓雾"], ["roof", "屋顶"], ["roofnight", "夜间屋顶"],
] as const;
export const DEFAULT_MAP: MapSpec = { id: "day", rows: 5, cols: 9, night: false, waterRows: [], roof: false, fog: false };
export function mapFor(view: View): MapSpec { return view.map ?? DEFAULT_MAP; }
export function stageFor(view: View, art?: Art | null) {
  const map = mapFor(view), image = art?.manifest.maps?.[map.id];
  const lawn = image?.lawn;
  const cellHeight = map.roof || map.rows === 6 ? 85 : 100;
  return { width: 800, height: 600, left: lawn?.x ?? 40, top: lawn?.y ?? (map.roof ? 70 : 80), cellWidth: (lawn?.width ?? 720) / map.cols, cellHeight: (lawn?.height ?? cellHeight * map.rows) / map.rows, rows: map.rows, cols: map.cols, roofSlope: image?.roofSlope ?? (map.roof ? [100, 80, 60, 40, 20, 0, 0, 0, 0] : []) };
}
export function slopeAt(x: number, slope: number[]) {
  if (!slope.length) return 0;
  const col = Math.max(0, Math.min(slope.length - 1, x / 1000 - .5));
  const low = Math.floor(col), high = Math.min(slope.length - 1, low + 1);
  return slope[low] + (slope[high] - slope[low]) * (col - low);
}
export function terrainReason(spec: Spec, map: MapSpec) {
  if (spec.scenes?.length && !spec.scenes.includes(map.id)) return "本场景不可用";
  if (spec.waterOnly && !map.waterRows.length) return "需要水面";
  if (spec.roofOnly && !map.roof) return "仅屋顶可用";
  return "";
}
export function layerName(plant: Plant) {
  return plant.layer === "support" ? "基座" : plant.layer === "shell" ? "外壳" : plant.layer === "instant" ? "效果" : "本体";
}
export function plantsAt(view: { readonly plants: readonly Plant[] }, row: number, col: number) {
  return view.plants.filter(plant => plant.row === row && (plant.col === col || plant.kind === "cobcannon" && plant.col + 1 === col));
}
export function plantKind(plant: Plant) {
  return plant.kind;
}
export function visibleCell(view: View, row: number, col: number) {
  return !mapFor(view).fog || !!view.visibility?.cells[row]?.[col];
}
