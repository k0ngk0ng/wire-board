import { useEffect, useState, type ReactNode } from "react";
import { X } from "lucide-react";
import type { Act, Card, Room } from "./types";
import {
  gemBonus,
  gemCardDescription,
  gemCardName,
  gemDefaultPayment,
  gemGoldNeeded,
  gemNames,
  gemPaymentCards,
  gemPaymentError,
} from "./splendor-orient-state";

export function SplendorPurchase({
  room,
  card,
  conquest,
  act,
  busy,
  onClose,
  renderCard,
  renderCost,
}: {
  room: Room;
  card: Card;
  conquest: boolean;
  act: Act;
  busy: boolean;
  onClose: () => void;
  renderCard: (card: Card, onClick?: () => void) => ReactNode;
  renderCost: (cost: number[]) => ReactNode;
}) {
  const g = room.game!,
    s = g.splendor!,
    p = s.players[room.you];
  const viewer = room.spectating || !p;
  const [open, setOpen] = useState(true);
  const [ids, setIDs] = useState<number[]>([]);
  const [tokens, setTokens] = useState<number[]>(() =>
    viewer ? Array(6).fill(0) : gemDefaultPayment(p, card),
  );
  useEffect(() => {
    setIDs([]);
    setOpen(true);
    setTokens(viewer ? Array(6).fill(0) : gemDefaultPayment(p, card));
  }, [card.id, room.version, room.you, viewer]);
  const blocked =
    !!s.strongholds?.[card.id] && s.strongholds[card.id].player !== room.you;
  const available =
    s.market.flat().some((c) => c.id === card.id) ||
    (!viewer && p.reserved?.some((c) => c.id === card.id));
  const acting =
    !viewer &&
    room.status === "playing" &&
    !g.finished &&
    g.turn === room.you &&
    !room.seats[room.you]?.autoPlay &&
    !p.eliminated &&
    available &&
    (conquest
      ? g.phase === "gem_conquest" && s.conquestCard === card.id
      : g.phase === "turn");
  const cost = card.cost.map((n, color) =>
    Math.max(0, n - (viewer ? 0 : p.bonus[color])),
  );
  const choices = viewer ? [] : gemPaymentCards(p, card);
  const sacrificed = card.orient === "sacrifice";
  const error = viewer ? "" : gemPaymentError(p, card, tokens, ids);
  const selected = choices.filter((c) => ids.includes(c.id));
  const toggle = (id: number) => {
    const next = ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id];
    setIDs(next);
    setTokens(gemDefaultPayment(p, card, next, tokens));
  };
  return (
    <section
      className={`discard-panel gem-purchase-panel ${open ? "" : "is-collapsed"}`}
      aria-label={`${conquest ? "征服" : "查看"}${gemCardName(card)}`}
    >
      <header>
        <h3>
          {gemCardName(card)} · {card.points} 分
        </h3>
        <button
          className="subtle"
          aria-expanded={open}
          onClick={() => setOpen(!open)}
        >
          {open ? "查看牌桌" : "继续选择"}
        </button>
        <button
          className="icon-button"
          aria-label="关闭卡牌详情"
          onClick={onClose}
        >
          <X size={18} />
        </button>
      </header>
      {open && (
        <div className="gem-purchase-content">
          <div className="purchase-preview">
            {renderCard(card)}
            <div>
              <p>{gemCardDescription(card)}</p>
              {!sacrificed && (
                <>
                  <small>{viewer ? "购买费用" : "扣除永久折扣后的费用"}</small>
                  {renderCost(cost)}
                </>
              )}
            </div>
          </div>
          {!viewer && (
            <>
              {!sacrificed && cost.some(Boolean) && (
                <div className="payment-grid">
                  {cost.map(
                    (need, color) =>
                      need > 0 && (
                        <label key={color}>
                          {gemNames[color]} · 需要 {need}
                          <select
                            aria-label={`支付${gemNames[color]}数量`}
                            value={tokens[color]}
                            disabled={!acting || busy}
                            onChange={(e) => {
                              const next = [...tokens];
                              next[color] = Number(e.target.value);
                              setTokens(gemDefaultPayment(p, card, ids, next));
                            }}
                          >
                            {Array.from(
                              { length: Math.min(need, p.tokens[color]) + 1 },
                              (_, n) => (
                                <option key={n} value={n}>
                                  {n} 枚宝石 +{" "}
                                  {Math.ceil(
                                    (need - n) /
                                      (p.tradingPosts?.includes(4) ? 2 : 1),
                                  )}{" "}
                                  枚黄金
                                </option>
                              ),
                            )}
                          </select>
                        </label>
                      ),
                  )}
                </div>
              )}
              {!!choices.length && (
                <fieldset
                  className="gem-payment-cards"
                  disabled={!acting || busy}
                >
                  <legend>
                    {sacrificed ? "选择两张要弃置的发展卡" : "可弃置黄金卡支付"}
                  </legend>
                  <div className="gem-payment-card-grid">
                    {choices.map((c) => (
                      <div
                        key={c.id}
                        className={ids.includes(c.id) ? "selected" : ""}
                      >
                        {renderCard(c, () => toggle(c.id))}
                        <button
                          type="button"
                          className="outline"
                          aria-pressed={ids.includes(c.id)}
                          onClick={() => toggle(c.id)}
                        >
                          {ids.includes(c.id) ? "已选 · 取消" : "选择弃置"}
                        </button>
                      </div>
                    ))}
                  </div>
                </fieldset>
              )}
              {sacrificed ? (
                <p className="gem-payment-summary">
                  已选 {ids.length}/2 张。失去{" "}
                  {selected.reduce((n, c) => n + c.points, 0)} 分及{" "}
                  {selected.reduce((n, c) => n + gemBonus(c), 0)} 枚
                  {gemNames[card.sacrificeColor ?? 0]}
                  永久折扣。已获得的贵族和贸易站保留。
                </p>
              ) : (
                <div className="gem-payment-summary">
                  <p>
                    实体黄金 {tokens[5]} 枚 / 持有 {p.tokens[5]} 枚
                    {ids.length > 0 && `；弃置 ${ids.length} 张黄金卡`}
                  </p>
                  {ids.length > 0 && (
                    <label>
                      实体黄金用量
                      <select
                        aria-label="支付实体黄金数量"
                        disabled={!acting || busy}
                        value={tokens[5]}
                        onChange={(e) =>
                          setTokens([
                            ...tokens.slice(0, 5),
                            Number(e.target.value),
                          ])
                        }
                      >
                        {Array.from(
                          { length: Math.max(p.tokens[5], tokens[5]) + 1 },
                          (_, n) => (
                            <option value={n} key={n}>
                              {n} 枚
                            </option>
                          ),
                        )}
                      </select>
                      <small>
                        虚拟黄金使用{" "}
                        {gemGoldNeeded(p, card.cost, tokens) - tokens[5]}{" "}
                        枚，每张至少使用一枚，多余不找零。
                      </small>
                    </label>
                  )}
                </div>
              )}
              {!!error && (
                <p className="gem-payment-error" role="status">
                  {error}
                </p>
              )}
              <div className="form-grid gem-purchase-actions">
                <button
                  className="primary"
                  data-card-acquire={card.id}
                  disabled={!acting || busy || blocked || !!error}
                  onClick={() =>
                    void act({
                      type: conquest ? "gem_conquest" : "buy",
                      card: card.id,
                      tokens,
                      cards: ids,
                    })
                  }
                >
                  {blocked
                    ? "对手要塞占据"
                    : sacrificed
                      ? "弃置两张并购买"
                      : ids.length
                        ? "弃置黄金卡并购买"
                        : conquest
                          ? "确认征服并购买"
                          : "购买卡牌"}
                </button>
                {!conquest && !p.reserved?.some((c) => c.id === card.id) && (
                  <button
                    className="outline"
                    disabled={
                      !acting ||
                      busy ||
                      blocked ||
                      (p.reserved?.length ?? 0) >= 3
                    }
                    onClick={() => void act({ type: "reserve", card: card.id })}
                  >
                    预留{s.bank[5] > 0 ? " · 黄金 +1" : ""}
                  </button>
                )}
              </div>
            </>
          )}
        </div>
      )}
    </section>
  );
}
