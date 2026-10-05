import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { createPortal } from "react-dom";
import type { Room } from "./types";
import { visibleFlightAnchor } from "./flight-anchor";
import "./splendor-token-animation.css";

type TokenFlight = {
  key: string;
  color: number;
  player: number;
  returning: boolean;
  bank: string;
  avatar: string;
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
  const layer = useRef<HTMLDivElement>(null);
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
    const fresh: TokenFlight[] = [];
    for (const event of events) {
      const avatar = `[data-player-seat="${event.player}"] .avatar`;
      const returning = event.action === "return" || event.action === "pay";
      let order = 0;
      event.tokens.forEach((count, color) => {
        if (!count) return;
        const bank = `[data-bank-color="${color}"] .token`;
        for (let n = 0; n < count; n++) {
          fresh.push({
            key: `${event.id}:${color}:${n}`,
            color,
            player: event.player,
            returning,
            bank,
            avatar,
            style: {
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
  useLayoutEffect(() => {
    if (!flights.length) return;
    let frame = 0;
    const update = () => {
      frame = 0;
      // Batch geometry reads before writes. Scroll updates do not rerender
      // React or restart the transform animation / its staggered delay.
      const anchors = new Map<string, ReturnType<typeof visibleFlightAnchor>>();
      const anchor = (selector: string) => {
        if (!anchors.has(selector)) {
          const element = document.querySelector(selector);
          anchors.set(selector, element ? visibleFlightAnchor(element) : null);
        }
        return anchors.get(selector);
      };
      const measurements = flights.map((flight) => {
        const bankPoint = anchor(flight.bank);
        const playerPoint = anchor(flight.avatar);
        return {
          key: flight.key,
          from: flight.returning ? playerPoint : bankPoint,
          to: flight.returning ? bankPoint : playerPoint,
        };
      });
      for (const { key, from, to } of measurements) {
        const token = layer.current?.querySelector<HTMLElement>(
          `[data-token-flight="${key}"]`,
        );
        if (!token) continue;
        token.style.visibility = from && to ? "visible" : "hidden";
        if (!from || !to) continue;
        token.style.left = `${from.x - 19}px`;
        token.style.top = `${from.y - 19}px`;
        token.style.setProperty("--token-x", `${to.x - from.x}px`);
        token.style.setProperty("--token-y", `${to.y - from.y}px`);
      }
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    update();
    window.addEventListener("scroll", schedule, true);
    window.addEventListener("resize", schedule);
    window.visualViewport?.addEventListener("scroll", schedule);
    window.visualViewport?.addEventListener("resize", schedule);
    const observer = new ResizeObserver(schedule);
    const observed = new Set<Element>();
    for (const flight of flights) {
      for (const selector of [flight.bank, flight.avatar]) {
        for (
          let element = document.querySelector(selector);
          element;
          element = element.parentElement
        ) {
          if (!observed.has(element)) {
            observed.add(element);
            observer.observe(element);
          }
        }
      }
    }
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      window.removeEventListener("scroll", schedule, true);
      window.removeEventListener("resize", schedule);
      window.visualViewport?.removeEventListener("scroll", schedule);
      window.visualViewport?.removeEventListener("resize", schedule);
    };
  }, [flights]);
  useEffect(() => {
    if (!flights.length) return;
    const timer = setTimeout(() => setFlights([]), 3000);
    return () => clearTimeout(timer);
  }, [flights]);
  return createPortal(
    <div
      ref={layer}
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
