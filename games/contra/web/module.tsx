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
  resources: () => [],
  isInputRelease: (action) => action.type === "release",
  createPresentation: () => createRuntimeRoomPresentation(readRuntimeVisible),
  Board,
  Rules: () => (
    <p>
      方向键移动和瞄准，B 射击，A 跳跃。原地按「下」 + A 可穿过单向平台；
      生命耗尽后可按 A 借用队友的备用生命。Start
      暂停或继续，双人合作由第一位玩家控制暂停。 通关八关后结算。
      手机和平板使用游戏区内的半透明触控浮层，可通过全屏按钮切换；方向区按住滑动可连续输入八向，斜向同时按住相邻两个方向。手指接触范围同时覆盖
      A 与 B 时可同时射击和跳跃，滑向单键即切换为该键，松手全部释放。Turbo
      B、Turbo A 为独立连发键。
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
