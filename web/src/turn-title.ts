import { useEffect } from "react";
import type { Room } from "./types";

export function useTurnTitle(room?: Room) {
  const game = room?.game;
  const player =
    game?.splendor?.players[room?.you ?? -1] ||
    game?.rail?.players[room?.you ?? -1];
  const needsAction = !!(
    room &&
    game &&
    room.status === "playing" &&
    !game.finished &&
    !room.spectating &&
    room.you >= 0 &&
    player &&
    !player.eliminated &&
    (game.rail?.setup
      ? !game.rail.setupReady?.[room.you]
      : game.turn === room.you)
  );
  const prompt = game?.rail?.setup ? "请选择目的地" : "轮到你了";
  useEffect(() => {
    const original = document.title;
    let timer: ReturnType<typeof setInterval> | undefined;
    const sync = () => {
      clearInterval(timer);
      document.title = original;
      if (!needsAction || (!document.hidden && document.hasFocus())) return;
      document.title = `【${prompt}】${original}`;
      if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
      let highlighted = true;
      timer = setInterval(() => {
        highlighted = !highlighted;
        document.title = highlighted ? `【${prompt}】${original}` : original;
      }, 1000);
    };
    sync();
    document.addEventListener("visibilitychange", sync);
    window.addEventListener("focus", sync);
    window.addEventListener("blur", sync);
    return () => {
      clearInterval(timer);
      document.title = original;
      document.removeEventListener("visibilitychange", sync);
      window.removeEventListener("focus", sync);
      window.removeEventListener("blur", sync);
    };
  }, [needsAction, prompt, room?.id]);
}
