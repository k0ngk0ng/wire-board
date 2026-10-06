import { twoResponder } from "./catan-two-state";
import { caravanResponder, caravanPrompt } from "./catan-caravans-state";
import { fishResponder } from "./catan-fishing-state";
import { useEffect } from "react";
import type { Room } from "./types";

export function useTurnTitle(room?: Room) {
  const game = room?.game;
  const player =
    game?.splendor?.players[room?.you ?? -1] ||
    game?.rail?.players[room?.you ?? -1] ||
    game?.catan?.players[room?.you ?? -1] ||
    game?.carcassonne?.players[room?.you ?? -1];
  const needsAction = !!(
    room &&
    game &&
    room.status === "playing" &&
    !game.finished &&
    !room.spectating &&
    !room.seats[room.you]?.autoPlay &&
    room.you >= 0 &&
    (game.dota
      ? true
      : game.sanguosha
        ? !game.sanguosha.players[room.you]?.dead
        : player && !player.eliminated) &&
    (game.dota
      ? !!game.dota.actors?.includes(room.you)
      : game.sanguosha
        ? game.sanguosha.pending
          ? game.sanguosha.pending.canRespond
          : game.turn === room.you
        : twoResponder(room) !== undefined
          ? twoResponder(room) === room.you
          : caravanResponder(room) !== undefined
            ? caravanResponder(room) === room.you
            : fishResponder(room) !== undefined
              ? fishResponder(room) === room.you
              : game.catan?.cardEvent
                ? game.catan.cardEvent.players[0] === room.you
                : game.catan?.citiesKnights?.pending
                  ? game.catan.citiesKnights.pending.players[0] === room.you
                  : game.catan?.seafarers?.pirateIslands?.raid
                    ? game.catan.seafarers.pirateIslands.raid.rewards[0] ===
                      room.you
                    : game.catan?.seafarers?.tribe?.pending
                      ? game.catan.seafarers.tribe.pending.player === room.you
                      : game.catan?.helperPending
                        ? game.catan.helperPending.player === room.you
                        : game.catan?.goldPending
                          ? game.catan.goldPending.claims[0]?.player ===
                            room.you
                          : game.phase === "catan_discard"
                            ? (game.catan?.discardDue[room.you] || 0) > 0
                            : game.rail?.setup
                              ? !game.rail.setupReady?.[room.you]
                              : game.turn === room.you)
  );
  const prompt = game?.dota
    ? "请规划行动"
    : game?.sanguosha?.pending
      ? "请响应牌局"
      : game?.catan?.two?.pending
        ? "请完成中立建设"
        : game?.catan?.two?.trade
          ? "请选择归还资源"
          : game?.catan?.caravans?.pending
            ? caravanPrompt(game.catan)
            : room && fishResponder(room) !== undefined
              ? "请选择鱼筹码"
              : game?.catan?.cardEvent
                ? "请完成事件牌选择"
                : game?.catan?.citiesKnights?.pending
                  ? "请完成城市与骑士选择"
                  : game?.catan?.seafarers?.pirateIslands?.raid
                    ? "请选择防守奖励"
                    : game?.catan?.seafarers?.tribe?.pending
                      ? "请安放港口"
                      : game?.catan?.goldPending
                        ? "请选择金矿资源"
                        : game?.phase === "gem_copy"
                          ? "请选择复制奖励"
                          : game?.phase === "gem_free_card"
                            ? "请免费取得发展卡"
                            : game?.rail?.setup
                              ? "请选择目的地"
                              : "轮到你了";
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
