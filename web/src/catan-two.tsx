import { useEffect, useState } from "react";
import type { Room } from "./types";
import { Bundle, ResourcePicker } from "./catan-resources";
import { catanSeatColor } from "./catan-player-colors";
import {
  twoCanRespond,
  twoChoices,
  twoPick,
  twoSelected,
  twoNeutralName,
  twoReturnValid,
  twoChoiceName,
  twoRetreatTargets,
  twoRetreatEdges,
  twoRetreatCost,
  twoRetreatAction,
} from "./catan-two-state";
import type { TwoSelection, TwoRetreatSelection } from "./catan-two-state";
import "./catan-two.css";

export function CatanTwoMap({
  room,
  busy,
  selected,
  onSelect,
}: {
  room: Room;
  busy: boolean;
  selected: TwoSelection | null;
  onSelect: (s: TwoSelection | null) => void;
}) {
  const g = room.game!.catan!,
    q = g.two;
  if (!q?.pending || !twoCanRespond(room) || busy) return null;
  const sites = [
    ...new Map(
      (q.choices || []).map((c) => [`${c.edge}:${c.vertex}`, c]),
    ).values(),
  ];
  return (
    <g className="catan-two-map">
      {sites.map((c) => {
        const edge = c.edge >= 0 ? g.edges[c.edge] : undefined;
        const a = edge ? g.vertices[edge.a] : g.vertices[c.vertex],
          b = edge ? g.vertices[edge.b] : a;
        if (!a || !b) return null;
        const picked =
          selected?.sequence === q.sequence &&
          selected.edge === c.edge &&
          selected.vertex === c.vertex;
        const pick = () => onSelect(twoPick(g, c.edge, c.vertex));
        return (
          <g
            key={`${c.edge}:${c.vertex}`}
            className={`two-candidate ${picked ? "picked" : ""}`}
            role="button"
            tabIndex={0}
            aria-label={`选择中立${twoChoiceName(g, c)}位置 ${(edge ? c.edge : c.vertex) + 1}`}
            onClick={pick}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                pick();
              }
            }}
          >
            {edge ? (
              <>
                <line
                  x1={a.x + (b.x - a.x) * 0.2}
                  y1={a.y + (b.y - a.y) * 0.2}
                  x2={b.x + (a.x - b.x) * 0.2}
                  y2={b.y + (a.y - b.y) * 0.2}
                />
                <line className="two-hit" x1={a.x} y1={a.y} x2={b.x} y2={b.y} />
                <circle
                  className="two-hit-disc"
                  cx={(a.x + b.x) / 2}
                  cy={(a.y + b.y) / 2}
                  r={18}
                />
              </>
            ) : (
              <circle cx={a.x} cy={a.y} r={picked ? 16 : 13} />
            )}
          </g>
        );
      })}
    </g>
  );
}

