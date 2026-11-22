import { useEffect, useRef, useState, type ReactNode } from "react";
import type { Room } from "../types";
import { gameModule } from "../game-api/registry";

export function RoomSettings({
  room,
  host,
  busy,
  command,
  options,
  optionsDirty = false,
  onCapacityChange,
  playerCounts,
  children,
}: {
  room: Room;
  host: boolean;
  busy: boolean;
  command: (type: string, extra?: Record<string, unknown>) => void;
  options?: unknown;
  optionsDirty?: boolean;
  onCapacityChange?: (capacity: number) => void;
  playerCounts?: readonly number[];
  children?: ReactNode;
}) {
  const game = gameModule(room.kind)!;
  const [capacity, setCapacity] = useState(room.capacity);
  const [password, setPassword] = useState("");
  const [passwordDirty, setPasswordDirty] = useState(false);
  const submittedVersion = useRef<number | undefined>(undefined);
  useEffect(() => {
    setCapacity(room.capacity);
    onCapacityChange?.(room.capacity);
  }, [room.id, room.capacity, onCapacityChange]);
  useEffect(() => {
    setPassword("");
    setPasswordDirty(false);
  }, [room.id, room.locked]);
  useEffect(() => {
    if (busy || submittedVersion.current === undefined) return;
    if (room.version > submittedVersion.current) {
      setPassword("");
      setPasswordDirty(false);
    }
    submittedVersion.current = undefined;
  }, [busy, room.version]);
  const dirty = capacity !== room.capacity || passwordDirty || optionsDirty;
  const counts =
    playerCounts ??
    game.playerCounts ??
    Array.from(
      { length: game.maxPlayers - game.minPlayers + 1 },
      (value, index) => game.minPlayers + index,
    );
  return (
    <form
      className="room-settings"
      onSubmit={(event) => {
        event.preventDefault();
        if (!host || busy || !dirty) return;
        submittedVersion.current = room.version;
        command("room_settings", {
          capacity,
          ...(options === undefined ? {} : { options }),
          ...(passwordDirty ? { roomPassword: password } : {}),
        });
      }}
    >
      <div className="form-grid">
        <label>
          房间人数
          <select
            aria-label="房间人数"
            value={capacity}
            disabled={!host || busy || counts.length === 1}
            onChange={(event) => {
              const value = Number(event.target.value);
              setCapacity(value);
              onCapacityChange?.(value);
            }}
          >
            {counts.map((count) => (
              <option
                key={count}
                value={count}
                disabled={count < room.seats.length}
              >
                {count} 人
              </option>
            ))}
          </select>
        </label>
        {host && (
          <label>
            房间密码
            <input
              aria-label="房间密码"
              type="password"
              autoComplete="new-password"
              maxLength={72}
              disabled={busy}
              value={password}
              placeholder={room.locked ? "输入新密码" : "不设密码"}
              onChange={(event) => {
                setPassword(event.target.value);
                setPasswordDirty(true);
              }}
            />
          </label>
        )}
      </div>
      {host && room.locked && (
        <button
          type="button"
          className="subtle"
          data-room-password-remove
          disabled={busy}
          onClick={() => {
            setPassword("");
            setPasswordDirty(true);
          }}
        >
          移除密码
        </button>
      )}
      {children}
      {host && (
        <button
          className="outline"
          data-room-settings-save
          disabled={busy || !dirty}
          type="submit"
        >
          保存设置
        </button>
      )}
    </form>
  );
}
