import { assetURL, projectileBitmaps } from "./assets";
import { sampleProjectile } from "./motion";
import type { BattleEvent } from "./types";

export function Projectiles({
  event,
  elapsed,
}: {
  event: BattleEvent | undefined;
  elapsed: number;
}) {
  if (!event?.projectiles?.length) return null;
  return (
    <svg
      className="sa-projectiles"
      viewBox="0 0 640 480"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      {event.projectiles.map((p, index) => {
        const point = sampleProjectile(p, elapsed - event.at);
        if (!point) return null;
        const bitmap = (id: number, vertical: number) => {
          const f = projectileBitmaps[String(id)];
          return f ? (
            <image
              key={id}
              href={assetURL(f.file)}
              x={point.x + f.xoffset}
              y={point.y + f.yoffset + vertical}
              width={f.width}
              height={f.height}
            />
          ) : null;
        };
        const native = p.kind === "arrow" || p.kind === "stone";
        return (
          <g
            key={`${event.id}:${index}`}
            data-stoneage-projectile={p.kind}
            data-stoneage-projectile-target={p.target}
            data-stoneage-projectile-phase={point.phase}
            data-stoneage-projectile-x={point.x}
            data-stoneage-projectile-y={point.y}
          >
            {native ? (
              <>
                {point.phase === "flight" &&
                  bitmap(
                    p.kind === "arrow"
                      ? 25650 + Math.floor(point.course / 2)
                      : 25786,
                    0,
                  )}
                {bitmap(
                  p.kind === "arrow"
                    ? 25630 + Math.floor(point.course / 2)
                    : 25785,
                  -28,
                )}
              </>
            ) : (
              <g
                transform={`translate(${point.x} ${point.y - 28}) rotate(${elapsed / 3})`}
              >
                {p.kind === "boomerang" ? (
                  <path
                    d="M-15 7 Q-4-6 11-12 L14-7 Q2-4-9 12Z"
                    fill="#b5c5d0"
                    stroke="#403e38"
                    strokeWidth="2"
                  />
                ) : (
                  <>
                    <path d="M-12 11L10-11" stroke="#49361d" strokeWidth="4" />
                    <path
                      d="M0-13Q16-19 17-3L6 3Z"
                      fill="#c6d3d4"
                      stroke="#403e38"
                      strokeWidth="2"
                    />
                  </>
                )}
              </g>
            )}
          </g>
        );
      })}
    </svg>
  );
}
