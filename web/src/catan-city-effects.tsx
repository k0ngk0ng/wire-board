import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { createPortal } from "react-dom";
import type { Room, CatanProgressEvent } from "./types";
import { catanCityMotion } from "./catan-city-motion";
import { cityTrackKeys, cityTracks } from "./catan-city-state";
import { catanProgressNames } from "./catan-progress-names";
import { CatanProgressArt } from "./catan-progress";
import "./catan-city-effects.css";

type Flight = {
  event: CatanProgressEvent;
  style: CSSProperties;
  label: string;
};
export function CatanCityEffects({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const anchor = useRef<HTMLSpanElement>(null);
  const previous = useRef(room);
  const animations = useRef(new Set<Animation>());
  const [flights, setFlights] = useState<Flight[]>([]);
  const clear = () => {
    animations.current.forEach((a) => a.cancel());
    animations.current.clear();
    setFlights([]);
  };
  useLayoutEffect(() => {
    const before = previous.current;
    previous.current = room;
    // Ordinary rerenders/chat must not cancel a motion already in progress.
    if (
      room.version === before.version &&
      room.id === before.id &&
      room.you === before.you
    )
      return;
    clear();
    if (
      document.hidden ||
      matchMedia("(prefers-reduced-motion: reduce)").matches
    )
      return;
    const motion = catanCityMotion(before, room);
    const board = anchor.current?.closest(".catan-board, .explorer-board");
    if (!motion || !board) return;
    const g = room.game!.catan!,
      old = before.game!.catan!;
    const scale = g.explorer ? 1 : Math.max(0.64, (g.hexSize || 62) / 62);
    const animate = (
      node: Element | null,
      frames: Keyframe[],
      duration = 650,
      delay = 0,
    ) => {
      if (!node) return;
      const a = node.animate(frames, {
        duration,
        delay,
        easing: "cubic-bezier(.22,.7,.25,1)",
        fill: "backwards",
      });
      animations.current.add(a);
      a.onfinish = () => animations.current.delete(a);
    };
    const piece = (v: number) =>
      board.querySelector(`[data-city-piece="${v}"]`);
    for (const move of motion.moves) {
      const from = old.vertices[move.from],
        to = g.vertices[move.to];
      animate(piece(move.to), [
        {
          transform: `translate(${(from.x - to.x) / scale}px, ${(from.y - to.y) / scale}px)`,
        },
        { transform: "translate(0px, 0px)" },
      ]);
    }
    for (const flash of motion.flashes)
      animate(piece(flash.vertex), [
        { opacity: 0.25 },
        { opacity: 1, offset: 0.35 },
        { opacity: 0.5, offset: 0.6 },
        { opacity: 1 },
      ]);
    if (motion.merchant) {
      const from = old.tiles[motion.merchant.from],
        to = g.tiles[motion.merchant.to];
      if (from && to)
        animate(board.querySelector("[data-city-merchant]"), [
          {
            transform: `translate(${from.x - to.x}px, ${from.y - to.y}px)`,
            opacity: 0.65,
          },
          { transform: "translate(0px, 0px)", opacity: 1 },
        ]);
    }
    if (motion.ship) {
      const { from, to, attack } = motion.ship;
      animate(
        board.querySelector("[data-city-ship]"),
        attack
          ? [
              { transform: `translateX(${from * 100}%)`, opacity: 1 },
              { transform: "translateX(700%)", opacity: 1, offset: 0.4 },
              { transform: "translateX(700%)", opacity: 0, offset: 0.65 },
              { transform: "translateX(0%)", opacity: 0, offset: 0.66 },
              { transform: "translateX(0%)", opacity: 1 },
            ]
          : [
              { transform: `translateX(${from * 100}%)` },
              { transform: `translateX(${to * 100}%)` },
            ],
        attack ? 1100 : 650,
      );
    }
    const player = (p: number) =>
      document.querySelector(`[data-player-seat="${p}"] .avatar`);
    for (const p of motion.players)
      animate(player(p), [
        { transform: "scale(1)" },
        { transform: "scale(1.2)", offset: 0.4 },
        { transform: "scale(1)" },
      ]);
    const clamp = (n: number, max: number) => Math.max(8, Math.min(n, max));
    const center = (node: Element) => {
      const r = node.getBoundingClientRect();
      return {
        x: clamp(r.x + r.width / 2 - 70, innerWidth - 148),
        y: clamp(r.y + r.height / 2 - 40, innerHeight - 110),
      };
    };
    setFlights(
      motion.cards.flatMap((event, i): Flight[] => {
        const deck =
          event.track >= 0
            ? board.querySelector(`[data-progress-deck="${event.track}"]`)
            : board.querySelector(".catan-progress-stocks");
        const seat = player(event.player);
        const source =
          event.kind === "draw"
            ? deck
            : event.kind === "transfer"
              ? player(event.other)
              : seat;
        const target =
          event.kind === "draw" || event.kind === "transfer" ? seat : deck;
        if (!source || !target) return [];
        const from = center(source),
          to = center(target);
        const kind = {
          draw: "获得",
          play: "使用",
          return: "归还",
          transfer: "取得",
        }[event.kind];
        const card =
          event.card !== undefined
            ? catanProgressNames[event.card]
            : `${event.track >= 0 ? cityTracks[event.track] : ""}进步牌`;
        animate(target, [{ opacity: 0.45 }, { opacity: 1 }], 500, i * 90 + 550);
        return [
          {
            event,
            label: `${room.seats[event.player]?.name} · ${kind}${card}${event.count > 1 ? ` ×${event.count}` : ""}`,
            style: {
              left: from.x,
              top: from.y,
              "--city-flight-x": `${to.x - from.x}px`,
              "--city-flight-y": `${to.y - from.y}px`,
              animationDelay: `${i * 90}ms`,
            } as CSSProperties,
          },
        ];
      }),
    );
  }, [room]);
  useEffect(() => {
    const media = matchMedia("(prefers-reduced-motion: reduce)");
    const clearFlights = () => setFlights([]);
    document.addEventListener("visibilitychange", clear);
    window.addEventListener("resize", clearFlights);
    window.addEventListener("wheel", clearFlights, { passive: true });
    window.addEventListener("touchmove", clearFlights, { passive: true });
    media.addEventListener("change", clear);
    return () => {
      animations.current.forEach((a) => a.cancel());
      animations.current.clear();
      document.removeEventListener("visibilitychange", clear);
      window.removeEventListener("resize", clearFlights);
      window.removeEventListener("wheel", clearFlights);
      window.removeEventListener("touchmove", clearFlights);
      media.removeEventListener("change", clear);
    };
  }, []);
  useEffect(() => {
    if (!flights.length) return;
    const timer = setTimeout(() => setFlights([]), 3000);
    return () => clearTimeout(timer);
  }, [flights]);
  return (
    <>
      <span ref={anchor} hidden />
      {createPortal(
        <div className="catan-city-flights" aria-hidden="true" inert>
          {flights.map(({ event, style, label }) => (
            <div
              key={event.id}
              data-city-flight={event.id}
              data-flight-kind={event.kind}
              data-flight-player={event.player}
              className="catan-city-flight"
              style={style}
              onAnimationEnd={() =>
                setFlights((current) =>
                  current.filter((f) => f.event.id !== event.id),
                )
              }
            >
              {event.card !== undefined ? (
                <CatanProgressArt card={event.card} assets={assets} />
              ) : assets && event.track >= 0 ? (
                <img
                  src={`${assets}/catan/cities-knights/progress-back-${cityTrackKeys[event.track]}-v1.webp`}
                  alt=""
                />
              ) : (
                <span className="catan-neutral-progress">✦</span>
              )}
              <small>{label}</small>
            </div>
          ))}
        </div>,
        document.body,
      )}
    </>
  );
}
