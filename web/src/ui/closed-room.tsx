import type { Room } from "../types";
import { unratedDescription } from "./cooperative-rating";

export function ClosedRoomDescription({
  room,
}: {
  room: Pick<Room, "closeReason" | "game" | "result">;
}) {
  return (
    <p>
      {room.result?.ratingPolicy === "coop_campaign_v1"
        ? room.result.rated
          ? room.result.outcome === "expired"
            ? "挑战超过无人操作保留期限，已按全队失败结算。"
            : "已放弃本次挑战，全队按失败结算。"
          : unratedDescription(room.result)
        : room.closeReason === "unpublished"
          ? "等待中的牌桌已关闭，可以返回大厅选择其他游戏。"
          : room.closeReason === "admin"
            ? "本局已中止，不计胜负与积分。"
            : room.closeReason === "inactive"
              ? room.game?.paused
                ? "暂停对局连续 30 天无人操作，系统已自动关闭牌桌；本局不计胜负与积分。"
                : "连续 24 小时无人操作，系统已自动关闭牌桌；本局不计胜负与积分。"
              : "休息一下，或者准备下一局。"}
    </p>
  );
}
