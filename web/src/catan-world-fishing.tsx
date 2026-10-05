import { useEffect, useState } from "react";
import type { Act, Room } from "./types";
import { CatanFishingGrounds } from "./catan-fishing";
import { worldFishPreview } from "./catan-world-fishing-state";
import "./catan-new-world.css";

export function CatanWorldFishPreview({
  room,
  vertex,
  assets,
}: {
  room: Room;
  vertex: number | null;
  assets: string;
}) {
  const ground = worldFishPreview(room, vertex);
  if (!ground) return null;
  const g = room.game!.catan!;
  return (
    <g
      className="catan-world-fish-preview"
      pointerEvents="none"
      aria-label={`${ground.number}点渔场预览，尚未安放`}
    >
      <CatanFishingGrounds
        g={{
          ...g,
          fishing: {
            ...g.fishing!,
            map: {
              ...g.fishing!.map,
              grounds: [ground],
            },
          },
        }}
        assets={assets}
        total={0}
      />
      <polyline
        points={ground.vertices
          .map((id) => `${g.vertices[id].x},${g.vertices[id].y}`)
          .join(" ")}
        fill="none"
        stroke="#fff6a3"
        strokeWidth="5"
        strokeDasharray="7 4"
      />
    </g>
  );
}

export function CatanWorldFishChoice({
  room,
  assets,
  busy,
  act,
  vertex,
  clear,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  act: Act;
  vertex: number | null;
  clear: () => void;
}) {
  const game = room.game!,
    g = game.catan!,
    setup = g.fishing?.worldSetup;
  const [collapsed, setCollapsed] = useState(false);
  useEffect(
    () => setCollapsed(false),
    [room.id, setup?.index, vertex, room.you, room.spectating],
  );
  if (
    !setup ||
    setup.current === undefined ||
    game.phase !== "catan_world_fish" ||
    game.finished ||
    room.status !== "playing"
  )
    return null;
  const mine =
    !room.spectating &&
    room.you >= 0 &&
    game.turn === room.you &&
    !g.players[room.you]?.eliminated;
  const preview = worldFishPreview(room, vertex);
  return (
    <section
      className="catan-gold-choice catan-world-choice"
      aria-label="新世界渔场布局"
      onKeyDown={(e) => {
        if (e.key === "Escape") {
          e.preventDefault();
          e.currentTarget
            .querySelector<HTMLButtonElement>("header button")
            ?.focus();
          setCollapsed(true);
          e.stopPropagation();
        }
      }}
    >
      <header>
        <strong>
          {mine ? "安放当前渔场" : `${room.seats[game.turn].name} 正在安放渔场`}{" "}
          · {setup.index + 1}/{setup.total}
        </strong>
        <button
          type="button"
          aria-expanded={!collapsed}
          onClick={() => setCollapsed(!collapsed)}
        >
          {collapsed ? "展开" : "收起"}
        </button>
      </header>
      {!collapsed && (
        <div className="catan-gold-body">
          <div className="catan-world-current">
            {assets && (
              <img src={`${assets}/catan/fishing/ground-v1.webp`} alt="" />
            )}
            <div>
              <strong>当前：{setup.current} 点渔场</strong>
              <p>剩余 {setup.remaining} 个（含当前渔场）</p>
            </div>
          </div>
          <p>
            轮流安放到海岸凹角，避开港口和已有渔场。{setup.total}
            个放完后，由先手开始建村。
          </p>
          <p>限时 120 秒，超时自动安放。可收起面板查看地图。</p>
          {mine ? (
            <>
              <p aria-live="polite">
                {preview
                  ? `地图已预览 ${setup.current} 点渔场，确认后才安放。`
                  : "点击地图上亮起的海岸凹角，预览后再确认。"}
              </p>
              <div className="cloth-confirm-actions">
                {vertex !== null && (
                  <button disabled={busy} onClick={clear}>
                    重选位置
                  </button>
                )}
                <button
                  className="primary"
                  disabled={busy || !preview}
                  onClick={async () => {
                    if (!preview) return;
                    await act({ type: "catan_world_fish", vertex });
                    clear();
                  }}
                >
                  确认安放渔场
                </button>
              </div>
            </>
          ) : (
            <p>安放完成后轮到下一位玩家。</p>
          )}
        </div>
      )}
    </section>
  );
}
