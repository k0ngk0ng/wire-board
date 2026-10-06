import { useState, type ReactNode } from "react";
import type { Card, Room } from "./types";
import { AnimatedSlot } from "./animated-slot";
import { StrongholdBadge } from "./splendor-expansions";
import { orientBackStyle } from "./splendor-orient-art";

export function SplendorMarket({
  room,
  busy,
  onReserve,
  renderCard,
  assets,
}: {
  room: Room;
  busy: boolean;
  onReserve: (row: number) => void;
  renderCard: (card: Card) => ReactNode;
  assets: string;
}) {
  const g = room.game!,
    s = g.splendor!;
  const [orientPage, setOrientPage] = useState(false);
  const taking =
    room.status === "playing" &&
    !room.spectating &&
    !g.finished &&
    g.turn === room.you &&
    g.phase === "turn" &&
    !room.seats[room.you]?.autoPlay;
  const row = (tier: number, orient: boolean) => {
    const index = tier + (orient ? 3 : 0);
    const deck = (
      <button
        className={`deck tier-${tier} ${orient ? `orient-deck ${assets ? "has-orient-back" : ""}` : ""}`}
        style={
          orient
            ? assets
              ? orientBackStyle(assets, tier + 1)
              : undefined
            : { backgroundPosition: `${tier * 20}% 100%` }
        }
        disabled={
          !taking ||
          busy ||
          !s.remaining[index] ||
          (s.players[room.you]?.reserved?.length ?? 0) >= 3
        }
        onClick={() => onReserve(index)}
        aria-label={`从 ${orient ? "东方 " : ""}${tier + 1} 级牌堆预留一张牌`}
        aria-haspopup="dialog"
      >
        <span className="deck-ornament">✧</span>
        {orient && assets && (
          <img
            className="orient-deck-symbol"
            src={`${assets}/splendor/expansions/orient.webp`}
            alt=""
          />
        )}
        <strong>{["Ⅰ", "Ⅱ", "Ⅲ"][tier]}</strong>
        {orient && <span className="orient-deck-label">东方</span>}
        <small className="deck-count">
          {s.remaining[index]}
          <span className="sr-only"> 张 · 盲预留</span>
        </small>
      </button>
    );
    return (
      <div
        className={`market-row ${orient ? "orient-row" : "base-row"}`}
        data-splendor-tier={tier + 1}
        data-splendor-orient={orient ? "true" : "false"}
      >
        {!orient && deck}
        {Array.from({ length: orient ? 2 : 4 }, (_, slot) => {
          const card = s.market[index]?.[slot];
          return (
            <AnimatedSlot
              key={slot}
              marker={slot}
              identity={String(card?.id ?? "empty")}
            >
              {card?.id ? (
                <div className="gem-market-card">
                  {renderCard(card)}
                  <StrongholdBadge room={room} card={card} assets={assets} />
                </div>
              ) : (
                <div
                  className="dev-card card-placeholder"
                  aria-label="当前空位，效果结算后补牌；牌堆耗尽则保持空位"
                />
              )}
            </AnimatedSlot>
          );
        })}
        {orient && deck}
      </div>
    );
  };
  return (
    <div
      className={`market ${s.options?.orient ? "with-orient-market" : ""} ${orientPage ? "show-orient" : "show-base"}`}
    >
      {s.options?.orient && (
        <div className="gem-market-switch" aria-label="切换牌堆显示">
          <button
            aria-pressed={!orientPage}
            onClick={() => setOrientPage(false)}
          >
            基础牌 ·{" "}
            {
              s.market
                .slice(0, 3)
                .flat()
                .filter((c) => c.id > 0).length
            }{" "}
            张
          </button>
          <button aria-pressed={orientPage} onClick={() => setOrientPage(true)}>
            东方牌 ·{" "}
            {
              s.market
                .slice(3, 6)
                .flat()
                .filter((c) => c.id > 0).length
            }{" "}
            张
          </button>
        </div>
      )}
      {[2, 1, 0].map((tier) =>
        s.options?.orient ? (
          <div className="gem-market-tier" key={tier}>
            {row(tier, false)}
            {row(tier, true)}
          </div>
        ) : (
          <div key={tier} className="gem-base-tier">
            {row(tier, false)}
          </div>
        ),
      )}
    </div>
  );
}
