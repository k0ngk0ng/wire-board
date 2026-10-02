import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { createPortal } from "react-dom";
import { TrainFront } from "lucide-react";
import type { Room } from "./types";
type Flight = {
  id: number;
  style: CSSProperties;
  name: string;
  public: boolean;
};
export function HiddenDrawAnimation({ room }: { room: Room }) {
  const rail = room.game!.rail!;
  const seen = useRef({ room: room.id, id: rail.drawId ?? 0 });
  const [flights, setFlights] = useState<Flight[]>([]);
  useLayoutEffect(() => {
    const id = rail.drawId ?? 0;
    if (seen.current.room !== room.id || id < seen.current.id) {
      seen.current = { room: room.id, id };
      setFlights([]);
      return;
    }
    const events = (rail.drawEvents || []).filter(
      (event) => event.id > seen.current.id,
    );
    seen.current.id = id;
    if (!events.length) return;
    const clamp = (n: number, max: number) => Math.max(8, Math.min(n, max));
    const fresh = events.flatMap((event, index): Flight[] => {
      const source = document.querySelector(
        event.slot < 0
          ? ".train-deck"
          : `.train-market [data-market-slot="${event.slot}"]`,
      );
      const target = document
        .querySelector(`[data-player-seat="${event.player}"]`)
        ?.getBoundingClientRect();
      if (!source || !target) return [];
      const rect = source.getBoundingClientRect();
      const x = clamp(rect.x + rect.width / 2 - 28, innerWidth - 64),
        y = clamp(rect.y + rect.height / 2 - 18, innerHeight - 58);
      const style: CSSProperties = {
        left: x,
        top: y,
        "--draw-x": `${clamp(target.x + target.width / 2 - 28, innerWidth - 64) - x}px`,
        "--draw-y": `${clamp(target.y + target.height / 2 - 18, innerHeight - 58) - y}px`,
        animationDelay: `${index * 150}ms`,
      } as CSSProperties;
      if (event.color !== undefined) {
        const card = document.querySelector(".train-market .train-card");
        if (card)
          style.backgroundImage = getComputedStyle(card).backgroundImage;
        style.backgroundSize = "900% 100%";
        style.backgroundPosition = `${(event.color === 8 ? 0 : event.color + 1) * 12.5}% 0`;
      }
      return [
        {
          id: event.id,
          style,
          name: room.seats[event.player]?.name || "玩家",
          public: event.color !== undefined,
        },
      ];
    });
    setFlights((previous) => [...previous, ...fresh].slice(-10));
  }, [rail.drawId, rail.drawEvents, room.id, room.seats]);
  useEffect(() => {
    if (!flights.length) return;
    const timer = setTimeout(() => setFlights([]), 2800);
    return () => clearTimeout(timer);
  }, [flights]);
  return createPortal(
    <div aria-hidden="true" className="hidden-draw-animation">
      {flights.map((flight) => (
        <div
          key={flight.id}
          className={`hidden-train-flight ${flight.public ? "public-train-flight" : ""}`}
          style={flight.style}
          onAnimationEnd={() =>
            setFlights((previous) =>
              previous.filter((item) => item.id !== flight.id),
            )
          }
        >
          {!flight.public && <TrainFront size={22} />}
          <span>{flight.name}</span>
        </div>
      ))}
    </div>,
    document.body,
  );
}
