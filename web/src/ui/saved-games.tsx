import { useState, type ButtonHTMLAttributes } from "react";
import { Check, FolderOpen, RotateCcw, Save, Undo2 } from "lucide-react";
import { gameName } from "../game-api/registry";
import type { Room, SavedGame } from "../types";
import type { GameRoom } from "../game-api/module";
import { Modal } from "./board-host";
import { GameFeedback } from "./game-overlay-root";
import { RoomToolButton } from "./room-tool-button";
import "./saved-games.css";

function SaveControlButton({
  iconOnly,
  label,
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  iconOnly: boolean;
  label: string;
}) {
  return iconOnly ? (
    <RoomToolButton {...props} label={label}>
      {children}
    </RoomToolButton>
  ) : (
    <button {...props} type="button">
      {children}
      {label}
    </button>
  );
}

export function SaveGameControl({
  room,
  userID,
  busy,
  command,
  iconOnly = false,
}: {
  room: GameRoom;
  userID: string;
  busy: boolean;
  command: (kind: string) => void;
  iconOnly?: boolean;
}) {
  const [loadingCheckpoint, setLoadingCheckpoint] = useState(false);
  const [restarting, setRestarting] = useState(false);
  if (
    !room.supportsSave ||
    room.status !== "playing" ||
    room.spectating ||
    room.loading ||
    room.game?.finished
  )
    return null;
  const votes = room.saveVotes ?? [];
  const approved = votes.includes(userID);
  const frozen = votes.length > 0 && room.game?.paused;
  const humans = room.seats.filter((seat) => !seat.bot && !seat.left).length;
  return (
    <span className="saved-game-control">
      {room.supportsCheckpoint && (
        <SaveControlButton
          iconOnly={iconOnly}
          label="读取存档"
          className="subtle"
          data-load-checkpoint
          disabled={busy || frozen || !room.checkpointSavedAt}
          title={room.checkpointSavedAt ? "读取上次手动存档" : "尚无手动存档"}
          onClick={() => setLoadingCheckpoint(true)}
        >
          <FolderOpen size={16} />
        </SaveControlButton>
      )}
      {room.supportsCheckpoint && room.game?.restartable && (
        <SaveControlButton
          iconOnly={iconOnly}
          label="重新开始"
          className="subtle"
          data-restart-journey
          disabled={busy || frozen}
          onClick={() => setRestarting(true)}
        >
          <RotateCcw size={16} />
        </SaveControlButton>
      )}
      <SaveControlButton
        iconOnly={iconOnly}
        label={
          approved
            ? frozen
              ? "取消保存"
              : "撤回保存"
            : frozen
              ? "同意保存"
              : "保存并退出"
        }
        className="subtle"
        disabled={busy}
        data-save-game={approved ? undefined : ""}
        data-cancel-save={approved ? "" : undefined}
        onClick={() => command(approved ? "save_cancel" : "save")}
      >
        {approved ? <Undo2 size={16} /> : <Save size={16} />}
      </SaveControlButton>
      {frozen && !approved && (
        <SaveControlButton
          iconOnly={iconOnly}
          label="取消保存"
          className="subtle"
          data-decline-save
          disabled={busy}
          onClick={() => command("save_cancel")}
        >
          <Undo2 size={16} />
        </SaveControlButton>
      )}
      {votes.length > 0 && (
        <span role="status">
          {votes.length}/{humans} 人同意保存
        </span>
      )}
      {loadingCheckpoint && room.checkpointSavedAt && (
        <GameFeedback>
          <Modal
            title="读取上次存档"
            onClose={() => setLoadingCheckpoint(false)}
            initialFocus="[data-cancel-checkpoint]"
          >
            <p>
              回到 {new Date(room.checkpointSavedAt).toLocaleString("zh-CN")}{" "}
              的存档，放弃之后未保存的进度。
            </p>
            <div className="modal-actions">
              <button
                className="outline"
                data-cancel-checkpoint
                onClick={() => setLoadingCheckpoint(false)}
              >
                取消
              </button>
              <button
                className="primary"
                data-confirm-checkpoint
                disabled={busy}
                onClick={() => {
                  setLoadingCheckpoint(false);
                  command("load_checkpoint");
                }}
              >
                读取存档
              </button>
            </div>
          </Modal>
        </GameFeedback>
      )}
      {restarting && room.game?.restartable && (
        <GameFeedback>
          <Modal
            title="重新开始旅程"
            onClose={() => setRestarting(false)}
            initialFocus="[data-cancel-restart]"
          >
            <p>当前旅程和上次手动存档将被替换。</p>
            <div className="modal-actions">
              <button
                className="outline"
                data-cancel-restart
                onClick={() => setRestarting(false)}
              >
                取消
              </button>
              <button
                className="primary"
                data-confirm-restart
                disabled={busy}
                onClick={() => {
                  setRestarting(false);
                  command("restart");
                }}
              >
                重新开始
              </button>
            </div>
          </Modal>
        </GameFeedback>
      )}
    </span>
  );
}

export function SavedGames({
  saves,
  busy,
  onResume,
}: {
  saves: SavedGame[];
  busy: boolean;
  onResume: (id: string) => void;
}) {
  if (!saves.length) return null;
  return (
    <section className="saved-games" aria-labelledby="saved-games-title">
      <h2 id="saved-games-title">继续游戏</h2>
      <div className="saved-games-list">
        {saves.map((save) => (
          <article className="saved-game-row" key={save.id}>
            <div className="saved-game-details">
              <strong>{save.name}</strong>
              <span>
                {gameName(save.kind)} ·{" "}
                {save.players.map((player) => player.name).join("、")}
              </span>
              <time dateTime={new Date(save.savedAt).toISOString()}>
                {new Date(save.savedAt).toLocaleString("zh-CN", {
                  month: "numeric",
                  day: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </time>
            </div>
            <button
              className="outline"
              data-resume-save={save.id}
              disabled={busy}
              onClick={() => onResume(save.id)}
            >
              {save.status === "restoring" ? "确认继续" : "继续"}
            </button>
          </article>
        ))}
      </div>
    </section>
  );
}

export function RestoringSavedGame({ room }: { room: Room }) {
  const joined = room.resumeJoined ?? [];
  return (
    <section
      className="saved-games saved-game-restoring"
      data-saved-restoring
      aria-labelledby="saved-restoring-title"
    >
      <h2 id="saved-restoring-title">等待原队友</h2>
      <ul>
        {room.seats
          .filter((seat) => !seat.bot && !seat.left)
          .map((seat) => (
            <li key={seat.id}>
              <strong>{seat.name}</strong>
              <span>
                {joined.includes(seat.id) ? (
                  <>
                    <Check size={16} /> 已确认
                  </>
                ) : (
                  "尚未确认"
                )}
              </span>
            </li>
          ))}
      </ul>
    </section>
  );
}
