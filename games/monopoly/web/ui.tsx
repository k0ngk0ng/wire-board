import { useEffect, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import type { GameRoom } from "../../../web/src/game-api/module";

export const colors = [
  "#b74134",
  "#2c75a7",
  "#c8981c",
  "#368161",
  "#8656a5",
  "#cb752c",
];
export const playerName = (room: GameRoom, p: number) =>
  p < 0 ? "银行" : room.seats[p]?.name || `玩家 ${p + 1}`;
export const seatStyle = (p: number) =>
  ({ "--seat": colors[p % colors.length] }) as CSSProperties;
export const seconds = (deadline: number, now: number) =>
  Math.max(0, Math.ceil((deadline - now) / 1000));

export function Art({
  base,
  name,
  fallback,
  className = "",
}: {
  base: string;
  name: string;
  fallback?: ReactNode;
  className?: string;
}) {
  const src = base ? `${base}/monopoly/v1/images/${name}.webp` : "";
  const [failed, setFailed] = useState("");
  return src && failed !== src ? (
    <img
      className={className}
      src={src}
      alt=""
      onError={() => setFailed(src)}
    />
  ) : (
    <>{fallback}</>
  );
}

export function useServerTime(clock: {
  serverNow: number;
  receivedAt: number;
}) {
  const [now, setNow] = useState(performance.now());
  useEffect(() => {
    const id = setInterval(() => setNow(performance.now()), 250);
    return () => clearInterval(id);
  }, []);
  return clock.serverNow + Math.max(0, now - clock.receivedAt);
}

export function Dice({
  values,
  rolling = false,
}: {
  values: number[];
  rolling?: boolean;
}) {
  const dots = [
    [],
    [4],
    [0, 8],
    [0, 4, 8],
    [0, 2, 6, 8],
    [0, 2, 4, 6, 8],
    [0, 2, 3, 5, 6, 8],
  ];
  return (
    <div
      className={`mono-dice ${rolling ? "rolling" : ""}`}
      aria-label={`骰子：${values.join("、")}`}
    >
      {values.map((n, i) => (
        <span key={i} className="mono-die">
          {n ? (
            Array.from({ length: 9 }, (_, p) => (
              <i key={p} className={dots[n]?.includes(p) ? "dot" : ""} />
            ))
          ) : (
            <b>?</b>
          )}
        </span>
      ))}
    </div>
  );
}

export function GameButton({
  children,
  disabled,
  onClick,
  danger = false,
  primary = false,
}: {
  children: ReactNode;
  disabled?: boolean;
  onClick: () => void;
  danger?: boolean;
  primary?: boolean;
}) {
  return (
    <button
      type="button"
      className={`mono-button mono-game-action ${primary ? "primary" : ""} ${danger ? "mono-danger" : ""}`}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  );
}
