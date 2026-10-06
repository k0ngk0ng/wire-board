import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import type { Room } from "./types";
import {
  transportArrivalMotion,
  transportCargo,
  transportWagonPosition,
} from "./catan-transport-state";
import type { TransportArrivalMotion } from "./catan-transport-state";

type Arrival = TransportArrivalMotion & {
  wagon: { x: number; y: number };
  sitePoint: { x: number; y: number };
};

export function CatanTransportEffects({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const previous = useRef(room);
  const [events, setEvents] = useState<Arrival[]>([]);
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
      setEvents([]);
    const event = transportArrivalMotion(before, room);
    if (
      event &&
      !document.hidden &&
      !matchMedia("(prefers-reduced-motion: reduce)").matches
    ) {
      const g = room.game!.catan!,
        wagon = transportWagonPosition(g, event.player),
        sitePoint = g.tiles[g.transport!.map.sites[event.site].tile];
      if (!wagon || !sitePoint) return;
      setEvents((current) => [
        ...current.filter((e) => e.id !== event.id),
        { ...event, wagon, sitePoint: { x: sitePoint.x, y: sitePoint.y } },
      ]);
    }
  }, [room]);
  useEffect(() => {
    const clear = () => setEvents([]),
      media = matchMedia("(prefers-reduced-motion: reduce)");
    document.addEventListener("visibilitychange", clear);
    media.addEventListener("change", clear);
    return () => {
      document.removeEventListener("visibilitychange", clear);
      media.removeEventListener("change", clear);
    };
  }, []);
  useEffect(() => {
    if (!events.length) return;
    const timer = setTimeout(() => setEvents([]), 2500);
    return () => clearTimeout(timer);
  }, [events]);
  const g = room.game?.catan,
    t = g?.transport;
  if (!g || !t) return null;
  const scale = (g.hexSize || 62) / 62;
  return (
    <g className="transport-effects" pointerEvents="none" aria-hidden="true">
      {events.map((event) => {
        const tile = event.sitePoint,
          center = { x: event.wagon.x, y: event.wagon.y - 11 * scale },
          depot = { x: tile.x - 30 * scale, y: tile.y + 14 * scale };
        // Stay in SVG map coordinates: scroll, pan and zoom move the entire
        // effect together. Anchor to the delivery site even if the wagon leaves.
        return (
          <g key={event.id} data-transport-arrival={event.id}>
            {(["delivered", "loaded"] as const).map((kind) => {
              const cargo = event[kind];
              if (!cargo) return null;
              const from = kind === "delivered" ? center : depot,
                to = kind === "delivered" ? depot : center;
              return (
                <g key={kind} transform={`translate(${from.x},${from.y})`}>
                  <g
                    className="transport-cargo-flight"
                    data-transport-flight={kind}
                    style={
                      {
                        "--cargo-x": `${to.x - from.x}px`,
                        "--cargo-y": `${to.y - from.y}px`,
                        animationDelay:
                          kind === "loaded" && event.delivered
                            ? "520ms"
                            : "0ms",
                      } as CSSProperties
                    }
                  >
                    <circle
                      r={16 * scale}
                      fill="#fff3d5"
                      stroke="#946320"
                      strokeWidth={1.5}
                    />
                    {assets ? (
                      <image
                        href={`${assets}/catan/transport/cargo-${cargo}-v1.webp`}
                        x={-14 * scale}
                        y={-14 * scale}
                        width={28 * scale}
                        height={28 * scale}
                      />
                    ) : (
                      <text
                        textAnchor="middle"
                        className="transport-site-label"
                        y={4 * scale}
                      >
                        {transportCargo[cargo]}
                      </text>
                    )}
                  </g>
                </g>
              );
            })}
            <g
              transform={`translate(${tile.x},${tile.y - 22 * scale}) scale(${scale})`}
            >
              <g
                className="transport-arrival-label"
                onAnimationEnd={() =>
                  setEvents((current) =>
                    current.filter((e) => e.id !== event.id),
                  )
                }
              >
                <rect x="-65" y="-15" width="130" height="27" rx="8" />
                <text textAnchor="middle" y="3">
                  {event.delivered
                    ? `+1分 · +${event.gold}金币`
                    : `装载${transportCargo[event.loaded!]}`}
                </text>
              </g>
            </g>
          </g>
        );
      })}
    </g>
  );
}
