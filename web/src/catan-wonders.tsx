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

export function CatanWondersRules({ game }: { game: CatanState }) {
  return (
    <>
      <p>
        航海家 · 卡坦奇迹，{game.players.length}
        人。每人只能领取一座奇迹，领取后由自己独占。本局不使用海盗。
      </p>
      <ol>
        <li>
          <b>起始建设：</b>
          按顺序放一组村庄与道路或船只，再逆序放第二组；第二座村庄领取相邻资源。起始村庄限主岛，不能放在奇迹标记或叉号交点。
        </li>
        <li>
          <b>生产与强盗：</b>
          掷两颗骰子，对应地块向村庄发一张、城市发两张资源；强盗所在的地块不生产。金矿产出由玩家选择资源。掷出7时，手牌超过七张的玩家同时弃一半（向下取整），随后移动强盗并从相邻一位有资源的对手偷一张。
        </li>
        <li>
          <b>交易与建设：</b>
          掷骰后可交易、建造及购买发展卡。银行4:1、通用港口3:1、专用港口2:1；玩家交易需双方同意。村庄须接己方路线，并与所有建筑至少相隔两条边；城市升级已有村庄。
        </li>
        <li>
          <b>船只与探索：</b>
          一木一羊造船，沿海岸或海洋连接己方船只或建筑。道路与船只只能在己方建筑处接续。每个行动阶段可移动一艘未在本阶段新建、位于开放航线末端的船。每位玩家首次在每座小岛建村额外得1分。
        </li>
        <li>
          <b>领取奇迹：</b>
          满足卡面条件后免费领取，等级为0，不自动建成第一级。其他人不能再领取该奇迹，你也不能领取第二座。地图上的大桥、长城或灯塔条件要求在对应标记处拥有建筑。
        </li>
        <li>
          <b>建造奇迹：</b>
          每一级支付卡面相同的五份资源，最高4级。同一行动阶段可以连续建造多级；已经领取后不再重复检查领取条件。奇迹等级本身不增加分数。
        </li>
        <li>
          <b>获胜：</b>
          在自己的行动中，建成4级立即获胜；或者达到10分且已建奇迹等级严格领先其他所有玩家。并列、仅领取未建造或只有10分都不够。
        </li>
        <li>
          <b>发展卡与奖励：</b>
          每个行动阶段最多使用一张发展卡，新购卡在下次自己的行动阶段可用。胜利点自动加入私人总分。最长路线至少5段、最大骑士军队至少3名，各奖励2分；并列由原持有者保留，原持有者不在并列中则暂时无人持有。
        </li>
      </ol>
      {game.options?.fiveSix && (
        <p>
          五至六人使用7张奇迹卡，新增灯塔与大图书馆。①号玩家正常行动后，由左侧第三位②号玩家行动：不掷骰、不与其他玩家自由交易，可以建造奇迹、其他建设、使用发展卡及银行/港口交易，也可以获胜。两人结束后标记向下一位移动。
        </p>
      )}
      {game.options?.helpers && (
        <p>
          本局启用Helpers。点击自己的助手查看能力；使用后按提示翻面或更换。奇迹费用不享受道路、建筑或发展卡专用的助手折扣。
        </p>
      )}
      <p>
        起始建设、弃牌和资源选择限时120秒，超时自动处理；正常回合限时120秒，领取与建造奇迹共用该倒计时。私人资源与发展卡只有本人可见，结算后公开。
      </p>
    </>
  );
}

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
                <title>此交点不能放置起始村庄</title>
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
          建至 <b>4级</b> 即获胜，或 <b>10分且等级领先</b>。
        </p>
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
                          满足费用即可继续建造，本回合可建多级。已领取的奇迹不再检查领取条件。
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
                        在自己的行动中，将奇迹建至4级即获胜；或者达到10分，且已建等级严格高于其他所有玩家。并列不算领先，尚未建造也不能获胜。
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