type TokenAction = "trade" | "robber" | "knight";
const titles = {
  trade: "发起强制交易",
  robber: "将强盗移回沙漠",
  knight: "弃骑士换筹码",
};
export function CatanTwoPanel({
  room,
  assets,
  busy,
  act,
  selected,
  onSelect,
  retreat,
  onRetreat,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  act: (a: Record<string, unknown>) => Promise<unknown>;
  selected: TwoSelection | null;
  onSelect: (s: TwoSelection | null) => void;
  retreat: TwoRetreatSelection;
  onRetreat: (s: TwoRetreatSelection) => void;
}) {
  const g = room.game!.catan!,
    q = g.two,
    p = g.players[room.you];
  const [collapsed, setCollapsed] = useState(false),
    [give, setGive] = useState<number[]>(Array(g.bank.length).fill(0));
  const [stockCollapsed, setStockCollapsed] = useState(
    () => !!g.transport && window.matchMedia("(max-width: 600px)").matches,
  );
  const [mixedTrade, setMixedTrade] = useState(false);
  const [exchangeVertex, setExchangeVertex] = useState<number | null>(null);
  const [confirm, setConfirm] = useState<TokenAction | null>(null);
  useEffect(() => {
    setCollapsed(false);
    setGive(Array(g.bank.length).fill(0));
    setMixedTrade(false);
    setExchangeVertex(null);
    setConfirm(null);
  }, [
    room.id,
    room.you,
    room.spectating,
    room.seats[room.you]?.autoPlay,
    room.game!.turn,
    room.game!.round,
    room.game!.phase,
    q?.sequence,
    g.bank.length,
  ]);
  useEffect(() => {
    if (selected) setCollapsed(false);
  }, [selected]);
  useEffect(() => {
    if (retreat) setCollapsed(false);
  }, [retreat]);
  if (!q) return null;
  const playing = room.status === "playing" && !room.game!.finished;
  const mine = twoCanRespond(room),
    canUse =
      playing &&
      !room.spectating &&
      room.you >= 0 &&
      q.tokenWindow &&
      !room.seats[room.you]?.autoPlay &&
      !p?.eliminated;
  const hand = p?.resources || [0, 0, 0, 0, 0],
    cost = q.cost || 1;
  const available = !!canUse && !q.spent && q.tokens[room.you] >= cost;
  const opponent = g.players[1 - room.you]?.resourceCount || 0;
  const tradeCost = cost * (g.citiesKnights && mixedTrade ? 2 : 1);
  const tradeHand = g.citiesKnights && !mixedTrade ? hand.slice(0, 5) : hand;
  const canOpenTrade =
    available &&
    opponent > 0 &&
    hand.reduce((a, b) => a + b, 0) + Math.min(2, opponent) >= 2;
  const canTrade =
    available &&
    q.tokens[room.you] >= tradeCost &&
    opponent > 0 &&
    tradeHand.reduce((a, b) => a + b, 0) + Math.min(2, opponent) >= 2;
  const retreatTargets = twoRetreatTargets(room);
  const retreatEdges = twoRetreatEdges(room),
    retreatCost = twoRetreatCost(room);
  const attackMoves = q.attackMoves || {};
  const canRobber = g.attack
    ? available && Object.keys(attackMoves).length > 0
    : g.transport
      ? retreatEdges.length > 0
      : retreatTargets.length > 0;
  const retreatRequest = twoRetreatAction(room, retreat);
  const retreatValid = !!retreatRequest;
  const retreatOutside =
    (!!g.caravans && !g.rivers) || retreatTargets.includes(-1);
  const retreatName = g.rivers ? "沼泽" : "沙漠";
  const retreatAction = g.attack
    ? "将一个蛮族移到另一未征服沿海地块"
    : g.transport
      ? "移开一名蛮族"
      : retreatOutside
        ? "将强盗移出棋盘"
        : `将强盗移回${retreatName}`;
  const openConfirm = (action: TokenAction) => {
    setConfirm(action);
    setCollapsed(false);
    onRetreat(
      action === "robber"
        ? {
            tile: retreatTargets.length === 1 ? retreatTargets[0] : null,
            edge: null,
            piece: null,
          }
        : null,
    );
  };
  const closeConfirm = () => {
    setConfirm(null);
    onRetreat(null);
  };
  const canKnight =
    !!canUse &&
    (q.canExchangeKnight ??
      (!q.knightExchanged && p.knights > 0 && q.bank >= 2));
  const validConfirm =
    confirm === "trade"
      ? canTrade
      : confirm === "robber"
        ? canRobber && retreatValid
        : canKnight &&
          (!g.citiesKnights ||
            (exchangeVertex !== null &&
              (g.attack?.city
                ? q.exchangeKnightEdges
                : q.exchangeKnights
              )?.includes(exchangeVertex)));
  const choice = twoSelected(g, selected),
    choices = twoChoices(g, selected);
  const pending = playing && (q.pending || q.trade);
  return (
    <>
      <section
        className="catan-two-stock"
        aria-label={g.fishing ? "双人卡坦生产与中立势力" : "双人卡坦贸易筹码"}
      >
        <header>
          {assets && !g.fishing && (
            <img
              src={`${assets}/catan/two/trade-token-v1.webp`}
              alt="贸易筹码"
            />
          )}
          <div>
            <strong>
              双人卡坦
              {g.caravans?.attack
                ? `＋商队＋蛮族进攻${g.citiesKnights ? "＋城市与骑士" : ""}`
                : g.rivers?.transport
                  ? `＋河流＋运输${g.citiesKnights ? "＋城市与骑士" : ""}`
                  : q.seaKnights
                    ? `＋航海家＋城市与骑士${g.fishing ? "＋渔夫" : ""}`
                    : g.fishing
                      ? g.seafarers
                        ? "＋航海家＋渔夫"
                        : g.citiesKnights
                          ? "＋城市与骑士＋渔夫"
                          : "＋渔夫"
                      : g.citiesKnights
                        ? "＋城市与骑士"
                        : g.attack
                          ? g.caravans
                            ? "＋商队＋蛮族进攻"
                            : g.rivers
                              ? "＋河流＋蛮族进攻"
                              : "＋蛮族进攻"
                          : g.transport
                            ? "＋运输"
                            : g.rivers
                              ? "＋河流"
                              : g.caravans
                                ? "＋商队"
                                : q.seafarers
                                  ? "＋航海家"
                                  : ""}
            </strong>
            <small>
              {g.fishing ? (
                "不使用贸易筹码 · 鱼行动见捕鱼面板"
              ) : (
                <>
                  {room.you >= 0 && !room.spectating
                    ? `你的筹码 ${q.tokens[room.you]} · `
                    : ""}
                  筹码供应 {q.bank}
                  {q.tokenRule !== "ledger" && " / 20"}
                </>
              )}
            </small>
          </div>
          {g.transport && (
            <button
              className="two-stock-toggle"
              aria-label={stockCollapsed ? "展开双人行动" : "收起双人行动"}
              aria-expanded={!stockCollapsed}
              onClick={() => setStockCollapsed(!stockCollapsed)}
            >
              {stockCollapsed ? "展开" : "收起"}
            </button>
          )}
        </header>
        {!stockCollapsed && (
          <>
            {!g.fishing && (
              <div className="two-stocks">
                {q.tokens.map((n, i) => (
                  <span key={i}>
                    <i style={{ background: catanSeatColor(g, i) }} />
                    {room.seats[i]?.name}
                    <b>{n}</b>
                  </span>
                ))}
              </div>
            )}
            <div className="two-production" aria-label="本回合两次生产">
              {[0, 1].map((i) => (
                <span key={i} className={q.rolls[i] ? "done" : ""}>
                  {i + 1} 次生产
                  <b>{q.rolls[i] ?? (g.eventDeck ? "待抽" : "待掷")}</b>
                </span>
              ))}
            </div>
            <small>
              {g.eventDeck ? (
                "依次抽两张事件牌 · 同点数也结算 · 先完成回应再抽下一张"
              ) : (
                <>
                  两次生产总点数不同 · 每次先处理完弃牌与
                  {g.transport || g.attack ? "蛮族" : "强盗"}
                </>
              )}
            </small>
            <div className="two-neutral-status">
              {[-2, -3].map((owner, i) => (
                <span key={owner}>
                  <i style={{ background: catanSeatColor(g, owner) }} />
                  {twoNeutralName(owner)} · {q.neutralRoadLengths[i]} 段
                  {g.longestOwner === owner ? " · 最长路线" : ""}
                </span>
              ))}
            </div>
            {canUse && (
              <>
                <p>
                  {q.spent
                    ? "本回合已消费筹码"
                    : g.transport
                      ? `强制交易 ${cost} 枚 · 移开蛮族 ${retreatCost} 枚`
                      : `本次消费 ${cost} 枚 · 按公开分数计算`}
                </p>
                <div className="two-token-actions">
                  <button
                    disabled={busy || !canOpenTrade}
                    onClick={() => openConfirm("trade")}
                  >
                    强制交易
                  </button>
                  <button
                    disabled={busy || !canRobber}
                    onClick={() => openConfirm("robber")}
                  >
                    {g.attack
                      ? "移动蛮族"
                      : g.transport
                        ? "移开蛮族"
                        : retreatOutside
                          ? "移出强盗"
                          : "移回强盗"}
                  </button>
                  {(!g.attack || g.attack.city) && (
                    <button
                      disabled={busy || !canKnight}
                      onClick={() => openConfirm("knight")}
                    >
                      {g.citiesKnights ? "移除骑士换筹码" : "弃骑士换 2 枚"}
                    </button>
                  )}
                </div>
              </>
            )}
            <details>
              <summary>双人规则提示</summary>
              <p>
                建道路{g.rivers ? "、桥梁" : ""}
                或村庄后，还需免费为中立势力建设一次。中立势力不领资源
                {g.rivers || g.transport ? "、金币或筹码" : ""}，不行动。
                {!g.transport &&
                  !g.seafarers?.cloth &&
                  "中立势力可以取得最长路线。"}
              </p>
              <p>
                {g.fishing ? (
                  "每人起始五枚鱼筹码：1、1、2、2、3；建村不另领鱼。公开分数落后者每次鱼行动少付一鱼，隐藏胜利点不参与比较。用鱼修路也要为中立势力修路。"
                ) : (
                  <>
                    {g.transport
                      ? "货物地块旁建村得 1 枚筹码，沿海另得 1 枚，可叠加；起始城市不领村庄筹码。"
                      : g.caravans && !g.rivers
                        ? "沿海建村得 1 枚筹码。水源不算沙漠，不提供相邻建村的 2 枚奖励。"
                        : `在${retreatName}旁建村得 2 枚筹码，沿海得 1 枚，两者可叠加。`}
                    {g.citiesKnights
                      ? "每回合可消费筹码一次，另可移除一名自己的骑士，按等级换 1／2／3 枚筹码；供应须足够。总量 20 枚，用完须等筹码归还。初始城市不领取建村筹码。本站补充：建村奖励不足时只领剩余数量。"
                      : g.attack
                        ? "每回合可消费筹码一次，不使用弃骑士换筹码。起始城市不领取建村筹码。"
                        : "每回合可消费筹码一次，也可另弃一张已打出的骑士换 2 枚筹码。"}
                    {q.tokenRule === "ledger" &&
                      "本站补充规则：筹码用完继续记账发放，归还供应的筹码优先复用；与金币分别计算。"}
                  </>
                )}
              </p>
              {g.citiesKnights && (
                <p>
                  招募骑士后为中立势力招募一级骑士，无位置则修路；自己的一级升二级后，中立一级也升级。中立骑士不激活、不防御。两次生产各结算城市事件；炼金术仅第一次掷骰前可用。本站补充：中立骑士退让及叛变同级选择由行动玩家决定。
                </p>
              )}
              {g.attack && !g.attack.city && (
                <p>
                  真人与中立村庄分别触发登陆；共享中立骑士由双方移动，先移动自己的骑士。玩家失去骑士或未分得俘虏时获得2金币和1枚贸易筹码补偿。
                </p>
              )}
              {g.attack?.city && (
                <p>
                  本站双人组合：两家中立道路骑士由当前玩家移动和代办退让，不能激活或参战。真人与中立村庄分别触发登陆；真人骑士损失或降级领取3金币，不另领贸易筹码。
                </p>
              )}
              {g.transport && (
                <p>
                  中立路每段付1金币，每次马车移动合计：银行取一半（向上取整），对手取另一半。快速旅程第二次移动重新累计。本剧本不授予最长道路。
                </p>
              )}
            </details>
          </>
        )}
      </section>
      {(pending || confirm) && playing && (
        <section
          className="catan-gold-choice catan-two-choice"
          aria-label={pending ? "双人卡坦响应" : "确认贸易筹码行动"}
        >
          <header>
            <strong>
              {pending
                ? mine
                  ? q.trade
                    ? `选择交还 2 张${q.trade.mixed ? "资源或商品" : "资源"}`
                    : "为中立势力建设"
                  : `${room.seats[q.actor]?.name} 正在${q.trade ? "归还资源" : "建设中立棋子"}`
                : confirm === "robber"
                  ? g.attack
                    ? "移动蛮族"
                    : retreatAction
                  : titles[confirm!]}
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
              {pending ? (
                <>
                  <p>限时 120 秒，超时自动处理。可收起面板查看地图。</p>
                  {q.trade && mine && (
                    <>
                      <p>从现有手牌中选两张，也可以交还刚抽到的牌。</p>
                      {q.trade.drawn && (
                        <>
                          <small>刚抽到的牌 · 仅你可见</small>
                          <Bundle values={q.trade.drawn} assets={assets} />
                        </>
                      )}
                      <ResourcePicker
                        label={q.trade.mixed ? "交还资源或商品" : "交还资源"}
                        values={q.trade.mixed ? give : give.slice(0, 5)}
                        onChange={(n) =>
                          setGive([
                            ...n,
                            ...Array(Math.max(0, hand.length - n.length)).fill(
                              0,
                            ),
                          ])
                        }
                        limits={hand.map((n, i) =>
                          Math.min(
                            n,
                            give[i] +
                              Math.max(0, 2 - give.reduce((a, b) => a + b, 0)),
                          ),
                        )}
                        assets={assets}
                        disabled={busy}
                      />
                    </>
                  )}
                  {q.pending && mine && (
                    <>
                      <p>
                        {q.pending.kind === "settlement" &&
                        q.choices?.every((c) => c.vertex < 0)
                          ? "两家均无法建村，请改为中立势力修一条道路。"
                          : q.pending.kind === "bridge" &&
                              q.choices?.every(
                                (c) => twoChoiceName(g, c) === "道路",
                              )
                            ? "两家均无法建桥，请改为中立势力修一条道路。"
                            : q.pending.kind === "ship" && !q.buildingShip
                              ? "两家均无法造船，请改为中立势力修一条道路。"
                              : `点击地图上亮起的${q.buildingShip ? "船只" : q.pending.kind === "bridge" ? "桥梁" : ""}位置，再确认建设。`}
                      </p>
                      {selected && choices.length > 0 && (
                        <>
                          <p>
                            已选{twoChoiceName(g, selected)}位置 #
                            {(selected.vertex >= 0
                              ? selected.vertex
                              : selected.edge) + 1}
                          </p>
                          <div className="two-owner-choice">
                            {choices.map((c) => (
                              <button
                                key={c.owner}
                                aria-pressed={selected.owner === c.owner}
                                disabled={busy}
                                onClick={() =>
                                  onSelect({ ...selected, owner: c.owner })
                                }
                              >
                                <i
                                  style={{
                                    background: catanSeatColor(g, c.owner),
                                  }}
                                />
                                {twoNeutralName(c.owner)}
                              </button>
                            ))}
                          </div>
                        </>
                      )}
                      <div className="cloth-confirm-actions">
                        <button
                          disabled={busy || !selected}
                          onClick={() => onSelect(null)}
                        >
                          重选位置
                        </button>
                        <button
                          className="primary"
                          disabled={busy || !choice}
                          onClick={async () => {
                            if (!choice) return;
                            await act({
                              type: "catan_two_build",
                              target: -choice.owner - 2,
                              vertex: choice.vertex,
                              edge: choice.edge,
                            });
                            onSelect(null);
                          }}
                        >
                          确认免费建设
                        </button>
                      </div>
                    </>
                  )}
                </>
              ) : (
                confirm && (
                  <>
                    <p>
                      {confirm === "trade"
                        ? `消费 ${tradeCost} 枚筹码，随机取对手最多 2 张${mixedTrade ? "资源或商品" : "资源"}，再交还同类范围内的 2 张。确认后不能取消。`
                        : confirm === "robber"
                          ? `消费 ${retreatCost} 枚贸易筹码，${retreatAction}，不偷牌。`
                          : g.citiesKnights
                            ? "选择自己的一名骑士移除，按等级获得筹码。可能改变最长路线；供应不足时不能兑换。"
                            : "弃掉一张已打出的骑士，获得 2 枚筹码。可能失去最大骑士军队的 2 分。"}
                    </p>
                    {confirm === "trade" && g.citiesKnights && (
                      <div
                        className="two-owner-choice"
                        aria-label="强制交易类型"
                      >
                        {[false, true].map((mixed) => (
                          <button
                            key={String(mixed)}
                            disabled={busy}
                            aria-pressed={mixedTrade === mixed}
                            onClick={() => setMixedTrade(mixed)}
                          >
                            {mixed ? "资源与商品" : "仅普通资源"} ·{" "}
                            {cost * (mixed ? 2 : 1)} 枚
                          </button>
                        ))}
                      </div>
                    )}
                    {confirm === "knight" && g.citiesKnights && (
                      <div
                        className="two-owner-choice"
                        aria-label="选择兑换的骑士"
                      >
                        {(
                          (g.attack?.city
                            ? q.exchangeKnightEdges
                            : q.exchangeKnights) || []
                        ).map((vertex) => (
                          <button
                            key={vertex}
                            disabled={busy}
                            aria-pressed={exchangeVertex === vertex}
                            onClick={() => setExchangeVertex(vertex)}
                          >
                            {g.attack?.city ? "路线" : "交点"} #{vertex + 1} ·{" "}
                            {g.attack?.city
                              ? g.attack.city.knights.find(
                                  (n) => n.edge === vertex,
                                )?.strength
                              : g.citiesKnights!.knights.find(
                                  (n) => n.vertex === vertex,
                                )?.strength}{" "}
                            级骑士
                          </button>
                        ))}
                      </div>
                    )}
                    {confirm === "robber" && g.attack && (
                      <div className="two-owner-choice">
                        <label>
                          蛮族来源
                          <select
                            aria-label="蛮族来源"
                            disabled={busy}
                            value={retreat?.piece ?? ""}
                            onChange={(e) =>
                              onRetreat({
                                piece:
                                  e.target.value === ""
                                    ? null
                                    : Number(e.target.value),
                                tile: null,
                              })
                            }
                          >
                            <option value="">选择来源</option>
                            {Object.keys(attackMoves).map((id) => (
                              <option key={id} value={id}>
                                地块 #{Number(id) + 1} ·{" "}
                                {g.attack!.barbarians[Number(id)]} 个蛮族
                              </option>
                            ))}
                          </select>
                        </label>
                        <label>
                          移动目的地
                          <select
                            aria-label="蛮族目的地"
                            disabled={busy || retreat?.piece == null}
                            value={retreat?.tile ?? ""}
                            onChange={(e) =>
                              onRetreat({
                                ...retreat,
                                tile:
                                  e.target.value === ""
                                    ? null
                                    : Number(e.target.value),
                              })
                            }
                          >
                            <option value="">选择目的地</option>
                            {(retreat?.piece == null
                              ? []
                              : attackMoves[retreat.piece] || []
                            ).map((id) => (
                              <option key={id} value={id}>
                                地块 #{id + 1} · {g.attack!.barbarians[id]}{" "}
                                个蛮族
                              </option>
                            ))}
                          </select>
                        </label>
                      </div>
                    )}
                    {confirm === "robber" && g.transport && (
                      <>
                        <p>
                          选一名蛮族，再点地图上的空路。可收起面板查看地图。
                        </p>
                        <div
                          className="two-owner-choice two-barbarian-choice"
                          aria-label="选择移开的蛮族"
                        >
                          {g.transport.state.barbarians.map((edge, piece) => (
                            <button
                              key={piece}
                              disabled={busy}
                              aria-pressed={retreat?.piece === piece}
                              onClick={() =>
                                onRetreat({
                                  ...(retreat || { tile: null }),
                                  piece,
                                })
                              }
                            >
                              {assets && (
                                <img
                                  src={`${assets}/catan/attack/barbarian-v1.webp`}
                                  alt=""
                                />
                              )}
                              <span>
                                蛮族 {piece + 1}
                                <small>道路 #{edge + 1}</small>
                              </span>
                            </button>
                          ))}
                        </div>
                        <p>
                          {retreat?.piece != null
                            ? `已选蛮族 ${retreat.piece + 1}`
                            : "尚未选择蛮族"}{" "}
                          ·{" "}
                          {retreat?.edge != null &&
                          retreatEdges.includes(retreat.edge)
                            ? `空路 #${retreat.edge + 1}`
                            : "请在地图上选择空路"}
                        </p>
                      </>
                    )}
                    {confirm === "robber" &&
                      !retreatOutside &&
                      !g.transport &&
                      !g.attack && (
                        <>
                          <p>
                            选择{retreatName}
                            后确认。可收起面板，在地图上选择亮起的地块。
                          </p>
                          <div
                            className="two-owner-choice"
                            aria-label="强盗退回位置"
                          >
                            {retreatTargets.map((tile) => (
                              <button
                                key={tile}
                                disabled={busy}
                                aria-pressed={retreat?.tile === tile}
                                onClick={() => onRetreat({ tile })}
                              >
                                {retreatName} #{tile + 1}
                              </button>
                            ))}
                          </div>
                          <p>
                            {retreatValid
                              ? `已选${retreatName} #${retreat!.tile! + 1}`
                              : "请先选择位置"}
                          </p>
                        </>
                      )}
                    <div className="cloth-confirm-actions">
                      <button disabled={busy} onClick={closeConfirm}>
                        取消
                      </button>
                      <button
                        className="primary"
                        disabled={busy || !validConfirm}
                        onClick={async () => {
                          if (!validConfirm) return;
                          const action =
                            confirm === "robber"
                              ? retreatRequest
                              : {
                                  type: `catan_two_${confirm}`,
                                  ...(g.citiesKnights && confirm === "trade"
                                    ? {
                                        choice: mixedTrade
                                          ? "mixed"
                                          : "resources",
                                      }
                                    : {}),
                                  ...(g.citiesKnights && confirm === "knight"
                                    ? g.attack?.city
                                      ? { edge: exchangeVertex }
                                      : { vertex: exchangeVertex }
                                    : {}),
                                };
                          if (!action) return;
                          await act(action);
                          closeConfirm();
                        }}
                      >
                        确认
                        {confirm === "knight"
                          ? "兑换"
                          : `消费 ${confirm === "robber" ? retreatCost : tradeCost} 枚`}
                      </button>
                    </div>
                  </>
                )
              )}
            </div>
          )}
          {!collapsed && pending && q.trade && mine && (
            <button
              className="primary wide two-return-submit"
              disabled={busy || !twoReturnValid(hand, give, !!q.trade.mixed)}
              onClick={() => act({ type: "catan_two_return", give })}
            >
              确认交还 2 张
            </button>
          )}
        </section>
      )}
    </>
  );
}

