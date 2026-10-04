import { useEffect, useState } from "react";
import type { Act, Room } from "./types";
import "./catan-new-world.css";

const keys = ["wood", "brick", "wool", "grain", "ore"];
const names = ["木材", "砖块", "羊毛", "粮食", "矿石"];

export function CatanNewWorldPortChoice({
  room,
  assets,
  busy,
  act,
  edge,
  clear,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  act: Act;
  edge: number | null;
  clear: () => void;
}) {
  const game = room.game!,
    g = game.catan!,
    world = g.seafarers?.newWorld;
  const [collapsed, setCollapsed] = useState(false);
  useEffect(() => setCollapsed(false), [room.id, world?.index, edge]);
  if (
    !world ||
    world.current === undefined ||
    game.phase !== "catan_world_ports" ||
    game.finished ||
    room.status !== "playing"
  )
    return null;
  const mine =
    !room.spectating &&
    room.you >= 0 &&
    room.you === game.turn &&
    !g.players[room.you]?.eliminated;
  const resource = world.current,
    name = resource < 0 ? "通用港口 3:1" : `${names[resource]}港口 2:1`;
  return (
    <section
      className="catan-gold-choice catan-world-choice"
      aria-label="新世界港口布局"
    >
      <header>
        <strong>
          {mine ? "放置当前港口" : `${room.seats[game.turn].name} 正在放置港口`}{" "}
          · {world.index + 1}/{world.total}
        </strong>
        <button
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
              <img
                src={`${assets}/catan/port-${resource < 0 ? "any" : keys[resource]}-v1.webp`}
                alt=""
              />
            )}
            <div>
              <strong>{name}</strong>
              <p>剩余{world.remaining}个（含当前港口）</p>
            </div>
          </div>
          <p>
            港口放在陆地与海洋或外框之间，彼此至少隔开一条边。每人轮流放一个，全部放完后开始建村。
          </p>
          <p>限时120秒，超时自动安放。可收起面板查看地图。</p>
          {mine && (
            <>
              <p>
                {edge === null
                  ? "点击地图上亮起的海岸位置，再确认。"
                  : `已选择海岸 #${edge + 1}。`}
              </p>
              <div className="cloth-confirm-actions">
                {edge !== null && (
                  <button disabled={busy} onClick={clear}>
                    重选位置
                  </button>
                )}
                <button
                  className="primary"
                  disabled={
                    busy || edge === null || !g.legal.ports?.includes(edge)
                  }
                  onClick={async () => {
                    await act({ type: "catan_world_port", edge });
                    clear();
                  }}
                >
                  确认安放
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </section>
  );
}
