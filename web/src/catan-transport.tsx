import { useEffect, useState } from "react";
import type { Room, CatanState } from "./types";
import { CatanCoins } from "./catan-rivers";
import { CatanResource } from "./catan-resources";
import {
  catanPieceColors,
  catanColorIndex,
  catanSeatColor,
} from "./catan-player-colors";
import {
  transportCanAct,
  transportEdges,
  transportSelectedAction,
  transportSites,
  transportCargo,
  transportStepDescription,
  transportWagonPosition,
} from "./catan-transport-state";
import type { TransportPick } from "./catan-transport-state";
import "./catan-transport.css";
import { CatanTransportEffects } from "./catan-transport-effects";
type Props = {
  room: Room;
  assets: string;
  busy: boolean;
  selected: TransportPick;
  onSelect: (pick: TransportPick) => void;
};
export function CatanTransportSeat({
  game,
  seat,
  assets,
}: {
  game: CatanState;
  seat: number;
  assets: string;
}) {
  const w = game.transport?.state.wagons[seat];
  if (!w) return null;
  return (
    <span className="transport-seat">
      <CatanCoins count={w.gold} assets={assets} /> ·{" "}
      <span className="transport-seat-stat">马车 {w.level + 1}级</span> ·
      <span className="transport-seat-stat">已交货 {w.delivered}件</span>
      {w.cargo && (
        <>
          {" "}
          · 载货：
          {assets && (
            <img
              className="transport-cargo-icon"
              src={`${assets}/catan/transport/cargo-${w.cargo.cargo}-v1.webp`}
              alt=""
            />
          )}
          {transportCargo[w.cargo.cargo]}
        </>
      )}
    </span>
  );
}
export function CatanTransportMap({
  room,
  assets,
  busy,
  selected,
  onSelect,
}: Props) {
  const g = room.game!.catan!,
    t = g.transport;
  if (!t) return null;
  const scale = (g.hexSize || 62) / 62,
    can = transportCanAct(room) && !busy,
    edges = transportEdges(room, selected);
  const button = (label: string, fn: () => void) => ({
    role: "button",
    tabIndex: 0,
    "aria-label": label,
    onClick: fn,
    onKeyDown: (e: React.KeyboardEvent) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        fn();
      }
    },
  });
  return (
    <g className="transport-map">
      {t.map.sites.map((site, i) => {
        const v = g.vertices[site.center],
          tile = g.tiles[site.tile],
          origin = ["quarry", "glassworks", "castle"].indexOf(site.kind);
        return (
          <g key={i} pointerEvents="none">
            {site.blocked.map((id) => {
              const e = g.edges[id],
                a = g.vertices[e.a],
                b = g.vertices[e.b];
              return (
                <text
                  key={id}
                  x={(a.x + b.x) / 2}
                  y={(a.y + b.y) / 2 + 4}
                  className="transport-blocked"
                  textAnchor="middle"
                >
                  ×<title>禁止建道路，马车仍可经过</title>
                </text>
              );
            })}
            {!(g.rivers?.transport || g.caravans?.transport) && (
              <circle
                cx={v.x}
                cy={v.y}
                r={8 * scale}
                fill="#fce8b5"
                stroke="#795026"
                strokeWidth={2}
              />
            )}
            <text
              x={tile.x}
              y={
                tile.y +
                (g.rivers?.transport || g.caravans?.transport ? -35 : 39) *
                  scale
              }
              textAnchor="middle"
              className="transport-site-label"
            >
              {transportSites[site.kind]} · {t.state.supply[origin]}
              {(g.rivers?.transport || g.caravans?.transport) &&
                ` · 产${({ quarry: "砖", glassworks: "木", castle: "羊" } as Record<string, string>)[site.kind]}`}
            </text>
          </g>
        );
      })}
      {can &&
        edges.map((id) => {
          const e = g.edges[id],
            a = g.vertices[e.a],
            b = g.vertices[e.b];
          return (
            <g
              key={id}
              className={`transport-edge ${selected.edge === id ? "picked" : ""}`}
              {...button(`选择运输道路 ${id + 1}`, () =>
                onSelect({ ...selected, edge: id }),
              )}
            >
              <line x1={a.x} y1={a.y} x2={b.x} y2={b.y} />
              <line
                className="transport-edge-hit"
                x1={a.x}
                y1={a.y}
                x2={b.x}
                y2={b.y}
              />
            </g>
          );
        })}
      {t.state.barbarians.map((id, piece) => {
        const e = g.edges[id],
          a = g.vertices[e.a],
          b = g.vertices[e.b],
          selectable = can && t.barbarianPending;
        return (
          <g
            key={piece}
            className={`transport-barbarian ${selectable ? "selectable" : ""} ${selected.piece === piece ? "picked" : ""}`}
            style={{
              transform: `translate(${(a.x + b.x) / 2}px,${(a.y + b.y) / 2}px) scale(${scale})`,
            }}
            {...(selectable
              ? button(`选择蛮族 ${piece + 1}`, () =>
                  onSelect({ ...selected, piece }),
                )
              : {})}
          >
            <circle r="13" fill="#fff0cd" stroke="#7e6242" />
            {assets && (
              <image
                href={`${assets}/catan/attack/barbarian-v1.webp`}
                x="-9"
                y="-19"
                width="18"
                height="28"
                pointerEvents="none"
              />
            )}
            <text y="21" textAnchor="middle" className="transport-site-label">
              {piece + 1}
            </text>
            <title>
              蛮族 {piece + 1} · 道路 {id + 1}
            </title>
          </g>
        );
      })}
      {t.state.wagons.map((w, seat) => {
        const v = transportWagonPosition(g, seat);
        if (!v) return null;
        return (
          <g
            key={seat}
            className="transport-wagon"
            data-transport-wagon={seat}
            style={{ transform: `translate(${v.x}px,${v.y}px)` }}
            pointerEvents="none"
          >
            <ellipse
              cy={4 * scale}
              rx={13 * scale}
              ry={6 * scale}
              fill={catanSeatColor(g, seat)}
              stroke="#513c26"
              strokeWidth={1.5}
            />
            {assets && (
              <image
                href={`${assets}/catan/transport/wagon-${catanPieceColors[catanColorIndex(g, seat)]}-v1.webp`}
                x={-14 * scale}
                y={-27 * scale}
                width={28 * scale}
                height={32 * scale}
              />
            )}
            <title>
              {room.seats[seat].name} · 马车{w.level + 1}级 ·{" "}
              {w.cargo ? transportCargo[w.cargo.cargo] : "空车"}
            </title>
          </g>
        );
      })}
      <CatanTransportEffects room={room} assets={assets} />
    </g>
  );
}
export function CatanTransportPanel({
  room,
  assets,
  busy,
  selected,
  onSelect,
  act,
}: Props & { act: (a: Record<string, unknown>) => Promise<unknown> }) {
  const g = room.game!.catan!,
    t = g.transport,
    phase = room.game!.phase;
  const [collapsed, setCollapsed] = useState(false),
    [confirm, setConfirm] = useState(""),
    [fishIDs, setFishIDs] = useState<number[]>([]);
  const q = t?.state.travel,
    waiting =
      phase === "catan_transport_move" || phase === "catan_transport_barbarian";
  useEffect(() => {
    setCollapsed(!waiting && window.matchMedia("(max-width: 600px)").matches);
    setConfirm("");
  }, [
    room.id,
    room.you,
    phase,
    waiting,
    t?.state.sequence,
    q?.pending,
    q?.arrived,
  ]);
  useEffect(() => {
    if (selected.edge !== null) setCollapsed(false);
  }, [selected.edge]);
  const fishHandKey = JSON.stringify(t?.choices.fishTokens || []);
  useEffect(() => {
    setFishIDs([]);
  }, [room.id, room.you, phase, t?.state.sequence, q?.wheatUsed, fishHandKey]);
  if (!t) return null;
  const fishTokens = t.choices.fishTokens || [],
    fishCost = t.choices.fishCost || 2,
    fishPaid = fishTokens
      .filter((token) => fishIDs.includes(token.id))
      .reduce((sum, token) => sum + token.fish, 0);
  const can = transportCanAct(room),
    active = room.game!.turn,
    w = t.state.wagons[room.you],
    choices = t.choices;
  const pick = transportSelectedAction(room, selected),
    step = choices.steps?.find((s) => s.edge === selected.edge);
  const run = async (a: Record<string, unknown>) => {
    await act(a);
    setConfirm("");
    onSelect({ edge: null, piece: null });
    if (
      a.type === "catan_transport_step" &&
      !t.map.sites.some(
        (site) =>
          site.center ===
          t.choices.steps?.find((step) => step.edge === a.edge)?.to,
      ) &&
      window.matchMedia("(max-width: 600px)").matches
    )
      setCollapsed(true);
  };
  const move = (type: string, more: Record<string, unknown> = {}) =>
    run({ type, offer: t.state.sequence, ...more });
  const arrival = waiting && q && q.arrived >= 0 && !t.state.arrivalResolved;
  return (
    <section
      className={`catan-gold-choice transport-panel ${waiting ? "pending" : ""}`}
      aria-label="运输马车"
    >
      <header>
        <strong>
          {waiting
            ? can
              ? "你的运输行动"
              : `${room.seats[active]?.name} 正在${t.barbarianPending ? "移动蛮族" : "运输"}`
            : g.rivers?.transport || g.caravans?.transport
              ? `${g.caravans?.transport ? "商队" : "河流"}＋运输${t.knights ? "＋城市与骑士" : ""} · ${g.victoryTarget ?? 13}分获胜`
              : t.knights
                ? "运输＋城市与骑士 · 15分获胜"
                : "运输任务 · 13分获胜"}
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
          {!waiting && (
            <div className="transport-stock">
              {t.map.sites
                .filter(
                  (s, i, a) => a.findIndex((x) => x.kind === s.kind) === i,
                )
                .map((s) => (
                  <span key={s.kind}>
                    {transportSites[s.kind]}{" "}
                    <b>
                      {
                        t.state.supply[
                          ["quarry", "glassworks", "castle"].indexOf(s.kind)
                        ]
                      }
                    </b>
                  </span>
                ))}
              <span>
                金币供应{" "}
                <b>
                  {t.state.goldRule === "ledger" ? "不限" : t.state.goldBank}
                </b>
              </span>
            </div>
          )}
          {t.state.goldRule === "ledger" && !waiting && (
            <details className="small">
              <summary>本站补充规则 · 金币供应</summary>
              <p>
                交货与资源兑换的金币不足时继续记账发放，归还银行后优先复用；与双人贸易筹码分别计算。
              </p>
            </details>
          )}
          {w && !waiting && (
            <CatanTransportSeat game={g} seat={room.you} assets={assets} />
          )}
          {t.deckRecipe && !waiting && (
            <details className="small">
              <summary>本站牌组配置 · 五六人</summary>
              <p>原25张牌加8张骑士、2张道路建设、2张快速旅程，共37张。</p>
            </details>
          )}
          {waiting && <p>120秒内完成，可收起查看地图。</p>}
          {can && t.barbarianPending && (
            <>
              <p>
                选择一名蛮族，再选择一条空边。移到对手的道路可随机拿走1张资源。
              </p>
              <div className="transport-buttons">
                {t.state.barbarians.map((_, i) => (
                  <button
                    key={i}
                    aria-pressed={selected.piece === i}
                    onClick={() => onSelect({ ...selected, piece: i })}
                    disabled={busy}
                  >
                    蛮族 {i + 1}
                  </button>
                ))}
              </div>
            </>
          )}
          {can && waiting && !t.barbarianPending && q && (
            <>
              <div className="transport-movement">
                <span>
                  移动点 <b>{q.points}</b>
                </span>
                <span>
                  第 {t.moves}/{t.swift ? 2 : 1} 次移动
                </span>
                {q.wheatUsed && (
                  <span>
                    {q.fishUsed ? "本回合已用鱼加步" : "本回合已加粮"}
                  </span>
                )}
              </div>
              {q.pending >= 0 ? (
                <p>
                  驱赶成功，请选择空边放置蛮族 {q.pending + 1}
                  。这次移动不偷取资源。
                </p>
              ) : arrival ? (
                <>
                  <p>
                    已到达{transportSites[t.map.sites[q.arrived].kind]}。
                    {w?.cargo
                      ? `载货：${transportCargo[w.cargo.cargo]}。`
                      : "空车将自动装载这里剩余的一枚货物。"}
                  </p>
                  <div className="transport-buttons">
                    {t.canDeliver && (
                      <button
                        className="primary"
                        disabled={busy}
                        onClick={() =>
                          void move("catan_transport_arrival", {
                            choice: "deliver",
                          })
                        }
                      >
                        确认交货 · +1分 / +{q.level + 1}金币
                      </button>
                    )}
                    <button
                      disabled={busy}
                      onClick={() =>
                        void move("catan_transport_arrival", { choice: "keep" })
                      }
                    >
                      {w?.cargo ? "保留货物，结束装卸" : "确认装货"}
                    </button>
                  </div>
                </>
              ) : (
                <>
                  <p>选路线，再确认移动；到达中心后停下。</p>
                  {g.two && (
                    <small className="transport-neutral-tolls">
                      本次中立道路 {q.neutralTolls || 0} 段 · 已付银行{" "}
                      {Math.ceil((q.neutralTolls || 0) / 2)} 金币 / 对手{" "}
                      {Math.floor((q.neutralTolls || 0) / 2)} 金币
                    </small>
                  )}
                  {choices.canFish && (
                    <div className="attack-fish-payment">
                      <span>支付 {fishCost} 鱼 · +2 移动点</span>
                      <div
                        className="attack-picks"
                        aria-label="选择马车加步的鱼筹码"
                      >
                        {fishTokens.map((token) => (
                          <button
                            key={token.id}
                            disabled={busy}
                            aria-pressed={fishIDs.includes(token.id)}
                            className={
                              fishIDs.includes(token.id) ? "selected" : ""
                            }
                            onClick={() =>
                              setFishIDs((ids) =>
                                ids.includes(token.id)
                                  ? ids.filter((id) => id !== token.id)
                                  : [...ids, token.id],
                              )
                            }
                          >
                            {token.fish} 鱼
                          </button>
                        ))}
                      </div>
                      <small>
                        已选 {fishPaid} 鱼／费用 {fishCost}{" "}
                        鱼，多付不找零；与加粮共用每回合一次。
                      </small>
                      <button
                        disabled={busy || fishPaid < fishCost}
                        onClick={() =>
                          void move("catan_transport_fish", { tokens: fishIDs })
                        }
                      >
                        确认用鱼加步
                      </button>
                    </div>
                  )}
                  <div className="transport-buttons">
                    {choices.canWheat && (
                      <button
                        disabled={busy}
                        onClick={() => void move("catan_transport_wheat")}
                      >
                        支付粮食×1 · +2移动点
                      </button>
                    )}
                    {(choices.drive || []).map((piece) => (
                      <button
                        key={piece}
                        disabled={busy}
                        onClick={() =>
                          void move("catan_transport_drive", { card: piece })
                        }
                      >
                        驱赶蛮族 {piece + 1}（{7 - q.level}
                        {q.level > 1 ? "–6" : ""}成功）
                      </button>
                    ))}
                  </div>
                  {choices.canStop && (
                    <div className="transport-buttons">
                      {confirm === "stop" ? (
                        <>
                          <button
                            onClick={() => setConfirm("")}
                            disabled={busy}
                          >
                            继续移动
                          </button>
                          <button
                            disabled={busy}
                            onClick={() => void move("catan_transport_stop")}
                          >
                            确认放弃剩余 {q.points} 点
                          </button>
                        </>
                      ) : (
                        <button
                          disabled={busy}
                          onClick={() => setConfirm("stop")}
                        >
                          结束这次移动
                        </button>
                      )}
                    </div>
                  )}
                </>
              )}
            </>
          )}
          {can && (waiting || selected.knight !== undefined) && !arrival && (
            <div className="transport-confirm">
              <p>
                {selected.edge === null
                  ? "尚未选择道路"
                  : `道路 #${selected.edge + 1}${step ? `：${transportStepDescription(room, step)}` : ""}`}
              </p>
              <button
                className="primary wide"
                disabled={busy || !pick}
                onClick={() => pick && void run(pick)}
              >
                {t.barbarianPending || (q?.pending ?? -1) >= 0
                  ? "确认放置蛮族"
                  : selected.knight !== undefined
                    ? "确认骑士驱赶"
                    : "确认移动马车"}
              </button>
            </div>
          )}
          {can && phase === "catan_turn" && w && (
            <>
              {!!choices.knightChases?.length && (
                <fieldset className="transport-knight-chase">
                  <legend>骑士驱赶道路蛮族</legend>
                  <p>
                    选择骑士与相邻蛮族，再点击地图上的落点道路。确认后骑士转为未激活；若落在对手道路，随机偷取一张资源或商品。
                  </p>
                  <div className="transport-buttons">
                    {choices.knightChases.flatMap((choice) =>
                      choice.barbarians.map((piece) => (
                        <button
                          key={`${choice.vertex}-${piece}`}
                          disabled={busy}
                          aria-pressed={
                            selected.knight === choice.vertex &&
                            selected.piece === piece
                          }
                          onClick={() =>
                            onSelect({
                              knight: choice.vertex,
                              piece,
                              edge: null,
                            })
                          }
                        >
                          骑士 #{choice.vertex + 1} · 蛮族 {piece + 1}
                        </button>
                      )),
                    )}
                    {selected.knight !== undefined && (
                      <button
                        disabled={busy}
                        onClick={() => onSelect({ edge: null, piece: null })}
                      >
                        取消驱赶
                      </button>
                    )}
                  </div>
                </fieldset>
              )}
              {choices.upgradeCost ? (
                <>
                  <p>
                    升级至 {w.level + 2}级：
                    {choices.upgradeCost.map((n, c) =>
                      n ? (
                        <span key={c} className="transport-cost">
                          <CatanResource color={c} assets={assets} small />×{n}
                        </span>
                      ) : null,
                    )}
                  </p>
                  <div className="transport-buttons">
                    {confirm === "upgrade" ? (
                      <>
                        <button disabled={busy} onClick={() => setConfirm("")}>
                          取消
                        </button>
                        <button
                          className="primary"
                          disabled={busy || !choices.canUpgrade}
                          onClick={() =>
                            void run({ type: "catan_transport_upgrade" })
                          }
                        >
                          确认升级
                        </button>
                      </>
                    ) : (
                      <button
                        disabled={busy || !choices.canUpgrade}
                        onClick={() => setConfirm("upgrade")}
                      >
                        升级马车{w.level === 3 ? " · +1分" : ""}
                      </button>
                    )}
                  </div>
                </>
              ) : (
                <p>马车已满级 · +1分</p>
              )}
              <details>
                <summary>金币与银行交易 · 本回合已买 {t.bought}/2</summary>
                <div className="transport-coin-grid">
                  {g.bank.map((_, c) => (
                    <div key={c}>
                      <CatanResource color={c} assets={assets} small />
                      {c < 5 && (
                        <button
                          disabled={busy || !choices.buy?.includes(c)}
                          onClick={() =>
                            void run({ type: "catan_coin_buy", color: c })
                          }
                        >
                          2金币买
                        </button>
                      )}
                      <button
                        disabled={busy || !choices.sell?.includes(c)}
                        onClick={() =>
                          void run({ type: "catan_coin_sell", color: c })
                        }
                      >
                        {choices.rates?.[c]}张换1金
                      </button>
                    </div>
                  ))}
                </div>
              </details>
            </>
          )}
          {!waiting && (
            <p>
              先建设和交易，再移动马车。送达合适的货物可得1分与金币；空车到达货物点自动装货。
            </p>
          )}
        </div>
      )}
    </section>
  );
}