export function CatanTwoRetreatMap({
  room,
  busy,
  selected,
  onSelect,
  poly,
}: {
  room: Room;
  busy: boolean;
  selected: TwoRetreatSelection;
  onSelect: (s: TwoRetreatSelection) => void;
  poly: (id: number) => string;
}) {
  if (
    !selected ||
    busy ||
    (room.game?.catan?.caravans && !room.game?.catan?.rivers)
  )
    return null;
  const g = room.game!.catan!;
  if (g.transport) {
    const edges = twoRetreatEdges(room);
    if (!edges.length) return null;
    const pickButton = (label: string, fn: () => void) => ({
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
      <g className="catan-two-transport-retreat">
        {edges.map((id) => {
          const e = g.edges[id],
            a = g.vertices[e.a],
            b = g.vertices[e.b];
          return (
            <g
              key={id}
              className={`transport-edge ${selected.edge === id ? "picked" : ""}`}
              {...pickButton(`选择蛮族退回空路 ${id + 1}`, () =>
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
        {g.transport.state.barbarians.map((id, piece) => {
          const e = g.edges[id],
            a = g.vertices[e.a],
            b = g.vertices[e.b];
          return (
            <circle
              key={piece}
              className={selected.piece === piece ? "picked" : ""}
              cx={(a.x + b.x) / 2}
              cy={(a.y + b.y) / 2}
              r={(17 * (g.hexSize || 62)) / 62}
              aria-pressed={selected.piece === piece}
              {...pickButton(`选择移开蛮族 ${piece + 1}`, () =>
                onSelect({ ...selected, piece }),
              )}
            />
          );
        })}
      </g>
    );
  }
  const name = g.rivers ? "沼泽" : "沙漠";
  return (
    <g className="catan-two-retreat-map">
      {twoRetreatTargets(room).map((tile) => {
        const pick = () => onSelect({ tile });
        return (
          <polygon
            key={tile}
            points={poly(tile)}
            className={selected.tile === tile ? "picked" : ""}
            role="button"
            tabIndex={0}
            aria-pressed={selected.tile === tile}
            aria-label={`将强盗移回${name} #${tile + 1}`}
            onClick={pick}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                pick();
              }
            }}
          />
        );
      })}
    </g>
  );
}
