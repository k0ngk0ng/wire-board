import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { createPortal } from "react-dom";
import type { Room } from "./types";
import "./splendor-token-animation.css";

type TokenFlight = {
  key: string;
  color: number;
  player: number;
  returning: boolean;
  style: CSSProperties;
};

export function SplendorTokenAnimation({
  room,
  assets,
  colors,
  renderGem,
}: {
  room: Room;
  assets: string;
  colors: string[];
  renderGem: (color: number) => ReactNode;
}) {
  const g = room.game!.splendor!;
  const seen = useRef({ room: room.id, id: g.tokenEventId ?? 0 });
  const [flights, setFlights] = useState<TokenFlight[]>([]);
  useLayoutEffect(() => {
    const id = g.tokenEventId ?? 0;
    if (seen.current.room !== room.id || id < seen.current.id) {
      seen.current = { room: room.id, id };
      setFlights([]);
      return;
    }
    const events = (g.tokenEvents ?? [])
      .filter((e) => e.id > seen.current.id)
      .slice(-3);
    seen.current.id = id;
    if (!events.length || document.hidden) return;
    const center = (rect: DOMRect) => ({
      x: Math.max(8, Math.min(rect.x + rect.width / 2 - 19, innerWidth - 46)),
      y: Math.max(8, Math.min(rect.y + rect.height / 2 - 19, innerHeight - 46)),
    });
    const fresh: TokenFlight[] = [];
    for (const event of events) {
      const avatar = document.querySelector(
        `[data-player-seat="${event.player}"] .avatar`,
      );
      if (!avatar) continue;
      const player = center(avatar.getBoundingClientRect());
      const returning = event.action === "return" || event.action === "pay";
      let order = 0;
      event.tokens.forEach((count, color) => {
        if (!count) return;
        const supply = document.querySelector(
          `[data-bank-color="${color}"] .token`,
        );
        if (!supply) return;
        const bank = center(supply.getBoundingClientRect());
        const from = returning ? player : bank;
        const to = returning ? bank : player;
        for (let n = 0; n < count; n++) {
          fresh.push({
            key: `${event.id}:${color}:${n}`,
            color,
            player: event.player,
            returning,
            style: {
              left: from.x,
              top: from.y,
              "--token-x": `${to.x - from.x}px`,
              "--token-y": `${to.y - from.y}px`,
              backgroundColor: colors[color],
              backgroundPosition: `${color * 20}% 0`,
              animationDelay: `${order++ * 75}ms`,
            } as CSSProperties,
          });
        }
      });
    }
    if (fresh.length)
      setFlights((previous) => [...previous, ...fresh].slice(-36));
  }, [g.tokenEventId, g.tokenEvents, room.id, colors]);
  useEffect(() => {
    if (!flights.length) return;
    const timer = setTimeout(() => setFlights([]), 3000);
    return () => clearTimeout(timer);
  }, [flights]);
  return createPortal(
    <div
      className={`splendor-token-animation ${assets ? "with-artwork" : ""}`}
      aria-hidden="true"
      inert
      style={
        {
          "--splendor-tokens": assets
            ? `url("${assets}/splendor/tokens.webp")`
            : "none",
        } as CSSProperties
      }
    >
      {flights.map((flight) => (
        <span
          key={flight.key}
          className="token splendor-flying-token"
          style={flight.style}
          data-token-flight={flight.key}
          data-flight-player={flight.player}
          data-flight-color={flight.color}
          data-flight-direction={flight.returning ? "to-bank" : "to-player"}
          onAnimationEnd={() =>
            setFlights((previous) =>
              previous.filter((item) => item.key !== flight.key),
            )
          }
        >
          {renderGem(flight.color)}
        </span>
      ))}
    </div>,
    document.body,
  );
}
