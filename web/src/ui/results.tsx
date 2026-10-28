import { Crown, RotateCcw } from "lucide-react";
import type { Room } from "../types";
import { PlayerName } from "../profiles";
import { isIndependentGame } from "../game-api/registry";
import { catanResultDescription } from "../../../games/catan/web/catan-results";
import { splendorResultDescription } from "../../../games/splendor/web/splendor-city-state";
import { cooperativeOutcomeLabel } from "./cooperative-rating";
import { RatingChanges } from "./rating-changes";

// Platform settlement UI is part of the application, available before play.
// Finishing a match must never trigger a download of legacy game interfaces.
export function Results({
  room,
  host,
  busy,
  command,
}: {
  room: Room;
  host: boolean;
  busy: boolean;
  command: (t: string) => void;
}) {
  const g = room.game!;
  return (
    <section className="results">
      <div>
        <span className="eyebrow">对局结算</span>
        <h2>
          <Crown size={25} />
          {room.result?.unrated
            ? "练习结束"
            : room.result?.draw
              ? "本局和棋"
              : room.result?.cooperative
                ? room.result.ratingPolicy === "coop_campaign_v1"
                  ? cooperativeOutcomeLabel(room.result)
                  : g.winners.length > 0
                    ? "全队胜利"
                    : "合作挑战结束"
                : `${g.winners.map((i) => room.seats[i]?.name).join("、")} 获胜`}
        </h2>
        <p>
          {room.result?.unrated
            ? "练习对局不计积分，成绩已保存。"
            : room.result?.draw
              ? "本局没有获胜方，积分保持不变。"
              : room.result?.cooperative
                ? g.winners.length > 0
                  ? "大家共同完成了本局目标。"
                  : "本局挑战结束，没有获胜者。可以再来一局。"
                : isIndependentGame(room.kind)
                  ? "本局已结算。"
                  : g.sanguosha
                    ? "身份阵营结算：获胜阵营的成员共同获胜，包括已经阵亡的队友。"
                    : g.carcassonne
                      ? "地块用尽后结算未完成建筑与田地；总分最高者获胜，同分共同获胜。离场玩家不参与排名。"
                      : g.catan
                        ? catanResultDescription(g.catan)
                        : g.splendor
                          ? splendorResultDescription(g.splendor)
                          : "总分 = 路线分 + 目的地净得分 + 本地图奖励。"}
        </p>
      </div>
      {room.result && !room.result.unrated && (
        <RatingChanges result={room.result} />
      )}
      {g.rail && (
        <div className="score-table">
          {room.seats.map((p, i) => {
            const x = g.rail!.players[i];
            return (
              <div key={p.id}>
                <strong>
                  <PlayerName user={p} />
                </strong>
                <span>路线 {x.routeScore}</span>
                <span>
                  任务 {x.ticketScore > 0 ? "+" : ""}
                  {x.ticketScore}
                </span>
                <span>
                  {g.rail!.mapInfo.bonus === "network"
                    ? `网络 ${x.network || 0} 城`
                    : g.rail!.mapInfo.bonus === "tickets"
                      ? `完成 ${x.completed || 0} 张`
                      : `最长 ${x.longest} 节`}{" "}
                  / +{x.bonus}
                  {!!g.rail!.mapInfo.stations &&
                    ` · 车站 +${x.stationScore || 0}`}
                  {g.rail!.map === "india" && ` · 环游 +${x.mandalaScore || 0}`}
                </span>
                <b>{x.score} 分</b>
              </div>
            );
          })}
        </div>
      )}
      {host && (
        <button
          className="primary"
          disabled={busy}
          onClick={() => command("rematch")}
        >
          <RotateCcw size={17} />
          再来一局
        </button>
      )}
    </section>
  );
}
