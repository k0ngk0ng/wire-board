import { defineGame } from "../../../web/src/game-api/module";
import { Board } from "./Board";
import catalog from "./catalog";
import type { RoomRuntimeView } from "./runtime-room";
import {
  readRuntimeVisible,
  runtimeVisibleKey,
  useRuntimeVisible,
} from "./runtime-visible";
import { createRuntimeRoomPresentation } from "./runtime-result";

export default defineGame<RoomRuntimeView>({
  ...catalog,
  asyncBoard: true,
  isInputRelease: (action) => action.type === "release",
  createPresentation: () => createRuntimeRoomPresentation(readRuntimeVisible),
  Board,
  Rules: () => (
    <p>
      方向键移动和瞄准，B 射击，A 跳跃。原地按「下」 + A 可穿过单向平台；
      生命耗尽后可按 A 借用队友的备用生命。Start 暂停或继续，双人合作由第一位玩家控制暂停。
      通关八关后结算。
    </p>
  ),
  PlayerSummary: ({ room, data, seat }) => {
    const snapshot = useRuntimeVisible(
      runtimeVisibleKey(room.id, data.epoch),
      data.snapshot,
    );
    return (
      <strong>
        备用生命 {snapshot.players[data.seatActors[seat]]?.lives ?? 0}
      </strong>
    );
  },
});
