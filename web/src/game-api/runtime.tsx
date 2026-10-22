import { useEffect, useState, type ReactNode } from "react";
import { gameLoaded, loadGame, useGameRegistry } from "./registry";
export function Runtime({
  kind,
  children,
}: {
  kind: string;
  children: ReactNode;
}) {
  useGameRegistry();
  const [error, setError] = useState(""),
    [attempt, retry] = useState(0);
  useEffect(() => {
    let live = true;
    setError("");
    loadGame(kind).catch((e) => {
      if (live) setError(e.message);
    });
    return () => {
      live = false;
    };
  }, [kind, attempt]);
  if (error)
    return (
      <p role="alert">
        {error} <button onClick={() => retry((n) => n + 1)}>重试</button>
      </p>
    );
  return gameLoaded(kind) ? children : <p role="status">正在加载游戏设置…</p>;
}
