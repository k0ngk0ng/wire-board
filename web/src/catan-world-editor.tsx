import { caravanWorldPreview } from "./catan-caravan-world-preview";
import { useRailMapControls } from "./rail-map-controls";
import { useEffect, useId, useState } from "react";
import type { CatanNewWorldMap, Room } from "./types";
import "./catan-world-editor.css";
const names = ["森林", "山丘", "牧场", "田地", "山脉", "沙漠", "海洋", "金矿"];
const keys = ["wood", "brick", "wool", "grain", "ore", "desert", "sea", "gold"];
const colors = [
  "#668046",
  "#b97042",
  "#a6b95e",
  "#d6b85d",
  "#87918c",
  "#cfbd81",
  "#238bb2",
  "#c2a94f",
];
const numbers = [2, 3, 4, 5, 6, 8, 9, 10, 11, 12];
function centers(count: number) {
  const rows = count === 63 ? [8, 9, 10, 9, 10, 9, 8] : [5, 6, 7, 6, 7, 6, 5],
    starts = [0, -1, -2, -2, -3, -3, -3];
  const raw = rows.flatMap((n, r) =>
    Array.from({ length: n }, (_, c) => ({
      x: Math.sqrt(3) * (starts[r] + c + r / 2),
      y: 1.5 * r,
    })),
  );
  const min = Math.min(...raw.map((p) => p.x)),
    max = Math.max(...raw.map((p) => p.x)),
    size = Math.min(580 / (max - min + Math.sqrt(3)), 430 / 11);
  return {
    size,
    points: raw.map((p) => ({
      x: 340 + (p.x - (min + max) / 2) * size,
      y: 250 + (p.y - 4.5) * size,
    })),
  };
}
export function CatanWorldEditor({
  room,
  assets,
  host,
  busy,
  command,
  onDirty,
}: {
  room: Room;
  assets: string;
  host: boolean;
  busy: boolean;
  command: (t: string, extra?: Record<string, unknown>) => void;
  onDirty: (dirty: boolean) => void;
}) {
  const control = useRailMapControls({ aspect: 680 / 500, minMobileWidth: 0 });
  const saved = JSON.stringify(room.catanNewWorldMap),
    clip = useId();
  const [draft, setDraft] = useState<CatanNewWorldMap>(room.catanNewWorldMap!),
    [selected, setSelected] = useState<number | null>(null),
    [mode, setMode] = useState("swap");
  useEffect(() => {
    setDraft(JSON.parse(saved));
    setSelected(null);
  }, [saved]);
  const dirty = JSON.stringify(draft) !== saved;
  useEffect(() => {
    onDirty(dirty);
    return () => onDirty(false);
  }, [dirty, onDirty]);
  const { size, points } = centers(draft.hexes.length);
  const wateringHoles =
    room.catanScenario === "caravans-new-world"
      ? caravanWorldPreview(draft)
      : [];
  const poly = (id: number) =>
    Array.from({ length: 6 }, (_, k) => {
      const a = ((30 + k * 60) * Math.PI) / 180;
      return `${points[id].x + Math.cos(a) * size},${points[id].y + Math.sin(a) * size}`;
    }).join(" ");
  const choose = (id: number) => {
    if (!host || busy) return;
    if (mode === "swap" && selected !== null && selected !== id) {
      const hexes = draft.hexes.slice();
      [hexes[selected], hexes[id]] = [hexes[id], hexes[selected]];
      setDraft({ hexes });
      setSelected(null);
    } else setSelected(selected === id ? null : id);
  };
  const patch = (value: { resource: number; number: number }) => {
    if (selected !== null)
      setDraft({
        hexes: draft.hexes.map((h, i) => (i === selected ? value : h)),
      });
  };
  const terrainMax =
    draft.hexes.length === 63
      ? [7, 7, 7, 7, 7, 5, 26, 4]
      : [5, 5, 5, 5, 5, 3, 19, 2];
  const tile = selected === null ? undefined : draft.hexes[selected];
  return (
    <section className="catan-world-editor" aria-label="新世界开局地图">
      <header>
        <div>
          <h3>新世界 · 开局地图</h3>
          {room.catanScenario === "caravans-new-world" && (
            <p>
              商队采用本站水源规则：开局时将最靠近中心的可用内海改为水源，五六人两处，其余一处；生产地形和数字保持不变。
            </p>
          )}
          <p>
            {host
              ? "调整完成后应用地图，所有人重新准备。开局后地图不再重抽。"
              : "请查看这张地图，再点击准备；地图变化后需要重新准备。"}
          </p>
        </div>
        {host && (
          <button
            disabled={busy || dirty}
            onClick={() => command("catan_world_map_shuffle")}
          >
            重新随机地图
          </button>
        )}
      </header>
      <div className="catan-world-editor-body">
        <div className="catan-world-map-preview">
          <div className="catan-world-map-toolbar">
            <span>可缩放拖动</span>
            <button
              aria-label="缩小开局地图"
              disabled={control.zoom <= 1}
              onClick={() => control.zoomAt(control.zoom / 1.4)}
            >
              −
            </button>
            <button
              aria-label="重置开局地图缩放"
              onClick={() => control.zoomAt(1)}
            >
              {Math.round(control.zoom * 100)}%
            </button>
            <button
              aria-label="放大开局地图"
              disabled={control.zoom >= 3}
              onClick={() => control.zoomAt(control.zoom * 1.4)}
            >
              ＋
            </button>
          </div>
          <div
            className={`map-scroll catan-world-map-viewport ${control.dragging ? "is-dragging" : ""}`}
            ref={control.viewport}
            {...control.handlers}
          >
            <div className="map-canvas" style={control.canvasStyle}>
              <svg
                style={control.mapStyle}
                viewBox="0 0 680 500"
                aria-label="新世界地图预览"
              >
                <defs>
                  {points.map((_, i) => (
                    <clipPath key={i} id={`${clip}-${i}`}>
                      <polygon points={poly(i)} />
                    </clipPath>
                  ))}
                </defs>
                {draft.hexes.map((h, i) => (
                  <g
                    key={i}
                    role={host ? "button" : "img"}
                    tabIndex={host ? 0 : undefined}
                    aria-label={`地块 ${i + 1} ${wateringHoles.includes(i) ? "水源（原海洋）" : names[h.resource]}${h.number ? ` ${h.number}` : ""}`}
                    aria-pressed={host ? selected === i : undefined}
                    onClick={() => choose(i)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        choose(i);
                      }
                    }}
                  >
                    <polygon points={poly(i)} fill={colors[h.resource]} />
                    {assets && (
                      <image
                        href={`${assets}/catan/${h.resource >= 6 ? "seafarers/" : ""}terrain-${keys[h.resource]}-v1.webp`}
                        x={points[i].x - size}
                        y={points[i].y - size}
                        width={size * 2}
                        height={size * 2}
                        preserveAspectRatio="xMidYMid slice"
                        clipPath={`url(#${clip}-${i})`}
                      />
                    )}
                    <polygon
                      points={poly(i)}
                      fill="none"
                      stroke={selected === i ? "#ffdb4f" : "#efe3b8"}
                      strokeWidth={selected === i ? 5 : 1}
                    />
                    {wateringHoles.includes(i) && (
                      <g pointerEvents="none">
                        <circle
                          cx={points[i].x}
                          cy={points[i].y}
                          r={size * 0.6}
                          fill="#f6de9b"
                          stroke="#654929"
                          strokeWidth={2}
                        />
                        <ellipse
                          cx={points[i].x}
                          cy={points[i].y - size * 0.14}
                          rx={size * 0.3}
                          ry={size * 0.16}
                          fill="#31b6c0"
                          stroke="#267576"
                        />
                        <text
                          x={points[i].x}
                          y={points[i].y + size * 0.25}
                          textAnchor="middle"
                          fontSize={size * 0.27}
                          fontWeight="700"
                          fill="#493519"
                        >
                          水源
                        </text>
                      </g>
                    )}
                    {h.number > 0 && (
                      <g pointerEvents="none">
                        <circle
                          cx={points[i].x}
                          cy={points[i].y + 3}
                          r={size * 0.4}
                          fill="#fff2cb"
                          stroke="#866332"
                        />
                        <text
                          x={points[i].x}
                          y={points[i].y + 3}
                          dominantBaseline="central"
                          textAnchor="middle"
                          fill={
                            h.number === 6 || h.number === 8
                              ? "#b83223"
                              : "#332c1c"
                          }
                          fontSize={size * 0.52}
                          fontWeight="700"
                        >
                          {h.number}
                        </text>
                      </g>
                    )}
                  </g>
                ))}
              </svg>
            </div>
          </div>
        </div>
        <div className="catan-world-editor-controls">
          <div className="catan-world-counts">
            {names.map((name, r) => (
              <span key={name}>
                {name}{" "}
                <b>
                  {draft.hexes.filter((h) => h.resource === r).length}/
                  {terrainMax[r]}
                </b>
              </span>
            ))}
          </div>
          {host && (
            <>
              <label>
                调整方式
                <select
                  disabled={busy}
                  value={mode}
                  onChange={(e) => {
                    setMode(e.target.value);
                    setSelected(null);
                  }}
                >
                  <option value="swap">交换两块地形及数字</option>
                  <option value="paint">修改单块地形与数字</option>
                </select>
              </label>
              <p>
                {selected === null
                  ? "点击地图选择地块。"
                  : mode === "swap"
                    ? `已选地块 ${selected + 1}，再点另一块交换。`
                    : `正在编辑地块 ${selected + 1}。`}
              </p>
              {mode === "paint" && tile && (
                <>
                  <label>
                    地形
                    <select
                      disabled={busy}
                      value={tile.resource}
                      onChange={(e) => {
                        const resource = Number(e.target.value);
                        patch({
                          resource,
                          number:
                            resource === 5 || resource === 6
                              ? 0
                              : tile.number || 5,
                        });
                      }}
                    >
                      {names.map((name, i) => (
                        <option key={name} value={i}>
                          {name}
                        </option>
                      ))}
                    </select>
                  </label>
                  {tile.resource !== 5 && tile.resource !== 6 && (
                    <label>
                      数字
                      <select
                        disabled={busy}
                        value={tile.number}
                        onChange={(e) =>
                          patch({ ...tile, number: Number(e.target.value) })
                        }
                      >
                        {numbers.map((n) => (
                          <option key={n} value={n}>
                            {n}
                          </option>
                        ))}
                      </select>
                    </label>
                  )}
                </>
              )}
              <p>
                6和8不能相邻，金矿不能放红号；海洋与沙漠不放数字。地形与数字不超过盒内数量，并留足港口和起始建设空间。
              </p>
              {dirty && (
                <p className="catan-world-unsaved" role="status">
                  这是尚未应用的预览。应用后，其他玩家才能看到修改并重新准备。
                </p>
              )}
              <div className="catan-world-editor-actions">
                <button
                  disabled={busy || !dirty}
                  onClick={() => {
                    setDraft(JSON.parse(saved));
                    setSelected(null);
                  }}
                >
                  撤销本次修改
                </button>
                <button
                  className="primary"
                  disabled={busy || !dirty}
                  onClick={() =>
                    command("catan_world_map", { catanNewWorldMap: draft })
                  }
                >
                  应用地图并重新准备
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </section>
  );
}
