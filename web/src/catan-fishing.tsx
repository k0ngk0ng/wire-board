import { cityTrackKeys, cityTracks } from "./catan-city-state";
import { useEffect, useId, useRef, useState } from "react";
import type { Act, CatanState, Room } from "./types";
import { CatanResource } from "./catan-resources";
import {
  fishAction,
  fishActions,
  fishGroundGeometry,
  fishResponder,
  fishTurn,
  fishValue,
} from "./catan-fishing-state";
import "./catan-fishing.css";

const art = (assets: string, name: string) =>
  `${assets}/catan/fishing/${name}-v1.webp`;
function NumberDisk({
  assets,
  x,
  y,
  value,
  radius,
  active,
}: {
  assets: string;
  x: number;
  y: number;
  value: number;
  radius: number;
  active?: boolean;
}) {
  const id = useId();
  return (
    <g
      className={`fish-number ${active ? "active" : ""}`}
      transform={`translate(${x},${y})`}
      pointerEvents="none"
    >
      <defs>
        <clipPath id={id}>
          <circle r={radius} />
        </clipPath>
      </defs>
      <circle r={radius} fill="#087aa5" />
      {assets && (
        <image
          href={art(assets, "number")}
          x={-radius}
          y={-radius}
          width={radius * 2}
          height={radius * 2}
          clipPath={`url(#${id})`}
        />
      )}
      <text
        textAnchor="middle"
        dominantBaseline="central"
        fontSize={radius * 1.32}
      >
        {value}
      </text>
    </g>
  );
}
export function CatanFishLakeNumbers({
  g,
  tile,
  assets,
  total,
}: {
  g: CatanState;
  tile: number;
  assets: string;
  total: number;
}) {
  const lake = g.fishing?.map.lakes.find((l) => l.tile === tile),
    t = g.tiles[tile];
  if (!lake || !t) return null;
  const scale = (g.hexSize || 62) / 62;
  return (
    <g aria-label={`湖泊生产点数 ${lake.numbers.join("、")}`}>
      {lake.numbers.map((n, i) => (
        <NumberDisk
          key={n}
          assets={assets}
          value={n}
          radius={12 * scale}
          x={t.x + (i % 2 ? 14 : -14) * scale}
          y={t.y + (lake.numbers.length === 2 ? 0 : i < 2 ? -14 : 14) * scale}
          active={g.robber !== tile && total === n}
        />
      ))}
    </g>
  );
}
export function CatanFishingGrounds({
  g,
  assets,
  total,
}: {
  g: CatanState;
  assets: string;
  total: number;
}) {
  return (
    <g className="catan-fishing-grounds" pointerEvents="none">
      {g.fishing?.map.grounds.map((ground, i) => {
        const pos = fishGroundGeometry(g, ground.vertices);
        if (!pos) return null;
        return (
          <g key={i} aria-label={`渔场 ${i + 1}，点数 ${ground.number}`}>
            {assets ? (
              <g
                transform={`translate(${pos.x},${pos.y}) rotate(${pos.angle}) scale(${pos.scale})`}
              >
                <image
                  href={art(assets, "ground")}
                  x={-181.5}
                  y={-116.5}
                  width={182}
                  height={233}
                />
              </g>
            ) : (
              <polyline
                points={ground.vertices
                  .map((id) => `${g.vertices[id].x},${g.vertices[id].y}`)
                  .join(" ")}
                fill="none"
                stroke="#10a6cb"
                strokeWidth={14}
              />
            )}
            <NumberDisk
              assets={assets}
              value={ground.number}
              x={pos.labelX}
              y={pos.labelY}
              radius={(g.hexSize || 62) * 0.2}
              active={total === ground.number}
            />
          </g>
        );
      })}
    </g>
  );
}

