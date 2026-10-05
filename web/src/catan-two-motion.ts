import { useEffect, useLayoutEffect, useRef } from "react";
import type { Room } from "./types";
import { twoNeutralAdded } from "./catan-two-state";

export function useTwoNeutralMotion(room: Room) {
  const root = useRef<SVGSVGElement>(null);
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
    const added = twoNeutralAdded(before, room);
    if (
      !added ||
      document.hidden ||
      matchMedia("(prefers-reduced-motion: reduce)").matches
    )
      return;
    const piece = root.current?.querySelector(
      added.edge >= 0
        ? `[data-two-road="${added.edge}"]`
        : `[data-city-piece="${added.vertex}"]`,
    );
    if (!piece) return;
    // Animate the inner piece in map coordinates. The parent keeps its actual
    // position/rotation while pan, zoom and page scroll remain independent.
    const animation = piece.animate(
      [
        { transform: "translateY(-12px) scale(.75)", opacity: 0.25 },
        { transform: "translateY(0) scale(1)", opacity: 1 },
      ],
      { duration: 550, easing: "cubic-bezier(.22,.7,.25,1)" },
    );
    running.current.add(animation);
    animation.onfinish = () => running.current.delete(animation);
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
