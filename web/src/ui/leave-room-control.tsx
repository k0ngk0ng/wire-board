import type { Room } from "../types";
import { Modal } from "./board-host";

export function LeaveRoomConfirmation({
  room,
  busy,
  onCancel,
  onConfirm,
}: {
  room: Room;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const lastPlayer =
    room.seats.filter((seat) => !seat.left && !seat.bot).length === 1;
  return (
    <Modal
      title="离开房间？"
      onClose={onCancel}
      initialFocus="[data-leave-cancel]"
    >
      <p>
        {room.status === "restoring"
          ? "离开后将退出本次续玩确认，存档仍会保留。"
          : lastPlayer
            ? room.status === "playing"
              ? "离开后电脑将继续完成本局。"
              : "你是房间内最后一位玩家，离开后房间将关闭。"
            : room.host === room.seats[room.you]?.id
              ? "离开后将交接房主，其他玩家仍留在房间。"
              : "离开后将退出当前房间。"}
      </p>
      <div className="modal-actions">
        <button
          type="button"
          className="outline"
          data-leave-cancel
          onClick={onCancel}
        >
          留在房间
        </button>
        <button
          type="button"
          className="primary confirm-danger"
          data-leave-confirm
          disabled={busy}
          onClick={onConfirm}
        >
          确认离开
        </button>
      </div>
    </Modal>
  );
}
