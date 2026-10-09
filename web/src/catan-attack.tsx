import { AttackFishPayment } from "./catan-attack-fish";
import { useEffect, useState } from "react";
import type { Room, CatanState } from "./types";
import { CatanCoins } from "./catan-rivers";
import { CatanResource } from "./catan-resources";
import {
  catanSeatColor,
  catanPieceColors,
  catanColorIndex,
} from "./catan-player-colors";
import {
  attackCanRespond,
  attackChoices,
  attackSelectedAction,
  attackCardNames,
  emptyAttackSelection,
} from "./catan-attack-state";
import type { AttackSelection } from "./catan-attack-state";
import "./catan-attack.css";
import { CatanAttackCityMap, CatanAttackCityPanel } from "./catan-attack-city";

type Props = {
  room: Room;
  assets: string;
  busy: boolean;
  selected: AttackSelection;
  onSelect: (pick: AttackSelection) => void;
};
export function CatanAttackSeat({
  game,
  seat,
  assets,
}: {
  game: CatanState;
  seat: number;
  assets: string;
}) {
  const a = game.attack;
  if (!a) return null;
  return (
    <span className="attack-seat">
      {!game.rivers && !game.transport && (
        <CatanCoins count={a.gold[seat]} assets={assets} />
      )}{" "}
      <span>
        俘虏 <b>{a.prisoners[seat]}</b>（
        {Math.floor(a.prisoners[seat] / (a.city ? 3 : 2))}分）
      </span>{" "}
      <span>
        骑士 <b>{6 - a.knightsLeft[seat]}/6</b>
      </span>
    </span>
  );
}
export function CatanAttackMap({
  room,
  assets,
  busy,
  selected,
  onSelect,
}: Props) {
  const g = room.game!.catan!,
    a = g.attack;
  if (!a) return null;
  const mine = attackCanRespond(room),
    can = mine && !busy,
    legal = attackChoices(room, selected),
    scale = (g.hexSize || 62) / 62;
  const points = (id: number) =>
    g.tiles[id].vertices
      .map((v) => `${g.vertices[v].x},${g.vertices[v].y}`)
      .join(" ");
  const clickProps = (label: string, fn: () => void) => ({
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
  const pickTile = (id: number) => {
    if (a.wonderLanding) {
      if ((a.landingSources || []).includes(id))
        onSelect({ ...selected, sources: [id] });
      else onSelect({ ...selected, target: id });
      return;
    }
    if (a.pending?.card !== "treason") {
      onSelect({ ...selected, target: id });
      return;
    }
    if (legal.sources) {
      const sources = selected.sources.includes(id)
        ? selected.sources.filter((x) => x !== id)
        : [...selected.sources, id];
      onSelect({ ...selected, sources, destinations: [] });
    } else {
      const destinations = selected.destinations.includes(id)
        ? selected.destinations.filter((x) => x !== id)
        : (a.treasonCount ?? 2) === 1
          ? [id]
          : [...selected.destinations.slice(-1), id];
      onSelect({ ...selected, destinations });
    }
  };
  const knights = mine && a.endPlan ? a.previewKnights || a.knights : a.knights;
  return (
    <g className="attack-map">
      {!assets &&
        a.map.castles.map((id) => {
          const t = g.tiles[id];
          return (
            <g
              key={id}
              transform={`translate(${t.x},${t.y}) scale(${scale})`}
              pointerEvents="none"
            >
              <path
                d="M-27 22V-18H-20V-11H-11V-18H-4V-11H5V-18H12V-11H21V-18H28V22H8V5A8 8 0 0 0-8 5V22Z"
                fill="#d8c7a2"
                stroke="#5a422d"
                strokeWidth="3"
              />
              <text y="39" textAnchor="middle" className="attack-label">
                城堡
              </text>
              <title>城堡不生产资源 · 在此授勋，回合末必须离开</title>
            </g>
          );
        })}
      {(g.transport?.attack
        ? [
            ...a.map.coast,
            ...g.tiles
              .map((t) => t.id)
              .filter(
                (id) =>
                  !a.map.coast.includes(id) && !a.map.castles.includes(id),
              ),
          ]
        : [...a.map.coast, ...(a.map.reserves || [])]
      ).map((id, order) => {
        const t = g.tiles[id],
          n = a.barbarians[id],
          conquered = a.conquered.includes(id),
          possible = can && legal.tiles.includes(id),
          picked =
            selected.target === id ||
            selected.sources.includes(id) ||
            selected.destinations.includes(id);
        return (
          <g key={id}>
            {conquered && (
              <polygon
                points={points(id)}
                className="attack-conquered"
                pointerEvents="none"
              />
            )}
            {possible && (
              <polygon
                points={points(id)}
                className={`attack-tile-choice ${picked ? "picked" : ""}`}
                {...clickProps(`选择蛮族地块 ${id + 1}`, () => pickTile(id))}
              />
            )}
            {assets &&
              Array.from(
                {
                  length: g.transport?.sharedBarbarians
                    ? g.transport.sharedBarbarians.filter(
                        (p) => p.tile === id && p.edge < 0,
                      ).length
                    : Math.min(n, 3),
                },
                (_, i) => (
                  <image
                    key={i}
                    data-attack-barbarian={`${id}-${i}`}
                    href={`${assets}/catan/attack/barbarian-v1.webp`}
                    x={t.x + ((i - (n - 1) / 2) * 17 - 7.5) * scale}
                    y={t.y - 48 * scale}
                    width={15 * scale}
                    height={24 * scale}
                    preserveAspectRatio="xMidYMid meet"
                    pointerEvents="none"
                  />
                ),
              )}
            <g
              transform={`translate(${t.x},${t.y + (g.transport?.map.sites.some((s) => s.tile === id) ? 78 : 27) * scale}) scale(${scale})`}
              pointerEvents="none"
            >
              <rect
                x="-32"
                y="-10"
                width="64"
                height="20"
                rx="8"
                fill={conquered ? "#822d25" : "#fff1cf"}
              />
              <text
                textAnchor="middle"
                y="4"
                fill={conquered ? "#fff8e8" : "#442a1c"}
                fontSize="12"
                fontWeight="800"
              >
                {a.map.reserves?.includes(id)
                  ? `沙漠蛮族 ${n}`
                  : conquered
                    ? "已征服"
                    : `蛮族 ${n}/3`}
              </text>
              <title>
                战斗顺序 {order + 1} · 地块 {id + 1} · {n}个蛮族
                {conquered ? "，停止生产" : ""}
              </title>
            </g>
            {(selected.sources.includes(id) ||
              selected.destinations.includes(id)) &&
              can && (
                <text
                  x={t.x}
                  y={t.y - 27 * scale}
                  textAnchor="middle"
                  className="attack-label"
                  pointerEvents="none"
                >
                  {selected.sources.includes(id) ? "取走" : "放入"}
                </text>
              )}
          </g>
        );
      })}
      {can &&
        legal.edges.map((id) => {
          const e = g.edges[id],
            v = g.vertices[e.a],
            w = g.vertices[e.b];
          return (
            <g
              key={id}
              className={`attack-edge-choice ${selected.target === id ? "picked" : ""}`}
              {...clickProps(`选择骑士目的地 ${id + 1}`, () =>
                onSelect({ ...selected, target: id }),
              )}
            >
              <line
                x1={v.x + (w.x - v.x) * 0.2}
                y1={v.y + (w.y - v.y) * 0.2}
                x2={v.x + (w.x - v.x) * 0.8}
                y2={v.y + (w.y - v.y) * 0.8}
              />
              <circle
                cx={(v.x + w.x) / 2}
                cy={(v.y + w.y) / 2}
                r={18 * scale}
                fill="transparent"
                stroke="none"
              />
            </g>
          );
        })}
      {!a.city &&
        knights.map((k, i) => {
          const e = g.edges[k.edge],
            v = g.vertices[e.a],
            w = g.vertices[e.b],
            original = a.knights[i]?.edge,
            available =
              can &&
              a.endPlan &&
              a.moveChoices?.some((c) => c.from === original),
            picked = available && selected.from === original;
          return (
            <g
              key={`${k.player}-${original}`}
              className={`attack-knight ${available ? "selectable" : ""} ${picked ? "picked" : ""}`}
              transform={`translate(${(v.x + w.x) / 2},${(v.y + w.y) / 2}) scale(${scale})`}
              {...(available
                ? clickProps(
                    `选择${k.player < 0 ? "中立" : room.seats[k.player].name}的骑士 ${original + 1}`,
                    () =>
                      onSelect({ ...selected, from: original, target: null }),
                  )
                : { pointerEvents: "none" as const })}
            >
              <ellipse
                rx="16"
                ry="13"
                fill={picked ? "#ffe273" : "#fff2d5"}
                stroke={catanSeatColor(g, k.player)}
                strokeWidth="3"
              />
              {assets ? (
                <image
                  href={`${assets}/catan/attack/knight-${catanPieceColors[catanColorIndex(g, k.player)]}-v1.webp`}
                  x="-13"
                  y="-22"
                  width="26"
                  height="32"
                  pointerEvents="none"
                />
              ) : (
                <text textAnchor="middle" y="4">
                  ♞
                </text>
              )}
              <title>
                {k.player < 0 ? "中立" : room.seats[k.player].name}的骑士 · 路线{" "}
                {k.edge + 1}
                {a.endPlan && can && k.edge !== original ? "（未确认）" : ""}
              </title>
            </g>
          );
        })}
      {a.city && (
        <CatanAttackCityMap
          assets={assets}
          room={room}
          busy={busy}
          selected={selected}
          onSelect={onSelect}
        />
      )}
      {a.conqueredBuildings.map((id) => {
        const v = g.vertices[id];
        return (
          <g
            key={id}
            transform={`translate(${v.x},${v.y + 24 * scale}) scale(${scale})`}
            pointerEvents="none"
          >
            <rect x="-23" y="-8" width="46" height="16" rx="4" fill="#822d25" />
            <text textAnchor="middle" y="4" fill="white" fontSize="11">
              停用
            </text>
            <title>相邻地块全部被征服（外框不计），建筑暂停计分与港口</title>
          </g>
        );
      })}
    </g>
  );
}

export function CatanAttackPanel({
  room,
  assets,
  busy,
  selected,
  onSelect,
  act,
}: Props & { act: (a: Record<string, unknown>) => Promise<unknown> }) {
  const [collapsed, setCollapsed] = useState(false),
    [coin, setCoin] = useState<{ color: number; buy: boolean } | null>(null);
  const g = room.game!.catan!,
    a = g.attack;
  useEffect(() => {
    setCollapsed(false);
    setCoin(null);
  }, [
    room.id,
    room.you,
    room.game?.phase,
    a?.pending?.id,
    a?.pending?.neutral,
    a?.endPlan?.id,
    a?.wonderLanding?.cursor,
  ]);
  if (!a) return null;
  const mine = attackCanRespond(room),
    q = a.pending,
    plan = a.endPlan,
    pending = !!q || !!plan || !!a.wonderLanding;
  const actor =
    a.wonderLanding?.player ?? q?.player ?? plan?.player ?? room.game!.turn;
  const action = attackSelectedAction(room, selected);
  const submit = async (value: Record<string, unknown>) => {
    const result = await act(value);
    if (result !== false) {
      onSelect(emptyAttackSelection());
      setCoin(null);
    }
  };
  const own =
    room.status === "playing" &&
    !room.game!.finished &&
    !room.spectating &&
    room.you >= 0 &&
    !g.players[room.you]?.eliminated &&
    !room.seats[room.you]?.autoPlay;
  const canTrade =
    own &&
    !g.transport?.attack &&
    room.game!.turn === room.you &&
    room.game!.phase === "catan_turn";
  const coinOK = (color: number, buy: boolean) =>
    canTrade &&
    (buy
      ? color < 5 && a.bought < 2 && a.gold[room.you] >= 2 && g.bank[color] > 0
      : (g.players[room.you].resources?.[color] || 0) >=
          (g.players[room.you]?.rates?.[color] || 4) &&
        (a.goldRule === "ledger" || a.goldBank > 0));
  return (
    <section
      className={`catan-gold-choice attack-panel ${pending ? "pending" : ""}`}
      aria-label="蛮族进攻面板"
    >
      <header>
        <strong>
          {plan
            ? "骑士移动与战斗"
            : q
              ? q.neutral
                ? "放置中立骑士"
                : attackCardNames[q.card]
              : "蛮族进攻"}
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
          {a.city && (
            <CatanAttackCityPanel
              room={room}
              busy={busy}
              selected={selected}
              onSelect={onSelect}
              act={act}
            />
          )}
          <div className="attack-stock">
            <span>
              {a.landingSupplyRule === "desert-reserve"
                ? "沙漠供给"
                : "蛮族供应"}{" "}
              <b>
                {a.landingSupplyRule === "desert-reserve"
                  ? a.reserveRemaining
                  : a.city
                    ? "不限"
                    : a.supply}
              </b>
            </span>
            <span>
              金币供应 <b>{a.goldRule === "ledger" ? "不限" : a.goldBank}</b>
            </span>
            {!a.city && (
              <span className="attack-deck">
                {assets && (
                  <img
                    src={`${assets}/catan/attack/card-back-v1.webp`}
                    alt="发展卡背面"
                  />
                )}
                发展卡 <b>{a.devRemaining}</b>
              </span>
            )}
          </div>
          {a.twoRules && !a.city && (
            <p>中立骑士俘虏：{a.neutralPrisoners || 0}（不计入玩家分数）</p>
          )}
          {pending && !mine && (
            <p>
              {room.seats[actor].name} 正在
              {plan
                ? "安排骑士移动"
                : a.wonderLanding
                  ? "选择蛮族登陆"
                  : `处理${q!.neutral ? "中立骑士放置" : attackCardNames[q!.card]}`}
              。可收起面板查看地图。
            </p>
          )}
          {pending && mine && (
            <>
              <p>限时120秒，超时自动完成；可收起面板查看地图。</p>
              {q?.neutral && (
                <p>
                  这是全局首名骑士：免费放置一名双方共用的中立骑士，沿用本张牌的放置范围与剩余倒计时。
                </p>
              )}
              {a.wonderLanding ? (
                <>
                  <p>
                    登陆点数 {a.landingNumber}
                    。从最多蛮族的沙漠调出，填入同点数中蛮族最少的一格；平局由你选择。
                  </p>
                  <div className="attack-picks" aria-label="选择蛮族来源沙漠">
                    {a.landingSources?.map((id) => (
                      <button
                        key={id}
                        disabled={busy}
                        className={selected.sources[0] === id ? "selected" : ""}
                        onClick={() => onSelect({ ...selected, sources: [id] })}
                      >
                        沙漠 #{id + 1} · {a.barbarians[id]}个
                      </button>
                    ))}
                  </div>
                  <div className="attack-picks" aria-label="选择登陆地块">
                    {a.landingTargets?.map((id) => (
                      <button
                        key={id}
                        disabled={busy}
                        className={selected.target === id ? "selected" : ""}
                        onClick={() => onSelect({ ...selected, target: id })}
                      >
                        地块 #{id + 1} · {a.barbarians[id]}/3
                      </button>
                    ))}
                  </div>
                  <button
                    disabled={busy || !action}
                    onClick={() => action && submit(action)}
                  >
                    确认登陆
                  </button>
                </>
              ) : plan ? (
                <>
                  <p>
                    {a.twoRules &&
                      "先安排自己的骑士，再移动中立骑士；开始移动中立骑士后不能追加自己的移动。"}
                    每名骑士最多移动一次：通常3步，独立支付1粮可走5步。城堡里的骑士必须离开，随后统一结算沿海战斗。
                  </p>
                  <div className="attack-picks" aria-label="选择待移动骑士">
                    {a.moveChoices?.map((c) => (
                      <button
                        key={c.from}
                        className={selected.from === c.from ? "selected" : ""}
                        onClick={() =>
                          onSelect({ ...selected, from: c.from, target: null })
                        }
                        disabled={busy}
                      >
                        {a.knights.find((k) => k.edge === c.from)?.player === -2
                          ? "中立骑士"
                          : "骑士"}{" "}
                        #{c.from + 1}
                        {c.required ? " · 必须离开" : ""}
                      </button>
                    ))}
                  </div>
                  {selected.from !== null && (
                    <label className="attack-wheat">
                      <input
                        type="checkbox"
                        checked={selected.wheat}
                        disabled={
                          busy || (!selected.wheat && (a.previewWheat || 0) < 1)
                        }
                        onChange={(e) =>
                          onSelect({
                            ...selected,
                            wheat: e.target.checked,
                            fish: false,
                            tokens: [],
                            target: null,
                          })
                        }
                      />
                      <CatanResource color={3} assets={assets} />
                      支付1粮，最多5步（可用 {a.previewWheat}）
                    </label>
                  )}
                  {g.fishing?.attack && selected.from !== null && (
                    <AttackFishPayment
                      pick={selected}
                      onSelect={onSelect}
                      tokens={a.fishTokens || []}
                      cost={a.fishCost || 2}
                      available={
                        !!a.moveChoices?.find((c) => c.from === selected.from)
                          ?.fish?.length
                      }
                      disabled={busy}
                    />
                  )}
                  <p>
                    {selected.target !== null
                      ? `选中路线 #${selected.target + 1}`
                      : selected.from !== null
                        ? "请点击地图上亮起的目的地"
                        : a.twoRules
                          ? "点击可移动的己方或中立骑士，也可从上方选择"
                          : "先点击自己的骑士，或从上方选择"}
                  </p>
                  <button
                    disabled={busy || !action}
                    onClick={() => action && submit(action)}
                  >
                    加入移动计划
                  </button>
                  {!!plan.moves?.length && (
                    <ol className="attack-plan">
                      {plan.moves.map((m) => (
                        <li key={m.from}>
                          #{m.from + 1} → #{m.to + 1}
                          {m.fish
                            ? " · 鱼支付，5步内"
                            : m.wheat
                              ? " · 支付1粮"
                              : " · 3步内"}
                        </li>
                      ))}
                    </ol>
                  )}
                  <div className="attack-buttons">
                    <button
                      disabled={busy || !plan.moves?.length}
                      onClick={() =>
                        submit({
                          type: "catan_attack_move",
                          prompt: plan.id,
                          choice: "undo",
                        })
                      }
                    >
                      撤销最后一步
                    </button>
                    <button
                      className="primary"
                      disabled={busy || !a.canConfirm}
                      onClick={() =>
                        submit({
                          type: "catan_attack_move",
                          prompt: plan.id,
                          choice: "confirm",
                        })
                      }
                    >
                      确认移动并结算战斗
                    </button>
                  </div>
                  {!a.canConfirm && <small>还有城堡骑士尚未安排离开。</small>}
                </>
              ) : (
                <>
                  <div className="attack-card-explanation">
                    {assets && (
                      <img
                        className="attack-card-face"
                        src={`${assets}/catan/attack/card-${q!.card}-v1.webp`}
                        alt={`${attackCardNames[q!.card]}原版牌面`}
                      />
                    )}
                    <p>
                      {
                        {
                          capture:
                            "选择沿海地块，俘获1个蛮族。每2个俘虏计1分。",
                          knighthood:
                            "在城堡的空边放置1名骑士；本回合结束时再移动。",
                          swift_knight:
                            "在地图任意空边放置1名骑士；本回合结束时再移动。",
                          treason: `本次移动${a.treasonCount ?? 2}个蛮族：${a.fromBoard ? `从${a.fromBoard}个不同地块各取1个${a.fromBoard < (a.treasonCount ?? 2) ? "，其余从供应领取" : ""}` : "从供应领取"}；放入不同的未征服地块，同时获得2金币。`,
                        }[q!.card]
                      }
                    </p>
                  </div>
                  {q?.card === "treason" ? (
                    <>
                      <p>
                        取走：
                        {selected.sources
                          .map((id) => `#${id + 1}`)
                          .join("、") ||
                          (a.fromBoard ? "尚未选择" : "从供应领取")}{" "}
                        · 放入：
                        {selected.destinations
                          .map((id) => `#${id + 1}`)
                          .join("、") || "尚未选择"}
                      </p>
                      <button
                        disabled={busy}
                        onClick={() => onSelect(emptyAttackSelection())}
                      >
                        重选来源和目的地
                      </button>
                    </>
                  ) : (
                    <p>
                      {selected.target === null
                        ? "点击地图上亮起的位置"
                        : `已选择 #${selected.target + 1}`}
                    </p>
                  )}
                  <button
                    className="primary"
                    disabled={busy || !action}
                    onClick={() => action && submit(action)}
                  >
                    确认{q!.neutral ? "放置中立骑士" : attackCardNames[q!.card]}
                  </button>
                </>
              )}
            </>
          )}
          {!pending && (
            <>
              <p>
                {a.city
                  ? `自己回合达到${g.victoryTarget ?? 13}分获胜 · 每3个俘虏1分 · 道路骑士按等级战斗。`
                  : `自己回合达到${g.victoryTarget ?? 12}分获胜 · 每2个俘虏1分 · 不使用强盗和最大骑士军队。`}
              </p>
              {own && (
                <CatanAttackSeat game={g} seat={room.you} assets={assets} />
              )}
              {canTrade && (
                <>
                  <p>
                    2金币购买1张资源，每次行动最多2张（已购 {a.bought}
                    /2）；也可按港口比率出售资源。
                  </p>
                  <div className="attack-coin-grid">
                    {g.bank.map((_, color) => (
                      <div key={color}>
                        <CatanResource color={color} assets={assets} />
                        {[true, false].map((buy) => (
                          <button
                            key={String(buy)}
                            disabled={busy || !coinOK(color, buy)}
                            onClick={() => setCoin({ color, buy })}
                          >
                            {buy
                              ? "购买"
                              : `${g.players[room.you]?.rates?.[color] || 4}:1卖出`}
                          </button>
                        ))}
                      </div>
                    ))}
                  </div>
                  {coin && (
                    <div className="attack-buttons">
                      <span>
                        {coin.buy ? "支付2金币购买" : "出售资源换1金币"}{" "}
                        <CatanResource color={coin.color} assets={assets} />
                      </span>
                      <button disabled={busy} onClick={() => setCoin(null)}>
                        取消
                      </button>
                      <button
                        className="primary"
                        disabled={busy || !coinOK(coin.color, coin.buy)}
                        onClick={() =>
                          submit({
                            type: coin.buy
                              ? "catan_coin_buy"
                              : "catan_coin_sell",
                            color: coin.color,
                          })
                        }
                      >
                        确认兑换
                      </button>
                    </div>
                  )}
                </>
              )}
            </>
          )}
          {!a.city && a.treasonRule === "as-much-as-possible" && (
            <details>
              <summary>本站补充规则 · 叛变</summary>
              <p>
                棋子或合法目标不足时，必须完成当前能做的最多移动（最多2个）；完全无法移动时只领取2金币，不额外抽牌。
              </p>
            </details>
          )}
          {a.goldRule === "ledger" && (
            <details>
              <summary>本站补充规则 · 金币供应</summary>
              <p>
                实体金币用完后继续记账发放，战斗补偿和卡牌奖励照常领取；资源牌库存仍有限。
              </p>
            </details>
          )}
          {a.landingSupplyRule === "random-last" && (
            <details>
              <summary>本站补充规则 · 最后一枚蛮族</summary>
              <p>
                五至六人游戏中，若只剩1个蛮族而骰子命中两个可登陆地块，系统等概率随机选择其中一块放置，然后结束本次登陆。
              </p>
            </details>
          )}
          {a.end && a.end.battles.length > 0 && (
            <details className="attack-battles">
              <summary>最近战斗 · {room.seats[a.end.player].name}</summary>
              {a.end.battles.map((b) => (
                <div key={b.tile}>
                  <b>
                    地块 #{b.tile + 1}：{b.knights.length}骑士击退{b.barbarians}
                    蛮族
                  </b>
                  {b.prisoners.map(
                    (n, p) =>
                      (n > 0 || b.gold[p] > 0) && (
                        <p key={p}>
                          <span
                            style={{
                              color: "#39291b",
                              background: "#fff2d6",
                              borderLeft: `4px solid ${catanSeatColor(g, p)}`,
                              borderRadius: 4,
                              padding: "2px 4px",
                            }}
                          >
                            {room.seats[p]?.name || "中立骑士"}
                          </span>{" "}
                          俘虏 +{n} · 金币 +{b.gold[p]}
                          {!!b.tokens?.[p] && ` · 贸易筹码 +${b.tokens[p]}`}
                        </p>
                      ),
                  )}
                  {b.contests?.map((c, i) => (
                    <p key={i}>
                      俘虏分配掷骰：
                      {c.players
                        .map(
                          (p, j) =>
                            `${room.seats[p]?.name || "中立骑士"} ${c.dice[j]}`,
                        )
                        .join("、")}
                    </p>
                  ))}
                  <small>
                    {b.lossDie
                      ? `损失骰 ${b.lossDie} · ${b.lost?.length || 0}名骑士返回供应`
                      : "当前玩家获胜，不再掷损失骰"}
                  </small>
                </div>
              ))}
            </details>
          )}
        </div>
      )}
    </section>
  );
}
