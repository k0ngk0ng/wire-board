import { useContext, useState } from "react";
import { AssetsContext } from "../../../web/src/ui/board-host";
import {
  classicMapByID,
  previewSources,
  type ClassicMapOption,
} from "./classic-map-options";
import currentPreviews from "../data/current-map-previews.json";
import { mapNames } from "./types";

type CurrentPreview = {
  id: string;
  width: number;
  height: number;
  cells: number[];
  tiles?: number[];
};

function OriginalPreview({
  map,
  assets,
}: {
  map: ClassicMapOption;
  assets: string;
}) {
  const sources = previewSources(assets, map.preview);
  const [attempt, setAttempt] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const failed = attempt >= sources.length;
  return (
    <figure
      className="bubble-map-preview"
      data-map={map.id}
      data-preview-state={failed ? "error" : loaded ? "loaded" : "loading"}
    >
      <div
        className="bubble-map-preview-image"
        style={{ aspectRatio: `${map.width} / ${map.height}` }}
        aria-busy={!loaded && !failed}
      >
        {!failed && (
          <img
            src={sources[attempt]}
            alt={`${map.name}地图预览`}
            width={map.width}
            height={map.height}
            decoding="async"
            onLoad={() => setLoaded(true)}
            onError={() => {
              setLoaded(false);
              setAttempt((current) => current + 1);
            }}
          />
        )}
        {!loaded && (
          <span className="bubble-map-preview-status" role="status">
            {failed
              ? "预览暂时无法显示，仍可选择此地图。"
              : "正在加载地图预览…"}
          </span>
        )}
      </div>
      <figcaption>
        <strong>{map.name}</strong>
        <span>2002 留存版 · 按原资源重建；编号不对应现行国服。</span>
      </figcaption>
    </figure>
  );
}

function CurrentLayout({ map }: { map: CurrentPreview }) {
  const palette: Record<string, string> = {
    community10: "#9cab71",
    waterpark: "#8bbbbb",
    forest: "#94ab79",
    desert: "#c4ad7f",
  };
  return (
    <svg
      viewBox={`0 0 ${map.width * 10} ${map.height * 10}`}
      role="img"
      aria-label={`${mapNames[map.id]}布局预览`}
    >
      <rect width="100%" height="100%" fill={palette[map.id] ?? "#9cab71"} />
      {map.cells.map((cell, index) => {
        const x = (index % map.width) * 10;
        const y = Math.floor(index / map.width) * 10;
        const tile = map.tiles?.[index];
        if (cell === 1) {
          if (tile === 3) {
            return (
              <path
                key={index}
                d={`M${x + 5} ${y + 1}l4 8h-8z`}
                fill="#285143"
                stroke="#1c3e33"
                strokeWidth=".5"
              />
            );
          }
          const roof =
            tile === 5 ? "#537f97" : tile === 6 ? "#bb9f4b" : "#a45c4d";
          return (
            <g key={index}>
              <rect
                x={x + 0.6}
                y={y + 1}
                width="8.8"
                height="8"
                rx="1"
                fill={tile && tile >= 4 && tile <= 6 ? "#eee4c9" : "#5c7565"}
                stroke="#3c5348"
                strokeWidth=".6"
              />
              {tile && tile >= 4 && tile <= 6 ? (
                <path d={`M${x + 0.5} ${y + 4}l4.5-3.5 4.5 3.5z`} fill={roof} />
              ) : null}
            </g>
          );
        }
        if (cell === 2) {
          const fill =
            tile === 7 ? "#a75245" : tile === 8 ? "#c38547" : "#c5a46a";
          return (
            <g key={index}>
              <rect
                x={x + 1}
                y={y + 1}
                width="8"
                height="8"
                rx=".6"
                fill={fill}
                stroke="#795c3f"
                strokeWidth=".6"
              />
              <path
                d={`M${x + 2} ${y + 2}l6 6m0-6l-6 6`}
                stroke="#795c3f"
                strokeWidth=".6"
              />
            </g>
          );
        }
        return (
          <rect
            key={index}
            x={x}
            y={y}
            width="10"
            height="10"
            fill={tile === 1 ? "#b8b49f" : "none"}
            stroke="#f7f3dd"
            strokeOpacity=".16"
            strokeWidth=".4"
          />
        );
      })}
    </svg>
  );
}

export function MapPreview({ map }: { map: string }) {
  const assets = useContext(AssetsContext);
  const classic = classicMapByID[map];
  if (classic)
    return (
      <OriginalPreview key={`${map}:${assets}`} map={classic} assets={assets} />
    );
  const current = (currentPreviews.maps as CurrentPreview[]).find(
    (entry) => entry.id === map,
  );
  return (
    <figure
      className="bubble-map-preview"
      data-map={map}
      data-preview-state={
        current ? "schematic" : map === "random" ? "random" : "error"
      }
    >
      {current ? (
        <div
          className="bubble-map-preview-image"
          style={{ aspectRatio: `${current.width} / ${current.height}` }}
        >
          <CurrentLayout map={current} />
        </div>
      ) : (
        <div className="bubble-map-preview-random" aria-hidden="true">
          ?
        </div>
      )}
      <figcaption>
        <strong>{mapNames[map] ?? "地图预览"}</strong>
        <span>
          {map === "random"
            ? "开局时从全部可选地图中随机选择。"
            : map === "community10"
              ? "当前重建版布局示意，与 2002 留存版小区地图分开收录。"
              : current
                ? "自定义场地布局示意；木箱分布随对局变化。"
                : "此地图暂无预览。"}
        </span>
      </figcaption>
    </figure>
  );
}
