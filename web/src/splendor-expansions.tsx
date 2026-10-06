import { useEffect, useState, type ReactNode } from "react";
import { Castle, Store } from "lucide-react";
import type { Act, Card, GemPlayer, Room, SplendorOptions } from "./types";
import "./splendor-expansions.css";

export const gemPostNames = [
  "",
  "购牌馈赠",
  "择优预留",
  "额外宝石",
  "黄金增值",
  "贸易声望",
];

export function splendorRulesLabel(options?: SplendorOptions) {
  const names = [
    options?.cities && "城市",
    options?.tradingPosts && "贸易站",
    options?.orient && "东方",
    options?.strongholds && "要塞",
  ].filter(Boolean);
  return names.length ? `基础＋${names.join("＋")}` : "基础版";
}

export function SplendorOptionPicker({
  value,
  onChange,
  disabled = false,
}: {
  value?: SplendorOptions;
  onChange: (value: SplendorOptions) => void;
  disabled?: boolean;
}) {
  return (
    <fieldset className="gem-expansion-options" disabled={disabled}>
      <legend>璀璨宝石扩展</legend>
      <p>可以单独加入，也可以组合游玩。更换规则后需要重新准备。</p>
      <div>
        {(
          [
            ["tradingPosts", "贸易站", "达成条件，解锁持续能力", Store],
            ["strongholds", "要塞", "占据发展卡，集结三座发动征服", Castle],
          ] as const
        ).map(([key, name, description, Icon]) => (
          <button
            type="button"
            key={key}
            aria-pressed={!!value?.[key]}
            className={value?.[key] ? "selected" : ""}
            onClick={() => onChange({ ...value, [key]: !value?.[key] })}
          >
            <Icon size={22} />
            <span>
              <strong>{name}</strong>
              <small>{description}</small>
            </span>
          </button>
        ))}
      </div>
    </fieldset>
  );
}

export function gemGoldNeeded(p: GemPlayer, cost: number[], spent?: number[]) {
  const value = p.tradingPosts?.includes(4) ? 2 : 1;
  return cost.reduce(
    (total, n, i) =>
      total +
      Math.ceil(
        Math.max(0, n - p.bonus[i] - (spent?.[i] ?? p.tokens[i])) / value,
      ),
    0,
  );
}

export function StrongholdBadge({ room, card }: { room: Room; card: Card }) {
  const hold = room.game?.splendor?.strongholds?.[card.id];
  if (!hold) return null;
  return (
    <span
      className={`gem-stronghold-badge seat-${hold.player}`}
      title={`${room.seats[hold.player]?.name}的要塞：${hold.count} 座`}
    >
      <Castle size={15} />
      <b>{hold.count}</b>
      <small>{room.seats[hold.player]?.name}</small>
    </span>
  );
}

