import { mapById, type MapDef } from "./types";

const COLORS: Record<number, string> = {
  0: "#11161c",
  1: "#9a4c33",
  2: "#8e9aa8",
  3: "#27678f",
  4: "#2f6542",
  5: "#b7d2df",
};

const RING = [
  [5, 0],
  [7, 0],
  [5, 1],
  [6, 1],
  [7, 1],
  [5, 12],
  [7, 12],
  [5, 11],
  [6, 11],
  [7, 11],
];

export function MapPreview({
  mapId,
  className = "",
  small = false,
}: {
  mapId: string;
  className?: string;
  small?: boolean;
}) {
  const map: MapDef | undefined = mapById(mapId);
  if (!map)
    return <div className={`battlecity-map-missing ${className}`}>地图不可用</div>;
  return (
    <svg
      className={`battlecity-map-preview ${small ? "is-small" : ""} ${className}`}
      viewBox="0 0 208 208"
      role="img"
      aria-label={`${map.name}预览`}
    >
      {map.cells.map((row, y) =>
        row.split("").map((raw, x) => {
          const code = Number(raw);
          return (
            <rect
              key={`${x},${y}`}
              x={x * 16}
              y={y * 16}
              width={16}
              height={16}
              fill={COLORS[code] ?? COLORS[0]}
              stroke={code === 1 ? "#612e1f" : "transparent"}
              strokeWidth={code === 1 ? 1 : 0}
            />
          );
        }),
      )}
      {RING.map(([x, y]) => (
        <rect
          key={`ring-${x}-${y}`}
          x={x * 16 + 2}
          y={y * 16 + 2}
          width={12}
          height={12}
          fill="#b8643f"
          opacity={0.75}
        />
      ))}
      {[0, 1].map((team) => {
        const y = team === 0 ? 0 : 12;
        return (
          <g key={`base-${team}`}>
            <rect
              x={6 * 16 + 1}
              y={y * 16 + 1}
              width={14}
              height={14}
              rx={2}
              fill={team === 0 ? "#f0a16d" : "#86c4e2"}
              stroke="#11161c"
              strokeWidth={1}
            />
            <path
              d={`M ${6 * 16 + 4} ${y * 16 + 12} L ${6 * 16 + 8} ${y * 16 + 4} L ${6 * 16 + 12} ${y * 16 + 12} Z`}
              fill="#11161c"
              opacity={0.8}
            />
          </g>
        );
      })}
      {[
        [2, 2],
        [6, 2],
        [10, 2],
        [2, 10],
        [6, 10],
        [10, 10],
      ].map(([x, y]) => (
        <circle
          key={`spawn-${x}-${y}`}
          cx={x * 16 + 8}
          cy={y * 16 + 8}
          r={2.6}
          fill="#f0d49a"
          opacity={0.7}
        />
      ))}
    </svg>
  );
}
