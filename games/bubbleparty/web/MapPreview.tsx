import { useContext, useEffect, useRef, useState } from "react";
import { AssetsContext } from "../../../web/src/ui/board-host";
import {
  classicMapByID,
  previewSources,
  type ClassicMapOption,
} from "./classic-map-options";
import currentPreviews from "../data/current-map-previews.json";
import { mapNames } from "./types";
import { drawPreview } from "./canvas";
import { loadSprites } from "./sprites";

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
      </figcaption>
    </figure>
  );
}

function CurrentLayout({
  map,
  assets,
}: {
  map: CurrentPreview;
  assets: string;
}) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const [loaded, setLoaded] = useState(false);
  useEffect(() => {
    let active = true;
    void loadSprites(assets).then(() => {
      if (!active) return;
      const context = canvas.current?.getContext("2d");
      if (context) {
        drawPreview(context, map.cells, map.id, map.tiles);
        setLoaded(true);
      }
    });
    return () => {
      active = false;
    };
  }, [assets, map]);
  return (
    <figure
      className="bubble-map-preview"
      data-map={map.id}
      data-preview-state={loaded ? "loaded" : "loading"}
    >
      <div
        className="bubble-map-preview-image"
        style={{ aspectRatio: `${map.width} / ${map.height}` }}
        aria-busy={!loaded}
      >
        <canvas
          ref={canvas}
          width={map.width * 40}
          height={map.height * 40}
          role="img"
          aria-label={`${mapNames[map.id]}地图预览`}
          style={{ display: "block", width: "100%", height: "100%" }}
        />
        {!loaded && (
          <span className="bubble-map-preview-status" role="status">
            正在加载地图预览…
          </span>
        )}
      </div>
      <figcaption>
        <strong>{mapNames[map.id]}</strong>
        {map.id !== "community10" && <span>木箱位置每局随机。</span>}
      </figcaption>
    </figure>
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
  if (current)
    return (
      <CurrentLayout key={`${map}:${assets}`} map={current} assets={assets} />
    );
  return (
    <figure
      className="bubble-map-preview"
      data-map={map}
      data-preview-state={map === "random" ? "random" : "error"}
    >
      <div className="bubble-map-preview-random" aria-hidden="true">
        ?
      </div>
      <figcaption>
        <strong>{mapNames[map] ?? "地图预览"}</strong>
        <span>
          {map === "random" ? "开局时随机选择地图。" : "此地图暂无预览。"}
        </span>
      </figcaption>
    </figure>
  );
}
