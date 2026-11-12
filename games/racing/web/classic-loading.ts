import {
  classicAssetURL,
  classicResourceBase,
  fetchClassicFiles,
  selectClassicAssets,
  type ClassicAsset,
  type ClassicManifest,
  type ClassicProgress,
} from "./classic-assets";
import {
  classicPropWorld,
  classicTrackFromRoute,
  type ClassicPropCatalog,
  type NativeClassicRoute,
} from "./classic-road";
import { classicHornAssets } from "./classic-horn";
import type { Track, View } from "./types";

export type ClassicScenePack = {
  files: Map<string, ArrayBuffer>;
  manifest: ClassicManifest;
  track: Track;
  propCatalog: ClassicPropCatalog;
  timeOfDay: "day" | "night";
  vehicles: string[];
  total: number;
  codeBytes: number;
  base: string;
};
export type ClassicLoadProgress = ClassicProgress & { label: string };
export const classicLegacySounds = [
  "countdown",
  "start",
  "lap",
  "finish",
  "collision-light",
  "collision-heavy",
  "tires",
].map((name) => `/racing/v1/${name}.mp3`);

export function classicEngineAssets(
  manifest: ClassicManifest,
  id: string,
): ClassicAsset[] {
  return manifest.assets.filter((file) => file.path.startsWith(`audio/${id}/`));
}

export async function loadClassicScene(
  data: View,
  _legacyMediaBase: string,
  signal: AbortSignal,
  progress: (value: ClassicLoadProgress) => void,
  audioVehicle?: string,
): Promise<ClassicScenePack> {
  const codeResponse = await fetch("/game-load-manifest.json", {
    signal,
    cache: "no-cache",
  });
  if (!codeResponse.ok) throw Error("赛车程序清单读取失败");
  const codeManifestBytes = await codeResponse.arrayBuffer();
  const codeManifest = JSON.parse(
    new TextDecoder().decode(codeManifestBytes),
  ) as {
    assetBase: string;
    games: Record<string, { bytes: number }[]>;
    media: Record<string, number>;
  };
  const base = classicResourceBase(codeManifest.assetBase);
  const response = await fetch(classicAssetURL(base, "manifest.json"), {
    signal,
    cache: "force-cache",
  });
  if (!response.ok) throw Error("赛车资源清单读取失败");
  const manifestBytes = await response.arrayBuffer();
  signal.throwIfAborted();
  const manifest = JSON.parse(
    new TextDecoder().decode(manifestBytes),
  ) as ClassicManifest;
  const codeBytes = (codeManifest.games.racing ?? []).reduce(
    (n, file) => n + file.bytes,
    0,
  );
  if (!codeBytes) throw Error("缺少赛车程序大小记录");
  const legacySoundBytes = classicLegacySounds.reduce(
    (sum, path) => sum + (codeManifest.media[path] ?? 0),
    0,
  );
  const completed =
    codeBytes +
    legacySoundBytes +
    manifestBytes.byteLength +
    codeManifestBytes.byteLength;
  const timeOfDay = data.options.timeOfDay === "night" ? "night" : "day";
  const vehicles = [...new Set(data.cars.map((car) => car.vehicle ?? "p911"))];
  const specs = selectClassicAssets(
    manifest,
    data.track.id,
    timeOfDay,
    vehicles,
  );
  specs.push(...classicHornAssets(manifest));
  if (audioVehicle) specs.push(...classicEngineAssets(manifest, audioVehicle));
  let total = completed;
  const files = await fetchClassicFiles(specs, {
    base,
    signal,
    progress: (value) => {
      total = completed + value.total;
      progress({
        ...value,
        loaded: completed + value.loaded,
        total,
        label:
          value.cached === value.total
            ? "复用已校验的赛道与车型"
            : "加载赛道与车型",
      });
    },
  });
  signal.throwIfAborted();
  const raw = JSON.parse(
    new TextDecoder().decode(files.get(`tracks/${data.track.id}.route.json`)!),
  ) as NativeClassicRoute;
  const track = classicTrackFromRoute(raw);
  if (track.geometry !== data.track.geometry || track.id !== data.track.id)
    throw Error("赛道规则与画面版本不一致，请重新载入");
  const propCatalog = JSON.parse(
    new TextDecoder().decode(files.get(`props/${data.track.id}.json`)!),
  ) as ClassicPropCatalog & { track: string };
  const declared = manifest.propCatalogs?.[data.track.id];
  if (
    propCatalog.track !== data.track.id ||
    propCatalog.unit !== "metres" ||
    !declared ||
    declared.path !== `props/${data.track.id}.json` ||
    declared.catalogHash !== propCatalog.catalogHash ||
    declared.count !== propCatalog.props.length
  )
    throw Error("赛道物件清单与画面版本不一致");
  classicPropWorld(
    propCatalog,
    data.props ?? {
      catalog: propCatalog.catalogHash,
      broken: btoa(String.fromCharCode(0).repeat(Math.ceil(propCatalog.props.length / 8))),
    },
    track,
  );
  progress({ loaded: total, total, cached: 0, label: "初始化车辆与赛道画面" });
  return {
    files,
    manifest,
    propCatalog,
    track: { ...track, name: data.track.name },
    timeOfDay,
    vehicles,
    total,
    codeBytes,
    base,
  };
}