export function SplendorExpansionBoard({
  room,
  assets,
  act,
  busy,
  renderCard,
  renderCost,
  renderGem,
}: {
  room: Room;
  assets: string;
  act: Act;
  busy: boolean;
  renderCard: (card: Card, onClick?: () => void) => ReactNode;
  renderCost: (cost: number[]) => ReactNode;
  renderGem: (color: number) => ReactNode;
}) {
  const g = room.game!,
    s = g.splendor!;
  const mine = !room.spectating && room.you === g.turn && !g.finished;
  const [open, setOpen] = useState(true);
  const [target, setTarget] = useState(0);
  useEffect(() => {
    setOpen(true);
    setTarget(0);
  }, [g.phase, room.version]);
  const pending = mine && g.phase.startsWith("gem_");
  const rules = s.tradingPostRules || [];
  const actions = s.strongholdActions || [];
  const targets = s.market
    .flat()
    .filter((c) => c.id > 0 && actions.some((a) => a.card === c.id));
  const conquest = s.market.flat().find((c) => c.id === s.conquestCard);
  const title: Record<string, string> = {
    gem_post: "选择一个贸易站",
    gem_token: "领取贸易站宝石",
    gem_reserve: "选择一张预留卡",
    gem_stronghold: "布置一座要塞",
    gem_conquest: "发动征服",
  };
  return (
    <>
      {!!rules.length && (
        <section className="gem-trading-posts" aria-label="贸易站">
          {rules.map((rule) => (
            <details key={rule.id}>
              <summary>
                <Store size={16} />
                <strong>{rule.name}</strong>
                {renderCost(rule.cost)}
              </summary>
              {assets && (
                <img
                  className="gem-post-art"
                  src={`${assets}/splendor/expansions/post-${rule.id}.webp`}
                  alt={`${rule.name}原版贸易站牌`}
                  loading="lazy"
                  width={206}
                  height={138}
                />
              )}
              <p>{rule.description}</p>
              <small>需要对应颜色的发展卡张数，每回合最多获得一个。</small>
              <div className="gem-post-owners">
                {s.players.map((p, i) =>
                  p.tradingPosts?.includes(rule.id) ? (
                    <span key={i}>{room.seats[i]?.name}</span>
                  ) : null,
                )}
              </div>
            </details>
          ))}
        </section>
      )}
      {pending && (
        <section
          className={`discard-panel gem-effect-panel ${open ? "" : "is-collapsed"}`}
          aria-label={title[g.phase]}
        >
          <header>
            <h3>{title[g.phase]}</h3>
            <button
              className="subtle"
              aria-expanded={open}
              onClick={() => setOpen(!open)}
            >
              {open ? "查看牌桌" : "继续选择"}
            </button>
          </header>
          {open && (
            <div className="gem-effect-content">
              {g.phase === "gem_post" && (
                <div className="gem-post-choices">
                  {rules
                    .filter((r) => s.postChoices?.includes(r.id))
                    .map((r) => (
                      <button
                        key={r.id}
                        disabled={busy}
                        onClick={() =>
                          void act({ type: "gem_post", card: r.id })
                        }
                      >
                        {assets && (
                          <img
                            className="gem-post-choice-art"
                            src={`${assets}/splendor/expansions/post-${r.id}.webp`}
                            alt=""
                            width={206}
                            height={138}
                          />
                        )}
                        <strong>{r.name}</strong>
                        {renderCost(r.cost)}
                        <span>{r.description}</span>
                      </button>
                    ))}
                </div>
              )}
              {g.phase === "gem_token" && (
                <>
                  <p>拿取一枚非黄金宝石；之后再检查十枚上限。</p>
                  <div className="gem-effect-tokens">
                    {s.bank.slice(0, 5).map((n, i) => (
                      <button
                        key={i}
                        aria-label={`领取${["祖母绿", "钻石", "蓝宝石", "缟玛瑙", "红宝石"][i]}，供应 ${n} 枚`}
                        disabled={
                          busy || n === 0 || i === s.effects?.[0]?.exclude
                        }
                        onClick={() =>
                          void act({ type: "gem_token", color: i })
                        }
                      >
                        {renderGem(i)}
                        <b>{n}</b>
                      </button>
                    ))}
                  </div>
                </>
              )}
              {g.phase === "gem_reserve" && (
                <>
                  <p>
                    {s.reserveChoice?.length === 1
                      ? "牌堆只剩这一张牌，只有你能看到它。"
                      : "只有你能看到这两张牌。选中一张预留，另一张放回牌堆底部。"}
                  </p>
                  <div className="gem-effect-cards">
                    {s.reserveChoice?.map((c) => (
                      <div key={c.id}>
                        {renderCard(c)}
                        <button
                          className="primary"
                          disabled={busy}
                          onClick={() =>
                            void act({ type: "gem_reserve", card: c.id })
                          }
                        >
                          预留此牌
                        </button>
                      </div>
                    ))}
                  </div>
                </>
              )}
              {g.phase === "gem_stronghold" && (
                <>
                  <p>
                    选一张牌放置或移动己方要塞，也可移除对手单独一座要塞。对手两座以上的要塞受保护。
                  </p>
                  <div className="gem-stronghold-targets">
                    {targets.map((c) => (
                      <div
                        key={c.id}
                        className={target === c.id ? "selected" : ""}
                      >
                        {renderCard(c, () => setTarget(c.id))}
                        <StrongholdBadge room={room} card={c} />
                      </div>
                    ))}
                  </div>
                  <div className="gem-effect-actions">
                    {actions
                      .filter((a) => a.card === target)
                      .map((a) => (
                        <button
                          key={`${a.choice}-${a.target}`}
                          className="primary"
                          disabled={busy}
                          onClick={() => void act(a)}
                        >
                          {a.choice === "remove"
                            ? "移除对手的单座要塞"
                            : a.target === 0
                              ? "放置库存要塞"
                              : a.target === a.card
                                ? "保持这座要塞的位置"
                                : `从发展卡 #${a.target} 移动一座`}
                        </button>
                      ))}
                  </div>
                </>
              )}
              {g.phase === "gem_conquest" && conquest && (
                <>
                  <p>
                    三座要塞已集结，可以额外支付费用购买此牌。刚获得的宝石和折扣可用于此次购买。
                  </p>
                  <div className="gem-effect-cards">{renderCard(conquest)}</div>
                  <div className="form-grid">
                    <button
                      className="outline"
                      disabled={busy}
                      onClick={() => void act({ type: "gem_conquest_skip" })}
                    >
                      本回合不征服
                    </button>
                    <button
                      className="primary"
                      disabled={busy}
                      onClick={() =>
                        void act({ type: "gem_conquest", card: conquest.id })
                      }
                    >
                      征服并购买
                    </button>
                  </div>
                </>
              )}
            </div>
          )}
        </section>
      )}
    </>
  );
}
