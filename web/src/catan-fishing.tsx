import { cityTrackKeys, cityTracks } from "./catan-city-state";
import { useEffect, useId, useRef, useState } from "react";
import type { Act, CatanState, Room } from "./types";
import { CatanResource } from "./catan-resources";
import {
  fishAction,
  fishActionsFor,
  fishMapMode,
  fishGroundBlocked,
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
  blocked,
}: {
  assets: string;
  x: number;
  y: number;
  value: number;
  radius: number;
  active?: boolean;
  blocked?: boolean;
}) {
  const id = useId();
  return (
    <g
      className={`fish-number ${active && !blocked ? "active" : ""} ${blocked ? "blocked" : ""}`}
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
      {blocked && (
        <g
          className="fish-blocked-badge"
          transform={`translate(${radius * 0.9},${-radius * 0.9})`}
        >
          <circle r={radius * 0.47} />
          <path
            d={`M ${-radius * 0.18} ${-radius * 0.18} L ${radius * 0.18} ${radius * 0.18} M ${radius * 0.18} ${-radius * 0.18} L ${-radius * 0.18} ${radius * 0.18}`}
          />
        </g>
      )}
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
          y={
            t.y +
            ((lake.numbers.length === 2 ? 0 : i < 2 ? -14 : 14) +
              (g.robber === tile ? 14 : 0)) *
              scale
          }
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
  layer = "all",
}: {
  g: CatanState;
  assets: string;
  total: number;
  layer?: "all" | "artwork" | "numbers";
}) {
  return (
    <g
      className="catan-fishing-grounds"
      pointerEvents="none"
      aria-hidden={layer === "artwork" || undefined}
    >
      {g.fishing?.map.grounds.map((ground, i) => {
        const pos = fishGroundGeometry(g, ground.vertices);
        if (!pos) return null;
        const blocked = fishGroundBlocked(g, ground);
        return (
          <g
            key={i}
            aria-label={`渔场 ${i + 1}，点数 ${ground.number}${blocked ? "，海盗封锁，暂停产鱼" : ""}`}
          >
            <title>
              {blocked ? "海盗封锁：本渔场暂停产鱼" : `${ground.number} 点产鱼`}
            </title>
            {layer !== "numbers" &&
              (assets ? (
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
              ))}
            {layer !== "artwork" && (
              <NumberDisk
                assets={assets}
                value={ground.number}
                x={pos.labelX}
                y={pos.labelY}
                radius={(g.hexSize || 62) * 0.2}
                active={total === ground.number}
                blocked={blocked}
              />
            )}
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
  const [slot, setSlot] = useState<number | null>(null);
  const panel = useRef<HTMLElement>(null);
  useEffect(() => {
    setKind("");
    setIds([]);
    setColor(null);
    setTarget(null);
    setSlot(null);
  }, [
    room.id,
    room.game?.phase,
    room.game?.turn,
    room.game?.catan?.explorer?.sequence,
  ]);
  useEffect(() => {
    if (
      ["fish_road", "fish_ship", "fish_bridge"].includes(chosen?.type || "")
    ) {
      setCollapsed(false);
    }
  }, [chosen?.type, chosen?.id]);
  useEffect(() => {
    if (
      collapsed ||
      !["fish_road", "fish_ship", "fish_bridge"].includes(chosen?.type || "")
    )
      return;
    // Wait for the reopened panel, then reveal confirmation within either
    // the desktop action scroller or the mobile page, keeping the map choice.
    const frame = requestAnimationFrame(() => {
      panel.current?.querySelector(".fish-confirm-actions")?.scrollIntoView({
        block: window.innerWidth < 851 ? "center" : "nearest",
        behavior: "smooth",
      });
    });
    return () => cancelAnimationFrame(frame);
  }, [collapsed, chosen?.type, chosen?.id]);
  const g = room.game?.catan,
    f = g?.fishing;
  if (!g || !f) return null;
  const responder = fishResponder(room),
    replace = responder === room.you && f.canReplace && !room.spectating;
  const mine = fishTurn(room),
    hand = f.tokens.players[room.you]?.tokens || [];
  const activeKind = replace ? "catan_fish_replace" : kind;
  const mapMode = fishMapMode(activeKind),
    routeName =
      activeKind === "catan_fish_bridge"
        ? "桥梁"
        : activeKind === "catan_fish_ship"
          ? g.explorer
            ? "造船位置"
            : "船只"
          : "道路",
    blockedGrounds = f.map.grounds.filter((ground) =>
      fishGroundBlocked(g, ground),
    );
  const selection = {
    kind: activeKind,
    ids,
    color,
    target,
    slot,
    edge: mapMode && chosen?.type === mapMode ? chosen.id : null,
  };
  const action = fishAction(room, selection),
    paid = fishValue(room, ids),
    cost = f.legal.costs[activeKind] || 0;
  const submit = async (a: Record<string, unknown>) => {
    await act(a);
    setIds([]);
    setColor(null);
    setTarget(null);
    setSlot(null);
    setKind("");
    onMode("");
  };
  const selectKind = (next: string) => {
    setKind(next);
    setIds([]);
    setColor(null);
    setTarget(null);
    setSlot(null);
    onMode(fishMapMode(next));
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
          {f.two && (
            <p className="fish-notice">
              公开分数落后者，每次行动少付 1 鱼；以下费用已包含当前折扣。
            </p>
          )}
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
          {blockedGrounds.length > 0 && (
            <p className="fish-blockade-notice">
              海盗封锁{" "}
              {blockedGrounds.map((ground) => ground.number).join("、")}{" "}
              点渔场，暂不产鱼。
            </p>
          )}
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
              {fishActionsFor(g).map(([type, label]) => {
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
          {mine &&
            g.seafarers?.cloth &&
            !g.seafarers.cloth.villages.some((v) =>
              (v.traders || []).includes(room.you),
            ) && (
              <p className="fish-notice">
                与布匹村落建立贸易后，才可用2鱼驱离海盗。
              </p>
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
                <small>
                  {f.rivers
                    ? "在海岸渔场旁建设，点数掷中时领取筹码。"
                    : "在湖泊或渔场旁建设，点数掷中时领取筹码。"}
                </small>
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
          {mine &&
            g.explorer &&
            ["catan_fish_ship", "catan_fish_voyage"].includes(kind) && (
              <div
                className="fish-target-picker"
                role="group"
                aria-label="选择探索船"
              >
                {[
                  ...new Set(
                    kind === "catan_fish_voyage"
                      ? f.legal.voyages || []
                      : (f.legal.shipBuilds || [])
                          .filter((b) => b.edge === selection.edge)
                          .map((b) => b.slot),
                  ),
                ].map((id) => (
                  <button
                    key={id}
                    type="button"
                    disabled={busy}
                    aria-pressed={slot === id}
                    onClick={() => setSlot(id)}
                  >
                    船 {(id % 3) + 1}
                    {kind === "catan_fish_ship"
                      ? g.explorer!.fleet.positions[id] < 0
                        ? " · 待建"
                        : " · 拆除重造"
                      : " · 再次航行"}
                  </button>
                ))}
                {kind === "catan_fish_ship" && selection.edge === null && (
                  <small>先在地图选择造船位置，再选择船只。</small>
                )}
                {kind === "catan_fish_ship" &&
                  slot !== null &&
                  g.explorer.fleet.positions[slot] >= 0 && (
                    <p className="fish-notice">
                      拆除重造会归还原船及其货物；请选择你要重造的船。
                    </p>
                  )}
                {kind === "catan_fish_voyage" && (
                  <p className="fish-notice">
                    本站规则：每船最多再航行一次，获得4点及快速航行奖励；剩余点数不累加，羊毛仍每船每回合限一次。
                  </p>
                )}
              </div>
            )}
          {mine && !!mapMode && (
            <p className="fish-map-hint">
              {selection.edge !== null
                ? `已选${routeName} #${selection.edge + 1}`
                : `点击地图上亮起的${routeName}位置，再确认支付。`}
              <button
                type="button"
                onClick={() => {
                  setCollapsed(true);
                  onMode(mapMode);
                  showMap();
                }}
              >
                查看地图
              </button>
            </p>
          )}
          {mine && kind === "catan_fish_pirate" && (
            <p className="fish-notice">
              {g.explorer
                ? "本站规则：支付后本航行阶段所有己方船只免海盗通行费，海盗保留原位。"
                : "支付后将海盗移到场外，解除封锁；不会偷牌，也不会移动强盗。"}
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
