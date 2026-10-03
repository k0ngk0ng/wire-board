import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { createPortal } from "react-dom";
import type { Card, Room, SplendorCardEvent } from "./types";
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
}: {
  room: Room;
  assets: string;
  renderCard: (card: Card) => ReactNode;
}) {
  const splendor = room.game!.splendor!;
  const seen = useRef({ room: room.id, id: splendor.cardEventId ?? 0 });
  const reservedRects = useRef(new Map<number, DOMRect>());
  const [flights, setFlights] = useState<Flight[]>([]);

  useLayoutEffect(() => {
    const id = splendor.cardEventId ?? 0;
    const rememberReservations = () => {
      reservedRects.current = new Map(
        Array.from(
          document.querySelectorAll<HTMLElement>(".reserved [data-card-id]"),
        ).map((element) => [
          Number(element.dataset.cardId),
          element.getBoundingClientRect(),
        ]),
      );
    };
    if (seen.current.room !== room.id || id < seen.current.id) {
      seen.current = { room: room.id, id };
      setFlights([]);
      rememberReservations();
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
            event.source === "market"
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
          const height = (width * 7) / 5;
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
              label: `${room.seats[event.player]?.name || "玩家"} · ${event.action === "buy" ? "购买" : "预留"}`,
              style: {
                left: x,
                top: y,
                width,
                height,
                "--flight-x": `${dx}px`,
                "--flight-y": `${dy}px`,
                "--flight-mid-x": `${dx * 0.45}px`,
                "--flight-mid-y": `${dy * 0.45 - 30}px`,
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
    rememberReservations();
    // Keep pre-removal hand positions current when the player scrolls on mobile.
    window.addEventListener("scroll", rememberReservations, true);
    window.addEventListener("resize", rememberReservations);
    return () => {
      window.removeEventListener("scroll", rememberReservations, true);
      window.removeEventListener("resize", rememberReservations);
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
        } as CSSProperties
      }
    >
      {flights.map(({ event, style, arrivalStyle, label }) => (
        <div
          key={event.id}
          data-card-flight={event.id}
          data-flight-player={event.player}
        >
          <div className="splendor-card-flight" style={style}>
            {event.card ? (
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
