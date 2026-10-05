import { useEffect, useState } from "react";
import type { Act, CatanState, Room } from "./types";
import "./catan-tribe.css";

const resources = ["木材", "砖块", "羊毛", "粮食", "矿石"];
const keys = ["wood", "brick", "wool", "grain", "ore"];
const portName = (r: number) =>
  r < 0 ? "通用港口 3:1" : `${resources[r]}港口 2:1`;

function PortPicture({
  resource,
  assets,
}: {
  resource: number;
  assets: string;
}) {
  return assets ? (
    <img
      src={`${assets}/catan/port-${resource < 0 ? "any" : keys[resource]}-v1.webp`}
      alt=""
    />
  ) : (
    <span aria-hidden="true">⚓</span>
  );
}

export function CatanTribeRewards({
  game,
  assets,
}: {
  game: CatanState;
  assets: string;
}) {
  const tribe = game.seafarers?.tribe;
  if (!tribe) return null;
  const scale = Math.max(0.64, (game.hexSize || 62) / 62);
  const prizes = [
    ...(tribe.tokens || []).map((edge) => ({ edge, card: false })),
    ...tribe.development.map(({ edge }) => ({ edge, card: true })),
  ];
  return (
    <g className="catan-tribe-rewards" pointerEvents="none">
      {game.tiles.length === 49 &&
        [18, 26, 33].map((id) => (
          <polygon
            key={id}
            points={game.tiles[id].vertices
              .map((v) => `${game.vertices[v].x},${game.vertices[v].y}`)
              .join(" ")}
            fill="none"
            stroke="#284779"
            strokeWidth="3"
          >
            <title>可变布局时，此地块不能放置5、6、8、9</title>
          </polygon>
        ))}
      {prizes.map(({ edge, card }) => {
        const route = game.edges[edge],
          a = game.vertices[route.a],
          b = game.vertices[route.b];
        const label = card
          ? "发展卡奖励：造船或移船到此领取，按本次新购卡处理"
          : "胜利点奖励：造船或移船到此领取1分";
        return (
          <g
            key={`${edge}-${card}`}
            transform={`translate(${(a.x + b.x) / 2},${(a.y + b.y) / 2}) scale(${scale})`}
            role="img"
            aria-label={label}
          >
            {assets ? (
              <image
                href={`${assets}/catan/seafarers/${card ? "development-back" : "vp-token"}-v1.webp`}
                x={card ? -22 : -15}
                y="-15"
                width={card ? 44 : 30}
                height="30"
              />
            ) : (
              <>
                <rect
                  x="-17"
                  y="-12"
                  width="34"
                  height="24"
                  rx={card ? 3 : 12}
                  fill={card ? "#b95b3c" : "#e6bd58"}
                  stroke="#68431e"
                />
                <text
                  textAnchor="middle"
                  dominantBaseline="central"
                  fill="#fff"
                >
                  {card ? "？" : "+1"}
                </text>
              </>
            )}
            <title>{label}</title>
          </g>
        );
      })}
    </g>
  );
}

export function CatanTribeStock({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const tribe = room.game?.catan?.seafarers?.tribe;
  if (!tribe) return null;
  const held = !room.spectating ? tribe.heldPorts[room.you] || [] : [];
  return (
    <section className="catan-tribe-stock" aria-label="部落奖励">
      <h3>遗忘的部落</h3>
      <p>
        船只到达奖励边即可领取。
        {room.game?.catan?.fishing
          ? "村庄可建在主岛（含湖泊）旁；移动强盗时也限于主岛。"
          : "只能在有数字的陆地旁定居。"}
      </p>
      <div className="catan-tribe-counts">
        <span>
          胜利点剩余 <b>{tribe.tokens?.length || 0}</b>
        </span>
        <span>
          发展卡剩余 <b>{tribe.development.length}</b>
        </span>
        <span>
          港口剩余 <b>{tribe.ports?.length || 0}</b>
        </span>
      </div>
      {held.length > 0 && (
        <>
          <p>你的暂存港口：有合法海岸位置时必须安放，安放后才可使用。</p>
          <div className="catan-tribe-held">
            {held.map((r, i) => (
              <span key={i} title={portName(r)}>
                <PortPicture resource={r} assets={assets} />
                <small>{portName(r)}</small>
              </span>
            ))}
          </div>
        </>
      )}
    </section>
  );
}

export function CatanTribePortChoice({
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
  const g = room.game!.catan!,
    tribe = g.seafarers?.tribe,
    pending = tribe?.pending;
  const held = pending ? tribe?.heldPorts[pending.player] || [] : [];
  const [slot, setSlot] = useState(0);
  const [collapsed, setCollapsed] = useState(false);
  const heldKey = held.join(",");
  useEffect(() => {
    setSlot(0);
    setCollapsed(false);
  }, [room.id, pending?.player, heldKey]);
  useEffect(() => {
    if (edge !== null) setCollapsed(false);
  }, [edge]);
  if (
    !pending ||
    room.status !== "playing" ||
    room.game!.finished ||
    room.game!.phase !== "catan_port"
  )
    return null;
  const mine =
    !room.spectating &&
    pending.player === room.you &&
    !g.players[room.you]?.eliminated;
  return (
    <section
      className="catan-gold-choice catan-tribe-choice"
      aria-label="安放部落港口"
    >
      <header>
        <strong>
          {mine
            ? "安放领取的港口"
            : `${room.seats[pending.player].name} 正在安放港口`}
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
          <p>限时120秒，超时自动安放。可收起面板查看地图。</p>
          {mine ? (
            <>
              <div
                className="catan-tribe-port-options"
                role="group"
                aria-label="选择暂存港口"
              >
                {held.map((r, i) => (
                  <button
                    key={i}
                    type="button"
                    disabled={busy}
                    aria-pressed={slot === i}
                    onClick={() => setSlot(i)}
                  >
                    <PortPicture resource={r} assets={assets} />
                    <span>{portName(r)}</span>
                  </button>
                ))}
              </div>
              <p>
                {edge === null
                  ? "点击地图上亮起的海岸位置。港口须邻接自己的建筑，并与其他港口间隔至少一条边。"
                  : `已选择海岸位置 #${edge + 1}。确认后可立即使用。`}
                {g.fishing && "港口不能覆盖渔场占据的两条海岸边。"}
              </p>
              <div className="catan-tribe-confirm-buttons">
                {edge !== null && (
                  <button disabled={busy} onClick={clear}>
                    重选位置
                  </button>
                )}
                <button
                  className="primary"
                  disabled={
                    busy ||
                    !held.length ||
                    slot >= held.length ||
                    edge === null ||
                    !g.legal.ports?.includes(edge)
                  }
                  onClick={async () => {
                    await act({ type: "catan_port", slot, edge });
                    clear();
                  }}
                >
                  确认安放
                </button>
              </div>
            </>
          ) : (
            <p>完成后继续原来的行动。</p>
          )}
        </div>
      )}
    </section>
  );
}
