import { useEffect, useState } from "react";
import { swapRiverWorld } from "./catan-rivers-world-edit";
import type { Room } from "./types";

const colors = [
  "#668046",
  "#b97042",
  "#a6b95e",
  "#d6b85d",
  "#87918c",
  "#cfbd81",
  "#238bb2",
  "#c2a94f",
  "",
  "",
  "#496963",
];
const names = [
  "森林",
  "山丘",
  "牧场",
  "麦田",
  "山脉",
  "沙漠",
  "海洋",
  "金矿",
  "",
  "",
  "沼泽",
];
export function CatanRiversWorldPreview({
  room,
  host,
  busy,
  command,
  onDirty,
}: {
  onDirty: (dirty: boolean) => void;
  room: Room;
  host: boolean;
  busy: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
}) {
  const saved = JSON.stringify(room.catanRiversWorldMap ?? null);
  const [map, setMap] = useState(room.catanRiversWorldMap);
  const [selected, setSelected] = useState<number | null>(null);
  const [numbersOnly, setNumbersOnly] = useState(false);
  useEffect(() => {
    setMap(JSON.parse(saved) ?? undefined);
    setSelected(null);
  }, [saved]);
  const dirty = JSON.stringify(map ?? null) !== saved;
  useEffect(() => {
    onDirty(dirty);
    return () => onDirty(false);
  }, [dirty, onDirty]);
  const choose = (id: number) => {
    if (!host || busy || !map) return;
    if (selected === null) {
      setSelected(id);
      return;
    }
    setMap(swapRiverWorld(map, selected, id, numbersOnly));
    setSelected(null);
  };
  const rows =
    map?.hexes.length === 63 ? [8, 9, 10, 9, 10, 9, 8] : [5, 6, 7, 6, 7, 6, 5];
  const starts = [0, -1, -2, -2, -3, -3, -3];
  const points = rows.flatMap((n, r) =>
    Array.from({ length: n }, (_, c) => ({
      x: Math.sqrt(3) * (starts[r] + c + r / 2),
      y: r * 1.5,
    })),
  );
  const min = Math.min(...points.map((p) => p.x));
  const max = Math.max(...points.map((p) => p.x));
  const pos = points.map((p) => ({
    x: 36 + (p.x - min) * 32,
    y: 36 + p.y * 32,
  }));
  return (
    <section className="catan-world-editor" aria-label="河流新世界地图确认">
      <header>
        <div>
          <h3>河流新世界 · 地图确认</h3>
          <p>
            {map
              ? "开局使用当前地图。更换地图后所有玩家需要重新准备。"
              : "当前为开局随机地图；房主可以提前生成地图供大家确认。"}
          </p>
        </div>
        {host && (
          <div>
            <button
              disabled={busy || dirty}
              onClick={() => command("catan_rivers_world_shuffle")}
            >
              {map ? "换一张地图" : "生成预备地图"}
            </button>
            {map && (
              <button
                disabled={busy || dirty}
                onClick={() => command("catan_rivers_world_default")}
              >
                恢复开局随机
              </button>
            )}
          </div>
        )}
      </header>
      {map && host && (
        <div className="catan-world-editor-tools">
          <label>
            <input
              type="checkbox"
              checked={numbersOnly}
              disabled={busy}
              onChange={(e) => {
                setNumbersOnly(e.target.checked);
                setSelected(null);
              }}
            />
            只交换数字
          </label>
          <p>
            依次选择两个地块交换。河道地形固定；数字只能在有数字的地块间交换。应用时会检查库存、河口与数字相邻限制。
          </p>
          <button
            disabled={busy || !dirty}
            onClick={() =>
              command("catan_rivers_world_map", { catanRiversWorldMap: map })
            }
          >
            应用地图并重新准备
          </button>
          <button
            disabled={busy || !dirty}
            onClick={() => {
              setMap(JSON.parse(saved) ?? undefined);
              setSelected(null);
            }}
          >
            撤销修改
          </button>
          {dirty && <p role="status">有未应用的地图修改，请先应用或撤销。</p>}
        </div>
      )}
      {map && (
        <svg
          viewBox={`0 0 ${(max - min) * 32 + 72} 360`}
          style={{ width: "100%", maxHeight: 420 }}
          role="group"
          aria-label="河流地图，蓝线为河道"
        >
          {map.hexes.map((h, i) => (
            <g
              key={i}
              role={host ? "button" : undefined}
              tabIndex={host && !busy ? 0 : undefined}
              aria-label={`${i + 1}号 ${names[h.resource]} ${h.number || "无数字"}`}
              aria-pressed={selected === i}
              onClick={() => choose(i)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  choose(i);
                }
              }}
              style={{ cursor: host ? "pointer" : undefined }}
            >
              <title>{`${i + 1}号：${names[h.resource]}${h.number ? `，${h.number}点` : ""}`}</title>
              <polygon
                fill={colors[h.resource]}
                stroke={selected === i ? "#ffe34d" : "#f1dec0"}
                strokeWidth={selected === i ? 4 : 1}
                points={Array.from({ length: 6 }, (_, k) => {
                  const a = ((30 + k * 60) * Math.PI) / 180;
                  return `${pos[i].x + Math.cos(a) * 31},${pos[i].y + Math.sin(a) * 31}`;
                }).join(" ")}
              />
              {h.number > 0 && (
                <>
                  <circle cx={pos[i].x} cy={pos[i].y} r="12" fill="#f7e6bd" />
                  <text
                    x={pos[i].x}
                    y={pos[i].y + 4}
                    textAnchor="middle"
                    fontWeight="bold"
                    fontSize="13"
                    fill={
                      h.number === 6 || h.number === 8 ? "#b52e22" : "#312619"
                    }
                  >
                    {h.number}
                  </text>
                </>
              )}
            </g>
          ))}
          {map.channels.map((c, i) => (
            <polyline
              key={i}
              points={c.tiles
                .map((id) => `${pos[id].x},${pos[id].y}`)
                .join(" ")}
              fill="none"
              stroke="#6edbff"
              strokeWidth="5"
              strokeLinecap="round"
              strokeLinejoin="round"
              opacity="0.8"
              pointerEvents="none"
            />
          ))}
        </svg>
      )}
    </section>
  );
}