export function CatanFishingPanel({
  room,
  act,
  busy,
  assets,
  chosen,
  onMode,
  showMap,
}: {
  room: Room;
  act: Act;
  busy: boolean;
  assets: string;
  chosen: { type: string; id: number } | null;
  onMode: (mode: string) => void;
  showMap: () => void;
}) {
  const [collapsed, setCollapsed] = useState(false),
    [kind, setKind] = useState("");
  const [ids, setIds] = useState<number[]>([]),
    [color, setColor] = useState<number | null>(null),
    [target, setTarget] = useState<number | null>(null);
  const panel = useRef<HTMLElement>(null);
  useEffect(() => {
    if (chosen?.type === "fish_road") {
      setCollapsed(false);
      if (window.innerWidth < 700)
        panel.current?.scrollIntoView({ block: "center", behavior: "smooth" });
    }
  }, [chosen?.type, chosen?.id]);
  const g = room.game?.catan,
    f = g?.fishing;
  if (!g || !f) return null;
  const responder = fishResponder(room),
    replace = responder === room.you && f.canReplace && !room.spectating;
  const mine = fishTurn(room),
    hand = f.tokens.players[room.you]?.tokens || [];
  const activeKind = replace ? "catan_fish_replace" : kind;
  const selection = {
    kind: activeKind,
    ids,
    color,
    target,
    edge: chosen?.type === "fish_road" ? chosen.id : null,
  };
  const action = fishAction(room, selection),
    paid = fishValue(room, ids),
    cost = f.legal.costs[activeKind] || 0;
  const submit = async (a: Record<string, unknown>) => {
    await act(a);
    setIds([]);
    setColor(null);
    setTarget(null);
    setKind("");
    onMode("");
  };
  const selectKind = (next: string) => {
    setKind(next);
    setIds([]);
    setColor(null);
    setTarget(null);
    onMode(next === "catan_fish_road" ? "fish_road" : "");
  };
  const tokenSelectable =
    (mine && !!kind && kind !== "catan_fish_boot") || replace;
  return (
    <section
      ref={panel}
      className={`catan-fishing-panel ${collapsed ? "collapsed" : ""}`}
      aria-label="捕鱼筹码与行动"
    >
      <header>
        <strong>
          {replace ? "鱼筹码已满" : "捕鱼"}
          <small>供应 {f.tokens.remaining} 枚</small>
        </strong>
        <button
          type="button"
          onClick={() => setCollapsed(!collapsed)}
          aria-expanded={!collapsed}
        >
          {collapsed ? "展开" : "收起"}
        </button>
      </header>
      {!collapsed && (
        <div className="fish-panel-body">
          <div className="fish-player-counts" aria-label="公开鱼筹码数量">
            {f.tokens.players.map((seat, p) => (
              <div key={p} className={p === room.you ? "self" : ""}>
                {assets && <img src={art(assets, "back")} alt="" />}
                <span>{room.seats[p]?.name || `玩家${p + 1}`}</span>
                <b>{seat.count} 枚</b>
                {f.tokens.bootOwner === p && (
                  <span
                    className="fish-boot"
                    title={`持有旧靴子，需要 ${f.victoryTargets[p]} 分获胜`}
                  >
                    {assets && <img src={art(assets, "boot")} alt="旧靴子" />}{" "}
                    {f.victoryTargets[p]} 分胜
                  </span>
                )}
              </div>
            ))}
          </div>
          {responder !== undefined && !replace && (
            <p className="fish-notice">
              等待{room.seats[responder]?.name || "玩家"}选择是否更换鱼筹码。
            </p>
          )}
          {replace && (
            <p className="fish-notice">
              选择一枚筹码盲换，或保留现有筹码。换一次后停止本次领取，抽到旧靴子也不再补抽。
            </p>
          )}
          {mine && !replace && (
            <div className="fish-actions" aria-label="选择捕鱼行动">
              {fishActions
                .filter(
                  ([type]) =>
                    type !==
                    (g.citiesKnights
                      ? "catan_fish_dev"
                      : "catan_fish_progress"),
                )
                .map(([type, label]) => {
                  const enabled =
                    type === "catan_fish_boot"
                      ? f.legal.bootTargets.length > 0
                      : f.legal.actions.includes(type);
                  return (
                    <button
                      key={type}
                      type="button"
                      disabled={busy || !enabled}
                      aria-pressed={kind === type}
                      onClick={() => selectKind(type)}
                    >
                      <span>{label}</span>
                      <b>
                        {type === "catan_fish_boot"
                          ? "不耗鱼"
                          : `${f.legal.costs[type]} 鱼`}
                      </b>
                    </button>
                  );
                })}
            </div>
          )}
          {!room.spectating && room.you >= 0 && (
            <div className="fish-own-hand">
              <p>
                <strong>我的鱼</strong>
                <span>
                  {hand.length} 枚 · {hand.reduce((n, t) => n + t.fish, 0)} 鱼{" "}
                  <small>仅自己可见</small>
                </span>
              </p>
              <div
                className="fish-token-list"
                role="group"
                aria-label="选择鱼筹码"
              >
                {hand.map((token) => (
                  <button
                    type="button"
                    key={token.id}
                    disabled={busy || !tokenSelectable}
                    aria-pressed={ids.includes(token.id)}
                    aria-label={`${token.fish} 鱼筹码，编号 ${token.id + 1}`}
                    onClick={() =>
                      setIds((current) =>
                        replace
                          ? current.includes(token.id)
                            ? []
                            : [token.id]
                          : current.includes(token.id)
                            ? current.filter((id) => id !== token.id)
                            : [...current, token.id],
                      )
                    }
                  >
                    {assets && (
                      <img src={art(assets, `token-${token.fish}`)} alt="" />
                    )}
                    <b>{token.fish} 鱼</b>
                  </button>
                ))}
              </div>
              {!hand.length && (
                <small>在湖泊或渔场旁建设，点数掷中时领取筹码。</small>
              )}
            </div>
          )}
          {mine && kind === "catan_fish_progress" && g.citiesKnights && (
            <div
              className="fish-progress-picker"
              role="group"
              aria-label="选择进步牌堆"
            >
              {cityTracks.map((name, c) => (
                <button
                  type="button"
                  key={c}
                  disabled={
                    busy ||
                    !(f.legal.progressTracks || []).includes(c) ||
                    !g.citiesKnights!.progressRemaining[c]
                  }
                  aria-pressed={color === c}
                  onClick={() => setColor(c)}
                >
                  {assets && (
                    <img
                      src={`${assets}/catan/cities-knights/progress-back-${cityTrackKeys[c]}-v1.webp`}
                      alt=""
                    />
                  )}
                  <span>
                    {name} · {g.citiesKnights!.progressRemaining[c]} 张
                  </span>
                </button>
              ))}
              <small>
                从所选牌堆顶抽一张；普通牌面仅自己可见，胜利点牌立即公开。
              </small>
            </div>
          )}
          {mine && kind === "catan_fish_resource" && (
            <div
              className="fish-resource-picker"
              role="group"
              aria-label="选择银行资源"
            >
              {f.legal.resources.map((c) => (
                <button
                  type="button"
                  key={c}
                  disabled={busy}
                  aria-pressed={color === c}
                  onClick={() => setColor(c)}
                >
                  <CatanResource
                    color={c}
                    assets={assets}
                    count={g.bank[c]}
                    small
                  />
                </button>
              ))}
            </div>
          )}
          {mine && ["catan_fish_steal", "catan_fish_boot"].includes(kind) && (
            <div
              className="fish-target-picker"
              role="group"
              aria-label="选择对手"
            >
              {(kind === "catan_fish_boot"
                ? f.legal.bootTargets
                : f.legal.targets
              ).map((p) => (
                <button
                  type="button"
                  key={p}
                  disabled={busy}
                  aria-pressed={target === p}
                  onClick={() => setTarget(p)}
                >
                  {room.seats[p]?.name || `玩家${p + 1}`}
                </button>
              ))}
            </div>
          )}
          {mine && kind === "catan_fish_road" && (
            <p className="fish-map-hint">
              {selection.edge !== null
                ? `已选道路 #${selection.edge + 1}`
                : "点击地图上亮起的道路，再确认支付。"}
              <button
                type="button"
                onClick={() => {
                  setCollapsed(true);
                  onMode("fish_road");
                  showMap();
                }}
              >
                查看地图
              </button>
            </p>
          )}
          {(replace || (mine && !!kind)) && (
            <>
              <p className="fish-payment-summary">
                {replace
                  ? ids.length
                    ? "已选一枚，确认后盲抽替换。"
                    : "请选择一枚要换掉的筹码。"
                  : kind === "catan_fish_boot"
                    ? "只能传给公开分数不少于你的对手。"
                    : `已选 ${paid ?? 0} 鱼 / 费用 ${cost} 鱼${paid !== null && paid > cost ? `，多付 ${paid - cost} 鱼不找零` : ""}`}
              </p>
              <div className="fish-confirm-actions">
                {replace && (
                  <button
                    type="button"
                    disabled={busy}
                    onClick={() => {
                      const keep = fishAction(room, {
                        ...selection,
                        kind: "catan_fish_keep",
                        ids: [],
                      });
                      if (keep) void submit(keep);
                    }}
                  >
                    保留现有筹码
                  </button>
                )}
                <button
                  type="button"
                  className="fish-confirm"
                  disabled={busy || !action}
                  onClick={() => action && void submit(action)}
                >
                  {replace
                    ? "确认盲换"
                    : kind === "catan_fish_boot"
                      ? "确认传递"
                      : "确认支付"}
                </button>
              </div>
            </>
          )}
          {!!f.tokens.discard.length && (
            <details>
              <summary>公开弃置筹码 · {f.tokens.discard.length} 枚</summary>
              <div className="fish-discard">
                {[1, 2, 3].map((value) => (
                  <span key={value}>
                    {assets && (
                      <img src={art(assets, `token-${value}`)} alt="" />
                    )}
                    {value} 鱼 ×{" "}
                    {f.tokens.discard.filter((t) => t.fish === value).length}
                  </span>
                ))}
              </div>
            </details>
          )}
        </div>
      )}
    </section>
  );
}
