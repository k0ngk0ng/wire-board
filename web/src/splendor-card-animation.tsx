import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { createPortal } from "react-dom";
import type { Card, Noble, Room, SplendorCardEvent } from "./types";
import { visibleFlightAnchor } from "./flight-anchor";
import { orientBackStyle } from "./splendor-orient-art";
import {
  captureCardOrigin,
  resolveCardOrigin,
  type CardOrigin,
} from "./card-flight-origin";
import "./splendor-card-animation.css";

type Flight = {
  event: SplendorCardEvent;
  source: string | CardOrigin;
  target: string;
  width: number;
  height: number;
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
  const reservedRects = useRef(new Map<number, CardOrigin>());
  const nobleRects = useRef(new Map<number, CardOrigin>());
  const layer = useRef<HTMLDivElement>(null);
  const [flights, setFlights] = useState<Flight[]>([]);

  // Capture hand/noble slots after server updates, before the next card is
  // removed. Relative origins track scrolling without extra layout reads.
  useLayoutEffect(() => {
    const id = splendor.cardEventId ?? 0;
    const capture = (
      selector: string,
      container: string,
      key: "cardId" | "nobleId",
    ) =>
      new Map(
        Array.from(document.querySelectorAll<HTMLElement>(selector)).map(
          (element) => [
            Number(element.dataset[key]),
            captureCardOrigin(
              element,
              element.closest<HTMLElement>(container)!,
            ),
          ],
        ),
      );
    const rememberSources = () => {
      reservedRects.current = capture(
        ".splendor-board .reserved [data-card-id]",
        ".reserved",
        "cardId",
      );
      nobleRects.current = capture(
        ".splendor-board .nobles [data-noble-id]",
        ".nobles-row",
        "nobleId",
      );
    };
    if (seen.current.room !== room.id || id < seen.current.id) {
      seen.current = { room: room.id, id };
      setFlights([]);
    } else {
      const events = (splendor.cardEvents ?? []).filter(
        (event) => event.id > seen.current.id,
      );
      seen.current.id = id;
      const fresh = document.hidden
        ? []
        : events.flatMap((event, index): Flight[] => {
            const target = `[data-player-seat="${event.player}"] .avatar`;
            const row = `.market-row[data-splendor-tier="${event.tier}"][data-splendor-orient="${event.orient ? "true" : "false"}"]`;
            const source =
              event.source === "nobles" && event.noble
                ? nobleRects.current.get(event.noble.id)
                : event.source === "market"
                  ? `${row} [data-market-slot="${event.slot}"]`
                  : event.source === "deck"
                    ? `${row} .deck`
                    : event.player === room.you && event.card
                      ? reservedRects.current.get(event.card.id)
                      : `[data-player-seat="${event.player}"] .reserved-count`;
            if (!source) return [];
            const rect =
              typeof source === "string"
                ? document.querySelector(source)?.getBoundingClientRect()
                : source.bounds;
            if (!rect) return [];
            const width = Math.min(104, Math.max(54, rect.right - rect.left));
            const height = event.noble ? width : (width * 7) / 5;
            return [
              {
                event,
                source,
                target,
                width,
                height,
                label: `${room.seats[event.player]?.name || "玩家"} · ${event.action === "noble" ? "贵族 +3 分" : event.action === "buy" ? "购买" : event.action === "free" ? "免费取得" : "预留"}`,
                style: { width, height, animationDelay: `${index * 160}ms` },
                arrivalStyle: { animationDelay: `${index * 160 + 700}ms` },
              },
            ];
          });
      if (fresh.length)
        setFlights((previous) => [...previous, ...fresh].slice(-12));
    }
    rememberSources();
    window.addEventListener("resize", rememberSources);
    return () => {
      window.removeEventListener("resize", rememberSources);
    };
  }, [splendor, room.id, room.you, room.seats]);

  useLayoutEffect(() => {
    if (!flights.length) return;
    let frame = 0;
    const update = () => {
      frame = 0;
      const anchors = new Map<string, ReturnType<typeof visibleFlightAnchor>>();
      const anchor = (selector: string) => {
        if (!anchors.has(selector)) {
          const element = document.querySelector(selector);
          anchors.set(selector, element ? visibleFlightAnchor(element) : null);
        }
        return anchors.get(selector);
      };
      // Read all layout first, then write only geometry. The keyed CSS
      // animations keep their progress and delays while the page scrolls.
      const measurements = flights.map((flight) => ({
        flight,
        from:
          typeof flight.source === "string"
            ? anchor(flight.source)
            : resolveCardOrigin(flight.source),
        to: anchor(flight.target),
      }));
      for (const { flight, from, to } of measurements) {
        const root = layer.current?.querySelector<HTMLElement>(
          `[data-card-flight="${flight.event.id}"]`,
        );
        const card = root?.querySelector<HTMLElement>(".splendor-card-flight");
        const arrival = root?.querySelector<HTMLElement>(
          ".splendor-card-arrival",
        );
        if (!root || !card || !arrival) continue;
        // An offscreen source can still yield a correctly attached arrival
        // label, but never a flight projected from an invented screen edge.
        card.style.visibility = from && to ? "visible" : "hidden";
        arrival.style.visibility = to ? "visible" : "hidden";
        if (from && to) {
          card.style.left = `${from.x - flight.width / 2}px`;
          card.style.top = `${from.y - flight.height / 2}px`;
          card.style.setProperty("--flight-x", `${to.x - from.x}px`);
          card.style.setProperty("--flight-y", `${to.y - from.y}px`);
        }
        if (to) {
          const view = window.visualViewport;
          const left = view?.offsetLeft ?? 0,
            top = view?.offsetTop ?? 0;
          const right =
            left + (view?.width ?? document.documentElement.clientWidth);
          const bottom =
            top + (view?.height ?? document.documentElement.clientHeight);
          // Only the text bubble is kept readable at the viewport edge.
          arrival.style.left = `${Math.max(left + 8, Math.min(to.x - 70, right - 148))}px`;
          arrival.style.top = `${Math.max(top + 8, Math.min(to.y + 14, bottom - 38))}px`;
        }
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
      const source =
        typeof flight.source === "string"
          ? document.querySelector(flight.source)
          : flight.source.reference;
      for (const start of [source, document.querySelector(flight.target)]) {
        for (let element = start; element; element = element.parentElement) {
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
    const timer = setTimeout(() => setFlights([]), 4000);
    return () => clearTimeout(timer);
  }, [flights]);

  return createPortal(
    <div
      ref={layer}
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
                className={`deck tier-${event.tier - 1} ${event.orient ? `orient-deck ${assets ? "has-orient-back" : ""}` : ""}`}
                style={
                  event.orient
                    ? assets
                      ? orientBackStyle(assets, event.tier)
                      : undefined
                    : { backgroundPosition: `${(event.tier - 1) * 20}% 100%` }
                }
              >
                <span className="deck-ornament">✧</span>
                <strong>{["Ⅰ", "Ⅱ", "Ⅲ"][event.tier - 1]}</strong>
                {event.orient && (
                  <span className="orient-deck-label">东方</span>
                )}
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
