import { catanSavedVictoryTarget } from "./catan-rule-context";
import { useEffect, useId, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { createPortal } from "react-dom";
import { Landmark, X, ChevronDown, ChevronUp, ArrowLeft } from "lucide-react";
import type { Act, CatanState, Room } from "./types";
import { catanSeatColor } from "./catan-player-colors";
import "./catan-wonders.css";

const artKeys = [
  "castle",
  "monument",
  "theater",
  "bridge",
  "wall",
  "lighthouse",
  "library",
];
const resources = ["wood", "brick", "wool", "grain", "ore"];
const resourceNames = ["木材", "砖块", "羊毛", "粮食", "矿石"];
const picture = (assets: string, id: number) =>
  `${assets}/catan/seafarers/wonder-${artKeys[id]}-v1.webp`;

function WonderSteps({ level }: { level: number }) {
  return (
    <span
      className="catan-wonder-steps"
      aria-label={`已建成 ${level} 级，共 4 级`}
    >
      {[1, 2, 3, 4].map((n) => (
        <span key={n} className={n <= level ? "built" : ""} aria-hidden="true">
          {n}
        </span>
      ))}
    </span>
  );
}

export function CatanWonderSeat({
  game,
  seat,
}: {
  game: CatanState;
  seat: number;
}) {
  const card = game.seafarers?.wonders?.cards.find((c) => c.owner === seat);
  if (!game.seafarers?.wonders) return null;
  return (
    <span className="catan-wonder-seat">
      <Landmark size={13} aria-hidden="true" />
      {card
        ? `${game.wonderRules?.find((r) => r.id === card.id)?.name ?? "奇迹"} · ${card.level === 0 ? "尚未建造" : `${card.level}/4 级`}`
        : "尚未领取奇迹"}
    </span>
  );
}

export function CatanWonderMarkers({
  game,
  assets,
  setup,
  focus,
}: {
  game: CatanState;
  assets: string;
  setup: boolean;
  focus: number | null;
}) {
  const clip = useId();
  const wonders = game.seafarers?.wonders;
  if (!wonders) return null;
  const scale = Math.max(0.64, (game.hexSize || 62) / 62);
  return (
    <g className="catan-wonder-markers" pointerEvents="none">
      <defs>
        <clipPath id={clip}>
          <circle r="13" />
        </clipPath>
      </defs>
      {wonders.markers.map((m) => {
        const v = game.vertices[m.vertex],
          card = wonders.cards.find((c) => c.id === m.card);
        const name =
          game.wonderRules?.find((r) => r.id === m.card)?.name ?? "奇迹";
        return (
          <g
            key={m.vertex}
            transform={`translate(${v.x},${v.y}) scale(${scale})`}
          >
            <title>
              {name}建造位置
              {setup ? "：起始建设禁止放置" : "：在此拥有建筑可满足领取条件"}
            </title>
            {focus === m.card && (
              <g className="catan-wonder-focus">
                <circle r="23" fill="none" stroke="#ffdc65" strokeWidth="4" />
                <rect
                  x="-24"
                  y="-44"
                  width="48"
                  height="18"
                  rx="4"
                  fill="#fff1b6"
                  stroke="#694d20"
                />
                <text
                  x="0"
                  y="-31"
                  textAnchor="middle"
                  fill="#4b3316"
                  fontSize="12"
                  fontWeight="700"
                >
                  {name}
                </text>
              </g>
            )}
            {v.level > 0 ? (
              <circle r="19" fill="none" stroke="#f5ca57" strokeWidth="3" />
            ) : (
              <>
                <circle
                  r="15"
                  fill="#fff3cc"
                  stroke={
                    card && card.owner >= 0
                      ? catanSeatColor(game, card.owner)
                      : "#594728"
                  }
                  strokeWidth="3"
                />
                {assets ? (
                  <image
                    href={picture(assets, m.card)}
                    x="-15"
                    y="-14"
                    width="30"
                    height="28"
                    clipPath={`url(#${clip})`}
                  />
                ) : (
                  <text
                    textAnchor="middle"
                    dominantBaseline="central"
                    fill="#48331c"
                    fontSize="14"
                  >
                    {name.slice(-1)}
                  </text>
                )}
              </>
            )}
          </g>
        );
      })}
      {setup &&
        wonders.setupBlocked
          .filter((v) => !wonders.markers.some((m) => m.vertex === v))
          .map((id) => {
            const v = game.vertices[id];
            return (
              <g
                key={id}
                transform={`translate(${v.x},${v.y}) scale(${scale})`}
              >
                <title>此交点不能放置起始建筑</title>
                <circle
                  r="10"
                  fill="#9f281e"
                  stroke="#f6e7c0"
                  strokeWidth="1.5"
                />
                <path d="M-4-4L4 4M4-4L-4 4" stroke="#fff5d9" strokeWidth="2" />
              </g>
            );
          })}
    </g>
  );
}

function WonderCost({
  cost,
  assets,
  hand,
}: {
  cost: number[];
  assets: string;
  hand?: number[];
}) {
  return (
    <span className="catan-wonder-cost" aria-label="每一级的建造费用">
      {cost.map(
        (n, c) =>
          n > 0 && (
            <span
              key={c}
              className={hand && hand[c] < n ? "missing" : ""}
              title={
                hand
                  ? `${resourceNames[c]}：需要 ${n}，持有 ${hand[c]}`
                  : `${resourceNames[c]} ${n}`
              }
            >
              {assets && (
                <img
                  src={`${assets}/catan/icon-${resources[c]}-v1.webp`}
                  alt=""
                />
              )}
              <span>{resourceNames[c]}</span>
              <b>{n}</b>
            </span>
          ),
      )}
    </span>
  );
}

export function CatanWondersPanel({
  room,
  assets,
  busy,
  act,
  locate,
}: {
  room: Room;
  assets: string;
  busy: boolean;
  act: Act;
  locate: (id: number) => void;
}) {
  const [open, setOpen] = useState(false),
    [collapsed, setCollapsed] = useState(false),
    [selected, setSelected] = useState<number | null>(null);
  const dialog = useRef<HTMLElement>(null),
    trigger = useRef<HTMLButtonElement>(null);
  const game = room.game!.catan!,
    wonders = game.seafarers?.wonders;
  const own = wonders?.cards.find(
    (c) => c.owner === room.you && room.you >= 0 && !room.spectating,
  );
  const rules = game.wonderRules ?? [];
  const canAct =
    room.status === "playing" &&
    !room.game!.finished &&
    !room.spectating &&
    room.you >= 0 &&
    !game.players[room.you]?.eliminated &&
    room.game!.turn === room.you &&
    room.game!.phase === "catan_turn";
  const card = wonders?.cards.find((c) => c.id === selected),
    rule = rules.find((r) => r.id === selected);
  const canClaim = !!card && canAct && !!game.wonderClaims?.includes(card.id);
  const canBuild = !!card && canAct && !!game.wonderBuilds?.includes(card.id);
  const hand = !room.spectating ? game.players[room.you]?.resources : undefined;
  const victoryTarget =
    (!room.spectating && room.you >= 0
      ? game.fishing?.victoryTargets[room.you]
      : undefined) ?? catanSavedVictoryTarget(game);
  const deficit =
    card?.owner === room.you && rule && hand
      ? rule.cost
          .flatMap((n, c) =>
            hand[c] < n ? [`${resourceNames[c]} ${n - hand[c]}`] : [],
          )
          .join("、")
      : "";
  const ownRule = rules.find((r) => r.id === own?.id);
  useEffect(() => {
    if (open) dialog.current?.focus({ preventScroll: true });
  }, [open, selected]);
  useEffect(() => {
    setOpen(false);
    setSelected(null);
  }, [room.id]);
  if (!wonders) return null;
  const close = () => {
    setOpen(false);
    trigger.current?.focus({ preventScroll: true });
  };
  const ownerName = (owner: number) =>
    owner >= 0 ? (room.seats[owner]?.name ?? "玩家") : "尚未领取";
  const show = () => {
    setOpen(true);
    setCollapsed(false);
    setSelected(null);
  };
  return (
    <>
      <section className="catan-wonders-panel" aria-label="奇迹建造">
        <header>
          <h3>
            <Landmark size={18} />
            卡坦奇迹
          </h3>
          <button ref={trigger} onClick={show}>
            查看奇迹 · {wonders.cards.length}
          </button>
        </header>
        <p>
          建至 <b>4级</b> 即获胜，或 <b>{victoryTarget}分且等级领先</b>。
        </p>
        {game.fishing && (
          <p>旧靴子只使持有者的分数门槛增加1分；建成4级仍直接获胜。</p>
        )}
        {own && ownRule ? (
          <button
            className="catan-wonder-own"
            onClick={() => {
              setOpen(true);
              setCollapsed(false);
              setSelected(own.id);
            }}
          >
            {assets && <img src={picture(assets, own.id)} alt="" />}
            <span>
              <strong>{ownRule.name}</strong>
              <WonderSteps level={own.level} />
            </span>
            <b>
              {canAct && game.wonderBuilds?.includes(own.id)
                ? "可以建造"
                : "查看"}
            </b>
          </button>
        ) : (
          <p className="catan-wonder-hint">
            {!room.spectating && game.wonderClaims?.length
              ? "已满足奇迹条件，可查看并确认领取。"
              : "每人只能领取一座奇迹，先满足条件再领取。"}
          </p>
        )}
      </section>
      {open &&
        createPortal(
          <section
            ref={dialog}
            tabIndex={-1}
            role="dialog"
            aria-label="卡坦奇迹"
            className="catan-gold-choice catan-wonders-dialog"
            onKeyDown={(e) => {
              if (e.key === "Escape") {
                e.stopPropagation();
                close();
              }
            }}
          >
            <header>
              <strong>{rule ? rule.name : "卡坦奇迹"}</strong>
              <div>
                <button
                  onClick={() => setCollapsed(!collapsed)}
                  aria-label={collapsed ? "展开奇迹面板" : "收起奇迹面板"}
                  aria-expanded={!collapsed}
                >
                  {collapsed ? (
                    <ChevronUp size={18} />
                  ) : (
                    <ChevronDown size={18} />
                  )}
                </button>
                <button onClick={close} aria-label="关闭奇迹面板">
                  <X size={18} />
                </button>
              </div>
            </header>
            {!collapsed && (
              <div className="catan-gold-body">
                {card && rule ? (
                  <>
                    <button
                      className="catan-wonder-back"
                      onClick={() => setSelected(null)}
                    >
                      <ArrowLeft size={14} />
                      全部奇迹
                    </button>
                    <div className="catan-wonder-detail">
                      {assets && (
                        <img
                          className="catan-wonder-art"
                          src={picture(assets, card.id)}
                          alt={`${rule.name}原画`}
                        />
                      )}
                      <div>
                        <strong>{ownerName(card.owner)}</strong>
                        <WonderSteps level={card.level} />
                        <p>
                          {card.owner < 0
                            ? "满足条件后可免费领取。"
                            : card.level === 0
                              ? "已经领取，尚未建造。"
                              : `已建成 ${card.level} / 4 级。`}
                        </p>
                      </div>
                    </div>
                    <p>
                      <b>领取条件：</b>
                      {rule.requirement}。
                    </p>
                    {wonders.markers.some((m) => m.card === card.id) && (
                      <button
                        className="catan-wonder-locate"
                        onClick={() => {
                          locate(card.id);
                          setCollapsed(true);
                        }}
                      >
                        在地图上定位{rule.name}
                      </button>
                    )}
                    <div className="catan-wonder-price">
                      <b>每级费用</b>
                      <WonderCost
                        cost={rule.cost}
                        assets={assets}
                        hand={hand}
                      />
                    </div>
                    {card.owner < 0 && (
                      <p>
                        领取者独占这座奇迹，每人只能领取一座；领取不会自动建成第一级。
                      </p>
                    )}
                    {card.owner >= 0 &&
                      card.owner === room.you &&
                      !room.spectating && (
                        <p>
                          {game.attack && game.seafarers?.scenario === "wonders"
                            ? "每级都需重新满足条件，被征服建筑不计；恢复条件后可继续建造。"
                            : "满足费用即可继续建造，本回合可建多级。已领取的奇迹不再检查领取条件。"}
                        </p>
                      )}
                    {canClaim ? (
                      <button
                        className="primary wide"
                        disabled={busy}
                        onClick={() =>
                          void act({
                            type: "catan_wonder_claim",
                            card: card.id,
                          }).then(() => setSelected(null))
                        }
                      >
                        确认领取{rule.name}（免费）
                      </button>
                    ) : card.owner === room.you &&
                      !room.spectating &&
                      !room.game!.finished ? (
                      <>
                        {!!deficit && (
                          <p className="catan-wonder-shortage">
                            还缺：{deficit}。
                          </p>
                        )}
                        <button
                          className="primary wide"
                          disabled={busy || !canBuild}
                          onClick={() =>
                            void act({
                              type: "catan_wonder_build",
                              card: card.id,
                            }).then(() => setSelected(null))
                          }
                        >
                          支付费用，建造至第 {Math.min(card.level + 1, 4)} 级
                        </button>
                        {!canAct && <p>在自己的正常行动或配对行动阶段建造。</p>}
                      </>
                    ) : (
                      card.owner < 0 &&
                      !room.spectating && (
                        <p>
                          {own
                            ? "你已经领取了另一座奇迹。"
                            : canAct
                              ? "当前尚未满足领取条件。"
                              : "在自己的行动阶段满足条件后领取。"}
                        </p>
                      )
                    )}
                  </>
                ) : (
                  <>
                    <p>
                      先满足条件并领取，再逐级支付费用建造。点击奇迹查看详情。
                    </p>
                    <div className="catan-wonder-list">
                      {wonders.cards.map((c) => {
                        const r = rules.find((r) => r.id === c.id);
                        if (!r) return null;
                        return (
                          <button
                            key={c.id}
                            className={`catan-wonder-card ${c.owner === room.you && !room.spectating ? "mine" : ""}`}
                            style={
                              {
                                "--wonder-owner":
                                  c.owner >= 0
                                    ? catanSeatColor(game, c.owner)
                                    : "#a28657",
                              } as CSSProperties
                            }
                            onClick={() => setSelected(c.id)}
                            aria-label={`查看${r.name}，${ownerName(c.owner)}，${c.level}级`}
                          >
                            {assets ? (
                              <img src={picture(assets, c.id)} alt="" />
                            ) : (
                              <Landmark size={36} />
                            )}
                            <span>
                              <strong>
                                {r.name}
                                <small>
                                  {game.wonderClaims?.includes(c.id) &&
                                  !room.spectating
                                    ? "可领取"
                                    : ownerName(c.owner)}
                                </small>
                              </strong>
                              {c.owner < 0 ? (
                                <span className="catan-wonder-requirement">
                                  {r.requirement}
                                </span>
                              ) : (
                                <WonderSteps level={c.level} />
                              )}
                              <WonderCost cost={r.cost} assets={assets} />
                            </span>
                          </button>
                        );
                      })}
                    </div>
                    <details className="catan-wonder-rules">
                      <summary>本剧本规则</summary>
                      <p>
                        在自己的行动中，将奇迹建至4级即获胜；或者达到
                        {victoryTarget}
                        分，且已建等级严格高于其他所有玩家。并列不算领先，尚未建造也不能获胜。
                      </p>
                      <p>
                        每位玩家首次在每座小岛建村额外得1分。起始村庄只能放在主岛，奇迹标记与叉号交点禁止起始建设。本剧本不使用海盗，保留最长路线与最大骑士军队奖励。
                      </p>
                    </details>
                  </>
                )}
              </div>
            )}
          </section>,
          document.body,
        )}
    </>
  );
}

export function CatanWondersStart({
  room,
  busy,
  act,
  selectedTile,
  clearTile,
}: {
  room: Room;
  busy: boolean;
  act: Act;
  selectedTile: number | null;
  clearTile: () => void;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const game = room.game!,
    g = game.catan!;
  useEffect(() => setCollapsed(false), [room.id, game.phase, selectedTile]);
  if (
    !g.seafarers?.wonders ||
    game.phase !== "catan_wonders_start" ||
    game.finished ||
    room.status !== "playing"
  )
    return null;
  const mine =
    !room.spectating &&
    room.you >= 0 &&
    room.you === game.turn &&
    !g.players[room.you]?.eliminated;
  return (
    <section
      className="catan-gold-choice catan-wonders-start"
      aria-label="选择初始强盗位置"
    >
      <header>
        <strong>
          {mine
            ? "选择初始强盗位置"
            : `${room.seats[game.turn].name} 正在选择强盗起点`}
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
          <p>
            先手选择一块沙漠作为强盗起点，再开始放置村庄与道路或船只。本剧本不使用海盗。
            {g.citiesKnights && "这里只记录起点；首次蛮族进攻后强盗才入场。"}
          </p>
          <p>限时120秒，超时自动选择。可收起面板查看地图。</p>
          {mine && (
            <>
              <p>
                {selectedTile === null
                  ? "点击地图上亮起的沙漠，再确认起点。"
                  : `已选择沙漠地块 #${selectedTile + 1}。`}
              </p>
              <div className="cloth-confirm-actions">
                {selectedTile !== null && (
                  <button disabled={busy} onClick={clearTile}>
                    重选位置
                  </button>
                )}
                <button
                  className="primary"
                  disabled={
                    busy ||
                    selectedTile === null ||
                    !g.legal.robber?.includes(selectedTile)
                  }
                  onClick={async () => {
                    await act({
                      type: "catan_wonders_start",
                      tile: selectedTile,
                    });
                    clearTile();
                  }}
                >
                  确认起点
                </button>
              </div>
            </>
          )}
        </div>
      )}
    </section>
  );
}
