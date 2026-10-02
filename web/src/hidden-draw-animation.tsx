import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import { createPortal } from "react-dom";
import { TrainFront } from "lucide-react";
import type { Room } from "./types";

type Flight = { id: number; style: CSSProperties };

export function HiddenDrawAnimation({ room }: { room: Room }) {
  const rail = room.game!.rail!;
  const seen = useRef({ room: room.id, id: rail.hiddenDrawId ?? 0 });
  const [flights, setFlights] = useState<Flight[]>([]);
  useLayoutEffect(() => {
    const id = rail.hiddenDrawId ?? 0;
    if (seen.current.room !== room.id || id < seen.current.id) {
      seen.current = { room: room.id, id };
      setFlights([]);
      return;
    }
    const events = (rail.hiddenDrawEvents || []).filter(
      (event) => event.id > seen.current.id,
    );
    seen.current.id = id;
    if (!events.length) return;
    const source = document
      .querySelector(".train-deck")
      ?.getBoundingClientRect();
    if (!source) return;
    const fresh = events.flatMap((event, index): Flight[] => {
      const target = document
        .querySelector(`[data-player-seat="${event.player}"]`)
        ?.getBoundingClientRect();
      if (!target) return [];
      const x = source.x + source.width / 2 - 28;
      const y = source.y + source.height / 2 - 18;
      return [
        {
          id: event.id,
          style: {
            left: x,
            top: y,
            "--draw-x": `${target.x + target.width / 2 - 28 - x}px`,
            "--draw-y": `${target.y + target.height / 2 - 18 - y}px`,
            animationDelay: `${index * 130}ms`,
          } as CSSProperties,
        },
      ];
    });
    setFlights((previous) => [...previous, ...fresh].slice(-10));
  }, [rail.hiddenDrawId, rail.hiddenDrawEvents, room.id]);
  useEffect(() => {
    if (!flights.length) return;
    const timer = setTimeout(() => setFlights([]), 2400);
    return () => clearTimeout(timer);
  }, [flights]);
  return createPortal(
    <div aria-hidden="true" className="hidden-draw-animation">
      {flights.map((flight) => (
        <div
          key={flight.id}
          className="hidden-train-flight"
          style={flight.style}
          onAnimationEnd={() =>
            setFlights((previous) =>
              previous.filter((item) => item.id !== flight.id),
            )
          }
        >
          <TrainFront size={22} />
        </div>
      ))}
    </div>,
    document.body,
  );
}
