import { useEffect, useLayoutEffect, useRef, useState } from "react";
import type { CSSProperties } from "react";
import type { Room } from "./types";
import { CatanPirate, CatanShip } from "./catan-seafarers";
import { catanColorIndex } from "./catan-player-colors";
import { catanFleetSteps, catanNewBattle } from "./catan-pirate-motion";

type Ghost = {
  id: string;
  x: number;
  y: number;
  dx: number;
  dy: number;
  angle: number;
  color: number;
  warship: boolean;
  token?: boolean;
};

export function CatanPirateEffects({
  room,
  assets,
}: {
  room: Room;
  assets: string;
}) {
  const game = room.game!.catan!,
    pirates = game.seafarers!.pirateIslands!;
  const fleet = useRef<SVGGElement>(null);
  const animation = useRef<Animation | null>(null);
  const seen = useRef({ room: room.id, game });
  const [ghosts, setGhosts] = useState<Ghost[]>([]);
  const [flash, setFlash] = useState<{
    id: number;
    vertex: number;
    won: boolean;
  } | null>(null);
  const scale = Math.max(0.64, (game.hexSize || 62) / 62);
  const tile = game.tiles[game.seafarers!.pirate];

  useLayoutEffect(() => {
    const previous = seen.current;
    seen.current = { room: room.id, game };
    const active =
      previous.room === room.id &&
      !document.hidden &&
      !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (!active || game.rollId < previous.game.rollId) {
      animation.current?.cancel();
      setGhosts([]);
      setFlash(null);
      return;
    }
    const steps = catanFleetSteps(previous.game, game);
    if (steps.length && fleet.current) {
      animation.current?.cancel();
      animation.current = fleet.current.animate(
        steps.map((id, i) => {
          const t = game.tiles[id];
          return {
            transform: `translate(${t.x}px, ${t.y}px)`,
            offset: i / (steps.length - 1),
          };
        }),
        {
          duration: Math.min(1100, (steps.length - 1) * 150 + 150),
          easing: "linear",
        },
      );
    } else if (
      game.seafarers!.pirate !== previous.game.seafarers?.pirate ||
      game.rollId !== previous.game.rollId
    ) {
      animation.current?.cancel();
    }
    const battle = catanNewBattle(previous.game, game);
    if (!battle) return;
    const fort = pirates.fortresses[battle.player],
      home = game.vertices[fort.root];
    const fresh: Ghost[] = battle.removed.flatMap((id) => {
      const edge = previous.game.edges[id];
      if (!edge?.ship || edge.owner !== battle.player) return [];
      const a = game.vertices[edge.a],
        b = game.vertices[edge.b];
      const x = (a.x + b.x) / 2,
        y = (a.y + b.y) / 2;
      return [
        {
          id: `${battle.id}:${id}`,
          x,
          y,
          dx: home.x - x,
          dy: home.y - y,
          angle:
            (((Math.atan2(b.y - a.y, b.x - a.x) * 180) / Math.PI + 270) % 180) -
            90,
          color: catanColorIndex(game, edge.owner),
          warship: !!edge.warship,
        },
      ];
    });
    const won = battle.warships > battle.die;
    if (won) {
      const vertex = game.vertices[fort.vertex];
      fresh.push({
        id: `${battle.id}:token`,
        x: vertex.x,
        y: vertex.y,
        dx: 0,
        dy: -45,
        angle: 0,
        color: 0,
        warship: false,
        token: true,
      });
    }
    setGhosts(fresh);
    setFlash({ id: battle.id, vertex: fort.vertex, won });
  }, [room.id, game, pirates]);

  useEffect(() => {
    if (!ghosts.length && !flash) return;
    const timer = setTimeout(() => {
      setGhosts([]);
      setFlash(null);
    }, 1300);
    return () => clearTimeout(timer);
  }, [ghosts, flash]);
  useEffect(() => {
    const clear = () => {
      animation.current?.cancel();
      setGhosts([]);
      setFlash(null);
    };
    const motion = window.matchMedia("(prefers-reduced-motion: reduce)");
    document.addEventListener("visibilitychange", clear);
    motion.addEventListener("change", clear);
    return () => {
      animation.current?.cancel();
      document.removeEventListener("visibilitychange", clear);
      motion.removeEventListener("change", clear);
    };
  }, []);

  return (
    <g pointerEvents="none">
      {tile && (
        <g
          ref={fleet}
          className="catan-moving-fleet"
          style={{ transform: `translate(${tile.x}px, ${tile.y}px)` }}
        >
          <g transform={`scale(${scale})`}>
            <CatanPirate assets={assets} />
          </g>
          <title>海盗舰队；沿箭头巡航，抵达后攻击相邻建筑并封锁船只</title>
        </g>
      )}
      <g aria-hidden="true">
        {ghosts.map((ghost) => (
          <g key={ghost.id} transform={`translate(${ghost.x},${ghost.y})`}>
            <g
              className="catan-naval-ghost"
              style={
                {
                  "--naval-dx": `${ghost.dx}px`,
                  "--naval-dy": `${ghost.dy}px`,
                } as CSSProperties
              }
            >
              <g transform={`rotate(${ghost.angle}) scale(${scale})`}>
                {ghost.token ? (
                  assets ? (
                    <image
                      href={`${assets}/catan/seafarers/pirate-lair-v1.webp`}
                      x="-16"
                      y="-16"
                      width="32"
                      height="32"
                    />
                  ) : (
                    <circle r="15" fill="#847041" />
                  )
                ) : (
                  <CatanShip
                    assets={assets}
                    player={ghost.color}
                    warship={ghost.warship}
                  />
                )}
              </g>
            </g>
          </g>
        ))}
        {flash && (
          <g
            key={flash.id}
            transform={`translate(${game.vertices[flash.vertex].x},${game.vertices[flash.vertex].y}) scale(${scale})`}
          >
            <circle
              className="catan-fortress-flash"
              r="24"
              fill="none"
              stroke={flash.won ? "#ffdf70" : "#c94737"}
              strokeWidth="5"
            />
          </g>
        )}
      </g>
    </g>
  );
}
