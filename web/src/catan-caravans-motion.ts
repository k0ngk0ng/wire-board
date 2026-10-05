import { useEffect, useLayoutEffect, useRef } from "react";
import type { Room } from "./types";
import {
  caravanAdded,
  caravanBonus,
  caravanGeometry,
} from "./catan-caravans-state";

export function useCaravanMotion(room: Room) {
  const root = useRef<SVGGElement>(null);
  const previous = useRef(room);
  const running = useRef(new Set<Animation>());
  const clear = () => {
    running.current.forEach((animation) => animation.cancel());
    running.current.clear();
  };
  useLayoutEffect(() => {
    const before = previous.current;
    previous.current = room;
    if (
      before.id !== room.id ||
      before.you !== room.you ||
      !!before.spectating !== !!room.spectating ||
      room.version < before.version ||
      room.version > before.version + 1 ||
      !["playing", "finished"].includes(room.status)
    )
      clear();
    const wagon = caravanAdded(before, room);
    if (
      !wagon ||
      document.hidden ||
      matchMedia("(prefers-reduced-motion: reduce)").matches
    )
      return;
    const g = room.game!.catan!,
      point = caravanGeometry(g, wagon)!;
    const piece = root.current?.querySelector(
      `[data-caravan-piece="${wagon.edge}"]`,
    );
    if (!piece) return;
    const animate = (node: Element, frames: Keyframe[]) => {
      const animation = node.animate(frames, {
        duration: 650,
        easing: "cubic-bezier(.22,.7,.25,1)",
      });
      running.current.add(animation);
      animation.onfinish = () => running.current.delete(animation);
    };
    // Inner coordinates face south; the outer SVG group handles the actual
    // edge rotation/scale. Map pan, zoom and page scroll need no screen anchors.
    const scale = (g.hexSize || 62) / 62;
    const distance =
      Math.hypot(point.to.x - point.from.x, point.to.y - point.from.y) /
      2 /
      scale;
    animate(piece, [
      { transform: `translateY(${-distance}px)`, opacity: 0.35 },
      { transform: "translateY(0px)", opacity: 1 },
    ]);
    const board = root.current?.closest("svg");
    for (const vertex of [point.from.id, point.to.id]) {
      if (caravanBonus(before.game!.catan!, vertex) || !caravanBonus(g, vertex))
        continue;
      const badge = board?.querySelector(`[data-caravan-bonus="${vertex}"]`);
      if (badge) animate(badge, [{ opacity: 0.2 }, { opacity: 1 }]);
    }
  }, [room]);
  useEffect(() => {
    const media = matchMedia("(prefers-reduced-motion: reduce)");
    document.addEventListener("visibilitychange", clear);
    media.addEventListener("change", clear);
    return () => {
      clear();
      document.removeEventListener("visibilitychange", clear);
      media.removeEventListener("change", clear);
    };
  }, []);
  return root;
}
