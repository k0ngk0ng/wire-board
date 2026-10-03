import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { createPortal } from "react-dom";
import type { Card, Noble, Room, SplendorCardEvent } from "./types";
import "./splendor-card-animation.css";

type Flight = {
  event: SplendorCardEvent;
  style: CSSProperties;
  arrivalStyle: CSSProperties;
  label: string;
};

export function SplendorCardAnimation({
  room,
  assets,
  renderCard,
  renderNoble,
}: {
  room: Room;
  assets: string;
  renderCard: (card: Card) => ReactNode;
  renderNoble: (noble: Noble) => ReactNode;
}) {
  const splendor = room.game!.splendor!;
  const seen = useRef({ room: room.id, id: splendor.cardEventId ?? 0 });
  const reservedRects = useRef(new Map<number, DOMRect>());
  const nobleRects = useRef(new Map<number, DOMRect>());
  const [flights, setFlights] = useState<Flight[]>([]);

  useLayoutEffect(() => {
    const id = splendor.cardEventId ?? 0;
    const rememberSources = () => {
      reservedRects.current = new Map(
        Array.from(
          document.querySelectorAll<HTMLElement>(".reserved [data-card-id]"),
        ).map((element) => [
          Number(element.dataset.cardId),
          element.getBoundingClientRect(),
        ]),
      );
      nobleRects.current = new Map(
        Array.from(
          document.querySelectorAll<HTMLElement>(
            ".splendor-board .nobles [data-noble-id]",
          ),
        ).map((element) => [
          Number(element.dataset.nobleId),
          element.getBoundingClientRect(),
        ]),
      );
    };
    if (seen.current.room !== room.id || id < seen.current.id) {
      seen.current = { room: room.id, id };
      setFlights([]);
      rememberSources();
      return;
    }
    const events = (splendor.cardEvents ?? []).filter(
      (event) => event.id > seen.current.id,
    );
    seen.current.id = id;
    const clamp = (value: number, max: number) =>
      Math.max(8, Math.min(value, max));
    const fresh = document.hidden
      ? []
      : events.flatMap((event, index): Flight[] => {
          const player = document.querySelector(
            `[data-player-seat="${event.player}"]`,
          );
          const target = (
            player?.querySelector(".avatar") ?? player
          )?.getBoundingClientRect();
          if (!target) return [];
          const row = `.market-row[data-splendor-tier="${event.tier}"]`;
          const source =
            event.source === "nobles" && event.noble
              ? (nobleRects.current.get(event.noble.id) ??
                document
                  .querySelector(".splendor-board .nobles")
                  ?.getBoundingClientRect())
              : event.source === "market"
                ? document
                    .querySelector(`${row} [data-market-slot="${event.slot}"]`)
                    ?.getBoundingClientRect()
                : event.source === "deck"
                  ? document
                      .querySelector(`${row} .deck`)
                      ?.getBoundingClientRect()
                  : ((event.player === room.you && event.card
                      ? reservedRects.current.get(event.card.id)
                      : undefined) ??
                    player
                      ?.querySelector(".reserved-count")
                      ?.getBoundingClientRect());
          if (!source) return [];
          const width = Math.min(104, Math.max(54, source.width));
          const height = event.noble ? width : (width * 7) / 5;
          const x = clamp(
            source.x + source.width / 2 - width / 2,
            innerWidth - width - 8,
          );
          const y = clamp(
            source.y + source.height / 2 - height / 2,
            innerHeight - height - 8,
          );
          const tx = clamp(target.x + target.width / 2, innerWidth - 24);
          const ty = clamp(target.y + target.height / 2, innerHeight - 24);
          const dx = tx - (x + width / 2),
            dy = ty - (y + height / 2);
          return [
            {
              event,
              label: `${room.seats[event.player]?.name || "玩家"} · ${event.action === "noble" ? "贵族 +3 分" : event.action === "buy" ? "购买" : "预留"}`,
              style: {
                left: x,
                top: y,
                width,
                height,
                "--flight-x": `${dx}px`,
                "--flight-y": `${dy}px`,
                animationDelay: `${index * 160}ms`,
              } as CSSProperties,
              arrivalStyle: {
                left: clamp(tx - 70, innerWidth - 148),
                top: clamp(ty + 14, innerHeight - 38),
                animationDelay: `${index * 160 + 700}ms`,
              },
            },
          ];
        });
    if (fresh.length)
      setFlights((previous) => [...previous, ...fresh].slice(-12));
    rememberSources();
    // Keep pre-removal card and noble positions current when the player scrolls.
    window.addEventListener("scroll", rememberSources, true);
    window.addEventListener("resize", rememberSources);
    return () => {
      window.removeEventListener("scroll", rememberSources, true);
      window.removeEventListener("resize", rememberSources);
    };
  }, [
    splendor.cardEventId,
    splendor.cardEvents,
    room.id,
    room.you,
    room.seats,
  ]);

  useEffect(() => {
    if (!flights.length) return;
    const timer = setTimeout(() => setFlights([]), 4000);
    return () => clearTimeout(timer);
  }, [flights]);

  return createPortal(
    <div
      className={`splendor-card-animation ${assets ? "with-artwork" : ""}`}
      aria-hidden="true"
      inert
      style={
        {
          "--splendor-cards": assets
            ? `url("${assets}/splendor/cards.webp")`
            : "none",
          "--splendor-nobles": assets
            ? `url("${assets}/splendor/nobles.webp")`
            : "none",
        } as CSSProperties
      }
    >
      {flights.map(({ event, style, arrivalStyle, label }) => (
        <div
          key={event.id}
          data-card-flight={event.id}
          data-flight-player={event.player}
          data-flight-action={event.action}
        >
          <div className="splendor-card-flight" style={style}>
            {event.noble ? (
              renderNoble(event.noble)
            ) : event.card ? (
              renderCard(event.card)
            ) : (
              <div
                className={`deck tier-${event.tier - 1}`}
                style={{ backgroundPosition: `${(event.tier - 1) * 20}% 100%` }}
              >
                <span className="deck-ornament">✧</span>
                <strong>{["Ⅰ", "Ⅱ", "Ⅲ"][event.tier - 1]}</strong>
              </div>
            )}
          </div>
          <span
            className="splendor-card-arrival"
            style={arrivalStyle}
            onAnimationEnd={() =>
              setFlights((previous) =>
                previous.filter((flight) => flight.event.id !== event.id),
              )
            }
          >
            {label}
          </span>
        </div>
      ))}
    </div>,
    document.body,
  );
}
